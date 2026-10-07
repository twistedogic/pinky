package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/twistedogic/pinky/internal/history"
	"github.com/twistedogic/pinky/internal/inject"
	pinkylsp "github.com/twistedogic/pinky/internal/lsp"
	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
	"github.com/twistedogic/pinky/internal/todo"
	"github.com/twistedogic/pinky/internal/workspace"
)

const (
	composeHeight = 4
	statusHeight  = 1
)

type state int

const (
	statePicking state = iota
	stateNav
	stateCompose
	stateCommentComposer
	stateFileNav
	stateFileView
	stateTodoList
	stateTodoEdit
	stateLSPPicker
	stateError
)

type sessionMsg struct {
	entries []session.Message
	err     error
}

// commentAnchor captures the target for a comment being composed.
// Message-kind anchors use byteA, byteC over m.latest.Text. File-kind
// anchors use filePath, lineStart, lineEnd with optional byteA/byteC
// inline range (0 means line-range only).
type commentAnchor struct {
	kind      render.CommentKind
	byteA     int
	byteC     int
	filePath  string
	lineStart int
	lineEnd   int
}

// tab identifies the active top-level view.
type tab int

const (
	tabMessage tab = iota
	tabFiles
	tabTodos
)

// fileSelection is the file viewer's visual selection range. The
// "moving end" of the selection is the file viewer's cursor
// (`cursor`, `charPos`); this struct stores only the anchor at the
// other end plus the mode flag. All values are 1-based; LineA is
// the anchor line, CharA is the byte offset into that line.
type fileSelection struct {
	Active bool
	LineA  int
	CharA  int
}

// lspPickerState holds the locations being shown by the LSP
// location picker (stateLSPPicker). Populated by the
// LocationsMsg handler; consumed by handleLSPPickerKey.
//
// WorkDir is the manager's rootURI path; used to relativise
// location URIs for the rendered "<relpath>:<line>:<col>" rows.
// ponytail: kept on the state so the picker doesn't need to
// capture the manager at construction time.
type lspPickerState struct {
	label      string
	locations  []pinkylsp.Location
	cursor     int
	workDir    string
}

// hoverModalState is the file-viewer hover pop-up. Visible flips
// on when a HoverMsg resolves with content; the viewport holds
// the (possibly multi-line) hover body and exposes j/k / PgUp /
// PgDn / Ctrl-D / Ctrl-U for internal scrolling.
type hoverModalState struct {
	visible  bool
	viewport viewport.Model
}

// lspManager is the slice of *pinkylsp.Manager the model layer
// uses. Declared as an interface so unit tests can substitute a
// fake without spawning gopls (and without exercising the real
// ServerStatus dispatch on every render).
type lspManager interface {
	DidOpen(ctx context.Context, path, content string)
	DidClose(ctx context.Context, path string)
	ServerStatus(path string) pinkylsp.ServerStatus
}

// lspBridge is the slice of *pinkylsp.Bridge the model layer
// uses. Tests substitute their own implementation.
type lspBridge interface {
	RequestDefinition(ctx context.Context, path string, line, char int) tea.Cmd
	RequestReferences(ctx context.Context, path string, line, char int) tea.Cmd
	RequestHover(ctx context.Context, path string, line, char int) tea.Cmd
}

// fileViewer holds the state for stateFileView: the raw content,
// the line index (parallel to lines, drives gutter flags), the
// cursor (line + char) with the preferred-column tracker for
// j/k, an optional visual selection whose anchor lives in `visual`
// and whose moving end is the cursor, and the viewport that
// scrolls the rendered content.
type fileViewer struct {
	path      string
	content   string
	lines     []string // raw lines, no trailing newline
	cursor    int      // 1-based line index; LSP target; visual moving end
	charPos   int      // ponytail: byte offset into cursor line (rune-aligned)
	preferred int      // last intended column, survives j/k
	visual    fileSelection
	lineIndex []bool // parallel to lines; true if a file-kind comment covers the line
	viewport  viewport.Model    // ponytail: scrollable view of the rendered file
}

type model struct {
	state state

	// Picking state
	agents     []session.AgentSession
	pickCursor int

	// Attached state
	pane string
	src  session.Source
	hist *history.History

	// Latest-message view state. pinky always renders the most recent
	// assistant message; older messages are not displayed.
	latest session.Message
	lines []string // "\n"-split source of m.latest.Text, 0-based
	lineStartOffsets []int // lines[i]'s first byte in m.latest.Text
	wrappedToSrc []int     // viewport yOffset → source-line index, for nav
	sourceToFirst []int    // source-line index → first wrapped yOffset, for nav

	viewport viewport.Model
	textarea textarea.Model

	width, height int

	streaming bool

	// Comments slice (message-kind + file-kind). Lives on the model;
	// cleared when the latest message text changes or attach() runs.
	comments []render.Comment

	// cursor is the single nav pointer. LineIdx is 1-based into
	// m.lines; CharPos is a byte offset into lines[LineIdx];
	// Preferred is the rightmost column reached by `l`, survives
	// `j`/`k` (matches the file viewer's preferred-column tracker).
	cursor render.NavCursor
	// selection tracks the inline visual selection range. Valid only
	// when nav.Visual == render.NavLine. ByteA is the anchor byte
	// (set by `v`); ByteC is the cursor byte (updated by motion).
	selection render.NavSelection
	// nav is the nav state machine state.
	nav render.NavState

	// includeComments toggles whether the next redirect will append
	// the comments appendix. Toggled by Ctrl+I in compose mode.
	includeComments bool

	// Tab state. m.tab records which top-level view is active.
	// Each tab saves the sub-state of the OTHER tabs so Tab
	// round-trips restore the view the user left in each.
	tab            tab
	messageReturn  state
	fileReturn     state
	todoReturn     state

	// File review tab state. fileEntries is the flat workspace
	// tree produced by workspace.Walk; fileCollapsed hides every
	// descendant of the named directory; fileCursor indexes into
	// the visible (non-collapsed) entries.
	fileEntries   []workspace.Entry
	fileCollapsed map[string]bool
	fileCursor    int
	fileRoot      string

	// Todo tab state. m.todos is the in-memory list (persisted to
	// ~/.local/share/pinky/<base(m.fileRoot)>/todos.json).
	// m.todoCursor indexes into m.todos. m.todoEdit is the buffer
	// for stateTodoEdit.
	todos      []todo.Item
	todoCursor int
	todoEdit   string
	todoPath   string

	// headerHeight is the number of terminal rows the top header
	// occupies (1 or 2). Recomputed by reflow() so other layout
	// code can subtract it from the row budget without re-rendering.
	headerHeight int

	// fileSearchActive + fileSearch implement `/` fuzzy filename
	// search inside stateFileNav. While active, collapse state is
	// ignored and the visible list is filtered by render.FindHits
	// against each entry's path.
	fileSearchActive bool
	fileSearch       []rune

	// navSearch is the search state for stateNav (the message
	// view). active = prompt is open. query = the current query
	// while the prompt is open. hits = the matches against
	// m.lines, recomputed on every render when active || query
	// is non-empty. cur = index of the "current" hit in hits
	// (the one under the cursor), or -1 if none. See
	// design.md for the lifecycle.
	navSearch searchState
	// fileSearchState is the search state for stateFileView
	// (the file content). Same shape as navSearch; lives
	// separately so a search in one view doesn't leak into the
	// other. (The `fileSearch` / `fileSearchActive` fields above
	// are the file navigator's filename filter; do not
	// confuse.)
	fileSearchState searchState

	// fileViewer is the state for stateFileView.
	fileViewer fileViewer

	// commentTa is the textarea used by stateCommentComposer; kept
	// separate from m.textarea (the redirect composer) so state
	// doesn't leak between modes.
	commentTa textarea.Model

	// commentAnchor captures the (byteA, byteC) for the next
	// comment. CharStart/End are derived when saving (CharStart ==
	// CharEnd == -1 means block-level).
	commentAnchor commentAnchor

	// LSP integration: a single Manager + Bridge is shared across
	// the session. nil-safe — tests that don't care about LSP
	// leave lsp unset. The fields are interface-typed so unit
	// tests can substitute a fake without spinning up powernap.
	lsp     lspManager
	lsphub  lspBridge

	// lspPicker is the state for stateLSPPicker (definition or
	// references). Populated by the LocationsMsg handler.
	lspPicker lspPickerState

	// hoverModal is the pop-up that replaced hoverFooter as the
	// primary hover surface. Visible flips on when a HoverMsg
	// resolves; the viewport is sized to fit the content (capped)
	// and supports j/k/PgUp/PgDn/Ctrl-D/Ctrl-U for scrolling.
	// Any other key dismisses.
	hoverModal hoverModalState

	// missingServerHint is the install hint shown when an LSP server
	// binary is not on PATH. Set from LocationsMsg.ServerMissing /
	// HoverMsg.ServerMissing; cleared by the same handlers on
	// the next key press.
	missingServerHint string

	// Error state: set when initialization or attach fails. The TUI
	// shows the message and exits on any key press.
	err error

	// help renders a one-line (short) or multi-column (full) footer
	// at the bottom of every view. ShowAll toggles between them on `?`.
	// Ponytail: a single line of keybindings at the bottom is enough
	// most of the time; the expanded view is opt-in via `?`.
	help help.Model
}

// cursorOnScreen returns true when the cursor's rendered line is
// within the viewport's visible range.
func (m *model) cursorOnScreen() bool {
	if len(m.lines) == 0 {
		return true
	}
	srcLine := m.cursor.LineIdx
	wrapTop := m.viewport.YOffset()
	wrapBot := wrapTop + m.viewport.Height()
	srcTop := m.wrappedYOffsetToSource(wrapTop)
	srcBot := m.wrappedYOffsetToSource(wrapBot - 1)
	return srcLine >= srcTop && srcLine <= srcBot
}

// scrollCursorIntoView adjusts the viewport's YOffset so the cursor
// sits one line inside the visible range. Called from handleNavKey
// after every motion. Per design D9 the viewport follows the cursor
// with a 1-line cushion (not vim's scrolloff=5; pinky's viewport is
// ~16 rows tall and a 5-line cushion would feel jumpy).
func (m *model) scrollCursorIntoView() {
	if len(m.lines) == 0 {
		return
	}
	if m.cursorOnScreen() {
		return
	}
	srcLine := m.cursor.LineIdx
	// Translate the target source line into wrapped-YOffset space
	// so the viewport actually lands on that markdown line.
	target := m.sourceYOffset(srcLine)
	off := max(target-m.viewport.Height()+1, 0)
	m.viewport.SetYOffset(off)
}

// newComposeTextareas builds the two textareas the model carries:
// a redirect composer (full-height) and a comment composer
// (single-line). Used by both newModel (state at startup) and
// idle (state at attach). ponytail: extracts a duplicated 9-line
// construction that drifted independently.
func newComposeTextareas() (textarea.Model, textarea.Model) {
	ta := textarea.New()
	ta.Placeholder = "redirect — Enter newline, Ctrl+S send, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)
	cta := textarea.New()
	cta.Placeholder = "comment — Esc cancel"
	cta.ShowLineNumbers = false
	cta.CharLimit = 0
	cta.SetHeight(1)
	return ta, cta
}

// newModel returns a picker model. Callers must either call
// setAgents (then run the picker), attach (skip the picker), or
// standalone() (no tmux / agent: file navigator only).
func newModel() model {
	vp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(20))
	ta, cta := newComposeTextareas()
	return model{
		state:     statePicking,
		viewport:  vp,
		textarea:  ta,
		commentTa: cta,
		help:      help.New(),
	}
}

func (m *model) setAgents(agents []session.AgentSession) {
	m.agents = agents
	m.pickCursor = 0
}

// viewportSize returns (w, h) for the viewport at idle state. If the
// window size hasn't been reported yet (m.width/m.height are 0), fall
// back to safe defaults; reflow() will resize once WindowSizeMsg
// fires.
func (m *model) viewportSize() (int, int) {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height - statusHeight
	if h < 1 {
		h = 20
	}
	return w, h
}

// idle binds source, history, and view state for the running pane.
// Called by attach() after session.Open + history.Open resolve.
func (m *model) idle(src session.Source, hist *history.History, pane string) {
	ta, cta := newComposeTextareas()
	w, h := m.viewportSize()

	m.pane = pane
	m.src = src
	m.hist = hist
	m.viewport = viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	m.textarea = ta
	m.commentTa = cta
	m.state = stateNav
	m.refreshViewport()
	m.comments = nil

	// Best-effort: record the pane's cwd for the file review tab.
	// Empty on failure (no tmux server).
	if pane != "" {
		if cwd, err := session.PaneCwd(pane); err == nil {
			m.setupForCwd(cwd)
		}
	}
}

// setupForCwd wires every per-workspace field (fileRoot, LSP
// manager + bridge, todo storage path) and refreshes the todo
// list. Called by idle() when the tmux pane has a cwd, and by
// standalone() with os.Getwd(). Extracted so the two paths
// don't drift.
func (m *model) setupForCwd(cwd string) {
	m.fileRoot = cwd
	// ponytail: one LSP Manager per pane session, owned by the
	// model. Lazy-spawns servers on first file-viewer query; no
	// goroutines fire until then.
	mgr := pinkylsp.New(cwd)
	m.lsp = mgr
	m.lsphub = pinkylsp.NewBridge(mgr)
	// Wire todo storage: ~/.local/share/pinky/<base(cwd)>/todos.json.
	// No XDG fallback per the add-todo-tab change.
	m.todoPath = todoStoragePath(cwd)
	m.refreshTodos()
}

// standalone drops pinky into the cwd file navigator when there is
// no tmux server or no pane running an agent. No session source is
// bound: polling stays off (src == nil) and the message tab shows
// its "waiting for agent…" placeholder. File review, todos, and LSP
// work as usual; restart pinky inside tmux to attach an agent.
func (m *model) standalone() {
	cwd, err := os.Getwd()
	if err != nil {
		m.err = fmt.Errorf("locate cwd: %w", err)
		m.state = stateError
		return
	}
	m.setupForCwd(cwd)
	m.tab = tabFiles
	m.enterFileNav()
}

// attach opens a session source + history for the given pane, then
// hands off to idle(). Seeds the viewport via refreshViewport so the
// placeholder renders immediately.
func (m *model) attach(pane string) error {
	src, err := session.Open(pane)
	if err != nil {
		return err
	}
	hist, err := history.Open(pane)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	m.idle(src, hist, pane)
	return nil
}

// refreshTodos loads m.todos from m.todoPath. A missing file is
// treated as an empty list (no error surfaced to the user). Called
// at attach time and after every save.
func (m *model) refreshTodos() {
	if m.todoPath == "" {
		m.todos = nil
		return
	}
	items, err := todo.Load(m.todoPath)
	if err != nil {
		if errors.Is(err, todo.ErrNotFound) {
			m.todos = nil
			return
		}
		// Surface other errors via the placeholder; don't crash.
		m.todos = nil
		return
	}
	m.todos = items
}

// persistTodos writes m.todos to m.todoPath atomically. Called on
// every mutation (add, toggle, edit, delete).
func (m *model) persistTodos() {
	if m.todoPath == "" {
		return
	}
	if err := todo.Save(m.todoPath, m.todos); err != nil {
		_ = err
	}
}

// todoStoragePath computes the on-disk path for the given pane cwd.
// Hard-coded to ~/.local/share/pinky; no XDG fallback per the
// add-todo-tab change. Two cwds with the same last segment
// intentionally share a file.
func todoStoragePath(cwd string) string {
	return filepath.Join(todoHomeDir(), filepath.Base(cwd), "todos.json")
}

// todoHomeDir is the root under which per-workspace todo files
// live. Default is ~/.local/share/pinky; tests override HOME.
func todoHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "pinky")
	}
	return filepath.Join(home, ".local", "share", "pinky")
}

func (m *model) selectAgent(idx int) error {
	if idx < 0 || idx >= len(m.agents) {
		return fmt.Errorf("invalid selection")
	}
	return m.attach(m.agents[idx].PaneID)
}

func (m model) Init() tea.Cmd {
	if m.state == stateNav && m.src != nil {
		return pollCmd(m.src)
	}
	return nil
}

func pollCmd(src session.Source) tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		msgs, err := src.NewMessages()
		if err != nil {
			return sessionMsg{err: err}
		}
		return sessionMsg{entries: msgs}
	})
}

// recoverPanic logs a model-state snapshot to make postmortems easier,
// then re-panics. Used at every Update entry point that touches the
// file viewer (the most panic-prone surface: cursor math, line
// slicing, visual-mode rendering).
func (m *model) recoverPanic(where string, extra ...any) {
	r := recover()
	if r == nil {
		return
	}
	fmt.Fprintf(os.Stderr,
		"pinky: panic in %s (state=%d, file=%q, cursor=%d/%d): %v\n%v\n%s\n",
		where, m.state, m.fileViewer.path, m.fileViewer.cursor, m.fileViewer.charPos,
		r, extra, debug.Stack())
	panic(r)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.recoverPanic("Update")
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		m.reflow()
		m.refreshViewport()
		// reflow already re-anchors the file cursor; refresh the
		// file viewport's cached content so it matches the new size.
		if m.state == stateFileView && m.fileViewer.path != "" {
			m.refreshFileView()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case sessionMsg:
		// ponytail: polling stops deliberately after the first
		// message; `r` is the manual gate, see proposal.md.
		if msg.err != nil {
			return m, nil
		}
		if len(msg.entries) == 0 {
			m.streaming = false
			return m, nil
		}
		// Find the latest assistant message in this poll. A user redirect
		// arriving after the agent's last reply should NOT replace the
		// view — we want the agent's most recent text, not the user's own.
		var last *session.Message
		for i := range msg.entries {
			if msg.entries[i].Role == session.RoleAssistant {
				last = &msg.entries[i]
			}
		}
		if last != nil {
			if last.Text != m.latest.Text {
				m.comments = nil
				// A new message wipes the search state — the
				// previous query and hits are no longer
				// meaningful, and the dim highlights would
				// otherwise paint over text that isn't there.
				m.navSearch.reset()
			}
			m.latest = *last
			if m.hist != nil {
				_ = m.hist.Append(string(last.Role), last.Text)
			}
			m.streaming = true
			m.refreshViewport()
			// ponytail: Bubble Tea's SetContent preserves YOffset
			// across calls. Pin to top on every commit so the
			// first sentence of the latest message is always
			// visible — the viewport's own scroll state is the
			// single source of truth, no custom buffer needed.
			m.viewport.GotoTop()
		}
		return m, nil
	}

	// ponytail: LSP replies only carry meaning while the file
	// viewer is the active view. Stale replies that arrive after
	// the user has navigated away are dropped silently — the
	// bridge's id-check already filters superseded ones.
	if msg, ok := msg.(pinkylsp.LocationsMsg); ok {
		return m.handleLocationsMsg(msg)
	}
	if msg, ok := msg.(pinkylsp.HoverMsg); ok {
		return m.handleHoverMsg(msg)
	}

	var cmds []tea.Cmd
	switch m.state {
	case stateCompose:
		var taCmd tea.Cmd
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)
	case stateNav:
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}
	return m, tea.Batch(cmds...)
}

// handleLocationsMsg routes a definition / references reply into
// the model: 0 results = silent, 1 = jump (definition only) /
// picker (references), N>1 = picker. ServerMissing surfaces the
// install hint footer.
func (m model) handleLocationsMsg(msg pinkylsp.LocationsMsg) (tea.Model, tea.Cmd) {
	if msg.ServerMissing && msg.InstallHint != "" {
		m.missingServerHint = msg.InstallHint
		m.refreshFileView()
		return m, nil
	}
	if msg.Err != nil {
		return m, nil
	}
	// Definition-on-one-result jumps; references-on-one-result
	// still goes through the picker so the user sees the
	// single row (matches D5's "always picker" rule).
	if len(msg.Locations) == 1 && msg.Kind == pinkylsp.KindDefinition {
		m.jumpToLocation(msg.Locations[0])
		return m, nil
	}
	if len(msg.Locations) == 0 {
		return m, nil
	}
	m.lspPicker = lspPickerState{
		label:     lspPickerLabels[msg.Kind],
		locations: msg.Locations,
		cursor:    0,
		workDir:   msg.WorkDir,
	}
	m.state = stateLSPPicker
	m.reflow()
	return m, nil
}

// lspPickerLabels maps an LSP picker Kind to the header label.
// Indexed by (Kind - 1) since KindHover (2) never reaches the
// picker (hover routes to HoverMsg). Definition=0 → index 0;
// References=1 → index 1.
var lspPickerLabels = [...]string{"definition", "references"}

// handleHoverMsg opens the hover modal with the resolved content.
// Empty / errored / server-missing replies stay silent (missing-
// server surfaces through the install-hint footer instead, so the
// user can still see it after dismissing the modal). The previous
// one-line footer slot is gone; the modal is the hover's primary
// surface now.
func (m model) handleHoverMsg(msg pinkylsp.HoverMsg) (tea.Model, tea.Cmd) {
	if msg.ServerMissing && msg.InstallHint != "" {
		m.missingServerHint = msg.InstallHint
		m.refreshFileView()
		return m, nil
	}
	if msg.Err != nil || msg.Contents == "" {
		return m, nil
	}
	m.showHoverModal(msg.Contents)
	m.refreshFileView()
	return m, nil
}

// showHoverModal sizes and populates the modal's viewport for one
// hover reply. Content is rendered as markdown (glamour) with the
// custom stylesheet in internal/render so headers/code blocks get
// accent colors that fit pinky's palette. Width is capped so the box
// doesn't stretch the full terminal on wide screens; height is
// content-driven up to ~15 rows so very long go-doc hovers stay
// scrollable. A custom KeyMap restricts the viewport to scroll keys
// only — h, l, d, b, f, space would otherwise conflict with the
// file viewer's own bindings.
func (m *model) showHoverModal(content string) {
	width := m.width - 4
	if width > 60 {
		width = 60
	}
	if width < 20 {
		width = 20
	}
	bodyW := width - 2 // border
	rendered := render.RenderMarkdown(content, bodyW)
	renderedLines := strings.Count(rendered, "\n") + 1
	height := renderedLines + 2 // +2 for the box's own padding/border
	maxHeight := m.height - 8
	if maxHeight < 5 {
		maxHeight = 5
	}
	if maxHeight > 15 {
		maxHeight = 15
	}
	if height > maxHeight {
		height = maxHeight
	}
	if height < 3 {
		height = 3
	}
	bodyH := height - 2
	if !m.hoverModal.visible {
		m.hoverModal.viewport = viewport.New(
			viewport.WithWidth(bodyW),
			viewport.WithHeight(bodyH),
		)
		m.hoverModal.viewport.KeyMap = hoverModalKeyMap
	}
	m.hoverModal.viewport.SetWidth(bodyW)
	m.hoverModal.viewport.SetHeight(bodyH)
	// ponytail: drop hoverModalBodyStyle here — glamour's own
    // stylesheet paints the body. The modal box's cyan border is
    // the only chrome; the inside reads as accent-on-default.
	m.hoverModal.viewport.SetContent(rendered)
	m.hoverModal.viewport.GotoTop()
	m.hoverModal.visible = true
}

// hideHoverModal flips the modal off without touching its
// viewport state — next showHoverModal resizes and resets.
func (m *model) hideHoverModal() {
	if m.hoverModal.visible {
		m.hoverModal.visible = false
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// `?` toggles between the short help footer and the expanded
	// (multi-column) help.
	if key.Matches(msg, defaultKeyMap.Help) {
		m.help.ShowAll = !m.help.ShowAll
		m.reflow()
		m.refreshViewport()
		return m, nil
	}

	// Tab toggles the message / file-review tab. Active in every
	// state except stateCommentComposer (let the textarea eat it),
	// statePicking (no agent yet), and stateError.
	if key.Matches(msg, defaultKeyMap.Tab) {
		switch m.state {
		case stateCommentComposer, statePicking, stateError:
			// swallowed
		default:
			return m, m.toggleTab()
		}
		return m, nil
	}

	switch m.state {
	case statePicking:
		return m.handlePickerKey(msg)
	case stateNav:
		return m.handleNavKey(msg)
	case stateCompose:
		return m.handleComposeKey(msg)
	case stateCommentComposer:
		return m.handleCommentComposerKey(msg)
	case stateFileNav:
		return m.handleFileNavKey(msg)
	case stateFileView:
		return m.handleFileViewKey(msg)
	case stateTodoList:
		return m.handleTodoListKey(msg)
	case stateTodoEdit:
		return m.handleTodoEditKey(msg)
	case stateLSPPicker:
		return m.handleLSPPickerKey(msg)
	case stateError:
		// Any key dismisses the error and quits.
		if key.Matches(msg, defaultKeyMap.QuitError) {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

// toggleTab advances one slot in the M → F → T → M cycle, saving
// the current sub-state to the slot for the tab we're leaving and
// restoring the saved sub-state for the tab we're entering.
func (m *model) toggleTab() tea.Cmd {
	switch m.tab {
	case tabMessage:
		// Save message sub-state; advance to files.
		m.messageReturn = m.clampMessageState(m.state)
		m.tab = tabFiles
		m.state = m.clampFileState(m.fileReturn)
		if m.state == stateFileNav && len(m.fileEntries) == 0 {
			m.enterFileNav()
		}
		m.reflow()
	case tabFiles:
		// Save file sub-state; advance to todos.
		m.fileReturn = m.clampFileState(m.state)
		m.tab = tabTodos
		m.state = m.clampTodoState(m.todoReturn)
		m.reflow()
	case tabTodos:
		// Save todo sub-state; cycle back to message.
		m.todoReturn = m.clampTodoState(m.state)
		m.tab = tabMessage
		m.state = m.clampMessageState(m.messageReturn)
		m.fileViewer.visual.Active = false
		m.reflow()
		m.refreshViewport()
	}
	return nil
}

// clampMessageState / clampFileState / clampTodoState coerce a
// possibly-stale return-state value into a valid sub-state for
// the destination tab. Anything outside the tab's allowed states
// (or zero) becomes the tab's default entry state.
func (m *model) clampMessageState(s state) state {
	if s != stateNav && s != stateCompose {
		return stateNav
	}
	return s
}

func (m *model) clampFileState(s state) state {
	if s != stateFileNav && s != stateFileView {
		return stateFileNav
	}
	return s
}

func (m *model) clampTodoState(s state) state {
	if s != stateTodoList && s != stateTodoEdit {
		return stateTodoList
	}
	return s
}

// escapeToMessage jumps back to the message tab from any other
// tab, saving the current tab's sub-state so a Tab round-trip
// restores it. Bound to `Esc` in the file and todo tabs (Tab
// cycles forward; Esc shortcuts directly to message).
func (m *model) escapeToMessage() tea.Cmd {
	switch m.tab {
	case tabMessage:
		return nil
	case tabFiles:
		m.fileReturn = m.clampFileState(m.state)
	case tabTodos:
		m.todoReturn = m.clampTodoState(m.state)
	}
	m.tab = tabMessage
	m.state = m.clampMessageState(m.messageReturn)
	m.fileViewer.visual.Active = false
	m.reflow()
	m.refreshViewport()
	return nil
}

// enterFileNav walks m.fileRoot with workspace.Walk and transitions
// to stateFileNav. The flat entry slice + collapse map is the
// single source of truth for the dir navigator; visibleFileEntries
// derives the visible rows on demand.
func (m *model) enterFileNav() {
	if m.fileRoot == "" {
		m.state = stateNav
		m.latest = session.Message{
			Role: session.RoleUser,
			Text: "[no workspace: pane cwd unavailable]",
		}
		m.refreshViewport()
		return
	}
	entries, err := workspace.Walk(m.fileRoot)
	if err != nil {
		m.state = stateNav
		m.latest = session.Message{
			Role: session.RoleUser,
			Text: "[workspace walk failed: " + err.Error() + "]",
		}
		m.refreshViewport()
		return
	}
	m.fileEntries = entries
	m.fileCollapsed = map[string]bool{}
	// ponytail: every dir starts collapsed. The user expands on
	// demand with `l`; the visible list stays short for deep trees.
	for _, e := range entries {
		if e.IsDir {
			m.fileCollapsed[e.Path] = true
		}
	}
	m.fileCursor = 0
	m.fileSearchActive = false
	m.fileSearch = nil
	m.state = stateFileNav
	m.reflow()
}

// visibleFileEntries returns the entries that should appear in
// the dir navigator. When file search is active and the query is
// non-empty, fuzzy matching overrides the collapse map: every
// entry whose path matches is shown in flat tree order. Otherwise
// the collapse map hides collapsed dirs and their descendants.
func (m model) visibleFileEntries() []workspace.Entry {
	if len(m.fileEntries) == 0 {
		return nil
	}
	if m.fileSearchActive && len(m.fileSearch) > 0 {
		query := string(m.fileSearch)
		out := make([]workspace.Entry, 0, len(m.fileEntries))
		for _, e := range m.fileEntries {
			if len(render.FindHits(query, []string{e.Path})) > 0 {
				out = append(out, e)
			}
		}
		return out
	}
	hidden := map[int]bool{}
	for i, e := range m.fileEntries {
		if !e.IsDir {
			continue
		}
		if m.fileCollapsed[e.Path] {
			for j := i + 1; j < len(m.fileEntries); j++ {
				if m.fileEntries[j].Depth <= e.Depth {
					break
				}
				hidden[j] = true
			}
		}
	}
	out := make([]workspace.Entry, 0, len(m.fileEntries)-len(hidden))
	for i, e := range m.fileEntries {
		if !hidden[i] {
			out = append(out, e)
		}
	}
	return out
}

// fuzzyMatch is gone; the file navigator now uses render.FindHits
// directly (see visibleFileEntries).

// searchState is the per-view state for the `/` fuzzy search
// surface. active means the prompt is open and typing appends to
// query. hits is the current match list (nil when there's no
// query or no matches); cur is the index of the match under the
// cursor, or -1 when the cursor isn't on a hit.
//
// Two instances live on the model: navSearch (stateNav) and
// fileSearchState (stateFileView). They share zero state — a
// search in one view doesn't leak into the other.
type searchState struct {
	active bool
	query  []rune
	hits   []render.Hit
	cur    int
}

// reset clears every field. Used by Esc, by new-message /
// new-file transitions, and by exitFileViewer.
func (s *searchState) reset() {
	s.active = false
	s.query = nil
	s.hits = nil
	s.cur = -1
}

// enter opens the search prompt with an empty query. The cursor
// does not move and no highlights are drawn yet (query is empty).
func (m *model) enterNavSearch() {
	m.navSearch.reset()
	m.navSearch.active = true
}

func (m *model) enterFileViewerSearch() {
	m.fileSearchState.reset()
	m.fileSearchState.active = true
}

// commitNavSearch closes the search prompt and jumps the nav
// cursor to the first match (in document order) at or after the
// current cursor position. Recomputes hits, sets cur, scrolls
// the cursor into view. No-op if the query is empty.
func (m *model) commitNavSearch() {
	m.navSearch.active = false
	if len(m.navSearch.query) == 0 {
		m.navSearch.hits = nil
		m.navSearch.cur = -1
		m.refreshViewport()
		return
	}
	m.navSearch.hits = render.FindHits(string(m.navSearch.query), m.lines)
	cur := firstHitAtOrAfter(m.navSearch.hits, m.cursor.LineIdx, m.cursor.CharPos)
	m.navSearch.cur = cur
	if cur >= 0 {
		h := m.navSearch.hits[cur]
		m.cursor.LineIdx = h.LineIdx
		m.cursor.CharPos = h.ByteA
		m.scrollCursorIntoView()
	}
	m.refreshViewport()
}

// commitFileSearch is the file-viewer analogue of commitNavSearch.
// Resets the file viewer's preferred column to the hit's charPos
// (since the cursor is teleported to an arbitrary byte).
func (m *model) commitFileSearch() {
	m.fileSearchState.active = false
	if len(m.fileSearchState.query) == 0 {
		m.fileSearchState.hits = nil
		m.fileSearchState.cur = -1
		return
	}
	m.fileSearchState.hits = render.FindHits(string(m.fileSearchState.query), m.fileViewer.lines)
	cur := firstHitAtOrAfter(m.fileSearchState.hits, m.fileViewer.cursor-1, m.fileViewer.charPos)
	m.fileSearchState.cur = cur
	if cur >= 0 {
		h := m.fileSearchState.hits[cur]
		m.fileViewer.cursor = h.LineIdx + 1
		m.fileViewer.charPos = h.ByteA
		m.fileViewer.preferred = h.ByteA
	}
}

// firstHitAtOrAfter returns the index of the first hit whose
// (LineIdx, ByteA) is at or after (afterLine, afterByte), or -1
// if no such hit exists. hits is assumed to be sorted by
// (LineIdx, ByteA).
func firstHitAtOrAfter(hits []render.Hit, afterLine, afterByte int) int {
	for i, h := range hits {
		if h.LineIdx > afterLine || (h.LineIdx == afterLine && h.ByteA >= afterByte) {
			return i
		}
	}
	return -1
}

// firstHitAtOrBefore returns the index of the last hit whose
// (LineIdx, ByteA) is at or before (beforeLine, beforeByte), or
// -1 if no such hit exists.
func firstHitAtOrBefore(hits []render.Hit, beforeLine, beforeByte int) int {
	last := -1
	for i, h := range hits {
		if h.LineIdx < beforeLine || (h.LineIdx == beforeLine && h.ByteA <= beforeByte) {
			last = i
		} else {
			break
		}
	}
	return last
}

// cycleSearchHit advances (*cur) by delta (wrapping modulo
// len(hits)) and returns the new index. If hits is empty, it
// is a no-op and returns -1. Callers should treat cur as
// opaque — they pass the address of their field and read back
// the updated value.
func cycleSearchHit(delta int, hits []render.Hit, cur *int) int {
	if len(hits) == 0 {
		*cur = -1
		return -1
	}
	*cur = ((*cur + delta) + len(hits)) % len(hits)
	return *cur
}

// handleNavSearchKey routes keys while the search prompt is open
// in stateNav. Printable runes append to the query; Backspace
// trims; Esc clears; Enter commits (jumps cursor, calls
// commitNavSearch). Arrow keys fall through to the viewport (the
// file navigator's `/` does the same — scroll without closing the
// prompt). Any other key (j/k/h/l/w/b/c/s/v/r/n/q) is consumed
// by the query.
func (m model) handleNavSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.Code {
	case tea.KeyEsc:
		m.navSearch.reset()
		m.refreshViewport()
		return m, nil
	case tea.KeyEnter:
		m.commitNavSearch()
		m.refreshViewport()
		return m, nil
	case tea.KeyBackspace:
		if len(m.navSearch.query) > 0 {
			m.navSearch.query = m.navSearch.query[:len(m.navSearch.query)-1]
			m.refreshViewport()
		}
		return m, nil
	}
	if r, ok := singleRune(msg); ok && r != 0 {
		// Skip '/' itself so the user can't re-open the prompt
		// mid-query; the active flag is the only state that
		// matters.
		if r == '/' {
			return m, nil
		}
		m.navSearch.query = append(m.navSearch.query, r)
		m.refreshViewport()
		return m, nil
	}
	// Arrow / PageUp / PageDown / Home / End — forward to viewport.
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

// handleFileSearchKey is the file-viewer analogue of
// handleNavSearchKey. Same shape; closes the prompt on Esc,
// commits on Enter.
func (m model) handleFileSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.Code {
	case tea.KeyEsc:
		m.fileSearchState.reset()
		m.refreshFileView()
		return m, nil
	case tea.KeyEnter:
		m.commitFileSearch()
		m.refreshFileView()
		return m, nil
	case tea.KeyBackspace:
		if len(m.fileSearchState.query) > 0 {
			m.fileSearchState.query = m.fileSearchState.query[:len(m.fileSearchState.query)-1]
			m.refreshFileView()
		}
		return m, nil
	}
	if r, ok := singleRune(msg); ok && r != 0 {
		if r == '/' {
			return m, nil
		}
		m.fileSearchState.query = append(m.fileSearchState.query, r)
		m.refreshFileView()
		return m, nil
	}
	var vpCmd tea.Cmd
	m.fileViewer.viewport, vpCmd = m.fileViewer.viewport.Update(msg)
	return m, vpCmd
}

// moveFileCursor clamps fileCursor into [0, len(visible)-1] after
// applying delta. Called from j/k handlers in handleFileNavKey.
func (m *model) moveFileCursor(delta int) {
	m.fileCursor = clampCursor(m.fileCursor, len(m.visibleFileEntries()), delta)
}

// clampCursor moves *cur by delta in the inclusive range
// [0, n-1]. On an empty range, sets *cur to 0. Used by the
// file navigator and the todo list, which share the same
// clamp-and-reset semantics (unlike the picker, which wraps).
func clampCursor(cur, n, delta int) int {
	if n == 0 {
		return 0
	}
	return min(max(cur+delta, 0), n-1)
}

// sendToPane is the package-level hook for the actual tmux
// dispatch. Tests swap it to capture submit calls without a live
// tmux. ponytail: global mutable state, scoped to tests via
// t.Cleanup.
var sendToPane = inject.Send

// singleRune returns the rune of msg if it's a single printable
// key press with no modifier keys (the v2 replacement for the v1
// `msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == X`
// pattern).
func singleRune(msg tea.KeyMsg) (rune, bool) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok || kp.Mod != 0 || kp.Code == 0 {
		return 0, false
	}
	if kp.Code < 32 || kp.Code == 127 { // control chars / del are not "single rune"
		return 0, false
	}
	return kp.Code, true
}

// isEsc reports whether msg is a plain Esc press (no modifiers).
func isEsc(msg tea.KeyMsg) bool {
	kp, ok := msg.(tea.KeyPressMsg)
	return ok && kp.Code == tea.KeyEsc && kp.Mod == 0
}

// isKeyRune reports whether msg is a plain key press of the given rune
// (no modifier keys). Replaces v1's
// `msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r`.
func isKeyRune(msg tea.KeyMsg, r rune) bool {
	kp, ok := msg.(tea.KeyPressMsg)
	return ok && kp.Mod == 0 && kp.Code == r
}

// dispatch sends text to the agent pane, recording to history on
// success and surfacing a "[send failed: ...]" placeholder in the
// main view on failure. Returns true on success.
func (m *model) dispatch(text string) bool {
	if m.hist != nil {
		_ = m.hist.Append(string(session.RoleUser), text)
	}
	if err := sendToPane(m.pane, text); err != nil {
		m.latest = session.Message{
			Role: session.RoleUser, Text: "[send failed: " + err.Error() + "]",
		}
		m.refreshViewport()
		m.viewport.GotoBottom()
		return false
	}
	return true
}

// submitAllComments sends every accumulated comment as a single
// redirect through the existing inject pipeline and clears the
// comment slice on success. With no comments to send it is a no-op.
// On inject failure the comments are kept (so the user can retry)
// and the error is surfaced via dispatch.
func (m *model) submitAllComments() {
	if len(m.comments) == 0 {
		return
	}
	if !m.dispatch(render.FormatCommentsAppendix(m.comments)) {
		return
	}
	m.comments = nil
	m.refreshViewport()
}
// enterCompose transitions to compose mode with the textarea reset
// and focused. Reached via `n` (ActionCompose).
func (m *model) enterCompose() {
	m.state = stateCompose
	m.nav = render.NavState{}
	m.textarea.Focus()
	m.textarea.Reset()
	m.reflow()
}

// enterCommentComposer seeds the comment composer with the given
// anchor and switches to stateCommentComposer.
func (m *model) enterCommentComposer(a commentAnchor) {
	m.commentAnchor = a
	m.commentTa.Reset()
	m.commentTa.Focus()
	m.nav = render.NavState{}
	m.state = stateCommentComposer
	m.reflow()
}

// handleCommentComposerKey processes a keypress in stateCommentComposer.
// Enter saves the new comment and returns to nav without sending; the
// accumulated batch flushes only via `s` in stateNav. Esc and q cancel
// (mirror the hover modal: the dialogue is modal, single-char dismiss
// keys consume the keypress so the user can't accidentally exit the
// file viewer or quit pinky while reading/canceling the prompt).
func (m model) handleCommentComposerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var taCmd tea.Cmd
		m.commentTa, taCmd = m.commentTa.Update(msg)
		return m, taCmd
	}
	switch kp.Code {
	case tea.KeyEsc:
		if kp.Mod == 0 {
			m.cancelCommentComposer()
			return m, nil
		}
	case tea.KeyEnter:
		if kp.Mod == 0 {
			m.saveComment()
			return m, nil
		}
	}
	if isKeyRune(msg, 'q') {
		m.cancelCommentComposer()
		return m, nil
	}
	var taCmd tea.Cmd
	m.commentTa, taCmd = m.commentTa.Update(msg)
	return m, taCmd
}

// cancelCommentComposer returns to idle without saving. Resets the
// textarea so the next composer open (via c) is clean — and so the
// buffer doesn't carry "wiped but not cleared" text within the same
// open-state if the user reopens before any other reset path runs.
func (m *model) cancelCommentComposer() {
	m.commentTa.Reset()
	m.commentTa.Blur()
	m.state = stateNav
	m.reflow()
}

// saveComment commits the comment to m.comments and returns to idle.
// Anchor comes from m.commentAnchor (set by enterCommentComposer).
func (m *model) saveComment() {
	text := m.commentTa.Value()
	if text == "" {
		m.cancelCommentComposer()
		return
	}
	a := m.commentAnchor
	switch a.kind {
	case render.CommentFile:
		m.saveFileComment(a, text)
	default:
		m.saveMessageComment(a, text)
	}
	m.commentTa.Blur()
}

// saveMessageComment is the message-kind path. The anchor is a
// global byte range (byteA, byteC) over m.latest.Text; the verbatim
// source slice is stored so the appendix can quote it.
func (m *model) saveMessageComment(a commentAnchor, text string) {
	ba, bc := a.byteA, a.byteC
	if ba < 0 || bc < 0 || ba >= len(m.latest.Text) || bc > len(m.latest.Text) {
		m.cancelCommentComposer()
		return
	}
	if ba > bc {
		ba, bc = bc, ba
	}
	src := m.latest.Text[ba:bc]
	c := render.Comment{
		Kind:      render.CommentMessage,
		ByteA:     ba,
		ByteC:     bc,
		Source:    src,
		Text:      text,
		CreatedAt: time.Now(),
	}
	m.comments = append(m.comments, c)
	m.state = stateNav
	m.reflow()
	m.refreshViewport()
}

// saveFileComment handles file-kind anchors: whole-file line-range
// (byteA/byteC zero) or inline byte-range (byteA > 0) comments. The
// byteA/byteC byte range is line-relative (anchored to lineStart),
// not file-relative — file-level offsets would slice across newlines.
func (m *model) saveFileComment(a commentAnchor, text string) {
	if a.filePath == "" {
		m.cancelCommentComposer()
		return
	}
	src := ""
	cs, ce := -1, -1
	if a.byteA >= 0 {
		cs, ce = a.byteA, a.byteC
		if cs > ce {
			cs, ce = ce, cs
		}
		lineIdx := a.lineStart - 1
		if lineIdx >= 0 && lineIdx < len(m.fileViewer.lines) {
			line := m.fileViewer.lines[lineIdx]
			if cs < 0 {
				cs = 0
			}
			if cs > len(line) {
				cs = len(line)
			}
			if ce > len(line) {
				ce = len(line)
			}
			src = line[cs:ce]
		}
	}
	c := render.Comment{
		Kind:      render.CommentFile,
		Path:      a.filePath,
		LineStart: a.lineStart,
		LineEnd:   a.lineEnd,
		ByteA:     cs,
		ByteC:     ce,
		Source:    src,
		Text:      text,
		CreatedAt: time.Now(),
	}
	m.comments = append(m.comments, c)
	m.refreshFileViewer()
	m.refreshFileView()
	m.state = m.fileReturnAfterComment()
	m.reflow()
}

// fileReturnAfterComment returns the file-review state to return
// to after saving a file-kind comment (file viewer when the anchor
// came from there, dir nav otherwise).
func (m *model) fileReturnAfterComment() state {
	if m.fileViewer.path != "" && m.fileViewer.path == m.commentAnchor.filePath {
		return stateFileView
	}
	return stateFileNav
}

// refreshFileViewer re-runs RenderFile for the current viewer
// to refresh the lineIndex (HasComment flags), after a comment
// was added that targets this file. The rendered body itself is
// cached in m.fileViewer.viewport (see refreshFileView), so
// callers that want the new gutter to show on screen must also
// call refreshFileView.
func (m *model) refreshFileViewer() {
	if m.fileViewer.path == "" {
		return
	}
	m.fileViewer.lineIndex = render.MarkLines(m.fileViewer.content, m.fileCommentsFor(m.fileViewer.path))
}

func (m model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, defaultKeyMap.Up):
		m.moveCursor(-1)
	case key.Matches(msg, defaultKeyMap.Down):
		m.moveCursor(+1)
	case key.Matches(msg, defaultKeyMap.Pick):
		if err := m.selectAgent(m.pickCursor); err != nil {
			m.state = stateError
			m.err = err
			return m, nil
		}
		// attach() already seeded the viewport with the placeholder
		// (or the current latest). Just kick off polling.
		return m, pollCmd(m.src)
	case key.Matches(msg, defaultKeyMap.QuitPick):
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) moveCursor(delta int) {
	n := len(m.agents)
	if n == 0 {
		return
	}
	m.pickCursor = (m.pickCursor + delta + n) % n
}

func (m model) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the search prompt is open, route every key through
	// the search handler (it ignores '/' itself and forwards
	// scroll keys to the viewport).
	if m.navSearch.active {
		return m.handleNavSearchKey(msg)
	}
	// Esc is a special key but is part of the nav surface (visual
	// exit). Route it through the SM so single source of truth.
	if isEsc(msg) {
		// Clear any committed search state. The dim highlights
		// would otherwise linger after the user dismisses the
		// search.
		if len(m.navSearch.hits) > 0 {
			m.navSearch.reset()
			m.refreshViewport()
			return m, nil
		}
		action := render.NavHandle(0x1b, &m.nav, &m.cursor, &m.selection, m.lines, m.lineStartOffsets)
		if action == render.ActionExitVisual {
			m.refreshViewport()
		}
		return m, nil
	}
	// '/' opens the search prompt. Checked before NavHandle so
	// the rune isn't routed to the nav state machine.
	if isKeyRune(msg, '/') {
		m.enterNavSearch()
		m.refreshViewport()
		return m, nil
	}
	// 'n' / 'N' cycle the search hits when a search is active
	// (i.e. a previous search's hits are still on screen). When
	// no search is active, 'n' falls through to NavHandle and
	// enters compose mode (the existing behaviour).
	if isKeyRune(msg, 'n') && len(m.navSearch.hits) > 0 {
		if m.navSearch.cur < 0 {
			// No current hit (e.g. cursor moved with j/k); start
			// at the first hit at or after the cursor.
			m.navSearch.cur = firstHitAtOrAfter(m.navSearch.hits, m.cursor.LineIdx, m.cursor.CharPos)
		} else {
			cycleSearchHit(+1, m.navSearch.hits, &m.navSearch.cur)
		}
		if m.navSearch.cur >= 0 {
			h := m.navSearch.hits[m.navSearch.cur]
			m.cursor.LineIdx = h.LineIdx
			m.cursor.CharPos = h.ByteA
			m.scrollCursorIntoView()
			m.refreshViewport()
		}
		return m, nil
	}
	if isKeyRune(msg, 'N') && len(m.navSearch.hits) > 0 {
		if m.navSearch.cur < 0 {
			m.navSearch.cur = firstHitAtOrBefore(m.navSearch.hits, m.cursor.LineIdx, m.cursor.CharPos)
		} else {
			cycleSearchHit(-1, m.navSearch.hits, &m.navSearch.cur)
		}
		if m.navSearch.cur >= 0 {
			h := m.navSearch.hits[m.navSearch.cur]
			m.cursor.LineIdx = h.LineIdx
			m.cursor.CharPos = h.ByteA
			m.scrollCursorIntoView()
			m.refreshViewport()
		}
		return m, nil
	}
	// Single-rune keys go through the nav state machine. Special
	// keys (arrows, PageUp/Down, Home/End) forward to the viewport.
	if r, ok := singleRune(msg); ok {
		// When a search is active, any of the cursor-motion keys
		// (j/k/h/l/w/b) resets the current-hit index — the next
		// 'n' will continue from the new cursor position, not
		// from the stale hit. Matches vim's "search anchor
		// resets on motion" rule.
		if m.navSearch.cur >= 0 && (r == 'j' || r == 'k' || r == 'h' || r == 'l' || r == 'w' || r == 'b') {
			m.navSearch.cur = -1
		}
		action := render.NavHandle(r, &m.nav, &m.cursor, &m.selection, m.lines, m.lineStartOffsets)
		switch action {
		case render.ActionNone:
			// unrecognised rune — forward to viewport (so keys like '/'
			// for find, etc., could still work in future).
		case render.ActionBlockDown, render.ActionBlockUp,
			render.ActionRuneLeft, render.ActionRuneRight,
			render.ActionWordRight, render.ActionWordLeft:
			m.scrollCursorIntoView()
			m.refreshViewport()
		case render.ActionEnterVisual:
			m.refreshViewport()
		case render.ActionExitVisual:
			m.refreshViewport()
		case render.ActionComment:
			m.enterCommentComposer(buildCommentAnchor(m.nav.Visual, &m.cursor, &m.selection, m.lines, m.lineStartOffsets))
		case render.ActionSend:
			m.handleSend()
		case render.ActionRefresh:
			return m, pollCmd(m.src)
		case render.ActionQuit:
			return m, tea.Quit
		case render.ActionCompose:
			m.enterCompose()
		}
		return m, nil
	}
	// Forward unhandled special keys to viewport (arrows, pgup/dn, etc.)
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

// handleFileNavKey drives the dir navigator: j/k move the cursor
// through visible entries, h collapses (or jumps to parent), l
// expands (or jumps to first child), Enter opens a file or
// toggles a dir, c opens the comment composer for the current
// file, `/` activates fuzzy search. s flushes accumulated
// comments, q/Tab/Esc leave the tab.
func (m model) handleFileNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.fileSearchActive {
		return m.handleFileNavSearchKey(msg)
	}
	visible := m.visibleFileEntries()
	if isEsc(msg) || key.Matches(msg, defaultKeyMap.FileNavBack) {
		return m, m.escapeToMessage()
	}
	switch {
	case isKeyRune(msg, 'q'):
		return m, tea.Quit
	case isKeyRune(msg, 's'):
		m.handleSend()
		return m, nil
	case isKeyRune(msg, 'j'):
		m.moveFileCursor(+1)
		return m, nil
	case isKeyRune(msg, 'k'):
		m.moveFileCursor(-1)
		return m, nil
	case isKeyRune(msg, 'h'):
		m.fileNavCollapseOrParent()
		return m, nil
	case isKeyRune(msg, 'l'):
		m.fileNavExpandOrChild()
		return m, nil
	case isKeyRune(msg, '/'):
		m.enterFileSearch()
		return m, nil
	case key.Matches(msg, defaultKeyMap.FileNavComment) || isKeyRune(msg, 'c'):
		// Whole-file comment, file-only. No-op on directories.
		if m.fileCursor < 0 || m.fileCursor >= len(visible) {
			return m, nil
		}
		entry := visible[m.fileCursor]
		if entry.IsDir {
			return m, nil
		}
		m.commentAnchor = commentAnchor{
			kind:      render.CommentFile,
			filePath:  entry.Path,
			lineStart: 1,
			lineEnd:   m.fileLineCount(entry.Path),
			byteA:     -1,
			byteC:     -1,
		}
		m.enterCommentComposer(m.commentAnchor)
		return m, nil
	}
	if msg.(tea.KeyPressMsg).Code == tea.KeyEnter {
		if m.fileCursor < 0 || m.fileCursor >= len(visible) {
			return m, nil
		}
		entry := visible[m.fileCursor]
		if entry.IsDir {
			m.fileCollapsed[entry.Path] = !m.fileCollapsed[entry.Path]
			return m, nil
		}
		m.openFileViewer(entry.Path)
		return m, nil
	}
	return m, nil
}

// enterFileSearch turns on the fuzzy filter. The query starts
// empty; the cursor stays on the entry it was on.
func (m *model) enterFileSearch() {
	m.fileSearchActive = true
	m.fileSearch = nil
}

// handleFileNavSearchKey routes keys while the search input is
// open. Printable runes go to the query (including j/k/h/l/c/s
// — muscle memory has to give way to typing here, so navigation
// is via Up/Down arrow keys). Backspace trims the query. Esc
// cancels, Enter confirms by opening/toggling the cursor entry
// (read from the still-filtered visible list, then the filter is
// turned off so the file viewer / collapse toggle operates on the
// regular tree).
func (m model) handleFileNavSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if isEsc(msg) {
		m.fileSearchActive = false
		m.fileSearch = nil
		return m, nil
	}
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.Code {
	case tea.KeyEnter:
		visible := m.visibleFileEntries()
		if m.fileCursor < 0 || m.fileCursor >= len(visible) {
			m.fileSearchActive = false
			m.fileSearch = nil
			return m, nil
		}
		entry := visible[m.fileCursor]
		m.fileSearchActive = false
		m.fileSearch = nil
		if entry.IsDir {
			m.fileCollapsed[entry.Path] = !m.fileCollapsed[entry.Path]
			return m, nil
		}
		m.openFileViewer(entry.Path)
		return m, nil
	case tea.KeyUp:
		m.moveFileCursor(-1)
		return m, nil
	case tea.KeyDown:
		m.moveFileCursor(+1)
		return m, nil
	case tea.KeyBackspace:
		if len(m.fileSearch) > 0 {
			m.fileSearch = m.fileSearch[:len(m.fileSearch)-1]
			m.moveFileCursor(0)
		}
		return m, nil
	}
	if kp.Mod == 0 {
		if r, ok := singleRune(msg); ok && r != 0 {
			m.fileSearch = append(m.fileSearch, r)
			m.moveFileCursor(0)
			return m, nil
		}
	}
	return m, nil
}

// fileNavCollapseOrParent implements `h`: collapse the cursor's
// directory if expanded, otherwise jump the cursor to its parent
// directory entry. Top-level entries have no parent in the tree
// and the call is a no-op for them.
func (m *model) fileNavCollapseOrParent() {
	visible := m.visibleFileEntries()
	if m.fileCursor < 0 || m.fileCursor >= len(visible) {
		return
	}
	entry := visible[m.fileCursor]
	if entry.IsDir && !m.fileCollapsed[entry.Path] {
		m.fileCollapsed[entry.Path] = true
		return
	}
	parent := filepath.Dir(entry.Path)
	if parent == "" || parent == "." || parent == "/" {
		return
	}
	if vi := slices.IndexFunc(visible, func(e workspace.Entry) bool {
		return e.Path == parent
	}); vi >= 0 {
		m.fileCursor = vi
	}
}

// fileNavExpandOrChild implements `l`: expand the cursor's
// directory if collapsed, otherwise move the cursor to the next
// visible entry (a child of the current dir). No-op on files
// or empty dirs.
func (m *model) fileNavExpandOrChild() {
	visible := m.visibleFileEntries()
	if m.fileCursor < 0 || m.fileCursor >= len(visible) {
		return
	}
	entry := visible[m.fileCursor]
	if !entry.IsDir {
		return
	}
	if m.fileCollapsed[entry.Path] {
		delete(m.fileCollapsed, entry.Path)
		return
	}
	// Move cursor to the next visible entry if it's strictly
	// deeper than the current dir (i.e., a visible child).
	if m.fileCursor+1 < len(visible) {
		next := visible[m.fileCursor+1]
		if next.Depth > entry.Depth {
			m.fileCursor++
		}
	}
}

// fileLineCount returns the number of source lines in path, or 1
// if the file can't be read (a comment on an unreadable file still
// has a valid anchor).
func (m *model) fileLineCount(path string) int {
	full := m.fileRoot + "/" + path
	data, err := os.ReadFile(full)
	if err != nil {
		return 1
	}
	if len(data) == 0 {
		return 1
	}
	return strings.Count(string(data), "\n") + 1
}

// openFileViewer loads path's content and transitions to
// stateFileView.
func (m *model) openFileViewer(path string) {
	// A new file wipes the file-viewer search state — the prior
	// query and hits are no longer meaningful, and the dim
	// highlights would otherwise paint over text that isn't
	// there.
	m.fileSearchState.reset()
	full := m.fileRoot + "/" + path
	data, err := os.ReadFile(full)
	if err != nil {
		m.latest = session.Message{
			Role: session.RoleUser,
			Text: "[file read failed: " + err.Error() + "]",
		}
		m.refreshViewport()
		return
	}
	content := string(data)
	lineIndex := render.MarkLines(content, m.fileCommentsFor(path))
	w, h := m.viewportSize()
	h-- // ponytail: one line for the file-viewer header above the viewport.
	if h < 1 {
		h = 1
	}
	m.fileViewer = fileViewer{
		path:      path,
		content:   content,
		lines:     strings.Split(strings.TrimRight(content, "\n"), "\n"),
		cursor:    1,
		charPos:   0,
		preferred: 0,
		visual:    fileSelection{LineA: 1, CharA: 0},
		lineIndex: lineIndex,
		viewport:  viewport.New(viewport.WithWidth(w), viewport.WithHeight(h)),
	}
	m.state = stateFileView
	// ponytail: notify the LSP server that the file is open so
	// definition/references/hover have something to look at. Best
	// effort — a missing server (or one that's still spawning) is
	// a no-op; the user just sees no replies until the server is
	// ready, and the next explicit query will re-trigger.
	if m.lsp != nil {
		m.lsp.DidOpen(context.Background(), full, content)
	}
	m.reflow()
	m.refreshFileView()
}

// fileCommentsFor returns the file-kind comments that target path.
func (m *model) fileCommentsFor(path string) []render.Comment {
	var out []render.Comment
	for _, c := range m.comments {
		if c.Kind == render.CommentFile && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

// handleFileViewKey routes keys in stateFileView: j/k move the
// line cursor, h/l move the column cursor, v enters visual,
// c opens the comment composer, s flushes, d/R/K fire LSP
// queries, Esc returns to dir nav, Tab returns to message tab.
func (m model) handleFileViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the search prompt is open, route every key through
	// the search handler.
	if m.fileSearchState.active {
		return m.handleFileSearchKey(msg)
	}
	// Esc with committed hits (no active prompt) clears the
	// search state, mirroring the message view's behaviour.
	if isEsc(msg) && len(m.fileSearchState.hits) > 0 {
		m.fileSearchState.reset()
		m.refreshFileView()
		return m, nil
	}
	m.recoverPanic("handleFileViewKey",
		len(m.fileViewer.lines), m.fileViewer.visual.Active)
	// Hover modal: scroll keys route to the modal's viewport so
	// long go-doc hovers stay readable; Esc and q dismiss the
	// modal without doing anything else (so reading a hover
	// doesn't accidentally quit the app or leave the file
	// viewer); everything else dismisses and falls through to the
	// normal handler (so `d` after `K` dismisses the modal AND
	// fires the definition query).
	if m.hoverModal.visible {
		km := m.hoverModal.viewport.KeyMap
		if key.Matches(msg, km.Down) || key.Matches(msg, km.Up) ||
			key.Matches(msg, km.PageDown) || key.Matches(msg, km.PageUp) ||
			key.Matches(msg, km.HalfPageDown) || key.Matches(msg, km.HalfPageUp) {
			var vpCmd tea.Cmd
			m.hoverModal.viewport, vpCmd = m.hoverModal.viewport.Update(msg)
			return m, vpCmd
		}
		if isEsc(msg) || isKeyRune(msg, 'q') {
			m.hideHoverModal()
			m.refreshFileView()
			return m, nil
		}
		m.hideHoverModal()
		m.refreshFileView()
	}

	// Esc is handled before rune routing so visual-mode exit feels
	// like the message viewer's.
	if isEsc(msg) {
		if m.fileViewer.visual.Active {
			m.fileViewer.visual.Active = false
			m.refreshFileView()
			return m, nil
		}
		m.exitFileViewer()
		return m, nil
	}

	if key.Matches(msg, defaultKeyMap.FileNavBack) && !isEsc(msg) {
		m.exitFileViewer()
		return m, nil
	}

	switch {
	case isKeyRune(msg, 'q'):
		// q returns to the file navigator (stateFileNav) — same
		// surface as Esc. Hard quit is reserved for the picker /
		// picker-tabs where there's nothing to pop out of.
		m.exitFileViewer()
		return m, nil
	case isKeyRune(msg, '/'):
		// '/' opens the search prompt. (The active check at the
		// top of this handler already routed an open prompt
		// elsewhere.)
		m.enterFileViewerSearch()
		m.refreshFileView()
		return m, nil
	case isKeyRune(msg, 'n') && len(m.fileSearchState.hits) > 0:
		if m.fileSearchState.cur < 0 {
			m.fileSearchState.cur = firstHitAtOrAfter(m.fileSearchState.hits, m.fileViewer.cursor-1, m.fileViewer.charPos)
		} else {
			cycleSearchHit(+1, m.fileSearchState.hits, &m.fileSearchState.cur)
		}
		if m.fileSearchState.cur >= 0 {
			h := m.fileSearchState.hits[m.fileSearchState.cur]
			m.fileViewer.cursor = h.LineIdx + 1
			m.fileViewer.charPos = h.ByteA
			m.fileViewer.preferred = h.ByteA
			m.refreshFileView()
		}
		return m, nil
	case isKeyRune(msg, 'N') && len(m.fileSearchState.hits) > 0:
		if m.fileSearchState.cur < 0 {
			m.fileSearchState.cur = firstHitAtOrBefore(m.fileSearchState.hits, m.fileViewer.cursor-1, m.fileViewer.charPos)
		} else {
			cycleSearchHit(-1, m.fileSearchState.hits, &m.fileSearchState.cur)
		}
		if m.fileSearchState.cur >= 0 {
			h := m.fileSearchState.hits[m.fileSearchState.cur]
			m.fileViewer.cursor = h.LineIdx + 1
			m.fileViewer.charPos = h.ByteA
			m.fileViewer.preferred = h.ByteA
			m.refreshFileView()
		}
		return m, nil
	case isKeyRune(msg, 's'):
		m.handleSend()
		return m, nil
	case isKeyRune(msg, 'v'):
		m.fileViewer.visual.Active = !m.fileViewer.visual.Active
		if m.fileViewer.visual.Active {
			// anchor at the current cursor; the moving end is the
			// cursor itself, so nothing else needs to be seeded.
			m.fileViewer.visual.LineA = m.fileViewer.cursor
			m.fileViewer.visual.CharA = m.fileViewer.charPos
		}
		m.refreshFileView()
		return m, nil
	case key.Matches(msg, defaultKeyMap.FileViewDefinition):
		// ponytail: clear the missing-hint footer so it doesn't
		// linger when the user fires a new query. (The hover modal
		// was already dismissed by the routing block above if it
		// was open.)
		m.missingServerHint = ""
		if m.lsphub == nil {
			return m, nil
		}
		return m, m.requestLSP(m.lsphub.RequestDefinition)
	case key.Matches(msg, defaultKeyMap.FileViewReferences):
		m.missingServerHint = ""
		if m.lsphub == nil {
			return m, nil
		}
		return m, m.requestLSP(m.lsphub.RequestReferences)
	case key.Matches(msg, defaultKeyMap.FileViewHover):
		// K while the modal is open: the modal-routing block above
		// already dismissed it. Fire the new query as usual; the
		// resulting HoverMsg replaces the modal content.
		m.missingServerHint = ""
		if m.lsphub == nil {
			return m, nil
		}
		return m, m.requestLSP(m.lsphub.RequestHover)
	}

	if isKeyRune(msg, 'j') {
		m.missingServerHint = ""
		// Reset the current-hit index on motion so the next
		// 'n' continues from the new cursor position, not
		// from the stale hit.
		m.fileSearchState.cur = -1
		m.fileViewMoveLine(+1)
		return m, nil
	}
	if isKeyRune(msg, 'k') {
		m.missingServerHint = ""
		m.fileSearchState.cur = -1
		m.fileViewMoveLine(-1)
		return m, nil
	}

	// h / l: always advance the column cursor. fileViewMoveRune
	// updates visual.CharC when visual is active so the cyan
	// highlight tracks the cursor exactly as today.
	if isKeyRune(msg, 'l') {
		m.missingServerHint = ""
		m.fileSearchState.cur = -1
		m.fileViewMoveRune(+1)
		return m, nil
	}
	if isKeyRune(msg, 'h') {
		m.missingServerHint = ""
		m.fileSearchState.cur = -1
		m.fileViewMoveRune(-1)
		return m, nil
	}
	// w / b: word motion. Reuses the same render.NextWordStart /
	// PrevWordStart as stateNav; cursor is the visual moving end,
	// so the selection tracks automatically.
	if isKeyRune(msg, 'w') {
		m.missingServerHint = ""
		m.fileViewMoveWord(+1)
		return m, nil
	}
	if isKeyRune(msg, 'b') {
		m.missingServerHint = ""
		m.fileViewMoveWord(-1)
		return m, nil
	}

	if isKeyRune(msg, 'c') {
		m.missingServerHint = ""
		m.openFileComment()
		return m, nil
	}
	// ponytail: forward scroll keys (arrows, PageUp/Down) to the
	// file viewport so the user can scroll without moving the
	// cursor. Home/End aren't in the viewport's default keymap;
	// handle them explicitly so the position-indicator and sticky-
	// bottom reattach patterns stay intuitive.
	kp, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch kp.Code {
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			if kp.Mod == 0 {
				var vpCmd tea.Cmd
				m.fileViewer.viewport, vpCmd = m.fileViewer.viewport.Update(msg)
				return m, vpCmd
			}
		case tea.KeyHome:
			if kp.Mod == 0 {
				m.fileViewer.viewport.GotoTop()
				return m, nil
			}
		case tea.KeyEnd:
			if kp.Mod == 0 {
				m.fileViewer.viewport.GotoBottom()
				return m, nil
			}
		}
	}
	return m, nil
}

// handleLSPPickerKey routes keys in stateLSPPicker: j/k move the
// cursor through locations, Enter selects (jumps), Esc dismisses
// back to stateFileView, q quits pinky. Matches the session
// picker's surface so muscle memory carries over.
func (m model) handleLSPPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isEsc(msg):
		m.dismissLSPPicker()
		return m, nil
	case key.Matches(msg, defaultKeyMap.Pick):
		if m.lspPicker.cursor >= 0 && m.lspPicker.cursor < len(m.lspPicker.locations) {
			m.jumpToLocation(m.lspPicker.locations[m.lspPicker.cursor])
		}
		m.dismissLSPPicker()
		return m, nil
	case key.Matches(msg, defaultKeyMap.Up):
		m.moveLSPPickerCursor(-1)
		return m, nil
	case key.Matches(msg, defaultKeyMap.Down):
		m.moveLSPPickerCursor(+1)
		return m, nil
	case key.Matches(msg, defaultKeyMap.QuitPick):
		return m, tea.Quit
	}
	return m, nil
}

// moveLSPPickerCursor clamps the picker's cursor into [0, len-1]
// after applying delta. No-op when the picker is empty.
func (m *model) moveLSPPickerCursor(delta int) {
	n := len(m.lspPicker.locations)
	if n == 0 {
		m.lspPicker.cursor = 0
		return
	}
	m.lspPicker.cursor = min(max(m.lspPicker.cursor+delta, 0), n-1)
}

// handleTodoListKey drives the todo list view. Keys per the
// add-todo-tab spec: j/k move the cursor, a opens the editor on a
// blank item, space toggles done, e edits, d deletes, Esc jumps
// back to the message tab, Tab cycles to the next tab, q quits.
// `s` is intentionally absent — todos are personal and never
// flushed to the agent.
func (m model) handleTodoListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if isEsc(msg) {
		return m, m.escapeToMessage()
	}
	switch {
	case isKeyRune(msg, 'q'):
		return m, tea.Quit
	case isKeyRune(msg, 'j') || key.Matches(msg, defaultKeyMap.Down):
		m.moveTodoCursor(+1)
	case isKeyRune(msg, 'k') || key.Matches(msg, defaultKeyMap.Up):
		m.moveTodoCursor(-1)
	case isKeyRune(msg, 'a'):
		m.todos = append(m.todos, todo.Item{})
		m.todoCursor = len(m.todos) - 1
		m.todoEdit = ""
		m.state = stateTodoEdit
		m.persistTodos()
	case isKeyRune(msg, ' '):
		if m.todoCursor >= 0 && m.todoCursor < len(m.todos) {
			m.todos[m.todoCursor].Done = !m.todos[m.todoCursor].Done
			m.persistTodos()
		}
	case isKeyRune(msg, 'e'):
		if m.todoCursor >= 0 && m.todoCursor < len(m.todos) {
			m.todoEdit = m.todos[m.todoCursor].Text
			m.state = stateTodoEdit
		}
	case isKeyRune(msg, 'd'):
		if len(m.todos) > 0 && m.todoCursor >= 0 && m.todoCursor < len(m.todos) {
			m.todos = append(m.todos[:m.todoCursor], m.todos[m.todoCursor+1:]...)
			if m.todoCursor >= len(m.todos) {
				m.todoCursor = max(0, len(m.todos)-1)
			}
			m.persistTodos()
		}
	}
	return m, nil
}

// moveTodoCursor clamps m.todoCursor into [0, len-1].
func (m *model) moveTodoCursor(delta int) {
	m.todoCursor = clampCursor(m.todoCursor, len(m.todos), delta)
}

// handleTodoEditKey drives the todo edit view. Enter commits the
// edit to the focused item, persists, and returns to stateTodoList.
// Esc discards the edit and returns to stateTodoList. Tab / q
// behave the same as in the list view.
func (m model) handleTodoEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if isEsc(msg) {
		m.todoEdit = ""
		m.state = stateTodoList
		return m, nil
	}
	if key.Matches(msg, defaultKeyMap.Tab) {
		return m, m.toggleTab()
	}
	kp, ok := msg.(tea.KeyPressMsg)
	switch {
	case isKeyRune(msg, 'q'):
		return m, tea.Quit
	case ok && kp.Code == tea.KeyEnter:
		if m.todoCursor >= 0 && m.todoCursor < len(m.todos) {
			m.todos[m.todoCursor].Text = m.todoEdit
			m.persistTodos()
		}
		m.todoEdit = ""
		m.state = stateTodoList
	case ok && kp.Code == tea.KeyBackspace:
		runes := []rune(m.todoEdit)
		if len(runes) > 0 {
			m.todoEdit = string(runes[:len(runes)-1])
		}
	default:
		if ok && len(kp.Text) > 0 {
			m.todoEdit += kp.Text
		}
	}
	return m, nil
}

// dismissLSPPicker returns to stateFileView and clears the picker
// state. If the file viewer has no current path (e.g. the user
// dismissed after a cross-file jump that already closed the
// viewer), fall back to stateFileNav.
func (m *model) dismissLSPPicker() {
	m.lspPicker = lspPickerState{}
	if m.fileViewer.path != "" {
		m.state = stateFileView
	} else {
		m.state = stateFileNav
	}
	m.reflow()
}

// fileViewMoveLine moves the cursor line by delta. The on-screen
// charPos is `min(preferred, len(newLine))`, rune-snapped to the
// nearest UTF-8 boundary so the cursor never lands mid-rune. The
// preferred tracker is held unchanged so j/k preserves the
// intended column across lines; the visual selection's anchor
// stays put and the moving end is the cursor itself.
func (m *model) fileViewMoveLine(delta int) {
	n := len(m.fileViewer.lines)
	if n == 0 {
		return
	}
	m.fileViewer.cursor = min(max(m.fileViewer.cursor+delta, 1), n)
	// ponytail: clamp charPos to the new line's byte length and
	// snap to a rune boundary. LSP and the renderer both expect
	// rune-aligned byte offsets; centralising here means every
	// caller gets the same answer.
	lineLen := len(m.fileViewer.lines[m.fileViewer.cursor-1])
	m.fileViewer.charPos = render.SnapToRuneStart(
		m.fileViewer.lines[m.fileViewer.cursor-1],
		min(m.fileViewer.preferred, lineLen),
	)
	m.scrollFileCursorIntoView()
	m.refreshFileView()
}

// scrollFileCursorIntoView mirrors scrollCursorIntoView for the
// file viewer: if the cursor's 0-indexed line falls outside
// [YOffset, YOffset+Height), scroll so it lands on the last
// visible line. Same 1-line cushion as the message view.
func (m *model) scrollFileCursorIntoView() {
	if len(m.fileViewer.lines) == 0 {
		return
	}
	target := m.fileViewer.cursor - 1
	top := m.fileViewer.viewport.YOffset()
	bot := top + m.fileViewer.viewport.Height() - 1
	if target >= top && target <= bot {
		return
	}
	if target < top {
		m.fileViewer.viewport.SetYOffset(target)
	} else {
		m.fileViewer.viewport.SetYOffset(target - m.fileViewer.viewport.Height() + 1)
	}
}

// fileViewMoveRune moves the column cursor (fileViewer.charPos)
// by one rune within the current line. The preferred column
// tracker tracks the rightmost column reached (max on `l`,
// unchanged on `h`); the cursor is the visual moving end, so
// no separate visual field needs updating here.
func (m *model) fileViewMoveRune(delta int) {
	lineIdx := m.fileViewer.cursor - 1
	if lineIdx < 0 || lineIdx >= len(m.fileViewer.lines) {
		return
	}
	line := m.fileViewer.lines[lineIdx]
	c := m.fileViewer.charPos
	if delta > 0 {
		if c >= len(line) {
			return
		}
		_, sz := utf8.DecodeRuneInString(line[c:])
		c += sz
	} else {
		if c <= 0 {
			return
		}
		_, sz := utf8.DecodeLastRuneInString(line[:c])
		c -= sz
	}
	m.fileViewer.charPos = c
	// ponytail: preferred tracks the rightmost column reached on
	// `l`; `h` retreats the cursor but leaves preferred unchanged
	// so the next `j`/`k` to a long line restores the column.
	if delta > 0 {
		m.fileViewer.preferred = max(m.fileViewer.preferred, c)
	}
	m.refreshFileView()
}

// fileViewMoveWord moves the cursor to the start of the next or
// previous word. Blank lines are separators; punctuation runs are
// words on their own (vim's iskeyword model). `delta > 0` advances,
// `delta < 0` retreats. `w` updates `preferred` like `l`; `b` leaves
// it like `h`. The cursor is the visual moving end, so the
// selection tracks automatically — no separate visual field to
// update.
func (m *model) fileViewMoveWord(delta int) {
	n := len(m.fileViewer.lines)
	if n == 0 {
		return
	}
	li := m.fileViewer.cursor - 1 // render pkg is 0-based
	var nli, ncp int
	if delta > 0 {
		nli, ncp = render.NextWordStart(m.fileViewer.lines, li, m.fileViewer.charPos)
	} else {
		nli, ncp = render.PrevWordStart(m.fileViewer.lines, li, m.fileViewer.charPos)
	}
	m.fileViewer.cursor = nli + 1 // back to 1-based
	m.fileViewer.charPos = ncp
	if delta > 0 && ncp > m.fileViewer.preferred {
		m.fileViewer.preferred = ncp
	}
	m.scrollFileCursorIntoView()
	m.refreshFileView()
}

// requestLSP fires one of the three LSP queries at the
// word under the file viewer's cursor. nil when m.lsphub is
// unset (no LSP manager was wired up — tests that don't care).
// Each of the bridge methods has the same signature, so we
// accept the function reference instead of repeating the same
// boilerplate three times. ponytail: shorter than 3 cases
// with the same body.
func (m *model) requestLSP(issue func(ctx context.Context, path string, line, char int) tea.Cmd) tea.Cmd {
	if m.lsphub == nil || m.fileViewer.path == "" {
		return nil
	}
	line, char := m.wordAtCursor()
	return issue(context.Background(), m.fileRoot+"/"+m.fileViewer.path, line, char)
}

// wordAtCursor returns the (line, byte-offset) of the start of
// the [A-Za-z0-9_] run containing the cursor's charPos. When
// charPos falls on a non-word byte (whitespace, punctuation,
// the byte that immediately follows the end of an identifier,
// or the line-end position one past the last byte), charPos is
// returned unchanged — the LSP server is then free to return
// zero results, which the caller treats as silent.
func (m *model) wordAtCursor() (int, int) {
	lineIdx := m.fileViewer.cursor - 1
	if lineIdx < 0 || lineIdx >= len(m.fileViewer.lines) {
		return m.fileViewer.cursor, m.fileViewer.charPos
	}
	line := m.fileViewer.lines[lineIdx]
	c := m.fileViewer.charPos
	if c < 0 {
		c = 0
	}
	if c >= len(line) {
		// ponytail: at or past the line end there's no byte to
		// classify; LSP gets the position and returns 0 results.
		return m.fileViewer.cursor, c
	}
	if !isWordByte(line[c]) {
		return m.fileViewer.cursor, c
	}
	// ponytail: walk back over the word to its start so the LSP
	// server sees the word's leading byte, not a position in the
	// middle of it.
	for c > 0 && isWordByte(line[c-1]) {
		c--
	}
	return m.fileViewer.cursor, c
}

// isWordByte reports whether b is a member of the code-identifier
// alphabet [A-Za-z0-9_]. Code identifiers only — no Unicode
// categories. LSP servers typically don't tokenise Unicode
// identifiers anyway, and pinky isn't a prose editor.
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// exitFileViewer leaves stateFileView, sends didClose for the
// current path, and clears transient LSP state. Called from
// handleFileViewKey on Esc / Tab. The LSP close is best-effort;
// the server may already be shutting down and errors are dropped.
func (m *model) exitFileViewer() {
	if m.fileViewer.path != "" && m.lsp != nil {
		full := m.fileRoot + "/" + m.fileViewer.path
		m.lsp.DidClose(context.Background(), full)
	}
	m.hideHoverModal()
	m.missingServerHint = ""
	m.fileViewer.visual.Active = false
	// Leaving the file viewer wipes the file-viewer search
	// state — re-entering doesn't restore the prior query.
	m.fileSearchState.reset()
	m.state = stateFileNav
	m.reflow()
}

// jumpToLocation moves the cursor to loc. Same-file locations
// only mutate cursor + charPos + scroll. Cross-file locations
// close the current viewer and open a new one at (line, char).
//
// loc's Range.Start is 0-based per the LSP spec; pinky's file
// viewer is 1-based, so we add 1 to land on the source line.
// Char stays 0-based (matches pinky's byte-offset model).
//
// ponytail: relative path resolution reuses filepath.Rel so the
// caller doesn't need to know the manager's rootURI.
func (m *model) jumpToLocation(loc pinkylsp.Location) {
	uriPath, err := loc.URI.Path()
	if err != nil || uriPath == "" {
		return
	}
	rel, err := filepath.Rel(m.fileRoot, uriPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Outside the workspace root — treat as a no-op so the
		// user can dismiss the picker without teleporting.
		return
	}
	if rel != m.fileViewer.path {
		m.exitFileViewer()
		m.openFileViewer(rel)
		if m.state != stateFileView {
			return
		}
	}
	line := int(loc.Range.Start.Line) + 1 // LSP 0-based → 1-based
	char := int(loc.Range.Start.Character)
	if line < 1 {
		line = 1
	}
	if line > len(m.fileViewer.lines) {
		line = len(m.fileViewer.lines)
	}
	if char < 0 {
		char = 0
	}
	if char > len(m.fileViewer.lines[line-1]) {
		char = len(m.fileViewer.lines[line-1])
	}
	m.fileViewer.cursor = line
	m.fileViewer.charPos = char
	m.fileViewer.preferred = char
	// ponytail: jumps clear visual so a stale anchor doesn't drag
	// the selection across the new location.
	m.fileViewer.visual.Active = false
	m.fileViewer.visual.LineA = line
	m.fileViewer.visual.CharA = char
	m.scrollFileCursorIntoView()
	m.refreshFileView()
	m.hideHoverModal()
}

// openFileComment opens the comment composer with an anchor
// derived from the visual selection (or the current line when no
// visual is active):
//
//   - no visual OR single-line point visual:
//     file line-range anchored to the current line
//   - visual single-line with a non-empty byte range:
//     file-inline with min/max char and the verbatim excerpt
//   - visual multi-line (any char endpoints, including point):
//     file line-range covering the lines touched by the
//     selection; char endpoints are dropped (the format has no
//     multi-line inline kind).
func (m *model) openFileComment() {
	fv := &m.fileViewer
	a := commentAnchor{
		kind:     render.CommentFile,
		filePath: fv.path,
		// Default to line-range (no inline bytes). byteA/byteC
		// stay at -1 to signal whole-line; valid byte offsets are
		// >= 0 so the sentinel is unambiguous.
		byteA: -1,
		byteC: -1,
	}
	a.lineStart = fv.cursor
	a.lineEnd = fv.cursor
	if fv.visual.Active {
		aLine, aChar := fv.visual.LineA, fv.visual.CharA
		cLine, cChar := fv.cursor, fv.charPos
		if aLine < cLine {
			a.lineStart, a.lineEnd = aLine, cLine
		} else {
			a.lineStart, a.lineEnd = cLine, aLine
		}
		if a.lineStart == a.lineEnd && aChar != cChar {
			a.byteA, a.byteC = aChar, cChar
			if a.byteA > a.byteC {
				a.byteA, a.byteC = a.byteC, a.byteA
			}
		}
	}
	m.commentAnchor = a
	m.enterCommentComposer(a)
}

// buildCommentAnchor assembles (byteA, byteC) for the next comment.
// With an active selection, anchor is the selection range; otherwise
// anchor covers the whole current source line (every byte from the
// line's start to its end, inclusive of the trailing newline).
func buildCommentAnchor(visual render.NavMode, cur *render.NavCursor, sel *render.NavSelection, lines []string, lineStartOffsets []int) commentAnchor {
	if visual == render.NavLine && sel.ByteA != sel.ByteC {
		a, c := sel.ByteA, sel.ByteC
		if a > c {
			a, c = c, a
		}
		return commentAnchor{kind: render.CommentMessage, byteA: a, byteC: c}
	}
	if cur.LineIdx < 0 || cur.LineIdx >= len(lines) {
		return commentAnchor{kind: render.CommentMessage, byteA: -1, byteC: -1}
	}
	a := lineStartOffsets[cur.LineIdx]
	c := a + len(lines[cur.LineIdx]) + 1 // +1 for the trailing "\n"
	return commentAnchor{kind: render.CommentMessage, byteA: a, byteC: c}
}

// handleSend is the universal `s` dispatch. In nav it batch-sends
// the accumulated comments (no-op if empty). In compose it sends the
// textarea content with the optional comments appendix. Called from
// handleNavKey and (via the same ActionSend) from handleComposeKey.
func (m *model) handleSend() {
	switch m.state {
	case stateNav, stateFileNav, stateFileView:
		// ponytail: `s` flushes accumulated comments from any
		// file-review state too — the user can stage comments
		// across file views without bouncing back to the message.
		m.submitAllComments()
	case stateCompose:
		text := m.textarea.Value()
		if text == "" {
			return
		}
		if m.includeComments && len(m.comments) > 0 {
			text += "\n\n---\n" + render.FormatCommentsAppendix(m.comments)
		}
		if !m.dispatch(text) {
			return
		}
		m.state = stateNav
		m.textarea.Blur()
		m.textarea.Reset()
		m.reflow()
	}
}

func (m model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, defaultKeyMap.IncludeComments):
		m.includeComments = !m.includeComments
		return m, nil
	case key.Matches(msg, defaultKeyMap.Cancel):
		m.state = stateNav
		m.textarea.Blur()
		m.textarea.Reset()
		m.reflow()
		return m, nil
	}
	// `s` is the universal send — dispatch via NavHandle so the
	// single-rune path is the source of truth (matches D3 / D4).
	if isKeyRune(msg, 's') {
		m.handleSend()
		return m, nil
	}
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, taCmd
}

func (m *model) reflow() {
	if m.height == 0 || m.width == 0 {
		return
	}
	vpHeight := m.height - statusHeight - m.helpHeight()
	if m.state == stateCompose || m.state == stateCommentComposer {
		vpHeight -= composeHeight + 1
	}
	if m.state == stateFileView {
		vpHeight-- // ponytail: header line above the file viewport.
		if m.missingServerHint != "" {
			vpHeight-- // ponytail: install-hint footer.
		}
	}
	// Top header: 1 or 2 rows depending on width / cwd length.
	// Compute once and subtract so layout stays in one place.
	m.headerHeight = strings.Count(m.headerView(), "\n") + 1
	vpHeight -= m.headerHeight
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(vpHeight)
	if m.fileViewer.path != "" {
		m.fileViewer.viewport.SetWidth(m.width)
		m.fileViewer.viewport.SetHeight(vpHeight)
		// ponytail: every reflow that changes the file viewport
		// size can leave the cursor's line off the new visible
		// range — resize, help toggle (?), tab/state change all
		// route through here. Re-pull the cursor into view once
		// at the source instead of patching every caller.
		m.scrollFileCursorIntoView()
	}
	if m.state == stateCompose || m.state == stateNav {
		m.textarea.SetWidth(m.width)
	}
}

// helpHeight reports the number of terminal lines the help footer
// will need. Short help is 1 line; full help grows with the largest
// group in the current state's FullHelp(). Computed from the live
// bindings so adding a new key to a group automatically extends the
// reserved space.
func (m *model) helpHeight() int {
	if !m.help.ShowAll {
		return 1
	}
	maxN := 0
	for _, g := range m.FullHelp() {
		maxN = max(maxN, len(g))
	}
	return max(maxN, 1)
}

// ShortHelp returns the keybindings rendered in the one-line help
// footer. Curated per state so the most useful keys (not all of them)
// stay visible without truncation. `?` is included where expanding
// to full help is meaningful.
func (m model) ShortHelp() []key.Binding {
	switch m.state {
	case statePicking:
		return []key.Binding{
			defaultKeyMap.Up, defaultKeyMap.Down,
			defaultKeyMap.Pick, defaultKeyMap.Help,
		}
	case stateNav:
		return []key.Binding{
			defaultKeyMap.NavComment,
			defaultKeyMap.NavSend,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
		}
	case stateCompose:
		return []key.Binding{
			defaultKeyMap.IncludeComments,
			defaultKeyMap.Cancel, defaultKeyMap.Help,
		}
	case stateCommentComposer:
		return []key.Binding{
			defaultKeyMap.SaveComment,
			defaultKeyMap.Cancel,
		}
	case stateFileNav:
		return []key.Binding{
			defaultKeyMap.FileNavOpen,
			defaultKeyMap.FileNavComment,
			defaultKeyMap.FileNavSearch,
			defaultKeyMap.NavSend,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
		}
	case stateFileView:
		return []key.Binding{
			defaultKeyMap.FileViewComment,
			defaultKeyMap.FileViewDefinition,
			defaultKeyMap.FileViewReferences,
			defaultKeyMap.FileViewHover,
			defaultKeyMap.NavSend,
			defaultKeyMap.FileNavBack,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
		}
	case stateTodoList:
		return []key.Binding{
			defaultKeyMap.Up, defaultKeyMap.Down,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
			defaultKeyMap.QuitPick,
		}
	case stateTodoEdit:
		return []key.Binding{
			defaultKeyMap.SaveComment,
			defaultKeyMap.Cancel,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
		}
	case stateLSPPicker:
		return []key.Binding{
			defaultKeyMap.Up, defaultKeyMap.Down,
			defaultKeyMap.Pick, defaultKeyMap.QuitPick,
		}
	case stateError:
		return []key.Binding{defaultKeyMap.QuitError}
	}
	return nil
}

// FullHelp returns every keybinding for the current state, grouped
// for the expanded (multi-column) help view. Reuses the same group
// ordering the short view picks from, so adding a binding to a group
// here also surfaces it in the long view.
func (m model) FullHelp() [][]key.Binding {
	switch m.state {
	case statePicking:
		return [][]key.Binding{
			{defaultKeyMap.Up, defaultKeyMap.Down},
			{defaultKeyMap.Pick},
			{defaultKeyMap.QuitPick, defaultKeyMap.Help},
		}
	case stateNav:
		return [][]key.Binding{
			{defaultKeyMap.NavBlockDown, defaultKeyMap.NavBlockUp, defaultKeyMap.NavRuneLeft, defaultKeyMap.NavRuneRight, defaultKeyMap.NavWordRight, defaultKeyMap.NavWordLeft},
			{defaultKeyMap.NavVisual, defaultKeyMap.NavComment, defaultKeyMap.NavSend, defaultKeyMap.NavCompose, defaultKeyMap.NavRefresh},
			{defaultKeyMap.NavSearch, defaultKeyMap.NavSearchNext, defaultKeyMap.NavSearchPrev},
			{defaultKeyMap.Tab, defaultKeyMap.NavQuit, defaultKeyMap.Help},
		}
	case stateCompose:
		return [][]key.Binding{
			{defaultKeyMap.Newline},
			{defaultKeyMap.IncludeComments},
			{defaultKeyMap.Cancel, defaultKeyMap.Help},
		}
	case stateCommentComposer:
		return [][]key.Binding{
			{defaultKeyMap.SaveComment},
			{defaultKeyMap.Cancel},
		}
	case stateFileNav:
		return [][]key.Binding{
			{defaultKeyMap.FileNavDown, defaultKeyMap.FileNavUp, defaultKeyMap.FileNavCollapse, defaultKeyMap.FileNavExpand},
			{defaultKeyMap.FileNavOpen, defaultKeyMap.FileNavComment, defaultKeyMap.FileNavSearch, defaultKeyMap.NavSend},
			{defaultKeyMap.FileNavBack, defaultKeyMap.Tab, defaultKeyMap.Help},
		}
	case stateFileView:
		return [][]key.Binding{
			{defaultKeyMap.FileViewDown, defaultKeyMap.FileViewUp, defaultKeyMap.FileViewLeft, defaultKeyMap.FileViewRight, defaultKeyMap.FileViewWordRight, defaultKeyMap.FileViewWordLeft},
			{defaultKeyMap.FileViewVisual, defaultKeyMap.FileViewComment, defaultKeyMap.NavSend},
			{defaultKeyMap.FileViewSearch, defaultKeyMap.FileViewSearchNext, defaultKeyMap.FileViewSearchPrev},
			{defaultKeyMap.FileViewDefinition, defaultKeyMap.FileViewReferences, defaultKeyMap.FileViewHover},
			{defaultKeyMap.FileViewBack, defaultKeyMap.Tab, defaultKeyMap.Help},
		}
	case stateLSPPicker:
		return [][]key.Binding{
			{defaultKeyMap.Up, defaultKeyMap.Down},
			{defaultKeyMap.Pick, defaultKeyMap.QuitPick},
		}
	case stateError:
		return [][]key.Binding{
			{defaultKeyMap.QuitError, defaultKeyMap.Help},
		}
	}
	return nil
}

// statusBarStyle paints the bottom status line as a full-width bar so
// pinky visually anchors to the terminal edges instead of leaving
// whitespace on the right.
var statusBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("241")).
	Background(lipgloss.Color("236")).
	Padding(0, 1)

// visualModeStyle is the bright accent used for the [VISUAL] chip in
// the status line when visual mode is active. Picked so it stands
// out against the dim status-bar background and is unambiguous about
// the mode (visual mode has no other persistent on-screen indicator).
// Cyan (51) matches the gutter focus indicator; magenta (212) is
// reserved for the picker header accent.
var visualModeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("232")).
	Background(lipgloss.Color("51")).
	Bold(true).
	Padding(0, 1)

// activeTabStyle / inactiveTabStyle paint the two cells of the
// top-of-screen tab header (Message / Files). Active cell uses
// cyan (51) fill to match the gutter focus indicator and the
// [VISUAL] chip; inactive is dim (241). Replaces the previous
// status-line `tabChipStyle` which carried the same info with
// less ink.
var (
	activeTabStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("51")).
			Foreground(lipgloss.Color("232")).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Padding(0, 1)
)

// dimStyle is the foreground-only dim color used for non-emphatic
// chrome (header cwd, picker row labels). Same color as
// statusBarStyle foreground so the dim palette stays consistent.
var dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

// headerStyle / selectedStyle / normalStyle are the recurring
// row-render trio: bold magenta for the section header, bold
// green for the focused row, plain grey for the rest. Reused by
// the dir navigator, the LSP picker, and the agent-session picker.
var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

// pendingIdleStyle / pendingArmedStyle paint the header's right
// pending-comment count. Yellow always (zero still visible), bold
// when > 0 to flag "unsent stuff waiting".
// ponytail: zero-still-visible is intentional — hiding it on 0
// trades discoverability for one fewer color cue.
var (
	pendingIdleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("228"))
	pendingArmedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true)
)

// headerAbbrevLevels is the sequence of maxSegs values the header
// tries in order before falling back to a 2-line split on `/`.
var headerAbbrevLevels = []int{2, 1}

// hoverModalBoxStyle (inlined at the call site) paints the hover
// pop-up as a rounded cyan border with one-cell padding. The body
// itself is rendered in cyan italic so the box reads as related
// to the old footer but visually distinct from the file body
// underneath. Defined inline at the one call site in
// overlayHoverModal.

// hoverModalKeyMap restricts the modal's viewport to scroll keys
// only. The viewport's default KeyMap binds h/l/b/f/space/etc.
// — all of which would conflict with the file viewer's own
// bindings and fire unwanted motions on the underlying cursor.
// Scrolling keys route to the viewport; anything else falls
// through to the modal's "dismiss + handle normally" path.
var hoverModalKeyMap = viewport.KeyMap{
	Down:         key.NewBinding(key.WithKeys("down", "j")),
	Up:           key.NewBinding(key.WithKeys("up", "k")),
	PageDown:     key.NewBinding(key.WithKeys("pgdown")),
	PageUp:       key.NewBinding(key.WithKeys("pgup")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
}

func (m *model) refreshViewport() {
	if m.latest.Text == "" {
		m.lines = nil
		m.lineStartOffsets = nil
		m.wrappedToSrc = nil
		m.sourceToFirst = nil
		m.viewport.SetContent(dimStyle.Italic(true).Render("waiting for agent…"))
		return
	}
	// ponytail: word-wrap to fit the viewport before the gutter is
	// prepended. Without this, long lines get truncated on the right
	// by the viewport's ansi.Cut. Width - 1 leaves room for the
	// single-cell gutter character.
	var wrapWidth int
	if w := m.viewport.Width(); w > 1 {
		wrapWidth = w - 1
	}
	m.lines = strings.Split(m.latest.Text, "\n")
	m.lineStartOffsets = make([]int, len(m.lines))
	{
		offset := 0
		for i, ln := range m.lines {
			m.lineStartOffsets[i] = offset
			offset += len(ln) + 1
		}
	}
	guttered, w2s := render.LineGutter(m.latest.Text, m.comments, wrapWidth)
	m.wrappedToSrc = w2s
	m.sourceToFirst = make([]int, len(m.lines))
	for i := 1; i < len(w2s); i++ {
		if w2s[i] != w2s[i-1] && m.sourceToFirst[w2s[i]] == 0 {
			m.sourceToFirst[w2s[i]] = i
		}
	}
	// Apply the inline-block cursor at (lineIdx, charPos). The
	// gutter logic above ran without knowing where the cursor is;
	// the cursor paint is layered on top.
	if m.cursor.LineIdx >= 0 && m.cursor.LineIdx < len(m.lines) {
		guttered = render.ApplyInlineCursor(
			guttered,
			m.lines,
			m.cursor.LineIdx,
			m.cursor.CharPos,
			m.sourceToFirst,
			wrapWidth,
		)
	}
	// Search dim: highlight every non-current match in m.navSearch.hits
	// with a background colour. The cursor's paint above already
	// covers the current hit with the inverted-block style, so
	// we skip the index at m.navSearch.cur here. The dim splice
	// preserves any glamour-set background (e.g. the 228-on-236
	// code-block style) on the cells around each hit.
	if len(m.navSearch.hits) > 0 {
		for i, h := range m.navSearch.hits {
			if i == m.navSearch.cur {
				continue
			}
			guttered = render.SpliceStyleAcrossWrap(
				guttered, m.lines, h,
				"\x1b[48;5;58m",
				wrapWidth, m.sourceToFirst,
			)
		}
	}
	m.viewport.SetContent(guttered)
}

// wrappedYOffsetToSource translates a viewport YOffset (in wrapped
// lines) back to the underlying source-line index. Returns 0 when
// the viewport is empty / no wrap map.
func (m *model) wrappedYOffsetToSource(y int) int {
	if len(m.wrappedToSrc) == 0 {
		return 0
	}
	if y < 0 {
		return m.wrappedToSrc[0]
	}
	if y >= len(m.wrappedToSrc) {
		return m.wrappedToSrc[len(m.wrappedToSrc)-1]
	}
	return m.wrappedToSrc[y]
}

// sourceYOffset returns the first wrapped YOffset that belongs to
// the given source line. Used to scroll the viewport to a block
// boundary (e.g. when the cursor moves to a new block).
func (m *model) sourceYOffset(src int) int {
	if src < 0 || src >= len(m.sourceToFirst) {
		return 0
	}
	return m.sourceToFirst[src]
}

func (m model) View() tea.View {
	// Help footer is rendered for every state; `?` flips it between
	// the one-line short view and the multi-column full view.
	helpView := m.help.View(m)
	// Top header is shared by every attached state. The picker
	// (no agent yet) and the error screen skip it.
	header := m.headerView()
	var content string
	switch m.state {
	case statePicking:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.pickerView(),
			helpView,
		))
	case stateNav:
		var navParts []string
		navParts = append(navParts, header, m.viewport.View())
		if m.navSearch.active {
			navParts = append(navParts,
				dimStyle.Render("/"+string(m.navSearch.query)+"▏"))
		}
		navParts = append(navParts, helpView, m.statusLine())
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left, navParts...))
	case stateCompose:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.viewport.View(),
			m.textarea.View(),
			helpView,
			m.statusLine(),
		))
	case stateCommentComposer:
		// Render the viewport the user came from so a file-kind
		// comment composer overlays the file viewer (not the
		// message viewport, which would look like the user got
		// teleported to the last message).
		body := m.viewport.View()
		if m.commentAnchor.kind == render.CommentFile && m.fileViewer.path != "" {
			body = m.fileViewView()
		}
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			body,
			m.commentTa.View(),
			helpView,
			m.statusLine(),
		))
	case stateFileNav:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.fileNavView(),
			helpView,
			m.statusLine(),
		))
	case stateFileView:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.fileViewView(),
			maybeHoverFooter(m),
			helpView,
			m.statusLine(),
		))
		content = m.overlayHoverModal(content)
	case stateTodoList:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.todoListView(),
			helpView,
			m.statusLine(),
		))
	case stateTodoEdit:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.todoListView(),
			m.todoEditView(),
			helpView,
			m.statusLine(),
		))
	case stateLSPPicker:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.lspPickerView(),
			helpView,
			m.statusLine(),
		))
	case stateError:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.errorView(),
			helpView,
		))
	default:
		content = m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			helpView,
			m.statusLine(),
		))
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// padRight appends spaces so s reaches exactly width visible cells.
// Returns s unchanged when width is non-positive or s already fills
// the line.
func padRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	pad := width - ansi.StringWidth(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// shortenCwd abbreviates an absolute path to ~/last-N-segments so
// the header line stays compact at typical widths. Already-~/
// paths are returned unchanged. maxSegs is the number of trailing
// segments to keep (maxSegs=2 keeps the last two: "/a/b/c" →
// "~/b/c"). Empty input returns empty.
func shortenCwd(path string, maxSegs int) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		return path
	}
	cleaned := strings.TrimPrefix(path, "/")
	parts := strings.Split(cleaned, "/")
	if len(parts) <= maxSegs {
		return path
	}
	return "~/" + strings.Join(parts[len(parts)-maxSegs:], "/")
}

// splitOnSlash returns (prefix, rest) where prefix is the longest
// slash-separated prefix of s whose width is <= max. If s fits
// entirely, rest is empty. If s has no slash that can be split
// (single segment), returns (s, "") — caller decides how to render
// the overflow. Used as the header's last-resort wrap.
func splitOnSlash(s string, max int) (string, string) {
	if max <= 0 || ansi.StringWidth(s) <= max {
		return s, ""
	}
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return s, ""
	}
	var prefix strings.Builder
	for i, p := range parts {
		seg := p
		if i > 0 {
			seg = "/" + p
		}
		if i > 0 && ansi.StringWidth(prefix.String()+seg) > max {
			return prefix.String(), strings.Join(parts[i:], "/")
		}
		prefix.WriteString(seg)
	}
	return s, ""
}

// headerView renders the top header: cwd (left, dim) + pending
// comment count (right, yellow / bold when > 0). Wrap pipeline:
// try maxSegs=2, fall back to maxSegs=1, fall back to a 2-line
// split on `/`. Always pads to m.width; returns the raw line when
// m.width is 0 (pre-WindowSizeMsg).
func (m *model) headerView() string {
	width := m.width
	tabRow := m.tabHeader()
	if width <= 0 {
		// ponytail: pre-WindowSizeMsg fallback matches the old
		// statusLine behavior — return the natural-width content
		// with the tab row on top.
		return tabRow + "\n" + m.headerLine(shortenCwd(m.fileRoot, 2), len(m.comments))
	}
	count := len(m.comments)
	for _, maxSegs := range headerAbbrevLevels {
		cwd := padRight(m.headerLine(shortenCwd(m.fileRoot, maxSegs), count), width)
		if ansi.StringWidth(cwd) <= width {
			return tabRow + "\n" + cwd
		}
	}
	// Last resort: split path across two lines; count moves to line 3
	// (after the tab row).
	path := shortenCwd(m.fileRoot, 1)
	if a, b := splitOnSlash(path, width); b != "" {
		style := pendingIdleStyle
		if count > 0 {
			style = pendingArmedStyle
		}
		return tabRow + "\n" +
			padRight(dimStyle.Render(a), width) + "\n" +
			padRight(style.Render(fmt.Sprintf("%d pending", count)), width)
	}
	return tabRow + "\n" + padRight(m.headerLine(path, count), width)
}

// tabHeader renders the three-cell row above the cwd header in
// canonical Message / Files / Todos order. The active cell carries
// the filled background; inactive cells are dim. Per the
// `tab-header` and `add-todo-tab` specs.
func (m *model) tabHeader() string {
	msgStyle := inactiveTabStyle
	fileStyle := inactiveTabStyle
	todoStyle := inactiveTabStyle
	switch m.tab {
	case tabMessage:
		msgStyle = activeTabStyle
	case tabFiles:
		fileStyle = activeTabStyle
	case tabTodos:
		todoStyle = activeTabStyle
	}
	return msgStyle.Render("Message") + " " +
		fileStyle.Render("Files") + " " +
		todoStyle.Render("Todos")
}

// headerLine builds one row of the header: dim cwd on the left,
// yellow pending count on the right, no padding (caller pads
// once the wrap decision is made). Extracted so headerView's
// try/abbreviate loop can compare widths cheaply.
func (m *model) headerLine(path string, count int) string {
	left := dimStyle.Render(path)
	style := pendingIdleStyle
	if count > 0 {
		style = pendingArmedStyle
	}
	right := style.Render(fmt.Sprintf("%d pending", count))
	return left + right
}

// fillWidth pads every line of s to m.width so the rendered output
// spans the full terminal. Before WindowSizeMsg fires, m.width is 0
// and we return s unchanged.
func (m model) fillWidth(s string) string {
	if m.width <= 0 {
		return s
	}
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(padRight(line, m.width))
	}
	return b.String()
}

// errorView renders a single centered error message. The TUI shows
// this when attach or session discovery fails; any key press quits.
// The help footer carries the "press any key" hint.
func (m model) errorView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	return headerStyle.Render("pinky: error") + "\n\n" +
		bodyStyle.Render(m.err.Error())
}

// fileNavView renders the workspace header followed by the tree
// of visible entries. When `/` search is active, a search input
// line is inserted between the header and the tree, and the tree
// shows only entries matching the fuzzy query (collapse map is
// ignored during search). Each tree line carries a 2-space indent
// per depth level, a collapse marker (▾ expanded, ▸ collapsed)
// for directories, and a cyan ▍ on the cursor's row.
func (m model) fileNavView() string {
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)

	visible := m.visibleFileEntries()
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("workspace — %s", m.fileRoot)))
	b.WriteByte('\n')
	if m.fileSearchActive {
		b.WriteString(promptStyle.Render("/") + " " + string(m.fileSearch) + dimStyle.Render("▏"))
		b.WriteByte('\n')
	}
	if len(visible) == 0 {
		if m.fileSearchActive {
			b.WriteString(dimStyle.Render("  (no matches)"))
		} else {
			b.WriteString(dimStyle.Render("  (empty)"))
		}
		return b.String()
	}
	for i, e := range visible {
		indent := strings.Repeat("  ", e.Depth-1)
		marker := " "
		if e.IsDir && !m.fileSearchActive {
			if m.fileCollapsed[e.Path] {
				marker = "▸"
			} else {
				marker = "▾"
			}
		}
		name := filepath.Base(e.Path)
		line := fmt.Sprintf("%s%s %s", indent, marker, name)
		if i == m.fileCursor {
			b.WriteString(selectedStyle.Render("▶ " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderFileContent builds the per-line rendered string for the
// file viewer: a yellow `▍` gutter on lines covered by a
// file-kind comment (empty gutter otherwise; no cursor gutter),
// right-aligned line numbers, an inline cyan highlight of the
// visual selection, and an inline block cursor at the file
// viewer's (cursor, charPos) rendered with an inverted
// background. The cursor's ANSI is layered over the selection's
// so the cursor byte is visible even when it overlaps the
// selection range.
func (m model) renderFileContent() string {
	if len(m.fileViewer.lines) == 0 {
		return dimStyle.Render("(empty file)")
	}
	const yellow = "\x1b[38;5;228m"
	width := len(strconv.Itoa(len(m.fileViewer.lines)))
	var b strings.Builder
	for i, content := range m.fileViewer.lines {
		ln := i + 1
		var gutter string
		if i < len(m.fileViewer.lineIndex) && m.fileViewer.lineIndex[i] {
			gutter = yellow + "▍" + "\x1b[0m"
		} else {
			gutter = " "
		}
		selected := applySelection(content, ln, m.fileViewer.visual, m.fileViewer.cursor, m.fileViewer.charPos)
		// Search dim: if this line has a non-current hit in
		// m.fileSearchState.hits, splice the dim background on
		// the hit's byte range. The cursor's paint (below) runs
		// after this pass so the current hit's inverted-block
		// style wins.
		if len(m.fileSearchState.hits) > 0 {
			for hi, h := range m.fileSearchState.hits {
				if hi == m.fileSearchState.cur {
					continue
				}
				if h.LineIdx == i && h.ByteC <= len(content) {
					selected = render.SpliceStyle(selected, "\x1b[48;5;58m", h.ByteA, h.ByteC)
				}
			}
		}
		var rendered string
		if ln == m.fileViewer.cursor {
			rendered = render.ApplyCursor(selected, content, m.fileViewer.charPos)
		} else {
			rendered = selected
		}
		fmt.Fprintf(&b, "%s %*d  %s\n", gutter, width, ln, rendered)
	}
	return strings.TrimRight(b.String(), "\n")
}

// todoListView renders the structured todo list (stateTodoList /
// stateTodoEdit). Each item is one row: `[ ]` or `[x]` + the
// item's text. The focused row carries a `▶` marker; the cursor
// wraps at the list ends.
func (m model) todoListView() string {
	if len(m.todos) == 0 {
		return dimStyle.Render("(no todos — press a to add)")
	}
	var b strings.Builder
	for i, item := range m.todos {
		marker := " "
		box := "[ ]"
		style := dimStyle
		if item.Done {
			box = "[x]"
			style = dimStyle.Strikethrough(true)
		}
		if i == m.todoCursor {
			marker = "▶"
			style = lipgloss.NewStyle().Bold(true)
		}
		row := fmt.Sprintf("%s %s %s", marker, box, item.Text)
		b.WriteString(style.Render(row))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// todoEditView renders the inline edit input beneath the list
// while in stateTodoEdit. Enter commits, Esc cancels.
func (m model) todoEditView() string {
	if m.state != stateTodoEdit {
		return ""
	}
	return fmt.Sprintf("> %s_", m.todoEdit)
}

// fileViewView renders the file-viewer header (path + optional
// `lines N-M of K` position indicator) followed by the file
// viewport's visible window. The header sits outside the viewport
// so the indicator stays visible while the body scrolls.
func (m model) fileViewView() string {
	header := fmt.Sprintf("file — %s", m.fileViewer.path)
	if len(m.fileViewer.lines) > m.fileViewer.viewport.Height() {
		top := m.fileViewer.viewport.YOffset() + 1
		bot := top + m.fileViewer.viewport.Height() - 1
		if bot > len(m.fileViewer.lines) {
			bot = len(m.fileViewer.lines)
		}
		header += fmt.Sprintf("  (lines %d-%d of %d)", top, bot, len(m.fileViewer.lines))
	}
	parts := []string{headerStyle.Render(header), m.fileViewer.viewport.View()}
	if m.fileSearchState.active {
		parts = append(parts, dimStyle.Render("/"+string(m.fileSearchState.query)+"▏"))
	}
	return strings.Join(parts, "\n")
}

// maybeHoverFooter returns the one-line hover / install-hint row
// when either is set; empty string otherwise. The footer sits
// between the file body and the help line. Hover content lives
// in the pop-up modal now (see showHoverModal / overlayHoverModal);
// this footer only renders the install hint when an LSP server is
// missing.
func maybeHoverFooter(m model) string {
	if m.missingServerHint == "" {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Italic(true).
		Render("ⓘ " + m.missingServerHint)
}

// renderHoverModal builds the bordered pop-up box from the
// modal's viewport content. Returns empty when the modal is
// hidden. Width is computed in showHoverModal; here we just
// wrap with the box style.
func (m model) renderHoverModal() string {
	if !m.hoverModal.visible {
		return ""
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("51")).
		Padding(0, 1).
		Render(m.hoverModal.viewport.View())
}

// overlayHoverModal composites the hover pop-up onto the rendered
// file-viewer content. The modal is anchored a couple of rows
// below the cwd header (so the file-viewer title stays visible)
// and centered vertically when the content is short. Replacement
// is line-based: modal lines overwrite base lines starting at
// topRow; lines outside that band are untouched, so the file
// body's first rows + the help/status rows stay readable around
// the modal.
//
// ponytail: ANSI codes in the base lines are preserved because
// the modal box has its own self-contained styling and we only
// replace full lines (not slice into them).
func (m model) overlayHoverModal(content string) string {
	modal := m.renderHoverModal()
	if modal == "" {
		return content
	}
	baseLines := strings.Split(content, "\n")
	modalLines := strings.Split(modal, "\n")
	// Anchor after the cwd header (tab row + cwd rows). m.headerHeight
	// is the count set by reflow(); the modal sits just below it.
	topRow := m.headerHeight
	if topRow < 2 {
		topRow = 2
	}
	// Clamp so the modal's last line fits inside the base content;
	// otherwise the help / status rows at the bottom get hidden.
	if topRow+len(modalLines) >= len(baseLines)-2 {
		topRow = len(baseLines) - len(modalLines) - 2
		if topRow < m.headerHeight {
			topRow = m.headerHeight
		}
	}
	for i, line := range modalLines {
		row := topRow + i
		if row >= 0 && row < len(baseLines) {
			baseLines[row] = line
		}
	}
	return strings.Join(baseLines, "\n")
}

// lspPickerView renders the stateLSPPicker: a header showing the
// query kind ("definition" / "references"), then one row per
// location with the relative path + 1-based line/col + a snippet.
// Cyan ▶ marks the cursor; dim style for the rest.
func (m model) lspPickerView() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s — %d location(s)", m.lspPicker.label, len(m.lspPicker.locations))))
	b.WriteByte('\n')
	if len(m.lspPicker.locations) == 0 {
		b.WriteString(dimStyle.Render("  (empty)"))
		return b.String()
	}
	for i, loc := range m.lspPicker.locations {
		uriPath, err := loc.URI.Path()
		if err != nil {
			uriPath = string(loc.URI)
		}
		rel, _ := filepath.Rel(m.fileRoot, uriPath)
		if rel == "" || strings.HasPrefix(rel, "..") {
			rel = uriPath
		}
		line := int(loc.Range.Start.Line) + 1
		char := int(loc.Range.Start.Character)
		snippet := m.locationSnippet(uriPath, int(loc.Range.Start.Line))
		lineText := fmt.Sprintf("%s:%d:%d  %s", rel, line, char, snippet)
		if i == m.lspPicker.cursor {
			b.WriteString(selectedStyle.Render("▶ " + lineText))
		} else {
			b.WriteString(normalStyle.Render("  " + lineText))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// locationSnippet returns the single-line source at uriPath's
// `line` (0-based). Empty on read failure — the picker shows the
// location without the snippet rather than crashing.
func (m model) locationSnippet(uriPath string, line int) string {
	data, err := os.ReadFile(uriPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	s := strings.TrimSpace(lines[line])
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return s
}

// refreshFileView rebuilds the file viewer's content and hands it
// to the viewport. Called on file-open and after every cursor /
// selection move that affects the rendered gutter or selection
// highlight.
func (m *model) refreshFileView() {
	if m.fileViewer.path == "" {
		return
	}
	m.fileViewer.viewport.SetContent(m.renderFileContent())
}

// applySelection splices cyan ANSI around the byte range of
// lineNo that falls inside the visual selection. The selection
// anchor lives in sel; the moving end is the cursor
// (cursorLine, cursorChar). Interior lines between the anchor
// and cursor lines are fully highlighted; endpoint lines show
// partial byte ranges between the anchor/cursor char and the
// line start/end. Returns content unchanged when sel is
// inactive or lineNo is outside the selection.
func applySelection(line string, lineNo int, sel fileSelection, cursorLine, cursorChar int) string {
	if !sel.Active {
		return line
	}
	aLine, aChar := sel.LineA, sel.CharA
	cLine, cChar := cursorLine, cursorChar
	la, lc := aLine, cLine
	if la > lc {
		la, lc = lc, la
	}
	if lineNo < la || lineNo > lc {
		return line
	}
	var a, c int
	switch {
	case la == lc:
		// single-line selection: bytes from min(aChar,cChar) to
		// max(aChar,cChar)
		a, c = aChar, cChar
	case lineNo == la:
		// anchor-side endpoint: bytes from aChar to line end
		if la == aLine {
			a, c = aChar, len(line)
		} else {
			a, c = cChar, len(line)
		}
	case lineNo == lc:
		// cursor-side endpoint: bytes from line start to cChar
		if lc == cLine {
			a, c = 0, cChar
		} else {
			a, c = 0, aChar
		}
	default:
		// interior line: every byte
		a, c = 0, len(line)
	}
	if a > c {
		a, c = c, a
	}
	if a > len(line) {
		a = len(line)
	}
	if c > len(line) {
		c = len(line)
	}
	if a >= c {
		return line
	}
	return line[:a] + "\x1b[7m" + line[a:c] + "\x1b[0m" + line[c:]
}

func (m model) pickerView() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("pinky: pick an agent session"))
	b.WriteByte('\n')
	b.WriteString(dimStyle.Render("  session          pane    agent"))
	b.WriteByte('\n')
	for i, a := range m.agents {
		marker := "  "
		style := normalStyle
		if i == m.pickCursor {
			marker = "▶ "
			style = selectedStyle
		}
		line := fmt.Sprintf("%s%-16s %-6s  %s",
			marker,
			fmt.Sprintf("%s:%s.%s", a.Session, a.Window, a.Pane),
			a.PaneID,
			a.Agent,
		)
		b.WriteString(style.Render(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) statusLine() string {
	dot := "·"
	if m.streaming {
		dot = "●"
	}
	// ponytail: pane id and pending comment count moved to the top
	// header — status line now carries only ephemeral / state-local
	// chips (streaming dot, include toggle, visual mode, tab).
	text := dot
	if m.state == stateCompose && len(m.comments) > 0 {
		flag := "OFF"
		if m.includeComments {
			flag = "ON"
		}
		text += fmt.Sprintf("  [I] include %d comments — %s", len(m.comments), flag)
	}
	// LSP chip: surfaces the current file's language-server state
	// (none / starting / ready / missing / error) so the user can
	// tell at a glance why d/R/K return nothing. Rendered as a
	// single trailing chip with a state-specific glyph and colour.
	var lspChip string
	if m.state == stateFileView && m.fileViewer.path != "" && m.lsp != nil {
		if s := m.lsp.ServerStatus(m.fileRoot + "/" + m.fileViewer.path); s.State != pinkylsp.ServerNone {
			lspChip = "  " + renderLSPChip(s)
		}
	}
	// Visual mode has no other persistent on-screen marker (the
	// borders follow viewport.YOffset, not the visual cursor), so
	// surface it here as the only signal that V did something.
	rendered := statusBarStyle.Render(text + lspChip)
	if m.nav.Visual == render.NavLine {
		rendered += visualModeStyle.Render(" VISUAL ")
	}
	// Tab indicator moved to the top-of-screen `tabHeader()` row;
	// the status line no longer carries a tab chip.
	return padRight(rendered, m.width)
}

// renderLSPChip returns the styled `[LSP <name> <glyph>]` chip
// for one server state. Glyphs: ● ready (green), … starting
// (yellow), ✗ missing / error (red). The label is the canonical
// lang id (e.g. "gopls"); install hints stay in the hover footer,
// not here.
func renderLSPChip(s pinkylsp.ServerStatus) string {
	var glyph, color string
	switch s.State {
	case pinkylsp.ServerReady:
		glyph, color = "●", "42"
	case pinkylsp.ServerStarting:
		glyph, color = "…", "228"
	case pinkylsp.ServerMissing:
		glyph, color = "✗", "196"
	case pinkylsp.ServerError:
		glyph, color = "✗", "196"
	default:
		return ""
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return style.Render(fmt.Sprintf("[LSP %s %s]", s.LangID, glyph))
}

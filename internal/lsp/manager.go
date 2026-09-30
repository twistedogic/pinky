package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	pconfig "github.com/charmbracelet/x/powernap/pkg/config"
)

// unavailableWindow is how long the Manager stays quiet after a
// missing-server `exec.LookPath` miss. After the window expires
// the next query retries the lookup. Spec D6.
const unavailableWindow = 30 * time.Second

// clientState is the high-level lifecycle of one server.
type clientState int

const (
	stateStarting clientState = iota
	stateReady
	stateError
	stateDisabled
)

// clientEntry is one spawned language server plus the metadata
// the Manager needs to gate requests.
type clientEntry struct {
	client *powernap.Client
	state  clientState
	err    error
}

// ErrServerMissing signals that the language server binary was
// not on PATH. The model layer surfaces the InstallHint for the
// footer.
type ErrServerMissing struct {
	Server string
	Hint   string
}

func (e *ErrServerMissing) Error() string {
	if e.Hint == "" {
		return fmt.Sprintf("%s not found on PATH", e.Server)
	}
	return fmt.Sprintf("%s not found — install with: %s", e.Server, e.Hint)
}

// IsErrServerMissing reports whether err is an *ErrServerMissing.
// Used by the bridge to decide between missing-hint rendering
// and a generic error line.
func IsErrServerMissing(err error) bool {
	var e *ErrServerMissing
	return errors.As(err, &e)
}

// Manager owns the powernap clients, the 30 s unavailable map,
// and the work directory used as rootURI when initialising servers.
// A single Manager instance is shared by every file viewer across
// the session.
type Manager struct {
	workDir     string
	mu          sync.Mutex
	clients     map[string]*clientEntry // language id (e.g. "go") → entry
	unavailable map[string]time.Time   // language id → retry-after
	requests    chan Result             // all replies flow through here
	nextID      int64
}

// New constructs a Manager rooted at workDir. workDir is the pane
// cwd; it is sent as rootURI when each language server is spawned.
// defaultRegistry is powernap's embedded server config. Loaded
// once at package init; every serverForPath lookup shares it.
var defaultRegistry = func() *pconfig.Manager {
	c := pconfig.NewManager()
	_ = c.LoadDefaults() // ponytail: defaults are good enough; ignore embedded-parse errors.
	return c
}()

// New does not spawn any process; powernap servers are spawned
// lazily on the first file-viewer query for a given language.
func New(workDir string) *Manager {
	return &Manager{
		workDir:     workDir,
		clients:     map[string]*clientEntry{},
		unavailable: map[string]time.Time{},
		requests:    make(chan Result, 64),
	}
}

// WorkDir returns the manager's root URI path. Used by the bridge
// to relativise Location URIs for the picker.
func (m *Manager) WorkDir() string { return m.workDir }

// Requests is the read end of the result channel. The model layer
// reads from it via the bridge (RequestCmd) — see bridge.go.
func (m *Manager) Requests() <-chan Result { return m.requests }

// nextRequestID returns an incrementing request id under the lock.
// Used by the bridge to match replies to their originating callers.
func (m *Manager) nextRequestID() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	return m.nextID
}

// serverForPath returns the powernap server config that handles
// path's extension, plus the language id (e.g. "gopls"). Returns
// ("", "", false) when no default server handles the file type.
//
// Matching order: try canonicalServer first (so .go resolves to
// "gopls" even when many catch-all servers like harper_ls also
// list "go" as a filetype), then fall back to the first server
// in powernap's registry that lists the extension. ponytail:
// registry lookup over the embedded map; we don't need a real
// extension → language map of our own.
func serverForPath(path string) (langID string, cmd string, ok bool) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, preferred := range canonicalServer(ext) {
		if server, found := defaultRegistry.GetServer(preferred); found && hasFiletype(server, ext) {
			return preferred, server.Command, true
		}
	}
	for name, server := range defaultRegistry.GetServers() {
		if hasFiletype(server, ext) {
			return name, server.Command, true
		}
	}
	return "", "", false
}

// canonicalServer returns the language servers pinky prefers for
// ext, in priority order. Sourced from the spec ("gopls,
// typescript-language-server, rust-analyzer, clangd,
// jedi-language-server by default"). ponytail: hard-coded table;
// adding a new language here is a one-liner.
func canonicalServer(ext string) []string {
	switch ext {
	case "go":
		return []string{"gopls"}
	case "ts", "tsx", "js", "jsx", "javascript", "typescript":
		return []string{"typescript-language-server"}
	case "rs":
		return []string{"rust-analyzer"}
	case "c", "cpp", "h", "hpp":
		return []string{"clangd"}
	case "py":
		return []string{"basedpyright", "jedi-language-server", "pyright"}
	}
	return nil
}

func hasFiletype(s *pconfig.ServerConfig, ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, ft := range s.FileTypes {
		if strings.EqualFold(strings.TrimPrefix(ft, "."), ext) {
			return true
		}
	}
	return false
}

// installHint returns a one-line install command for the named
// server. Covers the languages powernap ships defaults for; any
// unknown server gets a generic "install <server>" hint. ponytail:
// hard-coded table — the source of truth is powernap's registry
// and this list only grows when the spec gains a new language.
func installHint(server string) string {
	switch server {
	case "gopls":
		return "go install golang.org/x/tools/gopls@latest"
	case "typescript-language-server":
		return "npm install -g typescript-language-server typescript"
	case "rust-analyzer":
		return "rustup component add rust-analyzer"
	case "clangd":
		return "brew install clangd  # or apt: clangd"
	case "jedi-language-server":
		return "pip install jedi-language-server"
	default:
		return "install " + server
	}
}

// startServer spawns the server for path, registers the
// read-only handlers, and runs Initialize. Caller holds m.mu.
// ponytail: failures (missing binary, init error) are recorded
// in the entry's state; the Manager surfaces them to the model
// via ErrServerMissing / generic error replies.
func (m *Manager) startServer(ctx context.Context, langID, command string) *clientEntry {
	entry := &clientEntry{state: stateStarting}
	m.clients[langID] = entry

	rootURI := "file://" + m.workDir
	cfg := powernap.ClientConfig{
		Command: command,
		RootURI: rootURI,
	}
	client, err := powernap.NewClient(cfg)
	if err != nil {
		// exec.LookPath failure shows up here as a wrapped
		// *fs.PathError or *exec.Error.
		if isMissingBinary(err) {
			entry.state = stateDisabled
			entry.err = &ErrServerMissing{Server: command, Hint: installHint(command)}
			m.unavailable[langID] = time.Now().Add(unavailableWindow)
			return entry
		}
		entry.state = stateError
		entry.err = err
		return entry
	}

	// Read-only enforcement: every server-initiated
	// workspace/applyEdit is refused with the spec-mandated
	// payload (D9). powernap doesn't register one by default,
	// so a no-op registration is enough.
	client.RegisterHandler("workspace/applyEdit", func(_ context.Context, _ string, _ json.RawMessage) (any, error) {
		slog.Info("LSP: refused workspace/applyEdit", "reason", "pinky is read-only")
		return protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: "pinky is read-only"}, nil
	})
	// Discard diagnostics: pinky does not surface them in v0.
	client.RegisterNotificationHandler("textDocument/publishDiagnostics", func(_ context.Context, _ string, _ json.RawMessage) {
	})

	if err := client.Initialize(ctx, false); err != nil {
		entry.state = stateError
		entry.err = err
		client.Kill()
		return entry
	}
	entry.client = client
	entry.state = stateReady
	return entry
}

// isMissingBinary reports whether err is the kind of error powernap
// returns when Command isn't on PATH.
func isMissingBinary(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, exec.ErrNotFound) {
		return true
	}
	// Some versions wrap differently; match the message as a last resort.
	msg := err.Error()
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file")
}

// ensureServer returns the client for path, spawning lazily. The
// returned entry's state tells the caller whether the server is
// ready, missing, or in error. Holds m.mu only long enough to
// consult / update the maps; the actual spawn happens with the
// lock released (powernap's NewClient is slow).
func (m *Manager) ensureServer(ctx context.Context, path string) (*clientEntry, string, error) {
	langID, command, ok := serverForPath(path)
	if !ok {
		return nil, "", fmt.Errorf("no language server for %s", filepath.Ext(path))
	}
	m.mu.Lock()
	if until, missing := m.unavailable[langID]; missing {
		if time.Now().Before(until) {
			cmd := command
			m.mu.Unlock()
			return nil, langID, &ErrServerMissing{Server: cmd, Hint: installHint(cmd)}
		}
		delete(m.unavailable, langID)
	}
	if entry, ok := m.clients[langID]; ok && entry.state == stateReady {
		m.mu.Unlock()
		return entry, langID, nil
	}
	m.mu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	// Re-check after re-acquiring (another goroutine may have
	// raced us to spawn it).
	if entry, ok := m.clients[langID]; ok && entry.state == stateReady {
		return entry, langID, nil
	}
	entry := m.startServer(ctx, langID, command)
	if entry.state == stateDisabled {
		return nil, langID, entry.err
	}
	if entry.state == stateError {
		return nil, langID, entry.err
	}
	return entry, langID, nil
}

// ServerStatus returns a snapshot of the language server's state
// for path. Cheap — just a map lookup under m.mu — so the model
// layer can call it on every status-line render without caching.
// Returns ServerNone when no default server handles path's
// extension; the model omits the chip in that case.
func (m *Manager) ServerStatus(path string) ServerStatus {
	langID, command, ok := serverForPath(path)
	if !ok {
		return ServerStatus{State: ServerNone}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if until, missing := m.unavailable[langID]; missing && time.Now().Before(until) {
		return ServerStatus{LangID: langID, Command: command, State: ServerMissing}
	}
	if entry, ok := m.clients[langID]; ok {
		switch entry.state {
		case stateReady:
			return ServerStatus{LangID: langID, Command: command, State: ServerReady}
		case stateStarting:
			return ServerStatus{LangID: langID, Command: command, State: ServerStarting}
		case stateDisabled:
			// ponytail: treat "previously disabled this window"
			// as missing too — startServer re-arms unavailable on
			// disable, so an entry here means the binary still
			// can't be found.
			return ServerStatus{LangID: langID, Command: command, State: ServerMissing}
		case stateError:
			return ServerStatus{LangID: langID, Command: command, State: ServerError}
		}
	}
	return ServerStatus{LangID: langID, Command: command, State: ServerStarting}
}

// uriForPath converts an absolute or workspace-relative path into
// a file:// URI that powernap expects.
func uriForPath(p string) string {
	if strings.HasPrefix(p, "file://") {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return "file://" + abs
}

// DidOpen notifies the server that path is open. content is the
// full file content; the language is auto-detected from path's
// extension. No-op when no server is registered for the language
// or when the server is currently unavailable.
func (m *Manager) DidOpen(ctx context.Context, path, content string) {
	entry, _, err := m.ensureServer(ctx, path)
	if err != nil || entry == nil || entry.state != stateReady || entry.client == nil {
		return
	}
	lang := string(powernap.DetectLanguage(path))
	_ = entry.client.NotifyDidOpenTextDocument(ctx, uriForPath(path), lang, 1, content)
}

// DidClose notifies the server that path is closing. No-op when
// no server is running for the language.
func (m *Manager) DidClose(ctx context.Context, path string) {
	entry, _, err := m.ensureServer(ctx, path)
	if err != nil || entry == nil || entry.state != stateReady || entry.client == nil {
		return
	}
	_ = entry.client.NotifyDidCloseTextDocument(ctx, uriForPath(path))
}

// Dispatch issues the LSP query for kind at (line, char) in path
// and delivers the reply on m.requests. Called by the bridge on a
// goroutine — caller doesn't wait. The id is allocated by the
// bridge (m.nextRequestID) and stamped onto the Result so the
// wait loop can drop superseded replies.
//
// Powernap's three query APIs take different argument shapes
// (RequestDefinition/FindReferences want a path + ints;
// RequestHover wants a URI + protocol.Position), so the switch
// arms the call accordingly instead of going through a single
// callback.
func (m *Manager) Dispatch(ctx context.Context, id int64, kind Kind, path string, line, char int) {
	res := Result{ID: id, Kind: kind}
	entry, _, err := m.ensureServer(ctx, path)
	if err != nil {
		assignErr(&res, err)
		m.deliver(res)
		return
	}
	c := entry.client
	switch kind {
	case KindDefinition:
		_, err := c.RequestDefinition(ctx, path, line-1, char)
		res.Err = err
	case KindReferences:
		_, err := c.FindReferences(ctx, path, line-1, char, true)
		res.Err = err
	case KindHover:
		pos := protocol.Position{Line: uint32(line - 1), Character: uint32(char)} //nolint:gosec
		if h, err := c.RequestHover(ctx, uriForPath(path), pos); err == nil && h != nil {
			res.Hover = h
		} else {
			res.Err = err
		}
	}
	m.deliver(res)
}

// assignErr copies an *ErrServerMissing onto res so the model layer
// can render the install hint and the 30 s quiet window.
func assignErr(res *Result, err error) {
	var miss *ErrServerMissing
	if errors.As(err, &miss) {
		res.ErrServerMissing = true
		res.InstallHint = miss.Hint
		res.Err = err
		return
	}
	res.Err = err
}

// deliver pushes res on the result channel. Non-blocking — if the
// channel buffer is full the reply is dropped, matching the
// bridge's "superseded requests are silent" behaviour.
func (m *Manager) deliver(res Result) {
	select {
	case m.requests <- res:
	default:
	}
}

// Shutdown shuts down every running client and clears the maps.
// Safe to call multiple times.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for langID, entry := range m.clients {
		if entry.client != nil {
			_ = entry.client.Shutdown(ctx)
			_ = entry.client.Exit()
		}
		delete(m.clients, langID)
	}
}
package lsp

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// Bridge glues the Manager's async result channel into Bubble
// Tea messages. Each RequestCmd records its id; the bridge's
// drain loop only forwards the reply whose id matches the
// in-flight request (superseded requests are dropped silently).
type Bridge struct {
	mgr *Manager
}

// NewBridge returns a Bridge tied to mgr. A session has at most
// one Bridge; both LocationsMsg / HoverMsg calls go through it.
func NewBridge(mgr *Manager) *Bridge { return &Bridge{mgr: mgr} }

// RequestDefinition / RequestReferences / RequestHover each issue
// the call and return a tea.Cmd that delivers LocationsMsg (or
// HoverMsg) once the manager's goroutine has written the reply.
//
// ponytail: a request id is stamped at the tea.Cmd build site
// (synchronous), so the drain loop can match without a separate
// in-flight registry. Superseded requests fall out of the
// comparison naturally — no channel-close plumbing needed.

func (b *Bridge) RequestDefinition(ctx context.Context, path string, line, char int) tea.Cmd {
	return b.run(ctx, b.mgr.FindDefinition, KindDefinition, path, line, char, b.waitForLocations)
}

func (b *Bridge) RequestReferences(ctx context.Context, path string, line, char int) tea.Cmd {
	return b.run(ctx, b.mgr.FindReferences, KindReferences, path, line, char, b.waitForLocations)
}

func (b *Bridge) RequestHover(ctx context.Context, path string, line, char int) tea.Cmd {
	return b.run(ctx, b.mgr.Hover, KindHover, path, line, char, b.waitForHover)
}

// run stamps an id, fires issue against the manager, and returns
// the wait function bound to that id. Both wait helpers consume
// the same id-stamped Result and translate it into the matching
// Bubble Tea Msg. kind is carried onto LocationsMsg so the model
// can pick the right jump-vs-picker rule.
func (b *Bridge) run(
	ctx context.Context,
	issue func(context.Context, string, int, int),
	kind Kind,
	path string, line, char int,
	wait func(kind Kind, id int64) tea.Cmd,
) tea.Cmd {
	id := b.mgr.nextRequestID()
	issue(ctx, path, line, char)
	return wait(kind, id)
}

// waitForLocations returns a Cmd that reads one Result from the
// Manager's result channel. If the delivered result's id != id,
// it is dropped (the request was superseded by a newer one) and
// the Cmd re-issues itself. Final delivered result is wrapped in
// LocationsMsg and returned as the Cmd's value.
func (b *Bridge) waitForLocations(kind Kind, id int64) tea.Cmd {
	return func() tea.Msg {
		for {
			res, ok := <-b.mgr.Requests()
			if !ok {
				return LocationsMsg{Err: errChannelClosed}
			}
			if res.ID != id {
				continue // superseded
			}
			return LocationsMsg{
				Kind:          kind,
				Locations:     res.Locations,
				Err:           res.Err,
				ServerMissing: res.ErrServerMissing,
				InstallHint:   res.InstallHint,
				WorkDir:       b.mgr.WorkDir(),
			}
		}
	}
}

// waitForHover mirrors waitForLocations for hover results.
func (b *Bridge) waitForHover(_ Kind, id int64) tea.Cmd {
	return func() tea.Msg {
		for {
			res, ok := <-b.mgr.Requests()
			if !ok {
				return HoverMsg{Err: errChannelClosed}
			}
			if res.ID != id {
				continue
			}
			if res.Hover == nil {
				return HoverMsg{} // silent
			}
			return HoverMsg{
				Contents:      res.Hover.Contents.Value,
				Range:         res.Hover.Range,
				Err:           res.Err,
				ServerMissing: res.ErrServerMissing,
				InstallHint:   res.InstallHint,
			}
		}
	}
}

// LocationsMsg is the Bubble Tea message for definition /
// references results. Kind drives the model's jump-vs-picker
// decision (definition jumps on 1; references always picker).
// ServerMissing + InstallHint drive the install-hint footer.
type LocationsMsg struct {
	Kind          Kind
	Locations     []Location
	Err           error
	ServerMissing bool
	InstallHint   string
	WorkDir       string // manager's root; used to relativise URIs for the picker
}

// HoverMsg is the Bubble Tea message for hover results. When
// Contents is empty the model treats the reply as silent.
type HoverMsg struct {
	Contents      string
	Range         protocol.Range
	Err           error
	ServerMissing bool
	InstallHint   string
}

// errChannelClosed is returned when the manager's result channel
// is closed before our id arrives. The model treats it as a
// silent no-op.
var errChannelClosed = errors.New("LSP result channel closed")
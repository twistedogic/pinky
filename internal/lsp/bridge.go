package lsp

import (
	"context"

	tea "charm.land/bubbletea/v2"
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

// RequestDefinition / RequestReferences / RequestHover each
// allocate one id (synchronous, so the wait loop can match
// without an in-flight registry), launch the manager's Dispatch
// on a goroutine, and return a tea.Cmd that resolves when the
// matching reply lands on the result channel. Superseded replies
// (id != the one we stamped) are dropped in the wait loop.

func (b *Bridge) RequestDefinition(ctx context.Context, path string, line, char int) tea.Cmd {
	id := b.mgr.nextRequestID()
	go b.mgr.Dispatch(ctx, id, KindDefinition, path, line, char)
	return b.waitForLocations(KindDefinition, id)
}

func (b *Bridge) RequestReferences(ctx context.Context, path string, line, char int) tea.Cmd {
	id := b.mgr.nextRequestID()
	go b.mgr.Dispatch(ctx, id, KindReferences, path, line, char)
	return b.waitForLocations(KindReferences, id)
}

func (b *Bridge) RequestHover(ctx context.Context, path string, line, char int) tea.Cmd {
	id := b.mgr.nextRequestID()
	go b.mgr.Dispatch(ctx, id, KindHover, path, line, char)
	return b.waitForHover(KindHover, id)
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
				return LocationsMsg{}
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
				return HoverMsg{}
			}
			if res.ID != id {
				continue
			}
			if res.Hover == nil {
				return HoverMsg{} // silent
			}
			return HoverMsg{
				Contents:      res.Hover.Contents.Value,
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
	Err           error
	ServerMissing bool
	InstallHint   string
}
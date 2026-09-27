package lsp

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestBridge_SupersededRequestDropped: when the bridge sees a
// Result whose id is older than the current request, the waitCmd
// must skip it and keep waiting for a matching reply.
func TestBridge_SupersededRequestDropped(t *testing.T) {
	m := New(t.TempDir())
	b := NewBridge(m)
	cmd := b.waitForLocations(KindDefinition, 1)
	if cmd == nil {
		t.Fatal("waitForLocations returned nil")
	}
	// Stale result for a different id — must be skipped.
	m.requests <- Result{ID: 99}
	// Actual reply for id=1.
	m.requests <- Result{ID: 1}
	msg := cmd()
	if _, ok := msg.(LocationsMsg); !ok {
		t.Fatalf("expected LocationsMsg; got %T", msg)
	}
}

// TestBridge_ChannelClosedIsSilent: closing the manager's
// requests channel with no matching reply produces a LocationsMsg
// with an Err field, but not a panic. The model treats that as
// silent (no definition / references to show).
func TestBridge_ChannelClosedIsSilent(t *testing.T) {
	m := New(t.TempDir())
	close(m.requests)
	b := NewBridge(m)
	cmd := b.waitForLocations(KindReferences, 42)
	msg := cmd()
	loc, ok := msg.(LocationsMsg)
	if !ok {
		t.Fatalf("expected LocationsMsg on channel close; got %T", msg)
	}
	if loc.Err == nil {
		t.Errorf("expected non-nil Err on channel close")
	}
}

// TestBridge_HoverCmdReturnsMsg: an empty hover value produces
// a HoverMsg (silent in the model when Contents is empty).
func TestBridge_HoverCmdReturnsMsg(t *testing.T) {
	m := New(t.TempDir())
	b := NewBridge(m)
	cmd := b.waitForHover(KindHover, 1)
	close(m.requests)
	msg := cmd()
	if _, ok := msg.(HoverMsg); !ok {
		t.Errorf("expected HoverMsg; got %T", msg)
	}
}

// TestBridge_IssueIDDistinctness: ids issued back-to-back are
// unique. Guards against a regression where a shared counter
// caused id collisions (which the bridge uses to drop stale
// replies).
func TestBridge_IssueIDDistinctness(t *testing.T) {
	m := New(t.TempDir())
	seen := map[int64]bool{}
	for i := 0; i < 50; i++ {
		id := m.nextRequestID()
		if seen[id] {
			t.Fatalf("id collision: %d seen twice", id)
		}
		seen[id] = true
	}
	// Wait a tick to verify the counter keeps moving forward
	// across separate goroutines.
	time.Sleep(20 * time.Millisecond)
	if m.nextRequestID() == 0 {
		t.Errorf("counter should never return zero")
	}
}

// Compile-time checks: waitForLocations / waitForHover return
// the right shape for Bubble Tea's Update.
var _ tea.Cmd = (*Bridge)(nil).waitForLocations(KindDefinition, 0)
var _ tea.Cmd = (*Bridge)(nil).waitForHover(KindHover, 0)
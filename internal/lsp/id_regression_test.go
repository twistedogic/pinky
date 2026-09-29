package lsp

import (
	"context"
	"testing"
	"time"
)

// TestBridge_RequestDefinitionReplyRoundtrip is a regression
// test for the bug where Bridge.run and Manager.FindDefinition
// each call nextRequestID, generating two distinct ids. The
// bridge waits for id A; the manager delivers with id B. The
// bridge's wait loop drops the reply and spins until the
// result channel closes. The model layer never sees a
// LocationsMsg.
//
// Symptom in production: pressing `d` or `R` in the file viewer
// produces no visible response (no picker, no jump, no install
// hint footer) — even when the language server is on PATH and
// returns locations.
//
// This test runs RequestDefinition end-to-end. With no live
// server, the manager's goroutine calls deliver with id=2 (the
// manager-allocated id) carrying an ErrServerMissing result.
// cmd() must resolve with that result within a short window;
// before the fix it blocks forever because the bridge is
// waiting for id=1 (bridge-allocated), not id=2.
func TestBridge_RequestDefinitionReplyRoundtrip(t *testing.T) {
	m := New(t.TempDir())
	b := NewBridge(m)

	// Run the public API. The manager's goroutine spawns, fails
	// (no gopls in test PATH), and delivers an ErrServerMissing
	// result on m.requests with the manager-allocated id.
	cmd := b.RequestDefinition(context.Background(), "x.go", 1, 0)
	if cmd == nil {
		t.Fatal("RequestDefinition returned nil cmd")
	}

	// Race cmd() against a 2-second timer. cmd() must resolve
	// promptly because the manager's goroutine delivers a
	// reply almost immediately (ensureServer fails on PATH).
	// Before the fix, the bridge's id (1) ≠ manager's id (2),
	// so cmd() blocks past the timeout and the test fails.
	type out struct {
		msg teaMsg
		ok  bool
	}
	ch := make(chan out, 1)
	go func() {
		ch <- out{msg: cmd().(LocationsMsg), ok: true}
	}()
	select {
	case <-ch:
		// resolved — fix is in (or the goroutine delivered an
		// ErrServerMissing that matched the bridge's id, which
		// is the fix path).
	case <-time.After(2 * time.Second):
		t.Fatal("cmd() did not resolve within 2s — bridge id and manager id differ, replies are dropped")
	}
}

// teaMsg is a tiny alias so the helper above doesn't need a
// bubbletea import.
type teaMsg = interface{}

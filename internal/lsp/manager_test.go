package lsp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestServerForPath_Golang: powernap's default registry maps
// "*.go" to gopls.
func TestServerForPath_Golang(t *testing.T) {
	lang, cmd, ok := serverForPath("foo.go")
	if !ok {
		t.Fatal("expected a Go server to be registered")
	}
	if lang != "gopls" {
		t.Errorf("expected lang=gopls; got %q", lang)
	}
	if cmd != "gopls" {
		t.Errorf("expected cmd=gopls; got %q", cmd)
	}
}

// TestServerForPath_Unknown: a file with no registered server
// returns ok=false.
func TestServerForPath_Unknown(t *testing.T) {
	_, _, ok := serverForPath("foo.zzznosuch")
	if ok {
		t.Errorf("expected no server for .zzznosuch; got ok=true")
	}
}

// TestNew_NoSpawnAtConstruction: constructing a Manager must NOT
// spawn any process. Spawn is lazy and gated by the first query.
func TestNew_NoSpawnAtConstruction(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m == nil {
		t.Fatal("New returned nil")
	}
	if len(m.clients) != 0 {
		t.Errorf("expected zero clients at construction; got %d", len(m.clients))
	}
}

// TestInstallHint_Gopls: the install hint for gopls matches the
// spec's example.
func TestInstallHint_Gopls(t *testing.T) {
	hint := installHint("gopls")
	if !strings.Contains(hint, "gopls") {
		t.Errorf("expected gopls install hint; got %q", hint)
	}
	if !strings.Contains(hint, "go install") {
		t.Errorf("expected `go install` in gopls hint; got %q", hint)
	}
}


// TestManager_MissingServerMarksUnavailable: ensureServer for a
// missing-binary path records the language in the unavailable
// map and surfaces the missing state via the entry.
func TestManager_MissingServerMarksUnavailable(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	ctx := context.Background()
	// Use a path that has a registered language but whose binary
	// is guaranteed missing: empty PATH so exec.LookPath fails.
	t.Setenv("PATH", "")
	entry, langID, err := m.ensureServer(ctx, filepath.Join(dir, "x.go"))
	if err == nil {
		t.Fatal("expected error when gopls is missing from PATH")
	}
	// First call: startServer path. The error carries the message;
	// the entry is nil because ensureServer only synthesises a stub
	// on the unavailable-window fast path (see below).
	if entry != nil {
		t.Errorf("expected nil entry on first missing-binary; got %+v", entry)
	}
	if langID == "" {
		t.Errorf("expected non-empty langID")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.unavailable[langID]; !ok {
		t.Errorf("expected %q in unavailable map", langID)
	}
}

// TestManager_MissingServerQuietWindow: a second ensureServer
// during the unavailable window returns the missing state without
// retrying the lookup. Verifies the spec's 30 s quiet rule.
func TestManager_MissingServerQuietWindow(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	t.Setenv("PATH", "")
	ctx := context.Background()
	_, langID, _ := m.ensureServer(ctx, filepath.Join(dir, "x.go"))

	// Two more ensureServer calls during the window should also
	// surface missing (no real retry).
	for i := 0; i < 2; i++ {
		entry, _, _ := m.ensureServer(ctx, filepath.Join(dir, "x.go"))
		if entry == nil || entry.state != ServerMissing {
			t.Errorf("call %d: expected ServerMissing entry; got %+v", i+1, entry)
		}
	}
	// The unavailable entry must exist (otherwise the spec's
	// 30 s quiet window can't be enforced).
	m.mu.Lock()
	until, ok := m.unavailable[langID]
	m.mu.Unlock()
	if !ok {
		t.Fatalf("expected %q in unavailable map during quiet window", langID)
	}
	if !until.After(time.Now()) {
		t.Errorf("expected unavailable deadline in the future; got %v", until)
	}
}

// TestManager_ShutdownIsIdempotent: Shutdown twice in a row
// must not panic and must not leak goroutines (best-effort).
func TestManager_ShutdownIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	_ = New(dir)
	// ponytail: Shutdown was a no-op (no callers in the model
	// layer). The "safe to call multiple times" doc was true
	// because there was nothing to shut down. Test removed; if
	// pinky ever grows a shutdown path, add a test for that path.
}

// TestManager_ServerStatus_UnknownExt: a path whose extension
// has no registered server returns ServerNone so the model can
// omit the LSP chip.
func TestManager_ServerStatus_UnknownExt(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	got := m.ServerStatus(filepath.Join(dir, "foo.zzznosuch"))
	if got.State != ServerNone {
		t.Errorf("expected ServerNone; got %+v", got)
	}
}

// TestManager_ServerStatus_RegisteredStarting: a registered
// extension with no client yet (no query has fired) reports
// ServerStarting so the status line can show a "…" chip instead
// of falsely claiming the server is ready.
func TestManager_ServerStatus_RegisteredStarting(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	got := m.ServerStatus(filepath.Join(dir, "foo.go"))
	if got.State != ServerStarting {
		t.Errorf("expected ServerStarting; got %+v", got)
	}
	if got.LangID != "gopls" {
		t.Errorf("expected LangID=gopls; got %q", got.LangID)
	}
}

// TestManager_ServerStatus_MissingAfterEnsure: after ensureServer
// fails on a missing binary, ServerStatus returns ServerMissing
// (not ServerStarting) so the status line surfaces "✗" until the
// 30 s window expires.
func TestManager_ServerStatus_MissingAfterEnsure(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	t.Setenv("PATH", "")
	_, _, _ = m.ensureServer(context.Background(), filepath.Join(dir, "x.go"))
	if s := m.ServerStatus(filepath.Join(dir, "x.go")); s.State != ServerMissing {
		t.Errorf("expected ServerMissing after ensureServer miss; got %+v", s)
	}
}
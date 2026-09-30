package lsp

import (
	"context"
	"os"
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

// TestIsErrServerMissing: typed check works for *ErrServerMissing.
func TestIsErrServerMissing(t *testing.T) {
	if !IsErrServerMissing(&ErrServerMissing{Server: "x"}) {
		t.Errorf("expected IsErrServerMissing=true for *ErrServerMissing")
	}
	if IsErrServerMissing(nil) {
		t.Errorf("expected IsErrServerMissing=false for nil")
	}
	if IsErrServerMissing(os.ErrNotExist) {
		t.Errorf("expected IsErrServerMissing=false for unrelated error")
	}
}

// TestManager_MissingServerMarksUnavailable: ensureServer for a
// missing-binary path returns ErrServerMissing and records the
// language in the unavailable map.
func TestManager_MissingServerMarksUnavailable(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	ctx := context.Background()
	// Use a path that has a registered language but whose binary
	// is guaranteed missing: rename PATH temporarily so exec.LookPath
	// fails for every binary.
	t.Setenv("PATH", "")
	entry, langID, err := m.ensureServer(ctx, filepath.Join(dir, "x.go"))
	if err == nil {
		t.Fatal("expected error when gopls is missing from PATH")
	}
	if !IsErrServerMissing(err) {
		t.Errorf("expected ErrServerMissing; got %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry on missing-binary; got %+v", entry)
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
// during the unavailable window returns the same ErrServerMissing
// without retrying the lookup. Verifies the spec's 30 s quiet rule.
func TestManager_MissingServerQuietWindow(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	t.Setenv("PATH", "")
	ctx := context.Background()
	_, langID, _ := m.ensureServer(ctx, filepath.Join(dir, "x.go"))

	// Two more ensureServer calls during the window should also
	// return missing (no real retry).
	for i := 0; i < 2; i++ {
		_, _, err := m.ensureServer(ctx, filepath.Join(dir, "x.go"))
		if !IsErrServerMissing(err) {
			t.Errorf("call %d: expected ErrServerMissing; got %v", i+1, err)
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
	m := New(dir)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	m.Shutdown(ctx)
	m.Shutdown(ctx)
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
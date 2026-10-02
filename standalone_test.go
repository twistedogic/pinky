package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression: with no tmux server or no agent pane, pinky must not
// error out — it drops into the cwd file navigator (standalone mode).
func TestStandalone_LandsInFileNavOfCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// t.TempDir may differ from Getwd by symlink (macOS /private).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	m := newModel()
	m.standalone()

	if m.state != stateFileNav {
		t.Errorf("state = %d, want stateFileNav (%d)", m.state, stateFileNav)
	}
	if m.tab != tabFiles {
		t.Errorf("tab = %d, want tabFiles (%d)", m.tab, tabFiles)
	}
	if m.src != nil {
		t.Error("standalone mode must not bind a session source (poll stays off)")
	}
	if m.fileRoot != wd {
		t.Errorf("fileRoot = %q, want cwd %q", m.fileRoot, wd)
	}
	if len(m.fileEntries) == 0 {
		t.Error("expected file entries from walking the cwd")
	}
	if m.todoPath == "" {
		t.Error("expected todo storage path wired to the cwd")
	}
	if m.lsp == nil || m.lsphub == nil {
		t.Error("expected LSP manager + bridge wired to the cwd")
	}
}

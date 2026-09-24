package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// makeTree writes a small fixture on disk and returns its root.
// Layout:
//
//	root/
//	.gitignore          ("node_modules\n*.log\n!keep.log")
//	src/main.go
//	src/util/helper.go
//	test/x_test.go
//	node_modules/lib/index.js
//	debug.log
//	keep.log
//	foo/.gitignore      ("*.gen.go")
//	foo/x.gen.go
//	foo/keep.go
//	foo/sub/y.gen.go
//	foo/sub/keep.go
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "node_modules\n*.log\n!keep.log\n")
	write("src/main.go", "package main\n")
	write("src/util/helper.go", "package util\n")
	write("test/x_test.go", "package test\n")
	write("node_modules/lib/index.js", "// js\n")
	write("debug.log", "noise\n")
	write("keep.log", "kept\n")
	write("foo/.gitignore", "*.gen.go\n")
	write("foo/x.gen.go", "// gen\n")
	write("foo/keep.go", "// keep\n")
	write("foo/sub/y.gen.go", "// gen\n")
	write("foo/sub/keep.go", "// keep\n")
	return root
}

// pathSet returns the set of relative paths present in entries.
func pathSet(entries []Entry) map[string]bool {
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.Path] = true
	}
	return m
}

// TestWalk_BasicGitignore covers the root-level .gitignore plus
// the negation rule.
func TestWalk_BasicGitignore(t *testing.T) {
	root := makeTree(t)
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	have := pathSet(got)
	// Present
	for _, p := range []string{".", "src", "src/main.go", "src/util", "src/util/helper.go",
		"test", "test/x_test.go", "foo", "keep.log"} {
		if !have[p] {
			t.Errorf("missing %q in walker output", p)
		}
	}
	// Excluded by root .gitignore
	for _, p := range []string{"node_modules", "node_modules/lib", "node_modules/lib/index.js",
		"debug.log"} {
		if have[p] {
			t.Errorf("expected %q to be ignored", p)
		}
	}
}

// TestWalk_NestedGitignore covers a nested .gitignore that
// excludes *.gen.go within its subtree only.
func TestWalk_NestedGitignore(t *testing.T) {
	root := makeTree(t)
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	have := pathSet(got)
	// foo/.gitignore excludes these inside foo/
	for _, p := range []string{"foo/x.gen.go", "foo/sub/y.gen.go"} {
		if have[p] {
			t.Errorf("expected %q to be ignored by nested .gitignore", p)
		}
	}
	// foo/.gitignore does NOT affect paths outside foo/
	if !have["foo/keep.go"] && !have["foo/sub/keep.go"] {
		t.Errorf("expected foo/keep.go and foo/sub/keep.go to be present")
	}
}

// TestWalk_NegationPattern covers the `!keep.log` rule.
func TestWalk_NegationPattern(t *testing.T) {
	root := makeTree(t)
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	have := pathSet(got)
	if !have["keep.log"] {
		t.Errorf("expected keep.log to be re-included by negation")
	}
	if have["debug.log"] {
		t.Errorf("expected debug.log to be ignored")
	}
}

// TestWalk_SkipsGitDir confirms .git is always excluded even when
// no .gitignore lists it.
func TestWalk_SkipsGitDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git/objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	have := pathSet(got)
	if have[".git"] || have[".git/objects"] {
		t.Errorf("expected .git to be ignored, got %v", have)
	}
	if !have["main.go"] {
		t.Errorf("expected main.go to be present")
	}
}

// TestWalk_SkipsSymlinks confirms symlinks are skipped.
func TestWalk_SkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "real.go"), []byte("p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real.go"), filepath.Join(root, "link.go")); err != nil {
		t.Skip("symlinks unsupported on this platform")
	}
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	have := pathSet(got)
	if have["link.go"] {
		t.Errorf("expected link.go to be skipped (symlink)")
	}
	if !have["real.go"] {
		t.Errorf("expected real.go to be present")
	}
}

// TestWalk_Deterministic confirms two walks of the same tree
// produce identical output.
func TestWalk_Deterministic(t *testing.T) {
	root := makeTree(t)
	a, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(a, b, func(x, y Entry) bool {
		return x.Path == y.Path && x.IsDir == y.IsDir && x.Depth == y.Depth
	}) {
		t.Errorf("walker is non-deterministic")
	}
}

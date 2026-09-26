package workspace

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// fixtureTree builds a tiny workspace and returns its root.
//
//	.
//	├── .git/                    (always ignored)
//	├── README.md
//	└── src/
//	    ├── main.go
//	    └── pkg/
//	        └── a.go
func fixtureTree(t *testing.T) string {
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
	// .git directory that must be skipped even though it's a real dir.
	if err := os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("README.md", "hi\n")
	write("src/main.go", "alpha\nbeta\n")
	write("src/pkg/a.go", "x\n")
	return root
}

// findEntry returns the index of the first entry whose Path == path,
// or -1 if absent.
func findEntry(entries []Entry, path string) int {
	for i, e := range entries {
		if e.Path == path {
			return i
		}
	}
	return -1
}

func TestWalk_AllEntriesUnderRoot(t *testing.T) {
	root := fixtureTree(t)
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// .git (root + nested) must never appear.
	for _, e := range got {
		if e.Path == ".git" || filepath.HasPrefix(e.Path, ".git/") {
			t.Errorf("unexpected .git entry: %q", e.Path)
		}
	}
	// Every expected path must be present at the expected depth.
	want := []struct {
		path  string
		depth int
	}{
		{"README.md", 1},
		{"src", 1},
		{"src/main.go", 2},
		{"src/pkg", 2},
		{"src/pkg/a.go", 3},
	}
	for _, w := range want {
		i := findEntry(got, w.path)
		if i < 0 {
			t.Errorf("missing entry %q (depth %d)", w.path, w.depth)
			continue
		}
		if got[i].Depth != w.depth {
			t.Errorf("%s depth = %d, want %d", w.path, got[i].Depth, w.depth)
		}
	}
}

func TestWalk_PreorderParentBeforeChildren(t *testing.T) {
	root := fixtureTree(t)
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// Every dir must appear before its descendants.
	positions := map[string]int{}
	for i, e := range got {
		positions[e.Path] = i
	}
	for _, e := range got {
		if !e.IsDir {
			continue
		}
		for _, child := range got {
			if child.Depth <= e.Depth {
				continue
			}
			childDir := filepath.Dir(child.Path)
			if childDir == "." || childDir == e.Path ||
				filepath.HasPrefix(childDir, e.Path+"/") {
				if positions[child.Path] < positions[e.Path] {
					t.Errorf("dir %q (idx %d) appears after child %q (idx %d)",
						e.Path, positions[e.Path], child.Path, positions[child.Path])
				}
			}
		}
	}
}

func TestWalk_NotADirectory(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Walk(file); err == nil {
		t.Error("expected error when root is a file")
	}
}

func TestWalk_MissingRoot(t *testing.T) {
	if _, err := Walk("/no/such/dir/anywhere"); err == nil {
		t.Error("expected error when root doesn't exist")
	}
}

// TestWalk_NoSyntheticRoot: the walker must not emit a synthetic
// "." entry representing root; the TUI treats fileRoot as the
// implicit top of the tree.
func TestWalk_NoSyntheticRoot(t *testing.T) {
	root := fixtureTree(t)
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.Path == "." || e.Path == "" {
			t.Errorf("walker must not emit root sentinel; got %+v", e)
		}
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty walk result")
	}
	// Every returned entry must have a non-zero depth.
	for _, e := range got {
		if e.Depth < 1 {
			t.Errorf("entry %q has depth %d, want >=1", e.Path, e.Depth)
		}
	}
}

func TestWalk_TopLevelSorted(t *testing.T) {
	// Top-level entries must come back in lexical order so the TUI
	// renders deterministically without its own sort.
	root := t.TempDir()
	for _, name := range []string{"z.txt", "a.txt", "m.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range got {
		if e.Depth == 1 {
			names = append(names, e.Path)
		}
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("top-level not sorted: %v", names)
	}
}


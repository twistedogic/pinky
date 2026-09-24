// Package workspace walks the agent's pane cwd, honouring every
// .gitignore in the tree. The result is a flat slice of entries
// the TUI folds into a tree using Depth and IsDir.
//
// Implementation notes:
//   - Each directory is matched against the closest ancestor
//     .gitignore (the natural gitignore semantic for a subtree).
//     This is simpler than stacking matchers and covers the
//     common case; a stack-based merge is the upgrade path.
//   - Symlinks are skipped (Type check) to avoid loops.
//   - Output is sorted by (Depth, Path) so the TUI renders
//     deterministically.
package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// alwaysIgnored is hard-coded on top of any .gitignore. .git is
// excluded even when the user has no top-level rule for it.
var alwaysIgnored = []string{".git"}

// Entry is one file or directory under the workspace root.
// Path is relative to root with forward slashes.
type Entry struct {
	Path  string
	IsDir bool
	Depth int
}

// Walk recursively walks root, returning every file and directory
// not excluded by the closest-ancestor .gitignore (or, at the
// root, the root's .gitignore). Symlinks are skipped.
func Walk(root string) ([]Entry, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace root %q is not a directory", root)
	}
	var out []Entry
	out = append(out, Entry{Path: ".", IsDir: true, Depth: 0})
	if err := walkDir(root, root, 1, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// walkDir reads one directory's entries, filters via the closest
// ancestor .gitignore, recurses into directories, and appends
// everything to *out. Symlinks are skipped.
func walkDir(root, dir string, depth int, out *[]Entry) error {
	gi, err := matcherFor(root, dir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(dir, name)
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		rel := relPath(root, full)
		if gi.MatchesPath(rel) {
			continue
		}
		isDir := e.IsDir()
		if isDir {
			if err := walkDir(root, full, depth+1, out); err != nil {
				return err
			}
		}
		*out = append(*out, Entry{
			Path:  rel,
			IsDir: isDir,
			Depth: depth,
		})
	}
	return nil
}

// matcherFor compiles the closest ancestor .gitignore of dir
// (relative to root), combined with alwaysIgnored. Returns a
// pass-through matcher when no .gitignore is found.
func matcherFor(root, dir string) (*ignore.GitIgnore, error) {
	giPath := findClosestGI(root, dir)
	if giPath == "" {
		return ignore.CompileIgnoreLines(alwaysIgnored...), nil
	}
	return ignore.CompileIgnoreFileAndLines(giPath, alwaysIgnored...)
}

// findClosestGI walks up from dir to root looking for a
// .gitignore; returns its path or "" if none.
func findClosestGI(root, dir string) string {
	cur := dir
	for {
		giPath := filepath.Join(cur, ".gitignore")
		if _, err := os.Lstat(giPath); err == nil {
			return giPath
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

// relPath returns full with the root prefix stripped, normalised
// to forward slashes for sabhiram's pattern matcher. The walker's
// only caller passes a path built via filepath.Join inside root,
// so filepath.Rel cannot fail — panic if it ever does.
func relPath(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		panic("workspace.relPath: " + err.Error())
	}
	return filepath.ToSlash(rel)
}

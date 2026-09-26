// Package workspace walks the agent's pane cwd and returns a flat
// slice of entries the TUI renders as a tree. Output is sorted by
// (Depth, Path) so the TUI can fold it into a tree using the Depth
// field. .git is always skipped; symlinks are skipped to avoid
// loops. .gitignore is not honoured yet — add a matcher here when
// the cost of build artifacts and .git internals outweighs the dep.
package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Entry is one file or directory under the workspace root.
// Path is relative to root with forward slashes.
type Entry struct {
	Path  string
	IsDir bool
	Depth int
}

// Walk recursively walks root, returning every file and directory
// under it. Output is pre-order (parent before children) and
// sorted by path within each depth, so the TUI can render it as a
// flat tree. The root itself is not emitted — its children are
// depth 1, so the caller treats the root as the implicit top.
func Walk(root string) ([]Entry, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace root %q is not a directory", root)
	}
	var out []Entry
	if err := walkDir(root, root, 1, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// walkDir reads one directory's entries, recurses into subdirs,
// and appends everything to *out. .git and symlinks are skipped.
func walkDir(root, dir string, depth int, out *[]Entry) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == ".git" {
			continue
		}
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		full := filepath.Join(dir, name)
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		*out = append(*out, Entry{
			Path:  filepath.ToSlash(rel),
			IsDir: e.IsDir(),
			Depth: depth,
		})
		if e.IsDir() {
			if err := walkDir(root, full, depth+1, out); err != nil {
				return err
			}
		}
	}
	return nil
}

// DepthOf returns the depth of a slash-separated path: 1 for
// "foo", 2 for "foo/bar", etc. Used by tests and by the TUI when
// computing the indent for a path that came from outside Walk.
func DepthOf(path string) int {
	if path == "" || path == "." {
		return 0
	}
	return strings.Count(path, "/") + 1
}

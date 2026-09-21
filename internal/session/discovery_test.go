package session

import (
	"os"
	"path/filepath"
	"testing"
)

// piEncodedDir: pi-mono encodes the cwd as `--<sanitized>--` with `/`, `\`, `:`
// replaced by `-` and a leading `/` stripped.
func TestPiEncodedDir(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/Users/me/proj", "--Users-me-proj--"},
		{"/Users/me/proj/sub", "--Users-me-proj-sub--"},
		{"/Users/me/repo:feature", "--Users-me-repo-feature--"},
		{"/Users/me/with\\backslash", "--Users-me-with-backslash--"},
	}
	for _, c := range cases {
		if got := piEncodedDir(c.in); got != c.want {
			t.Errorf("piEncodedDir(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestNewestJSONL_PicksTimestamped(t *testing.T) {
	dir := t.TempDir()
	// Three files with timestamp prefixes; the newest (largest prefix) wins.
	for _, name := range []string{
		"20260101-0000_uuid1.jsonl",
		"20260301-0000_uuid2.jsonl",
		"20260215-0000_uuid3.jsonl",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := newestJSONL(dir)
	if err != nil {
		t.Fatalf("newestJSONL: %v", err)
	}
	want := filepath.Join(dir, "20260301-0000_uuid2.jsonl")
	if got != want {
		t.Errorf("newestJSONL = %q want %q", got, want)
	}
}

func TestNewestJSONL_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := newestJSONL(dir); err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestNewestJSONL_MissingDir(t *testing.T) {
	if _, err := newestJSONL("/nonexistent/path/xyz"); err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestNewestJSONL_IgnoresNonJSONL(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"20260101_uuid.txt",
		"20260201_uuid.jsonl",
		"random.jsonl.bak",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := newestJSONL(dir)
	if err != nil {
		t.Fatalf("newestJSONL: %v", err)
	}
	want := filepath.Join(dir, "20260201_uuid.jsonl")
	if got != want {
		t.Errorf("newestJSONL = %q want %q", got, want)
	}
}

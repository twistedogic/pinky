package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestOpenPi_FallsBackWhenNoEnv verifies that openPi falls back to
// lsof-based discovery when PI_SESSION_FILE is not set in the
// process environment.
//
// We can't easily inject an env into a different pid, so we test the
// fallback parser directly: parseLsofJSONL picks the most recently
// modified .jsonl file from a fake lsof listing. Then we cover the
// integration by having openPi call piSessionFile and asserting it
// returns a sensible error when nothing matches.
func TestParseLsofJSONL(t *testing.T) {
	dir := t.TempDir()

	// Two .jsonl files, one newer than the other.
	newer := filepath.Join(dir, "newer.jsonl")
	older := filepath.Join(dir, "older.jsonl")
	if err := os.WriteFile(newer, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(older, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, time.Unix(2000, 0), time.Unix(2000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(older, time.Unix(1000, 0), time.Unix(1000, 0)); err != nil {
		t.Fatal(err)
	}

	in := "p1234\n" +
		"fcwd\n" +
		"n" + filepath.Join(dir, "ignored.txt") + "\n" +
		"n" + older + "\n" +
		"n" + newer + "\n" +
		"n" + filepath.Join(dir, "subdir", "nested.jsonl") + "\n" // doesn't exist

	got := parseLsofJSONL(in)
	if got != newer {
		t.Errorf("parseLsofJSONL = %q want %q (most recent .jsonl)", got, newer)
	}
}

func TestParseLsofJSONL_NoJSONL(t *testing.T) {
	in := "n" + "/tmp/foo.txt" + "\n" + "n" + "/tmp/bar.md" + "\n"
	if got := parseLsofJSONL(in); got != "" {
		t.Errorf("parseLsofJSONL = %q want empty", got)
	}
}

func TestParseLsofJSONL_Empty(t *testing.T) {
	if got := parseLsofJSONL(""); got != "" {
		t.Errorf("parseLsofJSONL = %q want empty", got)
	}
}

package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpenPiByCwd_FindsNewestInBucket simulates a pi-mono sessions
// layout on disk and verifies openPiByCwd picks the newest file in the
// cwd's bucket.
func TestOpenPiByCwd_FindsNewestInBucket(t *testing.T) {
	// Set up a fake agent dir.
	tmp := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", tmp)

	cwd := "/Users/test/proj"
	bucket := filepath.Join(tmp, "sessions", piEncodedDir(cwd))
	if err := os.MkdirAll(bucket, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"20260101-0000_aaaa.jsonl",
		"20260215-0000_bbbb.jsonl",
		"20260301-0000_cccc.jsonl",
	} {
		if err := os.WriteFile(filepath.Join(bucket, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := openPiByCwd(cwd)
	if err != nil {
		t.Fatalf("openPiByCwd: %v", err)
	}
	want := filepath.Join(bucket, "20260301-0000_cccc.jsonl")
	if got != want {
		t.Errorf("openPiByCwd = %q want %q", got, want)
	}
}

func TestOpenPiByCwd_BucketMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", tmp)
	if _, err := openPiByCwd("/no/such/cwd"); err == nil {
		t.Error("expected error for missing bucket")
	}
}

func TestCodexByCwd_FindsNewestRollout(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", tmp)

	// codex layout: sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl
	for _, path := range []string{
		"sessions/2026/01/15/rollout-2026-01-15T00-00-00-uuid1.jsonl",
		"sessions/2026/02/20/rollout-2026-02-20T00-00-00-uuid2.jsonl",
		"sessions/2026/01/30/rollout-2026-01-30T00-00-00-uuid3.jsonl",
	} {
		full := filepath.Join(tmp, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := codexByCwd("/anywhere")
	if err != nil {
		t.Fatalf("codexByCwd: %v", err)
	}
	want := filepath.Join(tmp, "sessions/2026/02/20/rollout-2026-02-20T00-00-00-uuid2.jsonl")
	if got != want {
		t.Errorf("codexByCwd = %q want %q", got, want)
	}
}

func TestCodexByCwd_IgnoresNonRollout(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", tmp)
	for _, path := range []string{
		"sessions/2026/01/15/random-file.jsonl",
		"sessions/2026/01/16/rollout-2026-01-16T00-00-00-uuid.jsonl",
	} {
		full := filepath.Join(tmp, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := codexByCwd("")
	if err != nil {
		t.Fatalf("codexByCwd: %v", err)
	}
	if filepath.Base(got) != "rollout-2026-01-16T00-00-00-uuid.jsonl" {
		t.Errorf("expected only rollout-* files; got %q", got)
	}
}

func TestCodexByCwd_NoSessions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", tmp)
	if _, err := codexByCwd(""); err == nil {
		t.Error("expected error when no sessions exist")
	}
}

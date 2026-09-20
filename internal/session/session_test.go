package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPiParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	// Two assistant text messages, one tool call (skipped), one user.
	content := `{"type":"session","version":3,"id":"abc"}
{"type":"message","id":"1","timestamp":"2026-01-01T00:00:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":"first response"}]}}
{"type":"message","id":"2","timestamp":"2026-01-01T00:00:01.000Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"toolCall","name":"bash","arguments":{}},{"type":"text","text":"second response"}]}}
{"type":"message","id":"3","timestamp":"2026-01-01T00:00:02.000Z","message":{"role":"user","content":[{"type":"text","text":"hi"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src := &piSource{path: path}
	got, err := src.NewMessages()
	if err != nil {
		t.Fatal(err)
	}
	want := []Message{
		{Role: RoleAssistant, Text: "first response"},
		{Role: RoleAssistant, Text: "second response"},
		{Role: RoleUser, Text: "hi"},
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want=%d (got=%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i].Role != want[i].Role || got[i].Text != want[i].Text {
			t.Fatalf("[%d] got %+v want %+v", i, got[i], want[i])
		}
	}

	// Second call returns nothing (offset advanced).
	got, err = src.NewMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no new messages on second call, got %v", got)
	}
}

func TestCodexParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	content := `{"timestamp":"2026-01-01T00:00:00.000Z","type":"session_meta","payload":{}}
{"timestamp":"2026-01-01T00:00:01.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}
{"timestamp":"2026-01-01T00:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src := &codexSource{path: path}
	got, err := src.NewMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d (%v)", len(got), got)
	}
	if got[0].Role != RoleAssistant || got[0].Text != "hello" {
		t.Fatalf("msg[0] = %+v", got[0])
	}
	if got[1].Role != RoleUser || got[1].Text != "hi" {
		t.Fatalf("msg[1] = %+v", got[1])
	}
}

func parseTS(t *testing.T, s string) (ts time.Time) {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return
}

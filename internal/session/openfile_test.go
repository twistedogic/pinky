package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFile_PiFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"message","id":"1","timestamp":"2026-01-01T00:00:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":"hello"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer src.Close()

	msgs, err := src.NewMessages()
	if err != nil {
		t.Fatalf("NewMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hello" {
		t.Errorf("msgs = %+v want one message with text 'hello'", msgs)
	}
}

func TestOpenFile_CodexFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"response_item","timestamp":"2026-01-01T00:00:00.000Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer src.Close()

	msgs, err := src.NewMessages()
	if err != nil {
		t.Fatalf("NewMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hello" {
		t.Errorf("msgs = %+v want one message with text 'hello'", msgs)
	}
}

func TestOpenFile_Missing(t *testing.T) {
	if _, err := OpenFile("/nonexistent/path/session.jsonl"); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestOpenFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFile(path); err == nil {
		t.Error("expected error for empty file")
	}
}

func TestOpenFile_UnknownFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weird.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"something_else","foo":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFile(path); err == nil {
		t.Error("expected error for unknown format")
	}
}

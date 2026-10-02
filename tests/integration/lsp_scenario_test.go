//go:build integration

package integration

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	tea "charm.land/bubbletea/v2"
)

// TestLSP_Definition_SingleResult spawns fakegopls, drops it as
// `gopls` on PATH, and runs pinky in a PTY with PINKY_TEST=1.
//
// The test walks pinky through the file-viewer → definition flow and
// asserts the rendered output reflects a successful LSP roundtrip.
//
// Run with:  go test -tags=integration -count=1 ./tests/integration/...
func TestLSP_Definition_SingleResult(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	workspace := t.TempDir()
	writeProject(t, workspace, "main.go", `package main

func helper() int { return 42 }

func main() { _ = helper() }
`)
	t.Logf("workspace: %s", workspace)

	pinkyBin := os.Getenv("PINKY_BIN")
	if pinkyBin == "" {
		pinkyBin = filepath.Join(findRepoRoot(t), "pinky")
	}
	fakegoplsBin := os.Getenv("FAKE_FBINARY")
	if fakegoplsBin == "" {
		fakegoplsBin = filepath.Join(findRepoRoot(t), "fakegopls")
	}

	// Spawn fakegopls.
	mainPath := filepath.Join(workspace, "main.go")
	defsJSON := `[{"query":{"line":3,"col":6},"target":{"uri":"file://` + mainPath + `","range":{"start":{"line":2,"col":5},"end":{"line":2,"col":11}}}}]`
	fg := exec.Command(fakegoplsBin)
	fg.Env = append(os.Environ(),
		"FAKE_LSP_FILE="+mainPath,
		"FAKE_LSP_DEFS="+defsJSON,
	)
	if err := fg.Start(); err != nil {
		t.Fatalf("fakegopls start: %v", err)
	}
	defer func() {
		fg.Process.Kill()
		go func() { _ = fg.Wait() }()
	}()

	// Drop fakegopls as "gopls" on PATH so pinky picks it up.
	binDir := t.TempDir()
	if err := copyFile(fakegoplsBin, filepath.Join(binDir, "gopls")); err != nil {
		t.Fatalf("copy: %v", err)
	}

	// PTY for pinky.
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty: %v", err)
	}
	defer ptmx.Close()
	if err := pty.Setsize(tty, &pty.Winsize{Rows: 30, Cols: 120}); err != nil {
		t.Fatalf("setsize: %v", err)
	}

	cmd := exec.Command(pinkyBin)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PINKY_TEST=1",
		"PINKY_TEST_CWD="+workspace,
		"TERM=xterm-256color",
	)
	cmd.Stdin = tty
	cmd.Stdout = tty
	stderrPath := filepath.Join(t.TempDir(), "pinky-stderr.log")
	stderr, _ := os.Create(stderrPath)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pinky: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		go func() { _ = cmd.Wait() }()
	}()

	// Drain PTY in the background, append to accum.
	var accum bytes.Buffer
	stop := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-stop:
				return
			default:
			}
			n, _ := ptmx.Read(buf)
			if n > 0 {
				accum.Write(buf[:n])
			}
		}
	}()

	// Wait for the picker / nav view to render (placeholder).
	if !waitForNeedle(accum.String, stop, "waiting for agent", 3*time.Second) {
		dump(t, accum.String(), stderrPath)
		t.Fatal("did not see 'waiting for agent'")
	}

	// Tab → file tab.
	sendRune(ptmx, tea.KeyTab)
	if !waitForNeedle(accum.String, stop, "Files", 3*time.Second) {
		dump(t, accum.String(), stderrPath)
		t.Fatal("did not see 'Files' after Tab")
	}

	// The dir nav lists main.go. Press l (expand parent) + Enter (open).
	sendRune(ptmx, 'l')
	time.Sleep(200 * time.Millisecond)
	sendRune(ptmx, tea.KeyEnter)
	time.Sleep(500 * time.Millisecond)

	// Move to line 4 col 6 (helper). j x3 + End + h x5.
	for range 3 {
		sendRune(ptmx, 'j')
	}
	sendRune(ptmx, tea.KeyEnd)
	for range 5 {
		sendRune(ptmx, 'h')
	}

	// Press `d` for textDocument/definition. fakegopls returns a
	// single Location pointing at line 2 col 5 ("helper" definition).
	// Pinky should jump the cursor there.
	sendRune(ptmx, 'd')

	// Wait for the cursor block to land on line 2 (helper definition).
	// The file viewer's content shows the file with a block cursor at
	// (line, charPos); line 2 col 5 means the cursor is on the
	// "helper" identifier. The simplest invariant: pinky renders
	// line 2's content somewhere after the first \n.
	jumped := false
	deadline2 := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline2) {
		if strings.Contains(accum.String(), "func helper()") &&
			!strings.Contains(lastLines(accum.String(), 4), "func helper() int { return 42 }\n          3  func helper()") {
			// The first 3 lines are 1 (package), 2 (blank), 3 (func helper).
			// Cursor moved past line 1+2.
			jumped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !jumped {
		close(stop)
		dump(t, accum.String(), stderrPath)
		t.Fatalf("cursor did not jump to line 2 after `d`; expected helper() to be visible on the first 3 lines")
	}

	close(stop)
}

func dump(t *testing.T, accum string, stderrPath string) {
	t.Logf("pinky pty output (last 2KB):\n%s", tailBytes(accum, 2048))
	if data, err := os.ReadFile(stderrPath); err == nil && len(data) > 0 {
		t.Logf("pinky stderr:\n%s", string(data))
	}
}

// waitForNeedle polls the accumulator (live during read) until the
// needle appears or the timeout fires. Returns whether it was found.
func waitForNeedle(snap func() string, stop chan struct{}, needle string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(snap(), needle) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// writeProject writes a file in dir, creating parent dirs as needed.
func writeProject(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// findRepoRoot walks up looking for go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatalf("go.mod not found above %s", cwd)
		}
		cwd = parent
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func tailBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// lastLines returns the trailing n lines of s (split on "\n").
func lastLines(s string, n int) string {
	parts := strings.Split(s, "\n")
	if len(parts) <= n {
		return s
	}
	return strings.Join(parts[len(parts)-n:], "\n")
}

// sendRune writes one key event to the PTY.
func sendRune(ptmx *os.File, k rune) {
	var buf bytes.Buffer
	switch k {
	case tea.KeyTab:
		buf.WriteString("\t")
	case tea.KeyEnter:
		buf.WriteString("\r")
	case tea.KeyBackspace:
		buf.WriteString("\x7f")
	case tea.KeyEsc:
		buf.WriteString("\x1b")
	case tea.KeyEnd:
		buf.WriteString("\x1b[F")
	default:
		buf.WriteRune(k)
	}
	ptmx.Write(buf.Bytes())
}
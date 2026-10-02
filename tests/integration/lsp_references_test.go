//go:build integration

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	tea "charm.land/bubbletea/v2"
)

// TestLSP_References_MultipleResults_Picker exercises the references
// path end-to-end: spawn fakegopls, drive pinky to open the file
// viewer and press R on a symbol, then assert the picker appears
// with the expected refs (and NOT the empty-line / silent path that
// Dispatch used to fall into when the call's []Location return was
// discarded).
//
// Run with:  go test -tags=integration -count=1 ./tests/integration/...
func TestLSP_References_MultipleResults_Picker(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	workspace := t.TempDir()
	mainPath := filepath.Join(workspace, "main.go")
	writeProject(t, workspace, "main.go", `package main

func helper() int { return 42 }

func main() {
	_ = helper()
	_ = helper()
	_ = helper()
}
`)

	pinkyBin := os.Getenv("PINKY_BIN")
	if pinkyBin == "" {
		pinkyBin = filepath.Join(findRepoRoot(t), "pinky")
	}
	fakegoplsBin := os.Getenv("FAKE_FBINARY")
	if fakegoplsBin == "" {
		fakegoplsBin = filepath.Join(findRepoRoot(t), "fakegopls")
	}

	// Two refs at distinct (line, col) positions: one in main() at
	// the first helper() call, one at the third. fakegopls only
	// matches on (line, col), so the query at (4, 6) (the cursor's
	// first helper position) returns both back. We need at least 2
	// to verify the picker path (1-result would auto-jump).
	refsJSON := `[{"line":4,"col":6},{"line":6,"col":6}]`
	fg := exec.Command(fakegoplsBin)
	fg.Env = append(os.Environ(),
		"FAKE_LSP_FILE="+mainPath,
		"FAKE_LSP_REFS="+refsJSON,
	)
	if err := fg.Start(); err != nil {
		t.Fatalf("fakegopls start: %v", err)
	}
	defer func() {
		fg.Process.Kill()
		go func() { _ = fg.Wait() }()
	}()

	binDir := t.TempDir()
	if err := copyFile(fakegoplsBin, filepath.Join(binDir, "gopls")); err != nil {
		t.Fatalf("copy: %v", err)
	}

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
		// powernap inherits pinky's env into its gopls subprocess,
		// so the FAKE_LSP_* fixtures need to be on pinky's env (not
		// just fg's). powernap only adds explicit config.Environment
		// on top of os.Environ().
		"FAKE_LSP_FILE="+mainPath,
		"FAKE_LSP_REFS="+refsJSON,
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

	if !waitForNeedle(accum.String, stop, "waiting for agent", 3*time.Second) {
		dump(t, accum.String(), stderrPath)
		t.Fatal("did not see 'waiting for agent'")
	}

	// Tab → Files, l (expand root), Enter (open main.go).
	sendRune(ptmx, tea.KeyTab)
	if !waitForNeedle(accum.String, stop, "Files", 3*time.Second) {
		dump(t, accum.String(), stderrPath)
		t.Fatal("did not see 'Files' after Tab")
	}
	sendRune(ptmx, 'l')
	time.Sleep(200 * time.Millisecond)
	sendRune(ptmx, tea.KeyEnter)
	time.Sleep(500 * time.Millisecond)

	// Move cursor to line 4 col 6 (first helper() call inside main).
	for range 3 {
		sendRune(ptmx, 'j')
	}
	sendRune(ptmx, tea.KeyEnd)
	for range 5 {
		sendRune(ptmx, 'h')
	}

	// R triggers textDocument/references. The picker must surface
	// (header "references — N location(s)" with N >= 2). Before the
	// Dispatch []Location fix, N was always 0 and pinky stayed
	// silent in the file viewer — the test would fail on the
	// "references —" check below.
	sendRune(ptmx, 'R')

	pickerFound := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// The picker renders its label like "references — 2 location(s)".
		s := accum.String()
		if strings.Contains(s, "references") && strings.Contains(s, "location") {
			pickerFound = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !pickerFound {
		close(stop)
		dump(t, accum.String(), stderrPath)
		t.Fatal("picker did not appear after pressing R; references handler likely discarded locations")
	}
}
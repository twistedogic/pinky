//go:build integration

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestPinky_PTYSmoke: spawn pinky in a PTY with PINKY_TEST=1, read
// for 2 seconds, dump output.
func TestPinky_PTYSmoke(t *testing.T) {
	pinkyBin := os.Getenv("PINKY_BIN")
	if pinkyBin == "" {
		pinkyBin = filepath.Join(findRepoRoot(t), "pinky")
	}

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty: %v", err)
	}
	defer ptmx.Close()
	if err := pty.Setsize(tty, &pty.Winsize{Rows: 30, Cols: 120}); err != nil {
		t.Fatalf("setsize: %v", err)
	}

	stderrPath := filepath.Join(t.TempDir(), "pinky-stderr.log")
	stderr, _ := os.Create(stderrPath)

	cmd := exec.Command(pinkyBin)
	cmd.Env = append(os.Environ(),
		"PINKY_TEST=1",
		"PINKY_TEST_CWD="+t.TempDir(),
		"TERM=xterm-256color",
	)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		go func() { _ = cmd.Wait() }()
	}()

	var accum bytes.Buffer
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
			}
			n, _ := ptmx.Read(buf)
			if n > 0 {
				accum.Write(buf[:n])
			}
		}
	}()
	time.Sleep(2 * time.Second)
	close(done)
	t.Logf("pinky pty output (last 1KB):\n%s", tailBytes(accum.String(), 1024))
	if data, err := os.ReadFile(stderrPath); err == nil {
		t.Logf("pinky stderr:\n%s", string(data))
	}
}
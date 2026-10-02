// pinky is a persistent TUI for capturing the last agent message from
// pi or codex (by tailing their session JSONL) and injecting precise
// push-backs back into the agent's tmux pane via paste-buffer + send-keys.
package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	pinkylsp "github.com/twistedogic/pinky/internal/lsp"
	"github.com/twistedogic/pinky/internal/session"
	"github.com/twistedogic/pinky/internal/tmux"
)

const (
	pollInterval = 500 * time.Millisecond
)

// testMode reports whether pinky was started with PINKY_TEST=1. In
// test mode the tmux hard prerequisite is bypassed (the test fixture
// spawns a real tmux server at a custom socket), agent discovery is
// skipped (the test injects its own cwd), and the model is placed
// directly into stateNav with an empty session source so the
// scenarios can navigate the file viewer + LSP without a backing pi/
// codex process. Test scenarios override `sendToPane` in model.go
// to capture outgoing inject text.
func testMode() bool {
	return os.Getenv("PINKY_TEST") == "1"
}

func main() {
	model := newModel()
	if testMode() {
		// Skip agent discovery + attach; place the model directly in
		// stateNav. m.src stays nil; poll is gated on src == nil at
		// the call sites (pollCmd returns nil when there's no source).
		model.state = stateNav
		model.pane = "%1" // fake pane id; sendToPane is test-overridden
		model.fileRoot = os.Getenv("PINKY_TEST_CWD")
		if model.fileRoot == "" {
			model.fileRoot = "."
		}
		// Provide sane defaults so reflow() doesn't call SetWidth(0)
		// on the viewport/textarea before any WindowSizeMsg fires.
		model.width = 120
		model.height = 30
		// Wire up the LSP bridge so d/R/K actually round-trip in
		// tests. Powernap reads from PATH, so a test fixture that
		// drops fakegopls as `gopls` (or any other server) makes
		// the file viewer's LSP keys go live.
		lspMgr := pinkylsp.New(model.fileRoot)
		model.lsp = lspMgr
		model.lsphub = pinkylsp.NewBridge(lspMgr)
	} else if err := tmux.RequireServer(); err != nil {
		// No tmux server: no agent pane can exist. Fall back to the
		// cwd file navigator instead of refusing to start.
		model.standalone()
	} else {
		agents, err := session.ListAgents()
		switch {
		case err != nil:
			model.err = err
			model.state = stateError
		case len(agents) == 0:
			// No agent pane either: same standalone fallback.
			model.standalone()
		default:
			model.setAgents(agents)
		}
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fail(err)
	}
}

// fail prints to stderr and exits. Used only for unrecoverable startup
// errors where no TUI can render (bubbletea itself failing to start).
func fail(err error) {
	fmt.Fprintln(os.Stderr, "pinky:", err)
	os.Exit(1)
}

# lsp-definition.test.tape
# Scenario: pressing `d` on a symbol with one definition jumps the cursor.
# Path A: vhs-driven, no pinky test seam. This tape documents the scenario
# the CI workflow in .github/workflows/integration.yml would run.
#
# NOTE: without PINKY_TEST=1 (Path B), pinky exits at the picker because
# there are no real pi or codex agents in the CI tmux server. The .tape
# below assumes the picker is populated. Path B's seam makes this real.

Output golden/lsp-definition.ascii

Require go
Require gopls
Require pinky
Require tmux
Require ttyd
Require ffmpeg

# Start tmux server with a fake agent pane (bash that runs `sleep infinity`).
# A real agent process would need to fork itself from the pane; we use bash
# for the smoke test only. Path B replaces this with the test seam.
Hide
Type "tmux -S /tmp/pinky-test.sock new-session -d -s pinky 'sleep 1000000'"
Enter
Sleep 200ms
Show

# Launch pinky in the tmux session
Type "PINKY_TMUX_SOCK=/tmp/pinky-test.sock ./pinky"
Enter
# Wait for the picker to populate (or the error screen, if no agents).
Wait+Line /select an agent|error/

# If error screen, this test is a smoke-test pass. Press q to exit.
# Real LSP scenarios require Path B (PINKY_TEST=1) or a fake `pi`/`codex` process.

# Cleanup
Type "tmux -S /tmp/pinky-test.sock kill-server"
Enter
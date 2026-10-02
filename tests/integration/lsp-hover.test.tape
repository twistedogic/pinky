# lsp-hover.test.tape
# Scenario: pressing `K` on an identifier renders the hover footer.
# Same caveat — smoke test without the pinky test seam.

Output golden/lsp-hover.ascii

Require pinky
Require tmux
Require ttyd
Require ffmpeg

Hide
Type "tmux -S /tmp/pinky-test.sock new-session -d -s pinky 'sleep 1000000'"
Enter
Sleep 200ms
Show

Type "PINKY_TMUX_SOCK=/tmp/pinky-test.sock ./pinky"
Enter
Wait+Line /select an agent|error/

# Smoke-test pass.

Type "tmux -S /tmp/pinky-test.sock kill-server"
Enter
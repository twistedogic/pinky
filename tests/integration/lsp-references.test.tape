# lsp-references.test.tape
# Scenario: pressing `R` on a symbol with N references opens the picker.
# Same caveat as lsp-definition.test.tape — Path A is a smoke test without
# the pinky test seam.

Output golden/lsp-references.ascii

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
# Tasks

## 1. Baseline

- [ ] 1.1 Confirm the repo builds and all tests pass before any
  edits; verify by running `task check` and reading its exit code

## 2. Stop the recurring poll

- [ ] 2.1 In `model.go`'s `Update` `sessionMsg` case, replace the
  `return m, pollCmd(m.src)` at the end of the `err` branch with
  `return m, nil`; verify by reading the diff and confirming no
  symbol is lost (the `pollCmd` factory still exists at
  `model.go:325`)
- [ ] 2.2 Same file, replace the `return m, pollCmd(m.src)` at the
  end of the empty-poll branch (after `m.streaming = false`) with
  `return m, nil`; verify by reading the diff
- [ ] 2.3 Same file, replace the `return m, pollCmd(m.src)` at the
  end of the case (after the `if last != nil { … }` block) with
  `return m, nil`; verify by `go build ./...` exiting 0
- [ ] 2.4 Rewrite the `// ponytail:` block comment immediately above
  the `case sessionMsg:` line so it reads as a deliberate
  simplification note (e.g. "polling stops after the first message;
  `r` is the manual gate, see proposal.md") rather than a warning
  about a future bug; verify by reading the comment

## 3. Flip the polling tests

- [ ] 3.1 In `poll_test.go`, flip
  `TestSessionMsg_ReschedulesPoll_OnEntries` so it asserts
  `cmd == nil` after `Update(sessionMsg{entries: …})` instead of
  asserting `cmd != nil`; verify by running
  `go test -run TestSessionMsg_ReschedulesPoll_OnEntries ./...`
  and confirming the test passes
- [ ] 3.2 Same file, flip
  `TestSessionMsg_ReschedulesPoll_OnEmpty` to assert
  `cmd == nil` after `Update(sessionMsg{entries: nil})`; verify by
  running
  `go test -run TestSessionMsg_ReschedulesPoll_OnEmpty ./...`
  and confirming the test passes
- [ ] 3.3 Same file, rewrite
  `TestSessionMsg_ContinuousPollingDeliversNewMessages` to drive
  two manual refreshes: (a) first `pollCmd` resolves with the
  initial message, (b) the source appends a second message, (c)
  second `pollCmd` resolves with the second message, (d) assert
  `m.latest.Text` equals the second message; verify by running
  `go test -run TestSessionMsg_ContinuousPollingDeliversNewMessages
  ./...` and confirming the test passes

## 4. Verification

- [ ] 4.1 Run `task check` and confirm all tests pass (including the
  seven `picker_test.go` scenarios and any `model_test.go` /
  `view_test.go` cases that touch the polling handler); verify by
  reading the test output
- [ ] 4.2 Run `go build ./...` and confirm the binary compiles
  cleanly; verify by exit code 0
- [ ] 4.3 Run `task vet` and confirm no new vet diagnostics; verify
  by exit code 0
- [ ] 4.4 Manual smoke test: attach pinky to a real agent session,
  observe that the view populates once within ~500 ms of attach and
  does *not* advance on its own; press `r` and confirm a fresh
  fetch fires; verify by running the binary and observing the
  behaviour
## 1. Implementation

- [x] 1.1 In `keymap.go`: replace the `NavGroup` field with 10 individual bindings — `NavBlockDown`, `NavBlockUp`, `NavRuneLeft`, `NavRuneRight`, `NavVisual`, `NavComment`, `NavSend`, `NavCompose`, `NavRefresh`, `NavQuit`. Each carries `key.WithKeys(...)` matching the existing runes and `key.WithHelp(...)` with a one-line description.
- [x] 1.2 In `keymap.go`: remove the `NavGroup` definition from `defaultKeyMap` and replace with the 10 individual binding definitions.
- [x] 1.3 In `model.go`: update `ShortHelp` for `stateNav` to return `[]key.Binding{NavComment, NavSend, Help}` (curated subset that fits ≤80 cols).
- [x] 1.4 In `model.go`: update `FullHelp` for `stateNav` to return `[][]key.Binding` with three groups (motion / actions / quit+help).
- [x] 1.5 In `help_test.go`: update the substring assertions — `"nav"` and the collapsed key-list `"j k h l v c s q r n"` are replaced with per-key descriptors (`"comment composer"`, `"send all comments"`, `"next block"`, `"previous block"`, etc.).

## 2. Verification

- [x] 2.1 `go build ./...` clean.
- [x] 2.2 `go test ./...` all green.
- [x] 2.3 Manual render of the nav help (via a throwaway test) confirms each key appears on its own row with its description.

## 3. Archive

- [x] 3.1 `openspec validate expand-nav-keymap`. (Failed — change is a pure refactor with no spec delta; archived via `--skip-specs`.)
- [x] 3.2 `openspec archive --yes --skip-specs expand-nav-keymap`.
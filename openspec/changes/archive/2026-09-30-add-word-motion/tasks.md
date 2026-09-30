## 1. Word boundary helpers (TDD)

- [x] 1.1 Write `internal/render/word_test.go` with fixtures covering:
  ascii words, multibyte runes between word chars, punctuation-as-word,
  end-of-source (no next word), start-of-source (no previous word),
  blank-line skip, line-crossing (cursor on last char of line A,
  `w` lands on first word of line B).
- [x] 1.2 Add `internal/render/word.go` with `isWordRune`,
  `nextWordStart`, `prevWordStart`. All three are pure functions.
  `nextWordStart` / `prevWordStart` return `(lineIdx, charPos)`;
  blank-line skip is the explicit rule, not a side effect.
- [x] 1.3 Run `go test ./internal/render/...` and confirm green.

## 2. Wire into NavHandle

- [x] 2.1 Add two new `NavAction` constants to `internal/render/nav.go`:
  `ActionWordRight`, `ActionWordLeft`. Place them next to
  `ActionRuneRight` / `ActionRuneLeft` in the iota block.
- [x] 2.2 Add `case 'w':` / `case 'b':` arms to `NavHandle`. Both arms:
  call the boundary helper; assign `cur.LineIdx` and `cur.CharPos`;
  on `w`, set `cur.Preferred = max(cur.Preferred, cur.CharPos)`;
  when `st.Visual == NavLine`, set `sel.ByteC = byteOffset(...)`;
  return the matching action.
- [x] 2.3 Add scenarios to `internal/render/nav_test.go`: basic
  forward, basic back, cross-line forward, cross-line back, blank-line
  skip, visual-mode `vw` extends selection, `w` updates preferred
  while `b` leaves it.

## 3. Wire help-overlay bindings

- [x] 3.1 In `keymap.go`: add `NavWordRight` (`key.WithKeys("w")`,
  `key.WithHelp("w", "word forward")`) and `NavWordLeft`
  (`key.WithKeys("b")`, `key.WithHelp("b", "word back")`) to the
  `keyMap` struct and `defaultKeyMap` singleton.
- [x] 3.2 In `model.go`: confirm `handleNavKey` dispatches
  `ActionWordRight` / `ActionWordLeft` through the existing motion
  branch (no new `case` — they fall into
  `ActionBlockDown, ActionBlockUp, ActionRuneLeft, ActionRuneRight`).
  If they don't, add them to that case list.
- [x] 3.3 In `model.go`: extend the `FullHelp` for `stateNav` to
  include `NavWordRight` and `NavWordLeft` in the motion group.
  Verify with `go test ./...` (help_test.go substring assertions).

## 4. Spec delta

- [x] 4.1 In `openspec/changes/add-word-motion/specs/single-cursor-nav/spec.md`:
  - **MODIFIED** requirement `Single-letter nav surface in stateNav` —
  add two rows to the key table (`w`, `b`).
  - **ADDED** requirement `Word motion in stateNav` — defines word
  class, cross-line behaviour, blank-line rule, `preferred` update
  rule, visual-mode parity; with 5–6 scenarios (basic forward, basic
  back, cross-line, blank-line skip, preferred-update asymmetry,
  visual-mode extension).
- [x] 4.2 Run `openspec validate add-word-motion` and resolve any
  flagged issues.
- [x] 4.3 Run `openspec archive add-word-motion` to fold the spec
  delta into `openspec/specs/single-cursor-nav/spec.md`.

## 5. README

- [x] 5.1 In `README.md`, Nav key table: add two rows
  (`w` — Move cursor to start of next word;
  `b` — Move cursor to start of previous word).
  Place them after the `h` / `l` row.
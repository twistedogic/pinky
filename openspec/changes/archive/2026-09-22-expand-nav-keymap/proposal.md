## Why

The nav surface renders as one collapsed row in the help overlay —
`nav | j k h l v c s q r n` — with no per-key description. Users
pressing `?` see a list of keys but not what each one does. The rest
of the keymap already uses one `key.Binding` per action with a
short help string; nav is the only outlier.

## What Changes

- **`keyMap.NavGroup` is replaced with 10 individual bindings** —
  `NavBlockDown`, `NavBlockUp`, `NavRuneLeft`, `NavRuneRight`,
  `NavVisual`, `NavComment`, `NavSend`, `NavCompose`, `NavRefresh`,
  `NavQuit`. Each carries its own `key.WithHelp(key, description)`.
- **`ShortHelp` for `stateNav`** shows a curated subset
  (`NavComment` + `NavSend` + `Help`) so the one-line footer stays
  inside the 80-column budget.
- **`FullHelp` for `stateNav`** shows all 10 nav bindings across
  three columns (motion / actions / quit+help).
- **Help-test substring assertions** are updated to match the new
  per-key descriptors (e.g. `"nav"` → `"comment composer"`,
  `"send all comments"`, `"next block"`, `"previous block"`).
- **No spec delta** — `single-cursor-nav/spec.md` is the source of
  truth for the nav surface and is unchanged. This is a help-
  rendering refactor; the runtime key routing (`handleNavKey` →
  `NavHandle`) is untouched.

## Capabilities

### Modified Capabilities

None. The nav surface (`j k h l v c s q r n`) and its routing are
unchanged; only the help-rendering structure moved.

## Impact

- `keymap.go`: `NavGroup` field removed; 10 individual bindings
  added with per-key descriptions.
- `model.go`: `ShortHelp` for `stateNav` returns `NavComment`,
  `NavSend`, `Help`. `FullHelp` returns the 10 nav bindings in
  three groups + `Help`.
- `help_test.go`: substring assertions updated.
- No runtime behaviour change.
- No README change — README's per-key nav table was already more
  detailed than the help overlay.
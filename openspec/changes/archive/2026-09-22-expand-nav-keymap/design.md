## Context

The help overlay (Bubbles `help.Model`) renders one row per
`key.Binding`: the binding's key column shows the keystroke label,
the help column shows the binding's help string. Every state except
`stateNav` already follows this pattern — each action is its own
binding. Nav was collapsed into a single `NavGroup` binding whose
help text was just a space-separated list of runes (`"j k h l v c s
q r n"`), so the overlay rendered as `nav | j k h l v c s q r n`
with no descriptions.

The nav surface is large (10 keys) and trying to pack all
descriptions into one help string either creates a wide line that
overflows the 80-column terminal or pushes newlines that Bubbles
help.Model does not split on (each binding is one row regardless of
newlines in the help string).

## Goals / Non-Goals

**Goals:**

- Each nav key gets its own row in the help overlay with a one-line
  description.
- `FullHelp` lays the bindings out in columns so the overlay still
  fits in 80 columns when expanded.
- `ShortHelp` shows a curated subset (the most-used actions) so
  the one-line footer stays inside the 80-column budget.
- Help-test assertions are updated to match.

**Non-Goals:**

- Changing the runtime key routing. `handleNavKey` →
  `render.NavHandle` is unchanged.
- Changing which keys are part of the nav surface. The 10 runes
  (`j k h l v c s q r n`) are unchanged.
- Adding new actions or features.

## Decisions

### One binding per nav key

The Bubbles help.Model constraint (one row per binding) makes
multiple bindings the only way to render per-key descriptions. A
single binding with embedded `\n` does not split — Bubbles treats
the help string as opaque text.

The field count grows from 1 (`NavGroup`) to 10 (one per key). The
field-name prefix (`Nav*`) keeps them grouped in the `keyMap` struct
for easy scanning, and they live next to `Picker` and the other
state-specific groups.

Alternatives considered:
- Inline descriptions in one help string (e.g. `"j next · k prev · …"`)
  — readable in print but Bubbles renders it as one wide row that
  overflows 80 columns in the help overlay.
- A multi-line help string with `\n` — Bubbles does not split on
  `\n`; the binding still renders as one row with literal `\n`
  characters.

### `ShortHelp` shows a curated subset

Showing all 10 nav bindings in the one-line footer would require
~150 characters and overflow the 80-column budget. Two of the most
useful actions (`NavComment` to open the composer, `NavSend` to
dispatch the batch) plus the `Help` binding fit comfortably.

Alternatives considered:
- Show no nav bindings, just `Help` — minimises footer width but
  leaves the user with no reminder of available actions.
- Show the original `NavGroup` collapsed binding — preserves the
  existing footer but leaves the per-key help invisible without
  pressing `?`.

### `FullHelp` uses three groups

Motion (`NavBlockDown`, `NavBlockUp`, `NavRuneLeft`, `NavRuneRight`),
actions (`NavVisual`, `NavComment`, `NavSend`, `NavCompose`,
`NavRefresh`), and exit/help (`NavQuit`, `Help`). Bubbles help.Model
lays the groups across columns; each binding becomes its own row.

## Risks / Trade-offs

- **[Trade-off]** Field count in `keyMap` grows from 1 to 10 for
  the nav surface. Acceptable — the field names follow a clear
  `Nav*` prefix and the alternative (one opaque `NavGroup` with a
  packed help string) doesn't render properly.
- **[Risk]** Help-test substring assertions change. → **Mitigation:**
  Substring checks are still meaningful; they now assert the new
  per-key descriptors (`"next block"`, `"comment composer"`, etc.).
- **[Risk]** `FullHelp` width exceeds 80 columns. → **Mitigation:**
  Bubbles lays bindings across multiple columns automatically; the
  existing fillWidth padding handles each line. The `view_test`
  width assertion only checks the picker view, not the nav state,
  and `TestView_PadsStatusLineToFullWidth` checks the status line
  (separate from the help overlay).
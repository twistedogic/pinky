# pinky

A persistent TUI for capturing the last message from an AI coding agent
(`pi` or `codex`) running in a tmux pane, and injecting precise push-backs
back into the agent's input.

```
┌──────────────────────────┬─────────────────────────┐
│   Agent (pi / codex)     │   pinky (split, right)  │
│                          │                         │
│   agent streams...       │   tails agent session   │
│                          │   file, shows latest    │
│                          │   message rendered as   │
│                          │   markdown              │
│                          │                         │
│                          │   ─ compose (n) ─       │
│                          │   multi-line input      │
│   agent input ▌          │   s to send             │
└──────────────────────────┴─────────────────────────┘
```

```
┌──────────────────────────┬─────────────────────────┐
│   Agent (pi/codex/...)   │   pinky (split, right)  │
│                          │                         │
│   agent streams...       │   watches pane, shows   │
│                          │   last message + your   │
│                          │   redirects             │
│                          │                         │
│                          │   ─ compose (n) ─       │
│                          │   multi-line input      │
│   agent input ▌          │   s to send             │
└──────────────────────────┴─────────────────────────┘
```

## Requirements

- Go 1.22+ (built against 1.26.4)
- `tmux` 3.0+ on `PATH`
- `lsof` on `PATH` (used for both pi and codex session discovery when
  `PI_SESSION_FILE` is not in the process env)
- An agent process running in the target pane: **`pi`** or **`codex`**. Other agents error out at startup.

## Troubleshooting

Set `PINKY_DEBUG=1` to log session-discovery diagnostics to stderr:

```sh
PINKY_DEBUG=1 pinky
```

When pinky can't find the session file, the TUI shows an error with
diagnostic hints, including the exact commands to inspect the process
env and open files manually.

## Install

```sh
go install github.com/twistedogic/pinky@latest
```

Or build from source:

```sh
git clone https://github.com/twistedogic/pinky
cd pinky
go build .
```

## Run

Launch pinky without arguments; it enumerates every tmux pane that
contains a `pi` or `codex` agent and shows a picker:

```
pinky: pick an agent session
  session          pane    agent
▶ work:1.1         %12     codex
  main:0.0         %5      pi

↑/↓ navigate  Enter select  Ctrl+C quit
```

Use arrow keys to move, Enter to select. Pinky then transitions to its
running state bound to that pane.

### As a split-pane

Inside a tmux session, split a pane and start pinky there:

```sh
# bind a hotkey (add to ~/.tmux.conf)
bind-key p split-window -h -l 35% -c "#{pane_current_path}" "pinky"

# then: prefix + p   →   opens pinky; picker appears
```

Pinky works the same regardless of where it's launched — the picker is
always shown.

## Keys

### Nav (the main view)

The main view shows the latest complete agent message as rendered
markdown. Within that message:

| Key | Action |
|---|---|
| `j` | Move cursor to next source line |
| `k` | Move cursor to previous source line |
| `h` / `l` | Move cursor one rune left / right within the current source line |
| `v` | Enter visual mode (toggle; `v` again exits) |
| `Esc` | Exit visual mode |
| `c` | Open the comment composer (anchored to selection, or whole current line) |
| `s` | Send all accumulated comments in one tmux inject |
| `n` | Enter compose mode |
| `r` | Re-poll the agent session |
| `Tab` | Switch to the file review tab |
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle the keymap page |
| `↓` / `↑` | Scroll viewport one line (cursor unchanged) |
| `PageDown` / `PageUp` | Scroll viewport one page (cursor unchanged) |
| `Home` / `End` | Scroll viewport to top / bottom (cursor unchanged) |

Scroll keys (`↑`/`↓`/`PageDown`/`PageUp`/`Home`/`End`) move the
viewport only — the cursor does not change. `j`/`k` move the
cursor and the viewport follows with a 1-line cushion. The
focused byte is rendered as an inverted-block cursor at the
cursor's `charPos`.

### File review tab

The file review tab browses the agent pane's working directory
(with `.gitignore` honoured) and lets you stage file-anchored
comments. Pressing `Tab` from the message view enters the tab;
pressing `Tab` again (or `Esc` from the dir navigator) returns
to the message view, restoring the sub-state you left.

#### Dir navigator

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `h` | Collapse current directory, or jump to parent entry |
| `l` | Expand current directory, or jump to first child |
| `Enter` | Open file / toggle directory collapse |
| `c` | Comment on a file (whole-file anchor); no-op on a directory |
| `s` | Send all accumulated comments (msg + file) in one tmux inject |
| `Esc` / `Tab` | Return to the message view |
| `?` | Toggle the keymap page |

#### File viewer

| Key | Action |
|---|---|
| `j` / `↓` | Move the line cursor down; column cursor becomes `min(preferred, len(newLine))`; `preferred` survives the move |
| `k` / `↑` | Move the line cursor up; same column rule as `j` |
| `h` | Move the column cursor one rune left; `preferred` unchanged |
| `l` | Move the column cursor one rune right; `preferred` tracks the rightmost column reached |
| `v` | Enter / exit char visual selection at the cursor (anchor = cursor); `j`/`k`/`h`/`l` extend the selection across lines / chars |
| `c` | No visual -> comment the current line. Visual active: file-inline on single-line selection, file line-range on multi-line selection. |
| `d` | LSP definition at the **word under the cursor** — jumps on 1 result, picker on N>1, silent on 0 |
| `R` | LSP references at the **word under the cursor** — picker on N≥1, silent on 0 |
| `K` | LSP hover at the **word under the cursor** — footer line until the next key press |
| `s` | Send all accumulated comments |
| `Esc` | Exit visual if active; otherwise return to the dir navigator |
| `Tab` | Switch to the message view (visual is preserved across the round-trip) |
| `q` | Quit pinky (visual is discarded) |
| `?` | Toggle the keymap page |

The file viewer's cursor renders as an inline block (inverted
background) at the exact byte position, not as a left-gutter
marker. Lines covered by a file-kind comment show a yellow `▍`
in the left gutter. The active visual selection highlights the
covered byte range in cyan, with interior lines of a multi-line
selection fully highlighted. `d` / `R` / `K` speak Language
Server Protocol (LSP); the server binary is detected from the
file extension and lazily spawned on first use, so the first
query for a new language takes ~1 s and a missing server shows
a one-line install hint.

### Compose

| Key | Action |
|---|---|
| `Enter` | Newline |
| `i` | Toggle "include comments" appendix on the next send |
| `s` | Send the redirect (appends comments appendix when toggle is on; the appendix now mixes block-kind and file-kind comments) |
| `Esc` | Cancel and return to nav |
| `?` | Toggle the keymap page |

### Comment composer

| Key | Action |
|---|---|
| `Enter` | Save comment and return to nav (does not send) |
| `Esc` | Cancel draft and return to nav |

When the anchor came from a file, the saved comment is a
`file` or `file-inline` entry in the appendix:

- `- file "<path>" (lines X-Y): <text>` — line-range comment
- `- file-inline "<excerpt>" (line Z): <text>` — inline byte
  selection from a visual-mode `c`

When the anchor came from the message view, the saved comment is a
`comment on "<excerpt>": <text>` entry in the appendix. Excerpts
are truncated to 40 characters with `…`, with internal newlines
replaced by single spaces and runs of whitespace collapsed. The
appendix is split into two sections by comment kind:

```
Comments on the message:
- comment on "<excerpt>": <text>

Comments on files:
- file "<path>" (lines X-Y): <text>
- file-inline "<excerpt>" (line Z): <text>
```

The leading count line is omitted; the two sections are emitted
only when there is at least one entry of that kind.

### Picker

| Key | Action |
|---|---|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Select |
| `q` | Quit |
| `Ctrl+C` | Quit |

The cursor's byte position is rendered as an inverted-block cursor
at the cursor's `charPos` on the rendered line. Source lines
touched by a saved comment carry a yellow `▍` (228) in the
left-margin column; no other gutter or border is drawn.

## History

Every surfaced agent message and every user-sent redirect is appended
to:

- `$XDG_DATA_HOME/pinky/history.jsonl`, or
- `~/.local/share/pinky/history.jsonl`

History is append-only. On startup, pinky does not reload prior
history into the view — the main view starts fresh and shows only the
latest message from the live session.

## How it works

1. Pinky reads `#{pane_pid}` and `#{pane_current_path}` from the
   target tmux pane and walks its descendant processes (`pgrep -P`,
   `ps -c -o comm=`) until it finds one named `pi` or `codex`.
2. For `pi`, the session file is discovered in this order:
   - `PI_SESSION_FILE` env var (canonical pi-mono convention), or
   - cwd-based lookup: `<agent_dir>/sessions/--<encoded cwd>--/<timestamp>_<uuid>.jsonl`
     where `<agent_dir>` is `$PI_CODING_AGENT_DIR` (default `~/.pi/agent`)
     or `$PI_CODING_AGENT_SESSION_DIR`, or
   - `lsof -p <pid>` to find an open `.jsonl` (last resort).
3. For `codex`, the session file is discovered in this order:
   - cwd-based lookup under `$CODEX_HOME/sessions/` (default `~/.codex`)
     for `rollout-<timestamp>-<uuid>.jsonl`, newest across all date
     subdirs, or
   - `lsof -p <pid>` to find an open `.jsonl` under the sessions root.
4. Each poll, only newly-appended JSONL lines are parsed and surfaced as
   assistant messages (text content only — tool calls and thinking blocks
   are filtered at parse time).
5. User redirects are appended to the agent pane via tmux paste-buffer +
   send-keys Enter.


## Out of scope (v0)

- Any agent other than `pi` or `codex` (errors out at startup)
- zellij (and any non-tmux multiplexer)
- `tmux capture-pane`-based capture (replaced by session-file reading)
- Multi-pane watching
- "Agent done speaking" detection / notifications
- Tool call and thinking block rendering (filtered out at parse time)
- Templates, slash-commands
- Config file

## License

TBD.

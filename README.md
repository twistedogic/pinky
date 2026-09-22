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
PINKY_DEBUG=1 pinky --target %1
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

### Bypass the picker

If you already know which pane you want, pass `--target`:

```sh
pinky --target %12
```

`%12` is the tmux pane id (shown in `tmux display-message -p '#{pane_id}'`).

### Bypass pane discovery

If pinky can't find the session file via `PI_SESSION_FILE` or `lsof`
(e.g. pi running in a wrapper that strips the env, or in a container
without shared `/proc`), point pinky at the file directly:

```sh
pinky --session-file ~/.pi/sessions/abc.jsonl
```

The file format (pi vs. codex JSONL) is auto-detected from the first
line. Combine with `--target` if you want inject back into a specific
pane; otherwise compose stays local.

### As a split-pane

Inside a tmux session, split a pane and start pinky there:

```sh
# bind a hotkey (add to ~/.tmux.conf)
bind-key p split-window -h -l 35% -c "#{pane_current_path}" "pinky"

# then: prefix + p   →   opens pinky; picker appears
```

Pinky works the same regardless of where it's launched — the picker is
always shown unless `--target` is passed.

## Keys

### Nav (the main view)

The main view shows the latest complete agent message as rendered
markdown. Within that message:

| Key | Action |
|---|---|
| `j` / `↓` | Next block |
| `k` / `↑` | Previous block |
| `h` / `l` | Move one rune left / right inside the current block |
| `v` | Enter visual mode (toggle; `v` again exits) |
| `Esc` | Exit visual mode |
| `c` | Open the comment composer (anchored to selection, or whole block) |
| `s` | Send any pending comments (retry path after a failed send) |
| `n` | Enter compose mode |
| `r` | Re-poll the agent session |
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle the keymap page |

### Compose

| Key | Action |
|---|---|
| `Enter` | Newline |
| `i` | Toggle "include comments" appendix on the next send |
| `s` | Send the redirect (appends comments appendix when toggle is on) |
| `Esc` | Cancel and return to nav |
| `?` | Toggle the keymap page |

### Comment composer

| Key | Action |
|---|---|
| `Enter` | Save comment and send all accumulated comments in one batch |
| `Esc` | Cancel |

### Picker

| Key | Action |
|---|---|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Select |
| `q` | Quit |
| `Ctrl+C` | Quit |

The currently-focused block carries a cyan `▍` left-gutter on every
rendered line within its `StartLine..EndLine` range; commented
blocks additionally receive a yellow gutter tint and a footnote line
below the block.

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

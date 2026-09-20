# pinky

A persistent TUI for capturing the last message from an AI coding agent
(`pi` or `codex`) running in a tmux pane, and injecting precise push-backs
back into the agent's input.

```
┌──────────────────────────┬─────────────────────────┐
│   Agent (pi / codex)     │   pinky (split, right)  │
│                          │                         │
│   agent streams...       │   tails agent session   │
│                          │   file, shows messages  │
│                          │   + your redirects      │
│                          │                         │
│                          │   ─ compose (Ctrl+N) ─  │
│                          │   multi-line input      │
│   agent input ▌          │   Ctrl+S to send        │
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
│                          │   ─ compose (Ctrl+N) ─  │
│                          │   multi-line input      │
│   agent input ▌          │   Ctrl+S to send        │
└──────────────────────────┴─────────────────────────┘
```

## Requirements

- Go 1.22+ (built against 1.26.4)
- `tmux` 3.0+ on `PATH`
- `lsof` on `PATH` (for codex session discovery)
- An agent process running in the target pane: **`pi`** or **`codex`**. Other agents error out at startup.

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

| Key | Action |
|---|---|
| `Ctrl+N` | Enter compose mode |
| `Enter` | Newline (in compose) |
| `Ctrl+S` | Send the redirect to the agent |
| `Esc` | Cancel compose |
| `Ctrl+R` | Manual refresh: clear scrollback and re-capture |
| `PgUp` / `PgDn` | Scroll scrollback |
| `Ctrl+C` | Quit |

## History

Every captured agent line and every sent redirect is appended to:

- `$XDG_DATA_HOME/pinky/history.jsonl`, or
- `~/.local/share/pinky/history.jsonl`

On startup, prior history for the target pane is re-seeded into the
scrollback view.

## How it works

1. Pinky reads `#{pane_pid}` from the target tmux pane and walks its
   descendant processes (`pgrep -P`, `ps -c -o comm=`) until it finds
   one named `pi` or `codex`.
2. For `pi`, it reads the `PI_SESSION_FILE` env var from the pi process
   (`/proc/<pid>/environ` on Linux, `ps wwE` on macOS) and tails that
   JSONL file.
3. For `codex`, it uses `lsof -p <pid>` to find the open session JSONL
   under `~/.codex/sessions/` and tails that.
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
- Markdown rendering, templates, slash-commands
- Config file

## License

TBD.

# gen-commit-msg — UX spec

Date: 2026-05-11
Product spec: docs/leyline/specs/2026-05-11-gen-commit-msg-design.md
Surfaces: cli-only

UX spec approved - round 4 - 2026-05-11

## Surfaces enumerated

| Surface | Type | Purpose |
|---------|------|---------|
| `gen-commit-msg` | CLI command | Single entry point, no subcommands |
| TUI: spinner | bubbletea view | "Generating commit messages..." with animated spinner |
| TUI: result list | bubbletea view | Interactive scrollable list of commit message variants |
| TUI: pause overlay | bubbletea view | "Press any key to exit..." on error/exit |
| stdout | text output | Selected commit message (or direct output in quiet mode) |
| stderr | text output | Error messages, log output (when `--log-file -`) |
| Log file | file output | Structured log lines (when `--log-file <path>`) |

## User flows

### Flow 1 — Interactive generation (default)

1. User runs `gen-commit-msg` in a git repo with staged changes
2. Tool checks for staged files (no visible output)
3. Tool installs agent (if needed per `--install-agent`), starts opencode server
4. TUI displays spinner: `Generating commit messages...` with animated indicator
5. Tool creates session, sends prompt to opencode
6. Spinner continues while opencode generates responses
7. TUI transitions to result list: commit message variants with subject (first line) visible
8. User navigates with Up/Down arrows; selected item highlighted (`>` prefix, inverted colors)
9. User presses Enter to select
10. Selected commit message prints to stdout
11. Tool deletes session, stops opencode server
12. If `--pause on` or `--pause on-error` with error: overlay `Press any key to exit...`
13. Exit 0

Failure paths:
- **Not in git repo**: `Error: not a git repository` → stderr, exit 1
- **No staged files**: silent exit 0
- **opencode not found**: `Error: opencode not found. Is it installed?` → stderr, exit 1
- **Server fails to start (timeout)**: `Error: opencode server failed to start (no response after 30s)` → stderr, exit 1
- **Server fails to start (process exited)**: `Error: opencode server exited unexpectedly (exit code <N>)` → stderr, exit 1
- **Generation fails (auth)**: `Error: opencode returned 401 — is opencode authenticated?` → stderr, exit 1
- **Generation fails (timeout)**: `Error: request timed out after 60s` → stderr, exit 1
- **Generation fails (other API error)**: `Error: opencode returned <status>: <message>` → stderr, exit 1
- **Generation returns empty**: `Error: no commit messages generated` → stderr, exit 1

### Flow 2 — Quiet mode (`--quiet`)

1. User runs `gen-commit-msg --quiet`
2. Progress messages (server startup, request sending) and spinner are suppressed
3. If `--subject-count > 1`: interactive selection list is shown normally (quiet does not suppress it)
4. If `--subject-count 1`: result prints to stdout directly
5. Server stopped, session deleted
6. `--pause` behaves identically to non-quiet mode
7. Exit 0 (or error exit as in Flow 1)

### Flow 3 — Single variant, no body (`--subject-count 1 --body false`)

1. User runs with `-n 1 --body false`
2. TUI may show spinner briefly (unless `--quiet`)
3. Only one commit message requested, so no interactive list needed
4. Result printed to stdout
5. Exit 0

### Flow 4 — Version (`--version`, `-V`)

1. User runs `gen-commit-msg --version`
2. `gen-commit-msg <version>` printed to stdout
3. Exit 0
4. No server started, no TUI.

### Flow 5 — Help (`--help`, `-h`)

1. User runs `gen-commit-msg --help`
2. Usage text printed to stdout
3. Exit 0
4. No server started, no TUI.

### Flow 6 — No TTY

**Case A: `--subject-count > 1`**

1. User runs `gen-commit-msg` in a non-TTY context (CI, pipe, `$TERM=dumb`) with default `--subject-count 5`
2. Tool detects non-TTY before starting server
3. `Error: --subject-count > 1 requires an interactive terminal. Use --subject-count 1 for non-interactive mode.` → stderr
4. Exit 1
5. No server started, no TUI.

**Case B: `--subject-count 1`**

1. User runs `gen-commit-msg --subject-count 1` in a non-TTY context
2. Tool checks staged files, starts server silently (no progress output)
3. Creates session, sends prompt, waits for response
4. Result printed to stdout
5. Session deleted, server stopped
6. Exit 0
7. No TUI, no progress messages. `--quiet` is a no-op in this mode (nothing to suppress). `--pause` is a no-op in non-TTY mode (no TUI overlay to render).

## State matrix

| Surface | Empty | Loading | Error | Timeout | Success |
|---------|-------|---------|-------|---------|---------|
| CLI entry (no server/TUI) | No staged files: silent exit 0 | N/A — loading triggers TUI | Error on stderr, exit 1 | N/A | Version/help: text on stdout, exit 0 |
| TUI: spinner | N/A — only shown during loading | `Generating commit messages...` ⠋ | `Error: <msg>` shown, pause overlay if configured | `Error: opencode server failed to start (no response after 30s)` | Transition to result list |
| TUI: result list | N/A — would be error state (no results) | N/A — loading shown by spinner | `Error: <msg>` | N/A | List of variants with `>` selection cursor |
| TUI: pause overlay | N/A | N/A | `Press any key to exit...` | N/A | `Press any key to exit...` (if `--pause on`) |
| stdout | N/A — empty success exits silently | N/A | N/A | N/A | Commit message text, no framing |
| stderr | N/A | N/A | `Error: <message>` | `Error: <message>` | N/A (or log output if configured) |
| Log file | N/A | `{"time":"...","level":"INFO","msg":"starting server"}` | `{"time":"...","level":"ERROR","msg":"..."}` | `{"time":"...","level":"WARN","msg":"server readiness timeout"}` | `{"time":"...","level":"INFO","msg":"done"}` |

## Voice and tone

Three reference strings:

- **Error**: `Error: opencode server failed to start. Check 'opencode --version' and try again.`
- **Success (stdout)**: the commit message itself, e.g.:
  ```
  feat: add git diff retrieval for staged changes
  
  Uses git diff --staged to collect changes and passes them to the
  opencode agent for commit message generation.
  ```
- **Empty (no staged files)**: no output at all. Exit 0 silently.

## Output accessibility

- **Color independence**: spinner and TUI use only text characters. List selection indicated by `>` prefix and terminal inversion, not color alone. stdout output is plain text.
- **Screen-reader friendly**: error messages prefixed with `Error: `; all output is plain text. No ANSI escape sequences in stdout/stderr output (output can be piped).
- **Terminal width**: TUI adapts to terminal width. Commit message subject line wraps at terminal edge. Minimum 40 columns; below that the list truncates with `...`.
- **Motion**: spinner uses standard character cycling (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`). No flashing or rapid animation.
- **Focus management**: Up/Down arrows navigate list. Enter selects. Esc / Ctrl+C exits (treated as SIGINT → graceful shutdown).

## Platform / harness constraints

- **OS**: Linux, macOS. Windows support is a non-goal for v1.
- **Terminal**: any terminal supporting ANSI escape sequences (bubbletea requirement). Tested against xterm-256color, tmux, st, kitty, iTerm2.
- **Go version**: 1.22+
- **Dependencies**: bubbletea + bubbles/spinner (no native extensions required)
- **opencode CLI**: must be on PATH. Version-independent (any opencode that supports `serve --hostname --port` and the SDK v0.19.2 API).

## Non-goals

- No multi-line editing of the commit message in TUI
- No git hook installation
- No custom color themes or TUI configuration
- No paging (pipe to `less` manually if needed)
- No shell completion scripts in v1
- No clipboard integration
- No Windows support in v1

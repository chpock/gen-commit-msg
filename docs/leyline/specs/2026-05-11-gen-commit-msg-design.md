# gen-commit-msg — product spec

Date: 2026-05-11
Author: chpock
Surfaces: cli-only

Product spec approved - round 2 - 2026-05-11

## Problem

Generating high-quality git commit messages manually is tedious. opencode can generate them from the git diff, but there is no convenient CLI tool that manages the opencode server lifecycle, configures a dedicated agent, and provides an interactive TUI for selecting among generated variants.

## Goals

- Check for staged git changes; if none — exit immediately with no action
- Start `opencode serve --hostname 127.0.0.1 --port 0`, parse stdout for a line matching `opencode server listening on http://127.0.0.1:<port>` with a 30-second timeout, and guarantee shutdown on exit (SIGTERM/SIGINT)
- Idempotently create an agent `.md` file at `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/agents/gen-commit-msg.md`
- Create an opencode session and prompt it to generate commit messages, passing `--subject-count` and `--body` as prompt parameters (opencode accesses the git diff on its own)
- Delete the session and shut down the opencode server process after completion
- Display a TUI with a spinner during generation, then an interactive list of variants (subject + optional body)
- On user selection, output the chosen message to stdout
- Configurable via CLI flags and environment variables with clear precedence (flag > env > default)
- Autodetect non-TTY context: if not a terminal and `--subject-count > 1`, error with a message suggesting `--subject-count 1`

## Non-goals

- No `--model` flag (use whatever model the opencode server is configured for)
- No `--extra` prompt flag
- No writing to `.git/COMMIT_EDITMSG` (output to stdout only)
- No config file support (.env, yaml, toml, etc.)
- No git hook integration
- No automatic diff passing to opencode; the tool relies on opencode's built-in git diff access and passes only prompt parameters

## Constraints

- Go 1.22+
- `opencode` CLI must be installed and on `PATH`
- Must run inside a git repository
- Dependencies: `github.com/sst/opencode-sdk-go` v0.19.2, `github.com/charmbracelet/bubbletea` + `bubbles/spinner`
- CLI flag parsing via `spf13/pflag`
- Logging via `log/slog` (stdlib)
- Module path: `github.com/chpock/gen-commit-msg`

## Approaches considered

### Approach A — Monolithic `main.go` + flat helpers

All logic in one `main.go` with helper functions in the same package. Minimal files, minimal abstraction.

Trade-offs:
- Cost: low (fast to write)
- Risk: medium (hard to test and extend)
- Fit: low (breaks Go community conventions)
- Reversibility: low (refactoring a monolith is expensive)

### Approach B — Layered `internal/` packages (recommended)

```
cmd/gen-commit-msg/main.go   — entry point
internal/
  server/      — start/stop opencode serve process
  agent/       — create/verify agent .md file
  git/         — check staged files, get git diff
  opencode/    — API client (Session.New, Prompt, Command)
  tui/         — bubbletea model (steps, message selection)
  config/      — CLI flags and env var parsing
```

Each package has an interface and tests. `main.go` only wires components.

Trade-offs:
- Cost: medium (more files, but each is simple)
- Risk: low (each component isolated and testable)
- Fit: high (standard Go practice)
- Reversibility: high (replace any component independently)

### Approach C — Cobra CLI + Viper config

Full CLI framework with subcommands, config files, environment variable binding.

Trade-offs:
- Cost: high (framework boilerplate, over-engineered)
- Risk: low (Cobra is stable)
- Fit: low (tool does one thing, no subcommands needed)
- Reversibility: medium (Cobra permeates the code)

## Recommendation

**Approach B** — best balance of structure and simplicity. Standard Go layout, testable, minimal dependencies. CLI flags via `spf13/pflag`, logging via `log/slog`.

## CLI flags

Priority: CLI flag > env var > default.

| Flag | Short | Env var | Values | Default | Description |
|------|-------|---------|--------|---------|-------------|
| `--version` | `-V` | — | — | — | Print version and exit |
| `--help` | `-h` | — | — | — | Print help and exit |
| `--log-level` | `-l` | `GCM_LOG_LEVEL` | debug, info, warn, error | error | Log verbosity |
| `--log-file` | | `GCM_LOG_FILE` | path, `-` for stdout | (stderr) | Log output destination |
| `--subject-count` | `-n` | `GCM_SUBJECT_COUNT` | 1..N | 5 | Number of subject line variants to request |
| `--body` | | `GCM_BODY` | true, false | true | Whether to generate message body |
| `--quiet` | `-q` | `GCM_QUIET` | true, false | false | Suppress progress messages and spinner (not the selection list) |
| `--agent` | `-a` | `GCM_AGENT` | string | gen-commit-msg | opencode agent name |
| `--pause` | | `GCM_PAUSE` | on, off, on-error | on-error | Pause before exit behavior |
| `--install-agent` | | `GCM_INSTALL_AGENT` | always, if-not-exists, no | if-not-exists | Agent installation behavior |

Server hostname (`127.0.0.1`) and startup timeout (30s) are constants in the `server` package — not exposed as flags.

`--quiet` suppresses only progress output (server startup messages, request-sending status, spinner). It does NOT suppress the interactive subject selection list when `--subject-count > 1`. It does NOT affect `--pause` behavior.

## Agent .md prompt

```
You are a git commit message generator. Your task is to generate commit messages for the current git repository.

Rules:
- Output commit messages (both subject line and body) based on the git diff
- First line: subject (50-72 chars, imperative mood, lowercase, no period)
- Include a body if the diff warrants explanation
- Follow the conventional commits style if the diff clearly matches a type
  (feat, fix, refactor, docs, test, chore, style, perf, ci, build)
- Otherwise, use a plain descriptive subject
- Do not include any additional explanations, markdown formatting, code blocks,
  or backticks in the output
```

## Success criteria

- `go build ./cmd/gen-commit-msg` produces a working binary
- Running in a git repo with staged changes: starts server, shows TUI, generates messages
- Running in a git repo with no staged changes: exits silently
- `--subject-count 1 --body false`: returns exactly one subject line, no body
- Server starts within 30s timeout by parsing stdout for the listening URL; guarantees stop on process exit (SIGTERM/SIGINT handled)
- Agent file is created idempotently (not overwritten if it exists, unless `--install-agent always`)
- All flags have corresponding env var overrides with correct precedence
- Running without a TTY and `--subject-count > 1`: errors with a clear message suggesting `--subject-count 1`

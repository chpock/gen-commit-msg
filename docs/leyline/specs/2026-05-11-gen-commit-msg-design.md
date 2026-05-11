# gen-commit-msg — product spec

Date: 2026-05-11
Author: chpock
Surfaces: cli-only

Product spec approved - round 1 - 2026-05-11

## Problem

Generating high-quality git commit messages manually is tedious. opencode can generate them from the git diff, but there is no convenient CLI tool that manages the opencode server lifecycle, configures a dedicated agent, and provides an interactive TUI for selecting among generated variants.

## Goals

- Check for staged git changes; if none — exit immediately with no action
- Start a local `opencode serve` process on a random port, wait for readiness, and guarantee shutdown on exit (SIGTERM/SIGINT)
- Idempotently create an agent `.md` file at `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/agents/gen-commit-msg.md`
- Create an opencode session and prompt it to generate commit messages (opencode accesses the git diff on its own)
- Delete the session and shut down the opencode server process after completion
- Display a TUI with a spinner during generation, then an interactive list of variants (subject + optional body)
- On user selection, output the chosen message to stdout
- Configurable via CLI flags and environment variables with clear precedence (flag > env > default)

## Non-goals

- No `--model` flag (use whatever model the opencode server is configured for)
- No `--extra` prompt flag
- No writing to `.git/COMMIT_EDITMSG` (output to stdout only)
- No config file support (.env, yaml, toml, etc.)
- No git hook integration

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
| `--quiet` | `-q` | `GCM_QUIET` | true, false | false | Suppress TUI (output result directly) |
| `--agent` | `-a` | `GCM_AGENT` | string | gen-commit-msg | opencode agent name |
| `--pause` | | `GCM_PAUSE` | on, off, on-error | on-error | Pause before exit behavior |
| `--install-agent` | | `GCM_INSTALL_AGENT` | always, if-not-exists, no | if-not-exists | Agent installation behavior |

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
- `--quiet` flag: skips TUI, outputs result to stdout
- `--subject-count 1 --body false`: returns exactly one subject line, no body
- Server is guaranteed to stop on process exit (SIGTERM/SIGINT handled)
- Agent file is created idempotently (not overwritten if it exists, unless `--install-agent always`)
- All flags have corresponding env var overrides with correct precedence

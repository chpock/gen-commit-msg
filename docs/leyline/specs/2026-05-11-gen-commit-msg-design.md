# gen-commit-msg — product spec

Date: 2026-05-11
Author: chpock
Surfaces: cli-only

Product spec approved - round 4 - 2026-05-11

Deep-discovery pass complete - round 4 - 2026-05-11

Current harness and lifecycle requirements: [Pi and OpenCode harness support](2026-10-24-pi-harness-design.md). Its approved D1–D13 contract supersedes the original OpenCode-only orchestration. Historical approach comparisons below are retained as design history.

## Problem

Generating high-quality git commit messages manually is tedious. opencode can generate them from the git diff, but there is no convenient CLI tool that manages the opencode server lifecycle, configures a dedicated agent, and provides an interactive TUI for selecting among generated variants.

## Goals

- Check for staged git changes; if none — exit immediately with no action
- Start `opencode serve --hostname 127.0.0.1 --port 0`, parse stdout for a line matching `opencode server listening on http://127.0.0.1:<port>` with a 30-second timeout, then issue a lightweight API request to confirm the server is ready before proceeding. On parse failure, include the actual server output in the error message.
- Set `Pdeathsig: syscall.SIGKILL` on the opencode child process so the server dies with the parent even if cleanup code cannot run
- Handle SIGINT and SIGTERM signals for graceful shutdown (session deletion, server stop) before exit
- Each selected backend owns cleanup before returning results on success, failure or cancellation. OpenCode session deletion retains its 10-second TTY/five-second non-TTY deadline; server stop uses SIGTERM, up to five seconds, SIGKILL if needed and completed waiting. SIGKILL session leaks remain an accepted risk
- Idempotently create an agent `.md` file at `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/agents/<agent-name>.md` (the agent name comes from `--agent`, default `gen-commit-msg`)
- Select `auto|pi|opencode`, preferring an executable Pi in auto mode without fallback after its errors. Use a sessionless Pi RPC process or the existing OpenCode session/API path.
- Collect staged-change context in the application and pass its JSON plus subject/body parameters to either backend; OpenCode retains native structured output
- Delete the session and shut down the opencode server process after completion
- Display the selected backend's real progress steps, finish cleanup, then show the existing inline subject selection list; single candidates auto-select
- On user selection, output the chosen message to stdout
- Configurable via CLI flags and environment variables with clear precedence (flag > env > default)
- Autodetect non-TTY context: if not a terminal and `--subject-max > 1`, error with a message suggesting `--subject-max 1`. If not a terminal and `--subject-max 1`, run generation silently and print the result to stdout — no TUI, no progress output

## Non-goals

- No `--model` flag; model/provider/authentication remain configured through the selected harness
- No `--extra` prompt flag
- No implicit writing to `.git/COMMIT_EDITMSG`; an explicit `--output` destination follows the output-flag specification
- No config file support (.env, yaml, toml, etc.)
- No git hook integration
- No agent-side Git collection: preserve the application's versioned context collector and staged-only scope

## Constraints

- Go 1.22+
- The selected harness must be on PATH: Pi 1.4.0+ or the existing supported OpenCode API/serve CLI. Git remains required. Native Pi search tools additionally need cached/system rg and fd/fdfind as described in the harness spec
- Must run inside a git repository
- Dependencies: `github.com/sst/opencode-sdk-go` (latest version), `github.com/charmbracelet/bubbletea` + `bubbles/spinner` + `bubbles/list`
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
  git/         — check git repo and staged files
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
| `--log-level` | `-l` | `GCM_LOG_LEVEL` | none, debug, info, warn, error | none | Log verbosity |
| `--log-file` | | `GCM_LOG_FILE` | path | (stderr) | Log output destination |
| `--subject-min` | `-m` | `GCM_SUBJECT_MIN` | 1..N | 1 | Minimum number of subject line variants to request |
| `--subject-max` | `-x` | `GCM_SUBJECT_MAX` | 1..20 | 5 | Maximum number of subject line variants to request |
| `--body` | | `GCM_BODY` | true, false | true | Whether to generate message body |
| `--harness` | | `GCM_HARNESS` | auto, pi, opencode | auto | Select generation harness; auto prefers Pi |
| `--quiet` | `-q` | `GCM_QUIET` | true, false | false | Suppress progress messages and spinner (not the selection list) |
| `--agent` | `-a` | `GCM_AGENT` | string | gen-commit-msg | OpenCode-only agent name; ignored by Pi |
| `--pause` | | `GCM_PAUSE` | on, off, on-error | on-error | Pause before exit behavior |
| `--install-agent` | | `GCM_INSTALL_AGENT` | always, if-not-exists, no | if-not-exists | OpenCode-only agent installation behavior; ignored by Pi |
| `--output` | `-o` | `GCM_OUTPUT` | path | (stdout) | Write commit message to file instead of stdout. See `docs/leyline/specs/2026-05-15-output-flag-design.md` for full specification. |

OpenCode server hostname/startup and HTTP-client timeout remain unchanged. Pi uses 30 seconds for version/startup/readiness, independent 120-second prompt operations and five seconds of shutdown grace. These policies are constants, not new flags; see the harness spec.

`--quiet` suppresses only progress output (server startup messages, request-sending status, spinner). It does NOT suppress the interactive subject selection list when `--subject-max > 1`. It does NOT affect `--pause` behavior.

`--agent <name>` changes the agent file path to `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/agents/<name>.md` and the opencode agent name used in the session.

`--install-agent` behaviors:
- `if-not-exists` (default): create the agent file with the default prompt only if `<name>.md` does not already exist
- `always`: always overwrite the agent file with the default prompt
- `no`: never create or overwrite the agent file. If opencode returns an error about a missing agent, surface it to the user

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
- Running in a git repo with staged changes selects the requested/auto harness, executes its workflow and shows backend-specific progress before message selection
- Running in a git repo with no staged changes: exits silently
- `--subject-min 1 --subject-max 1 --body false`: returns exactly one subject line, no body
- Server starts within 30s timeout by parsing stdout for the listening URL, followed by a lightweight API health-check; server child process has `Pdeathsig: SIGKILL` set
- SIGINT and SIGTERM trigger graceful shutdown (session deletion, server stop) before exit
- Backend cleanup completes before selection/output on success, failure and cancellation; confirmed results survive cleanup warnings, which remain visible in quiet/non-TTY modes
- Agent file is created idempotently (not overwritten if it exists, unless `--install-agent always`); `--install-agent no` never installs and relies on opencode error for missing agent
- All flags have corresponding env var overrides with correct precedence
- Running without a TTY and `--subject-max > 1`: errors with a clear message suggesting `--subject-max 1`
- Running without a TTY and `--subject-max 1`: generates silently, prints result to stdout, no TUI

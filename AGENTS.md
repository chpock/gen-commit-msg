# AGENTS.md

## Commands

```
make build      # go build -ldflags "-X main.version=dev" -o gen-commit-msg ./cmd/gen-commit-msg
make test       # go test -count=1 -race ./...
make vet        # go vet ./...
make lint       # golangci-lint run ./...  (requires golangci-lint)
make fmt        # go fmt ./...
make clean      # rm -f gen-commit-msg
make all        # fmt → vet → test → build  (correct pre-commit order)
```

## Architecture

Single-binary Go CLI. Entrypoint: `cmd/gen-commit-msg/main.go`.

```
cmd/gen-commit-msg/main.go    — orchestrator (parse config, check git, start OpenCode server, run TUI)
internal/config/              — flag/env parsing via pflag; flags override GCM_* env vars
internal/git/                 — execs `git rev-parse --git-dir` and `git diff --staged --quiet`
internal/agent/               — writes agent prompt .md files to ~/.config/opencode/agents/
internal/server/              — spawns `opencode serve`, parses listen URL from stdout, health-checks
internal/opencode/            — SDK client for OpenCode sessions (create, prompt, parse JSON, delete)
internal/tui/                 — Bubble Tea TUI: spinner → list of commit messages → selection
```

## Config precedence

CLI flag > `GCM_*` env var > default. Key env vars: `GCM_SUBJECT_COUNT`, `GCM_BODY`, `GCM_QUIET`, `GCM_AGENT`, `GCM_LOG_LEVEL`, `GCM_LOG_FILE`, `GCM_PAUSE`, `GCM_INSTALL_AGENT`.

## Runtime dependencies

- `opencode` binary must be in PATH — this tool starts an OpenCode server process
- `git` must be available and the CWD must be inside a git repo with staged files
- Interactive TUI mode requires a TTY (stdout); non-TTY falls back to `--subject-count 1` output

## Testing

Standard Go tests, no external services needed. Run a single package:
```
go test -count=1 -race ./internal/config/
```

## Language

English only. All source code, comments, log messages, error output, and user-facing strings must be in English. No other locale is supported.

## Logging

Use `log/slog` (stdlib). Every significant action must be logged: inputs, outputs, decisions, errors. A reader should be able to reconstruct the app's execution flow from the log alone.

- `DEBUG`: fine-grained state transitions, intermediate data used for decisions
- `INFO`: lifecycle events (server started, session created, generation requested, generation completed, session deleted, server stopped)
- `WARN`: recoverable anomalies (invalid env var value, non-critical API error)
- `ERROR`: all failures that affect the outcome, including errors propagated through return values

All errors — including those caught and handled (e.g., session cleanup failures on shutdown) — must appear in the log. Unhandled exceptions (panics) must be recovered and logged at ERROR level before exit.

Configure via `--log-level` (debug/info/warn/error) and `--log-file` (path, or `-` for stdout). Default: `error` level, output to stderr.

## Commit message conventions

This project follows Conventional Commits: `type(scope): description`. When generating commit messages for this repo or contributing to it, use these types:

| Type | When |
|------|------|
| `feat` | New user-facing feature |
| `fix` | Bug fix |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `docs` | Documentation only |
| `test` | Adding or updating tests |
| `chore` | Build, CI, dependency updates, tooling |
| `style` | Formatting, whitespace (no logic change) |
| `perf` | Performance improvement |
| `ci` | CI pipeline changes |
| `build` | Build system or external dependencies |

### Subject line rules
- 50–72 characters
- Imperative mood, lowercase, no trailing period
- Format: `type(scope): description`
- Scope is optional, be specific and short (e.g. `git`, `config`, `tui`)

### Body rules
- Wrapped at 72 characters
- Explains *what* changed and *why* — not *how* (that's in the diff)
- Separated from subject by one blank line
- Can include bullet points (`- `)
- Optional: may include `BREAKING CHANGE:` footer if applicable

### Examples
```
feat(config): add --subject-count flag
```
```
fix(server): handle opencode process exit during startup

The server process could exit before printing the listen URL. Now
capturing stderr and including it in the error message.
```
```
chore(deps): bump opencode-sdk-go to v0.19.2
```

## Specifications

Project specifications live in `docs/leyline/`:

```
docs/leyline/
├── specs/      — product specs (what and why)
├── design/     — UX specs (user flows, surfaces, states)
└── plans/      — implementation plans and review logs
```

**Development workflow:**
1. Before implementing any feature, read the corresponding spec in `docs/leyline/specs/`
2. If the feature touches a user-facing surface, also read the UX spec in `docs/leyline/design/`
3. Implementation must match the approved spec; if a gap is found, flag it — do not silently diverge
4. **When implementation differs from spec**, update the spec file to reflect the actual behavior. Specs are living documents, not frozen artifacts
5. After completing a task, log review results in `docs/leyline/plans/<plan>-review-log.md`

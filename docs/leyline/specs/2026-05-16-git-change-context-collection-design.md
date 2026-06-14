# Git change context collection - product spec
Date: 2026-05-16
Author: human partner
Surfaces: cli-only

## Problem
The CLI currently uses git primarily to check whether staged files exist.
This weak context lowers commit message quality because the agent does not
receive a structured, high-signal representation of staged changes.

## Goals
- Add a first pipeline step before starting the OpenCode server: collecting
  information about current changes.
- Run a fixed set of git commands and capture real output, stderr, and exit
  code for each command.
- Build a single JSON context payload in the agreed format and pass it into
  the generation prompt.
- Truncate any command output longer than 200000 bytes and set
  `truncated: true` with a `truncation` object.
- Execute git commands with these additional arguments:
  `--no-pager -c color.ui=false -c core.quotepath=false`.
- Extend logging:
  - INFO: command execution started.
  - DEBUG: short completion summary per command.
  - TRACE: full command and full captured result payload.

## Command contract
- Commands run in this exact order and are grouped into sections:
  - staged_changes:
    1. `git diff --cached --name-status --find-renames --find-copies`
    2. `git diff --cached --stat --find-renames --find-copies --compact-summary`
    3. `git diff --cached --numstat --find-renames --find-copies`
    4. `git diff --cached --summary --find-renames --find-copies`
    5. `git diff --cached --dirstat=files,0 --find-renames --find-copies`
    6. `git diff --cached --no-ext-diff --no-color --find-renames --find-copies --submodule=short`
  - style_context:
    7. `git log -15 --format=%H%n%s%n%n%b%n%x1e`
    8. `git branch --show-current`
- Each command is executed with these additional git args:
  `--no-pager -c color.ui=false -c core.quotepath=false`.
- Required commands (`required=true`) failing (non-zero exit) must stop generation
  and return an error. Optional commands (`required=false`) are recorded in JSON
  with their real error fields and generation continues.
- Cancellation:
  - when parent context is canceled, command execution stops and collection
    returns a canceled error.
  - no dedicated per-command or full-step timeout is introduced in this round.

## JSON contract
- `format_version` is required and pinned to `1.0` for this feature.
- Top-level keys:
  - `format_version`
  - `staged_changes.outputs`
  - `style_context.outputs`
- Each command output object contains:
  - `id`, `command`, `description`, `required`, `output`, `stderr`,
    `exit_code`, `truncated`
  - optional `truncation` when `truncated=true`
- `command` in JSON stores the logical git command without the additional
  execution-only args.
- Truncation rules:
  - apply to `output` field only with max 200000 bytes.
  - `stderr` is captured as-is in this round.
  - strategy is `head_tail`.
  - `truncation.original_bytes` is counted before truncation.

## Security and logging guardrails
- Never log full command payloads at INFO or DEBUG levels.
- Full payload logging is allowed only at TRACE.
- Command outputs are copied to prompt context; this is intentional. If future
  secret-redaction requirements are added, they must be implemented as a
  dedicated preprocessing stage and covered by tests.
- TRACE support is required and mapped to the project's existing custom trace
  level implementation.

## Known gaps and follow-ups
- UTF-8 boundary-safe truncation is not enforced in this round. Truncation is
  byte-based and may split multibyte characters.
- Secret redaction is not implemented in this round and remains a future
  dedicated preprocessing stage.

## Operational concerns
- Rollback path: if collection causes reliability issues, bypass by reverting
  the feature commit.
- Ownership: maintainers of `internal/git` and `internal/opencode` own failures
  in this path.
- Runbook minimum:
  - identify failing command ID from logs.
  - if required command fails, fix repository/runtime precondition and rerun.
  - if optional command fails, inspect logs and confirm prompt still contains
    required staged-change outputs.

## Non-goals
- Changing the response schema returned by the AI (`subjects` and `body`).
- Changing commit message selection behavior in the TUI.
- Introducing new CLI flags for context collection customization.

## Constraints
- Keep current single-binary Go CLI architecture.
- Preserve existing git repository and staged-file prechecks.
- Support both TTY and non-TTY modes.
- Keep logs reconstructable from `slog` records.

## Approaches considered
### Approach A - inline in main orchestrator
Implement data collection directly in `cmd/gen-commit-msg/main.go` for both
TTY and non-TTY paths.

Trade-offs: quick but duplicates logic and further increases orchestration
complexity.

### Approach B - dedicated collector in internal/git (recommended)
Add a collector package in `internal/git` with command specs, execution,
truncation, and JSON serialization. Keep `main` focused on orchestration and
prompt wiring.

Trade-offs: slightly more initial code but better separation of concerns,
testability, and maintainability.

### Approach C - collect inside opencode client
Collect git context in `internal/opencode` directly before prompt dispatch.

Trade-offs: fewer orchestration changes but mixes concerns (git/process data
collection vs AI API client behavior).

## Recommendation
Use Approach B. It provides clean layering, deterministic testing for command
execution/truncation, and minimal duplication across TTY and non-TTY flows.

## Open questions
- None blocking. JSON shape, command list, truncation rules, and prompt text
  are fully specified.

## Success criteria
- New pre-server step runs and collects staged-change context.
- JSON payload matches the required structure with real command results.
- Output truncation behavior is implemented exactly for outputs > 200000 bytes.
- Prompt generation uses the new provided template with embedded JSON context.
- INFO/DEBUG/TRACE logging is added as required.
- `make all` passes.
- Required command failure behavior is deterministic and tested.
- Optional command failure behavior is deterministic and tested.
- Output truncation is tested for oversized output with `head_tail` metadata.
- Prompt includes exact JSON context block with versioned schema.

## Approvals
Product spec approved - round 1 - 2026-05-16
Product spec approved - round 2 - 2026-05-16

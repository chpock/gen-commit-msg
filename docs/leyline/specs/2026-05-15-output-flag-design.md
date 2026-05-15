# --output flag — product spec

Date: 2026-05-15
Author: user
Surfaces: cli-only

## Problem

Currently `gen-commit-msg` always writes the selected commit message to stdout. There is no way to direct the output to a file, which is needed for scripting (e.g., `gen-commit-msg --output /tmp/msg && git commit -F /tmp/msg`).

## Goals

- Add `--output` / `-o` flag to specify an output file path
- Add `GCM_OUTPUT` env var as an alternative
- When set, the selected commit message is written to the file instead of stdout
- When not set (default `""`), behavior is unchanged — output goes to stdout
- If the file cannot be created or written, exit with a clear error message

## Non-goals

- Appending to existing files (overwrite only)
- Affecting error/stderr output
- Changing TUI display behavior (TUI uses `/dev/tty`)

## Constraints

- CLI flag > `GCM_OUTPUT` env var > default (empty = stdout)
- The file is written only on successful message selection (not on cancel)
- Path resolution: relative to CWD, same as all other file operations in the tool

## Approaches considered

### Approach A — Single flag, overwrite

Add `--output` flag to config. When non-empty, open/truncate the file after TUI completes and write the result there. On write failure, error and exit.

Trade-offs: simplest, matches user request, zero ambiguity about append vs overwrite.

### Approach B — Append mode variant

Same as A but use `O_APPEND` or always create new.

Trade-offs: more complex, user didn't ask for it, can be added later if needed.

## Recommendation

**Approach A.** Single flag, overwrite semantics, minimal change.

## Changes needed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Output string` field, `--output`/`-o` flag in `initFlags()`, resolve from `GCM_OUTPUT` env in `ParseFlags()` |
| `cmd/gen-commit-msg/main.go` | In TUI path and non-interactive path: open file if `cfg.Output != ""`, write there instead of `os.Stdout` |
| `internal/config/config_test.go` | Add test for `--output` flag resolution, env var, default |

## Success criteria

- `gen-commit-msg --output /tmp/msg.txt` writes the selected message to `/tmp/msg.txt` instead of stdout
- Without `--output`, stdout behavior is unchanged
- `GCM_OUTPUT=/tmp/msg.txt gen-commit-msg` works equivalently
- File write errors are reported clearly and cause exit code 1

## Approvals

Product spec approved - round 1 - 2026-05-15

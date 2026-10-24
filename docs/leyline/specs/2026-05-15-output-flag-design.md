# --output flag — product spec

Date: 2026-05-15
Author: user
Surfaces: cli-only

## Problem

Shell redirection (`gen-commit-msg > /tmp/msg`) already works for scripting because the TUI renders to `/dev/tty` and stderr is separate. However, shell redirection is implicit — a reader of the script must notice the `>` to understand where output goes. An explicit `--output` flag makes intent visible at a glance and follows established CLI conventions (`curl -o`, `gcc -o`).

## Goals

- Add `--output` / `-o` flag to specify an output file path
- Add `GCM_OUTPUT` env var as an alternative
- When set, the selected commit message is written to the file instead of stdout
- When not set (default `""`), behavior is unchanged — output goes to stdout
- If the file cannot be created or written, exit with a clear error message
- Validate the output path early (before either harness starts) so unwritable-path errors surface immediately

## Non-goals

- Appending to existing files (overwrite only)
- Affecting error/stderr output
- Changing TUI display behavior (TUI uses `/dev/tty`)
- Creating parent directories automatically

## Constraints

- CLI flag > `GCM_OUTPUT` env var > default (empty = stdout)
- The file is opened for overwrite only when a non-empty message is ready to be written (not on cancel)
- Existing output contents and permissions are preserved during early validation and whenever no message is produced
- If `--output` is set but no message is produced (no staged files, user cancels selection), no file is created and exit code is 0 — same as current stdout behavior
- Path resolution: relative to CWD, same as all other file operations in the tool
- Parent directory of the output path must exist (no auto-creation)
- Symbolic links are accepted only when their target exists and is writable. A dangling output symlink is rejected during early validation without changing the link or creating its target.
- Early validation: check that the output path is writable after config parsing, before server start. Open existing files without truncation; for absent paths, create a probe exclusively and remove only that probe. Surface probe close/removal failures instead of ignoring them.

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
| `internal/config/config.go` | Add `Output string` field, `--output`/`-o` flag in `initFlags()`, resolve from `GCM_OUTPUT` env in `ParseFlags()`. Add `ValidateOutputPath()` method that checks writability. |
| `cmd/gen-commit-msg/main.go` | After config parsing, call `cfg.ValidateOutputPath()` before server start (both TUI and non-interactive paths). Both TUI and non-interactive paths use `writeSelectedMessage`, which checks for empty selection before opening the output destination, writes the message and newline, and closes the file. Both CLI modes complete backend-owned cleanup through the shared generation operation before selection/output; output failures cannot bypass that cleanup. See `2026-10-24-pi-harness-design.md`. |
| `internal/config/config_test.go` | Add test for `--output` flag resolution, env var, default. Add tests for `ValidateOutputPath` (writable path, non-existent parent, permission denied). |
| `cmd/gen-commit-msg/output_test.go`, `cmd/gen-commit-msg/cli_test.go` | Verify successful creation/overwrite, unchanged existing output and absent new output on empty selection, and surfaced open/write failures. Subprocess tests cover stdout/file routing and preservation on early exits. |

## Success criteria

- `gen-commit-msg --output /tmp/msg.txt` writes the selected message to `/tmp/msg.txt` instead of stdout
- Without `--output`, stdout behavior is unchanged
- `GCM_OUTPUT=/tmp/msg.txt gen-commit-msg` works equivalently
- File write errors are reported clearly and cause exit code 1
- Unwritable paths (non-existent parent dir, permission denied) fail with a clear error before server start
- With `--output` and no staged files or canceled selection, exit code 0, no new file created, and any existing file unchanged
- Early validation preserves existing files, writable symlink targets, hard links, and file permissions

## Approvals

Product spec approved - round 1 - 2026-05-15
Product spec approved - round 2 - 2026-05-15

## Deep-discovery round 1 classification

- (S) Problem statement rewritten: shell redirection acknowledged, --output justified as explicit intent
- (O) Changes table now distinguishes TUI vs non-interactive paths; behavioral tests added
- (O) Non-interactive cleanup documented: file-open failure must call cleanup() before exit
- (O) Early validation added: ValidateOutputPath() before server start
- (O) No-staged-files contract documented: no file created, exit 0
- (E) UX spec updated with GCM_OUTPUT env var table

Deep-discovery pass complete - round 1 - 2026-05-15

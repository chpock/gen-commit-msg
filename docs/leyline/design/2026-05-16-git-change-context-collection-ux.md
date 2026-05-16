# Git change context collection - UX spec
Date: 2026-05-16
Product spec: docs/leyline/specs/2026-05-16-git-change-context-collection-design.md
Surfaces: cli-only

## Commands enumerated
- Primary command surface: `gen-commit-msg` in TTY and non-TTY modes.
- New progress step in the pipeline: `Collecting information about current changes...`
  as step 0 before OpenCode server startup.
- Internal git commands run automatically and are not direct user commands.

## Help / usage text
- No flag changes are required.
- Default runtime behavior adds a pre-generation context collection stage.
- Failures must identify the command that failed and why generation cannot
  continue.

## Error and progress output formatting
- TTY progress now has five steps:
  1. Collecting information about current changes...
  2. Starting OpenCode...
  3. Creating session...
  4. Generating commit messages...
  5. Cleaning up OpenCode resources...
- If context collection fails, remaining steps are marked as skipped.
- Failure detail is concise and command-specific.
- Non-TTY mode remains non-interactive and prints only final output/errors.
- Required command failures stop generation; optional command failures do not.
- For required command failures, the surfaced error includes command ID and exit
  code from the collector error chain.

## Voice and tone
- Error: "Failed to collect information about current changes"
- Success: "Collecting information about current changes... done"
- Empty state: "No staged files found"

## Output accessibility
- Status meaning must not depend only on color.
- Messages should remain readable on standard terminal widths.
- Disable git color output with `-c color.ui=false` for stable text rendering.
- Disable quoted path escaping with `-c core.quotepath=false` for human-readable paths.

## Exit codes and their meanings
- 0: success or early no-staged-files exit.
- 1: runtime failure including context collection failure.
- 2: CLI usage/configuration error.

## Logging surface expectations
- INFO records command execution starts.
- DEBUG records compact completion data (`id`, `exit_code`, `truncated`, sizes).
- TRACE may include full command and full captured payload.
- Truncation metadata is emitted only when `output` exceeds 200000 bytes; stderr
  is not truncated in this round.

## Non-goals
- User-configurable command sets for context collection.
- Changes to selected commit message output formatting.
- Changes to OpenCode response schema.

## Approvals
UX spec approved - round 1 - 2026-05-16
UX spec approved - round 2 - 2026-05-16

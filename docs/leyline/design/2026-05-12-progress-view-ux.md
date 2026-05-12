# Progress View for Backend Steps - UX spec
Date: 2026-05-12
Product spec: docs/leyline/specs/2026-05-12-progress-view-design.md
Surfaces: single-screen-ui

UX spec approved - round 1 - 2026-05-12

## Surfaces enumerated
- **Progress view**: Vertical list of 5 steps with status indicators (pending/running/done/failed). Shown immediately when TUI starts, before message selection.
- **Message selection view**: Existing list of commit message subjects with ↑↓ navigation and Enter to select.
- **Error view**: Error message with "Press any key to exit" prompt. Shown on generation failure or progress step failure.

## User flows

### Flow 1 — Successful generation
1. User runs `gen-commit-msg` with staged files
2. Progress view appears — all 5 steps listed, all pending
3. Steps 1→5 complete in order: pending → running (spinner) → done (✓)
4. After step 5 completes, auto-transition to message selection view
5. User navigates with ↑↓, selects with Enter → message printed to stdout → program exits
6. (Single message: auto-selected, skips selection view)
7. (Zero messages: error view shown)

### Flow 2 — Progress step failure
1. Progress view appears — all 5 steps pending
2. A step transitions: pending → running → failed (✗)
3. Error message appears below the step list
4. Remaining pending steps stay dimmed (no further execution)
5. User presses any key → program exits with error code 1
6. Cleanup (session delete, server stop) runs silently during exit

### Flow 3 — Generation failure (step 3)
1. Steps 1-2 complete successfully (✓)
2. Step 3 transitions: running → failed (✗)
3. Error message appears below the step list
4. Steps 4-5 stay dimmed as pending
5. User presses any key → program exits with error code 1
6. Cleanup (session delete, server stop) runs silently

## State matrix

| Surface | Loading | Error | Success | Empty |
|---------|---------|-------|---------|-------|
| Progress view | All 5 steps shown; current step shows spinner; completed steps show ✓; pending steps dimmed | Failed step shows ✗; error text below step list; remaining steps dimmed; any-key-to-exit | All steps show ✓; auto-transition to message selection after 300ms | N/A — steps always present |
| Message selection | N/A — preceded by progress view | N/A — errors go to error view | List of messages; ↑↓ to navigate; Enter to select; selected message printed to stdout | N/A — handled as error ("no messages generated") |
| Error view | N/A — errors are immediate | Error message text; "Press any key to exit." prompt | N/A — error view is terminal | N/A |

Permission-denied, Offline: N/A — local CLI tool, no network auth required.

## Voice and tone
Three reference strings:
- **Error**: `Error: opencode server failed to start (no response after 30s)`
- **Success**: *(silent — selected message is printed to stdout; no success banner)*
- **Empty state**: `Error: no commit messages generated`

Voice is direct, technical, English-only. No emoji, no exclamation marks, no personal pronouns.

## Accessibility targets
- **WCAG level**: N/A (terminal TUI — text-based by nature; color independence is the primary concern)
- **Keyboard flow**: Ctrl+C / Esc exits at any point. ↑↓ to navigate message list. Enter to select. Any key to dismiss error.
- **Screen reader**: Step labels are plain text; status indicators (✓ / ✗) are ASCII characters readable by screen readers.
- **Motion**: Single spinner character updates — already minimal motion. No animations beyond spinner tick.
- **Color independence**: Status is conveyed by characters (✓, ✗, dimming) not color alone. The spinner is positional — no color dependency.

## Platform / harness constraints
- Terminal width: minimum 40 columns (existing constraint)
- Framework: bubbletea + bubbles (spinner, list)
- Output: `/dev/tty` for TUI; stderr fallback when TTY unavailable
- OS: Linux, macOS (existing platform support)

## Non-goals
- Non-TTY mode progress output (stays silent as today)
- Retry mechanism on step failure
- Progress percentage or time estimates
- Customizable step labels
- Step timing information
- Parallel step execution display

# Progress view for generation steps — UX spec

Date: 2026-05-12
Current harness contract: 2026-10-24
Product: `../specs/2026-05-12-progress-view-design.md`.
Harness UX: `2026-10-24-pi-harness-ux.md`.
Original UX approval: round 3, 2026-05-12.

## Surfaces

- Progress: the selected backend's full step list, status indicators and inline detail, shown before message selection.
- Selection: the existing inline subjects-only list, no alternate screen, status bar or key-binding help; one line per item.
- stdout/output file: only the final selected message.
- stderr: errors and warnings when progress is unavailable; never Pi protocol records.

## Initial lists

OpenCode:

1. Collecting information about current changes...
2. Starting OpenCode...
3. Creating session...
4. Generating commit messages...
5. Cleaning up OpenCode resources...

Pi:

1. Collecting information about current changes...
2. Starting Pi...
3. Generating commit messages...
4. Stopping Pi...

All labels are visible initially. Preserve existing pending/running/done/failed/interrupted/warning/skipped indicators and styles. Native corrective prompts update generation detail rather than adding lifecycle steps. Pi readiness completes before its startup checkmark.

## Successful flow

1. Show the selected list before work proceeds.
2. Complete collection and generation in dependency order, reporting real statuses.
3. Finish backend-owned cleanup before presenting candidates.
4. Preserve the existing 300ms successful transition, then clear progress and show the inline selection list.
5. Up/Down navigate; Enter selects and clears the list before output. A single candidate auto-selects without displaying a list.

The final output format and existing list coloring remain unchanged. No success banner or extra harness framing.

## Failure and cleanup

On failure, display concise operation-specific detail and skip dependent operations. Cleanup still runs for acquired resources and reports its actual outcome. Keep the original generation error as primary if cleanup also fails. Rich backend errors retain their detailed rendering without making the view OpenCode-specific.

After terminal completion, show the error using the existing pause/dismissal behavior. Zero results produce `Error: no commit messages generated`. Output destinations remain unchanged on failure.

When generation succeeded but cleanup produced a warning, display `Cleanup issue: <detail>` with the warning indicator and keep candidates available. Quiet/non-TTY callers receive the warning on stderr. If the TUI was canceled or failed before cleanup ended, later cleanup warnings still reach stderr. All warnings are logged; none enter commit-message stdout/file. Successful selection/output still exits 0.

## Quiet, non-TTY and cancellation

Quiet hides progress, not selection or warnings. Non-TTY single-result mode uses the same workflow without a TUI and emits output only after cleanup. Non-TTY multi-result requests retain the existing early rejection.

Esc/Ctrl+C cancel using the existing interaction behavior. Cancellation produces no message and does not skip resource cleanup. Resource handling remains outside the TUI; main waits for the shared operation after the UI returns if necessary.

## Accessibility and constraints

- Keep text/structural status indicators in addition to color and the existing Unicode/SGR handling.
- Preserve existing keyboard behavior, minimum 40-column handling, spinner motion and inline-list dimensions.
- TUI uses `/dev/tty` with the existing fallback. stdout remains plain selected-message text.
- In-place progress updates may not be announced by polling screen readers; no new accessibility mode is introduced.
- Authentication, provider connectivity, native search-helper availability and timeouts follow the harness UX contract; offline Pi still makes model requests.

## Non-goals

Progress percentages, timing estimates, parallel-step displays, customizable labels, new model/auth dialogs and a retry button are not introduced. Pi's bounded automatic format corrections belong to generation, not a generic failed-step retry mechanism.

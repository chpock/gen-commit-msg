# Progress View for Backend Steps - UX spec
Date: 2026-05-12
Product spec: docs/leyline/specs/2026-05-12-progress-view-design.md
Surfaces: single-screen-ui

UX spec approved - round 3 - 2026-05-12

Design-interrogation round 1 applied:
- (S) alternate screen buffer errors ephemeral → added stderr cleanup line + log-file ref
- (S) post-TUI cleanup invisible → "Cleaning up..." on stderr
- (S) step 4/5 failure loses messages → Flow 2b with ⚠ warning, messages still accessible
- (O) error view surface inconsistency → merged into Progress view
- (O) screen reader in-place transitions → documented limitation
- (O) any-key accidental dismissal → 1s debounce + deliberate keybinding
- (O) no log-file ref in error UX → added "Details:" line
- (O) sub-40-column undefined → error message + exit
- (S) TUI render race → goroutine starts after first View() (see product spec)
- (O) bright ANSI-dependent → SGR 1 for running, SGR 2 for pending/done

Design-interrogation pass complete - round 1 - 2026-05-12

## Surfaces enumerated
- **Progress view**: Vertical list of 5 steps with status indicators (pending/running/done/failed) + inline error display. Shown immediately when TUI starts, before message selection. All errors (step failures, zero messages) are shown inline on this surface.
- **Message selection view**: Existing list of commit message subjects with ↑↓ navigation and Enter to select.

## User flows

### Flow 1 — Successful generation
1. User runs `gen-commit-msg` with staged files
2. Progress view appears — all 5 steps listed, all pending
3. Steps 1→5 complete in order: pending → running (spinner) → done (✓)
4. After step 5 completes, auto-transition to message selection view
5. User navigates with ↑↓, selects with Enter → message printed to stdout → program exits
6. (Single message: auto-selected, skips selection view)
7. (Zero messages: inline error on progress view — steps 1-5 ✓, error text below)

### Flow 2 — Progress step failure (step 1 or 2)
1. Progress view appears — all 5 steps pending
2. A step transitions: pending → running → failed (✗)
3. Error detail appears as stepDetail below the step list
4. Steps that depend on the failed step are marked as skipped (`-`), sent explicitly by the goroutine
5. Cleanup steps (delete session, stop server) run if their prerequisites exist and show their real outcome (✓ / ⚠ / ✗)
6. When all steps report their final status (`allStepsDoneMsg`), the TUI transitions to error view showing the first failure
7. Dismiss with q, Esc, Enter or Ctrl+C → program exits with error code 1

### Flow 2b — Cleanup warning (step 4 or 5 failure after successful generation)
1. Steps 1-3 complete successfully (✓)
2. Step 4 or 5 transitions: running → warning (⚠)
3. Warning message ("Cleanup issue: <detail>") + log-file reference appear below step list
4. TUI auto-transitions to message selection view — messages are accessible
5. User selects message → stdout → exit
6. Cleanup timeout runs silently

### Flow 3 — Generation failure (step 3)
1. Steps 1-2 complete successfully (✓)
2. Step 3 transitions: running → failed (✗)
3. Error detail appears as stepDetail below the step list
4. Cleanup steps 4-5 run and show their real outcome (✓ / ⚠ / ✗) — delete session and stop server
5. When all steps report their final status (`allStepsDoneMsg`), the TUI transitions to error view showing the generation failure
6. Dismiss with q, Esc, Enter or Ctrl+C → program exits with error code 1

## State matrix

| Surface | Loading | Error | Success | Empty |
|---------|---------|-------|---------|-------|
| Progress view | All 5 steps shown; current step shows spinner; completed steps show ✓; pending steps dimmed | Failed step ✗ + error detail below list; dependent steps show `-` (skipped); cleanup steps show their real outcome (✓ / ⚠ / ✗); after `allStepsDoneMsg` transitions to error view; dismiss with q/Esc/Enter/Ctrl+C | All steps ✓; auto-transition to message selection after 300ms; cleanup warnings (⚠) show inline but still auto-transition | Steps all ✓; inline error "no commit messages generated" below list; same dismiss behavior |
| Message selection | N/A — preceded by progress view | N/A — all errors handled inline on progress view | List of messages; ↑↓ to navigate; Enter to select; selected message printed to stdout | N/A — zero messages handled on progress view |

Permission-denied, Offline: N/A — local CLI tool, no network auth required.

## Voice and tone
Reference strings:
- **Error (step 1)**: `Error: opencode server failed to start: connection refused`
- **Error (step 2)**: `Error: failed to create session: request timeout`
- **Error (step 3)**: `Error: failed to generate commit messages: context canceled`
- **Warning (step 4/5)**: `Cleanup issue: failed to delete session`
- **Empty state**: `Error: no commit messages generated`
- **Log-file ref**: `Details: ~/.config/opencode/logs/gen-commit-msg.log`
- **Cleanup progress**: `Cleaning up...` (printed to stderr after TUI exits)
- **Success**: *(silent — selected message is printed to stdout; no success banner)*

Voice is direct, technical, English-only. No emoji, no exclamation marks, no personal pronouns.

## Accessibility targets
- **WCAG level**: N/A (terminal TUI — text-based by nature; color independence is the primary concern)
- **Keyboard flow**: Ctrl+C / Esc exits at any point. ↑↓ to navigate message list. Enter to select. Error states: dismiss with q, Esc, Enter, or Ctrl+C after 1s debounce (prevents accidental dismissal from buffered input).
- **Screen reader**: Step labels are plain text; status indicators (✓ / ✗) are Unicode characters readable by most screen readers. Unicode fallback: `[OK]` / `[FAIL]`. Limitation: status indicators update in-place on existing lines; terminal screen readers that poll may not re-read single-character prefix changes. A future `--a11y` mode could append status lines instead of replacing them.
- **Motion**: Single spinner character updates — already minimal motion. Running step uses SGR 1 (bold); pending/done use SGR 2 (faint). On terminals without SGR support, structural prefixes degrade gracefully: `>` for running, `  ` for pending, `✓`/`[OK]` for done.
- **Color independence**: Status is conveyed by characters (✓, ✗, ⚠) and SGR styles, not color alone.

## Platform / harness constraints
- Terminal width: minimum 40 columns. Below 40 columns: show "Error: terminal too narrow. Minimum width: 40 columns." and exit.
- Framework: bubbletea + bubbles (spinner, list)
- Output: `/dev/tty` for TUI; stderr fallback when TTY unavailable. Status/cleanup messages printed to stderr outside TUI.
- OS: Linux, macOS (existing platform support)

## Non-goals
- Non-TTY mode progress output (stays silent as today)
- Retry mechanism on step failure
- Progress percentage or time estimates
- Customizable step labels
- Step timing information
- Parallel step execution display

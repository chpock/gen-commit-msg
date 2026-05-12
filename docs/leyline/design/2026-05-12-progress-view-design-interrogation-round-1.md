# Design-interrogation report

UX spec: `docs/leyline/design/2026-05-12-progress-view-ux.md`
Product spec: `docs/leyline/specs/2026-05-12-progress-view-design.md`
Date: 2026-05-12
Mode: inline
Questions asked: 100
Dimensions probed: State completeness, Flow failure paths, Accessibility target realism, Voice consistency, Platform conventions, Accessibility tree correctness (predicted), Cross-surface state leakage, Motion and color independence, Copy density and scannability, Per-platform divergence discipline, Internationalization and text expansion, Perceived latency and loading budgets, Information architecture coherence
Chain anchors:
  Q25: Chain has probed state completeness (TUI render timing, message list bounds), failure paths (cleanup visibility, exit codes, partial failures), and begun voice analysis (error copy ambiguity).
  Q50: Chain at state-matrix edge cases and failure path design (messages lost on step-4 failure, cleanup timeout UX, alternate screen buffer losing error messages, keyboard vs. mouse conventions); pivoting to accessibility realism, voice, and platform conventions.
  Q75: Chain at voice analysis, surface mapping inconsistency (Error view vs progress error), auto-selection UX, and cleanup failure UX; pivoting to screen-reader behavior, color/motion independence, and keyboard accessibility.

## Critical UX Issues

- **Alternate screen buffer renders errors ephemeral**: Progress view (state matrix, line 46) / Error view (line 48) — When the TUI exits after a step failure, bubbletea's default alternate screen buffer restores the original terminal. The error message is not in scrollback. A user who accidentally presses a key loses the error forever. The UX spec provides no path to review the error (no log-file pointer in the error view, no confirmation gate before exit). The product spec (line 89-95) mentions cleanup runs after TUI exit but does not address error persistence.

- **Post-TUI cleanup is invisible to the user**: Flow 2 (line 29-31), Flow 3 (line 38-40) — After user presses any key, the TUI exits, the shell prompt reappears, but the program continues running cleanup (session delete, server stop) with a timeout of unspecified duration. The user may type the next shell command while cleanup is still executing, creating a workflow-interruption hazard if cleanup produces output. Neither spec defines the timeout value, whether cleanup is silent, or how to signal that the program is still running after the TUI vanishes.

- **Step 4 failure loses generated messages without recovery**: Flow 2 (line 25-28) — If step 3 succeeds (messages generated) but step 4 (session deletion) fails, the UX shows "Error: failed to delete session: <detail>", then exits. The generated messages exist but are inaccessible. The user watches generation succeed, then sees a deletion error and gets nothing. Neither spec addresses whether messages can be recovered from a partially-failed pipeline.

- **Error view surface mapping is inconsistent**: Surfaces enumerated (line 11) vs. Flow 2 (line 25-28) vs. Flow 3 (line 34-37) vs. Flow 1 (line 22) — The "Error view" is enumerated as a third surface, but step-failure errors (Flows 2 and 3) overlay the error text on the Progress view surface in-place. The Error view is reached only for "zero messages generated" (Flow 1 line 22). A user sees step-3 failure on the Progress view but zero-messages on a different Error view — two different visual experiences for "no usable messages." The surface model conflates "error state on an existing surface" with "dedicated error surface."

- **Screen reader cannot reliably detect in-place step transitions**: Accessibility targets (line 65) — Step transitions update status indicators on existing lines. Terminal screen readers (Orca) poll content periodically; they may not detect the change from spinner to checkmark, especially if only the prefix character changed. The spec says "step labels are plain text" but the transition semantics (pending→running→done) rely entirely on a single-character change that screen readers may miss. No ARIA-live equivalent exists for the terminal; a structural change (new line appended) would be more detectable.

- **"Any key to exit" enables accidental error dismissal**: Error view (line 48), Accessibility targets (line 64) — Any keypress dismisses the error and exits the program. For screen-reader users navigating the interface, an accidental keystroke loses the error before it can be read. A deliberate keybinding (`q`, `Esc`, `Enter`) with a required release-and-press cycle would be more accessible. No debounce or confirmation gate exists.

- **No log-file reference in error UX**: Error view (line 48), Voice and tone (line 52-60) — The error view shows the error message and "Press any key to exit." but provides no pointer to persistent error details. The product spec (line 89-95) mentions cleanup runs after TUI exit, and the codebase uses slog for logging. The UX should direct users to the log file (especially when `--log-file` is configured) so the ephemeral TUI error has a persistent counterpart.

- **Undefined behavior below 40-column minimum**: Platform / harness constraints (line 70) — The spec declares minimum 40 columns but defines no fallback for narrower terminals. A 30-column IDE terminal is a valid TTY but below the minimum. The tool could crash, wrap incorrectly, or render an unusable interface. The UX should define a degradation strategy (truncate labels, switch to non-TTY mode, or refuse with an error message).

- **TUI render race with goroutine step execution**: Flow 1 (line 17) vs. product spec approach A (line 36-37) — The goroutine executes steps and sends messages via `p.Send()`. If the goroutine starts before the TUI's first `View()` renders, step 1 could complete (or fail) before the progress view is visible. The product spec says "the TUI starts earlier" but neither spec defines the goroutine start timing relative to `p.Run()`. An early step failure pre-render would produce an undefined UX.

- **"Bright" running state depends on terminal ANSI support**: Progress view Loading state (line 46) vs. product spec Visual states (line 65) — The product spec declares "Running: spinner + bright label" but "bright" is not defined as an ANSI code. On terminals without SGR 1 (bold) support, "bright" may render identically to "dimmed." If the spinner also fails (non-Unicode terminal), the running state is visually identical to pending. The UX spec's color-independence claim (line 67) does not address this collapse path.

## UX Strengths

- **Step-by-step progress directly solves the blank-terminal problem**: Progress view (line 9) — The 5-step pipeline is immediately visible, replacing 5-30 seconds of silence with actionable feedback. This is the core UX value proposition and it is well-specified.
- **Character-based status indicators with fallback**: Accessibility targets (line 65) — Using ✓/✗ rather than color alone, with a documented `[OK]`/`[FAIL]` fallback for non-Unicode terminals, demonstrates intentional status independence.
- **Clean TUI/goroutine architecture separation**: Product spec approach A (line 36-37) — The TUI model is a pure view holding no references to server/client/logic, making it independently testable and preventing state entanglement.
- **Ctrl+C exits at any point**: Accessibility targets (line 64) — The spec guarantees an escape hatch at every stage, preventing the user from being trapped in a stuck TUI session.
- **Post-TUI cleanup keeps the TUI non-blocking**: Flow 2 (line 30), Flow 3 (line 39) — Cleanup runs after `p.Run()` returns, so the TUI never hangs on a slow server shutdown. The architectural decision is sound even if its UX needs more attention.
- **300ms auto-transition is a deliberate pacing choice**: State matrix (line 46) — A 300ms delay before auto-advancing to message selection gives the user a visual "breathing moment" without being long enough to feel like a wait.

## Revised UX Proposal

### Progress view — error state (UX spec: Flow 2, Flow 3; state matrix line 46)

1. Add a 1-second debounce before accepting keypresses on error states. During the debounce, display "Press any key to exit..." as the prompt. After debounce expires, accept the keypress. This prevents accidental dismissal and gives screen readers time to announce the error.

2. Add a log-file path line below the error: "Details: /path/to/logfile (set with --log-file)". This gives the ephemeral TUI error a persistent reference.

3. If step-4 or step-5 fails after step 3 succeeded, append: "Commit messages were generated. They may be recoverable from the session." This acknowledges the partial-success state without requiring a full retry mechanism.

4. Define "bright" precisely: "SGR 1 (bold) for running label; SGR 2 (faint) for pending/done labels." Also add a fallback when neither works: prefix running steps with `>` instead of just spin/bright.

### Error view — surface architecture (UX spec line 11, line 48)

5. Merge the Error view into the Progress view: remove the separate Error view surface. Zero-messages after step 5 should display as an error overlay on the Progress view (same as step failures), not a separate surface. This unifies the user experience — all "no usable messages" outcomes share one visual pattern.

6. If the zero-messages case remains distinct for a reason, document the UX rationale for why it deserves a different surface than step-3 failure when both result in "no messages available."

### Post-TUI cleanup experience (UX spec Flow 2 lines 29-31, Flow 3 lines 39-40)

7. After the TUI exits on failure, print a one-line status to stderr before cleanup begins: "Cleaning up...". This signals to the user that the program is still active even though the TUI is gone. After cleanup completes (or times out), print nothing on success; print a one-line warning on cleanup failure.

8. Define the cleanup timeout value (e.g., 10 seconds for session delete, 5 seconds for server stop) in the UX spec so users and implementers share expectations.

### Accessibility (UX spec line 65)

9. Document that in-place line updates for step transitions may not be screen-reader-detectable. Recommend that the implementation append a new line for each transition (log-style), or provide a `--a11y` flag that switches from in-place updates to an appended-line progress format.

10. Change "Any key to exit" to `Press q or Esc to exit.` in error states. This makes dismissal deliberate and reduces accidental-exit risk for screen-reader users. Retain "Press any key to exit" only if the `--a11y` flag rationale favors extreme simplicity.

### Sub-40-column terminal degradation (UX spec line 70)

11. Define the sub-40-column fallback: truncate step labels with a trailing `…` or switch to a compact one-line progress format. If neither works, refuse with `Error: terminal too narrow (minimum 40 columns required)` and exit code 1.

### Goroutine timing (UX spec Flow 1 line 17)

12. Specify that the goroutine starts after the first `View()` call completes (after the TUI model processes `tea.WindowSizeMsg` or the first `Update` message), ensuring the initial render with all-5-steps-dimmed is visible before step execution begins.

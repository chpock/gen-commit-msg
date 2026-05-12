# Design-interrogation transcript — Round 1
Date: 2026-05-12
Mode: inline

Q1: Does the UX spec confirm Surfaces is not "none"? → Yes, line 4: "Surfaces: single-screen-ui". Valid for interrogation.

Q2: Are all 3 surfaces from "Surfaces enumerated" in the state matrix? → Yes. Progress view (line 9), Message selection (line 10), Error view (line 11) all have rows in the matrix (lines 46-48).

Q3: Is there a flicker where no steps are visible before the TUI renders? → Step labels are hardcoded (product spec lines 55-61), so the TUI can render all 5 immediately. The flicker concern is addressed by the pre-populated step list.

Q4: What does the user see between process start and first TUI render? → Neither spec addresses the `tea.NewProgram` → first `View()` gap. "TUI starts" is ambiguous.

Q5: If the goroutine starts before the TUI's first View(), could step 1 complete before it's displayed? → The product spec says goroutine sends via `p.Send()` but doesn't specify timing relative to `p.Run()`. Messages could be lost or already-processed before render.

Q6: Is the goroutine started before or after `p.Run()`? → Neither spec specifies. A specification gap.

Q7: If step 1 fails before the TUI renders, what does the user see? → The UX spec's Flow 2 assumes the progress view is visible when failure occurs, but pre-render failure is undefined.

Q8: Message selection view — what happens when terminal is resized below 40 columns? → Unspecified. The bubbletea list component may truncate or wrap incorrectly.

Q9: What if there are 100+ generated messages? → Neither spec bounds the message list. The UX spec doesn't mention scrolling or pagination.

Q10: Does bubbles/list handle 100+ items? → Yes, but the UX spec doesn't confirm it uses bubbles/list or describe scroll UX.

Q11: Is "no messages generated" rendered in Error view? → Yes via Flow 1 line 22. But Error view has only one generic row — no distinction between step failure and empty results.

Q12: Should step-3 failure and zero messages feel the same to the user? → Arguably no. Different root causes, same UX treatment. Could miss retry opportunity.

Q13: Does the user see cleanup progress after pressing any key? → No. TUI exits, cleanup runs silently. User sees blank terminal or shell prompt.

Q14: What if cleanup fails? → Neither spec defines UX of post-TUI cleanup failure. User isn't notified.

Q15: What exit code if TUI showed step failure but cleanup succeeded? → Error code 1 (Flow 2 line 31). But what about partial cleanup? Undefined.

Q16: What spinner character is used? → Neither spec specifies which bubbles spinner. The flow shows braille `⠋` but not declared.

Q17: Does the braille spinner have a Unicode fallback like ✓/✗? → No. If terminal lacks braille Unicode, spinner UX is undefined.

Q18: What does "bright" in "spinner + bright label" mean? → Not defined. Could be ANSI bold, bright color, or both. First color-dependent indicator in the spec.

Q19: If ANSI dim fails, can user distinguish pending from done? → Yes — checkmark distinguishes done from pending. But running vs pending: only spinner differentiates if dim/bright both fail.

Q20: How many terminals don't support ANSI dim/bright? → Modern Linux/macOS terminals do. Some CI/embedded/old xterms don't. Spec targets Linux/macOS where support is good.

Q21: Does "bright" as ANSI bold change font weight on some terminals? → Yes. Spec doesn't define "bright" precisely enough for consistent rendering.

Q22: Does error copy match the "direct, technical" voice? → "Error: opencode server failed to start: connection refused" — yes, direct and technical.

Q23: Is "connection refused" too technical for less-technical users? → Target audience is developers using git CLI. "Connection refused" is standard Unix parlance. Acceptable.

Q24: Is "context canceled" blame-free? → "Context canceled" sounds like user action but could be server timeout. Ambiguous agency. Subtle voice issue.

Q25: COHERENCE ANCHOR — Chain has probed state completeness (TUI render timing, message list bounds), failure paths (cleanup visibility, exit codes, partial failures), and begun voice analysis (error copy ambiguity).

Q26: What does the progress view look like when step 1 hasn't started? → All 5 steps dimmed, no spinner. Depends on goroutine timing (Q4-Q7).

Q27: Is 300ms auto-transition timing correct when step 3's results are already stored? → Yes, step 3 completes before step 4, so results exist. 300ms is purely visual.

Q28: What if step 3 produces 0 messages — when is the zero check? → After step 5 Done per product spec line 87. Steps 4-5 run wastefully but correctly.

Q29: What if step 3 succeeds but step 4 fails? → Messages were generated but UX shows error and exits. Recovery path missing.

Q30: If step 4 fails, does cleanup try step 5? → Flow 2 says remaining steps dimmed, no further execution. Server left running. Post-TUI cleanup may lack session ID.

Q31: What's the cleanup timeout? → Neither spec defines the duration. User waits with blank terminal for unknown time.

Q32: Does progress view have keyboard interaction besides Ctrl+C/Esc? → No, but spec doesn't explicitly say so. User may try keys and get no response.

Q33: Does Ctrl+C during step 1 interrupt server start? → TUI exits; goroutine keeps running. Cleanup tries to kill server but OS process group handling undefined.

Q34: Is 300ms delay noticeable or jarring? → Below 1s "flow of thought" threshold but above 0.1s "instant" threshold. Brief flicker.

Q35: After 30+ second generation, does 300ms feel like part of the wait? → No transition indicator. "Preparing results..." would help contextualize the pause.

Q36: Do screen readers reliably announce ✓/✗? → Optimistic claim. Orca + speech-dispatcher may read "check mark" — but depends on synthesizer. VoiceOver+Terminal may skip entirely.

Q37: Do screen readers detect in-place line changes (pending→running)? → Terminal screen readers poll; single-character prefix changes may not trigger re-reading. Accessibility gap.

Q38: Does screen reader announce "Press any key to exit"? → Yes, but may not finish reading before user presses key and program exits.

Q39: Is there debounce on "any key to exit"? → No. Accidental brush of keyboard dismisses error immediately.

Q40: Can user scroll up in terminal after TUI exits? → No — bubbletea uses alternate screen buffer (smcup/rmcup). Error is lost forever on exit.

Q41: Is alternate screen buffer behavior documented? → Neither spec mentions it. Significant omission for error recovery.

Q42: Does UX spec reference log file for error persistence? → No. No "Details in log file" guidance. Users won't know to check logs.

Q43: Mouse interaction — supported or not? → Spec only mentions keyboard. Bubbletea supports mouse but UX doesn't address it. Fine for keyboard-first tool.

Q44: Terminal compatibility matrix for braille spinner? → Not provided. Braille support varies across macOS Terminal, gnome-terminal, konsole, alacritty, st.

Q45: What ANSI SGR codes for "dimmed" and "bright"? → Not specified. Implementer may choose different codes than intended. Visual inconsistency risk.

Q46: Is dimmed vs bright structurally distinguishable without color? → Only spinner differentiates running from pending. If spinner fails, states collapse. Should add structural indicator like `[>]`.

Q47: Does message selection view show message numbers? → UX spec doesn't describe visual appearance — assumes existing bubbles/list implementation.

Q48: Does selected message include conventional commit type/scope in output? → Subject line per AGENTS.md. Output mechanism via /dev/tty vs stdout split unspecified.

Q49: What about pseudo-terminals (IDE terminals)? → PTYs appear as TTYs. Tool tries progress view but width may be <40. Undefined behavior.

Q50: COHERENCE ANCHOR — Chain at state-matrix edge cases and failure paths. Per Q50 pivot, now pressure accessibility realism, voice, platform conventions.

Q51: Does "Empty" voice break the `<operation> failed` pattern? → "no commit messages generated" doesn't follow the template but is prefixed with `Error:`. Minor.

Q52: Is "failed to start" user-centric? → Slightly passive. "The server didn't fail; the attempt to start it failed." Within acceptable "technical" voice.

Q53: Are step labels consistent with "direct, technical" voice? → Yes: gerund forms, no emotion, no pronouns. Labels: "Starting OpenCode...", etc.

Q54: Do ellipses in labels follow HIG conventions? → In HIG, ellipsis means "more input required." Here it means "in progress." Unintentional platform convention deviation.

Q55: Do ✓ and ✗ risk emoji presentation variants? → On some systems these may render as emoji. UX treats them as "characters" but doesn't address emoji variants.

Q56: What if upstream errors contain non-English text? → Not addressed. Localized OS errors would appear in English TUI. Locale handling gap.

Q57: Text expansion in labels if localized? → English labels: max ~29 chars. German/Finnish similar. Fit within 40 cols. But not a declared concern since English-only.

Q58: RTL text in generated commit messages? → English-only spec excludes RTL. But generated messages could contain RTL from diff. Bubbletea list may not handle RTL.

Q59: Does progress view create duration expectation mismatch? → Steps 1-2 fast, step 3 slow. User may think step 3 is stuck. No duration context.

Q60: Is there a loading budget defined? → No. No per-step expected or maximum durations. Product spec mentions 5-30s total but no distribution.

Q61: Server already running scenario? → Neither spec addresses. Should detect and show "Connected to existing server" rather than ambiguous start attempt.

Q62: Network latency impact on 300ms auto-transition? → 300ms is client-side. Terminal latency compounds the delay. Fixed delay doesn't account for network.

Q63: Cross-surface state leakage between Progress and Message selection? → Generated messages carry over. Step statuses reset. No leakage. Clean separation.

Q64: Does transition clear the alternate screen? → Yes, bubbletea replaces view. Intended UX. Old steps not preserved.

Q65: Single-message auto-selection — does user know why selection was skipped? → No. User sees progress then output appears and program exits. May think crash/error.

Q66: Should single-message auto-selection be documented as intentional? → Yes. UX spec should explain transparency rationale.

Q67: Is "silent success" right after a visual progress view? → Contrast between "all ✓" and "sudden output + exit" is stark. Unix philosophy of silence conflicts with progress-view engagement.

Q68: Is "exit" the right framing for transient errors? → "Exit" implies finality. Retry is non-goal, so honesty is correct. But path-to-resolution missing.

Q69: Does error view provide path to resolution? → No. No "check logs" or "retry with --debug" guidance. Debugging burden on user.

Q70: Is Error view a separate surface from Progress view in error state? → Flow 2/3 errors overlay on Progress view. Only zero-messages reaches Error view. Surface mapping inconsistency.

Q71: What reaches the Error view surface? → Only Flow 1 line 22 (zero messages). Step failures stay in Progress view.

Q72: Should step-3 failure and zero messages share a surface? → Arguably yes. Both mean "no messages." Different surfaces for similar outcomes create unnecessary divergence.

Q73: Are step-3 failure and zero messages meaningfully different errors? → Yes — one is an operation error, one is an empty result. But the UX treats them differently by surface.

Q74: What if generation returns error vs. empty list? → Two different UX paths for similar-looking outcomes. Error stays in Progress; empty goes to Error view.

Q75: COHERENCE ANCHOR — Chain at voice, surface inconsistency, auto-selection UX, cleanup UX. Per Q75 pivot, now pressure screen-reader behavior, color/motion independence.

Q76: What does a blind developer hear during progress? → Orca reads terminal line by line. Braille spinner may be read as "braille pattern dots-124" or skipped. ✓ may or may not be announced.

Q77: Does Orca detect in-place line changes? → Terminal readers poll periodically. If only prefix changed, Orca may miss it. No mechanism to ensure announcement.

Q78: Does Orca finish reading error before user can dismiss? → Short text + Orca speed may finish. But if user presses key quickly, reading stops mid-sentence.

Q79: Is "any key" accessible for screen reader users? → No. Accidental keypress during exploration dismisses error. Deliberate keybinding is more accessible.

Q80: Is full keyboard flow navigable without mouse? → Yes. ↑↓, Enter, Ctrl+C/Esc. Basic keyboard accessibility passes.

Q81: Is highlight bar visible to low-vision users? → Bubbletea list uses reverse video (high contrast). But spec doesn't confirm or specify contrast ratio.

Q82: Does reduced-motion apply to terminal apps? → CSS `prefers-reduced-motion` doesn't apply. No terminal standard. Spec's claim is correct but could offer static `...` option.

Q83: Is TUI distinguishable for color-blind users? → ✓/✗ are distinct shapes. Dim/bright is luminance. Spinner is positional. Passes basic color independence.

Q84: Is "dimmed" text readable on light-background terminals? → Faint (SGR 2) on white may be invisible. Light vs dark theme not addressed.

Q85: Does product spec address terminal theme detection? → No. OSC 11 is not universally supported. Spec should note potential low-contrast on certain themes.

Q86: Is error text scannable below step list? → Yes: 5 steps + 2-3 error lines + 1 prompt = 8-9 lines. Fits in standard 24-line terminal.

Q87: What if error detail is very long at 40 columns? → "dial tcp 127.0.0.1:49321: connect: connection refused" wraps to 3 lines. Still fits. No minimum height defined.

Q88: Are "Error view" and "Progress view error state" different surfaces? → UX spec treats them as one in enumeration but flows treat them differently. Architecture inconsistency.

Q89: Should state matrix have separate rows for each? → If they're different surfaces, yes. Currently conflated. Error view only describes zero-messages error, not step-failure error.

Q90: Does Error view ever need Loading or Success states? → N/A justified. Error view is terminal. But auto-transition from Progress to Error view has an unmodeled transition state.

Q91: Is 300ms auto-transition animated or hard cut? → Hard cut. No continuity between progress ✓ and message list. Not described in spec.

Q92: Does user know program is still running after TUI exits? → No. Shell prompt appears. User may type next command during silent cleanup. Workflow interruption hazard.

Q93: Could cleanup output interleave with user's next shell prompt? → If cleanup writes to stderr and stderr is visible, yes. Neither spec says cleanup is silent.

Q94: How does --pause flag interact with error UX? → Not addressed in UX spec. Should differentiate "pause" behavior from normal error dismissal.

Q95: What does --quiet mode flow look like? → Progress view skipped entirely. UX spec doesn't describe quiet mode flow.

Q96: What happens below 40 columns — degrade to non-TTY? → Not defined. TTY below 40 columns has undefined behavior.

Q97: Does progress view fit at exactly 40 columns? → Longest label "Stopping OpenCode server..." (28 chars) + "  Step 5: " (10) + indicator (2) = 40 chars. Barely fits.

Q98: Format analysis — actual character count of step prefix + label? → Step 3 label "Generating commit messages..." = 28 chars. "  Step 3: ✓ " = 12 chars. Total 40. Tight.

Q99: Step 4 and other steps character counts? → Step 4: 31 chars. Step 5: 40 chars. Step 3: 40 chars. All fit at exactly 40 columns but leave zero margin.

Q100: Are all state-matrix cells reachable from some flow? → Yes. Verified: all 12 cells (3 surfaces × 4 states) are either reachable via documented flows or justified N/A. Surface mapping issue (Q88-Q89) is architectural, not reachability.

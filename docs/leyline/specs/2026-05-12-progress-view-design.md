# Progress View for Backend Steps - product spec
Date: 2026-05-12
Author: chpock
Surfaces: single-screen-ui

Product spec approved - round 2 - 2026-05-12

Deep-discovery round 1 classification:
- (S) goroutine-vs-tea.Cmd architecture unspecified
- (O) no per-step error messages defined
- (R) 300ms auto-transition only in UX spec

## Problem
Currently, when the user runs `gen-commit-msg`, there is no feedback during server startup, session creation, and cleanup steps. The only visual feedback is a "Generating commit messages..." spinner that appears after the server and session are already set up. Users are left staring at a blank terminal for 5-30 seconds with no indication of what's happening.

## Goals
- Show step-by-step progress for all 5 backend operations before the message selection TUI appears
- Each step displays: a name, a running indicator (spinner), and a completion indicator (checkmark or failure)
- All 5 steps are visible from the start; the user can see the entire pipeline at a glance
- On failure, show the error and exit (respecting `--pause`)

## Non-goals
- Non-TTY mode stays unchanged (no progress output in non-interactive mode)
- Quiet mode hides the progress view entirely (same behavior as spinner hiding today)
- No retry mechanism for failed steps

## Constraints
- Must use bubbletea (existing TUI framework)
- Must not break non-TTY mode (`--subject-count 1` without terminal)
- Must respect `--quiet` flag (hide progress)
- Must respect `--pause` flag on error exit

## Approaches considered
### Approach A - Single TUI with progress phase
Add a new `stateProgress` state to the existing TUI model. The progress view displays all 5 steps as a vertical list, each with a status indicator. The TUI starts earlier — before server initialization. A goroutine in main.go executes steps 1-5 sequentially, sending step-transition messages to the TUI via `p.Send()`. The TUI model remains a pure view — it holds no references to server, client, or cleanup logic. After all steps complete, the TUI transitions to the message selection view.

Architecture contract: main.go is the orchestrator; the TUI model is a view that renders step states and forwards user input. Step execution and cleanup live outside the TUI.

Trade-offs: Moderate implementation cost; restructures `main.go` orchestration; uses established bubbletea goroutine+p.Send() pattern; highly reversible; TUI model remains testable without server/client dependencies.

### Approach B - Two sequential TUI programs
Run two separate bubbletea programs: first for progress, then for message selection.
Trade-offs: Higher complexity; need to pass state between programs; no compelling advantage over single-TUI.

### Approach C - Simple stdout progress lines
Print progress as plain text lines before the TUI starts.
Trade-offs: Low cost but poor UX; doesn't meet "all steps visible" and "spinner" requirements.

## Recommendation
Approach A — a single TUI with a progress phase. It's the natural fit for bubbletea, keeps the codebase simple, and meets all UX requirements.

## Step labels
| # | Label |
|---|-------|
| 1 | Starting OpenCode... |
| 2 | Creating session... |
| 3 | Generating commit messages... |
| 4 | Deleting session... |
| 5 | Stopping OpenCode server... |

## Visual states per step
- **Pending**: dimmed label
- **Running**: spinner + bright label
- **Done**: checkmark ✓ + dimmed label
- **Failed**: cross ✗ + dimmed label

**Unicode fallback**: ✓/✗ are Unicode characters. If the terminal does not support Unicode, fall back to `[OK]` for done and `[FAIL]` for failed.

## Per-step error messages
Each step failure must display a step-specific error message below the step list:

| Step | Error message template |
|------|----------------------|
| 1 | `Error: opencode server failed to start: <detail>` |
| 2 | `Error: failed to create session: <detail>` |
| 3 | `Error: failed to generate commit messages: <detail>` |
| 4 | `Error: failed to delete session: <detail>` |
| 5 | `Error: failed to stop OpenCode server: <detail>` |

The `<detail>` portion is the underlying error text from the operation.

## Transitions
- After the last step (step 5) reaches Done, the TUI waits 300ms then auto-advances to the message selection view.
- If the message count is 1, the TUI auto-selects and exits without rendering the selection view.
- If the message count is 0, the TUI transitions to the error view ("no commit messages generated").

## Failure cleanup
On any step failure:
1. The error message appears below the step list
2. Remaining pending steps stay dimmed — no further execution of the failed pipeline
3. The user presses any key to dismiss
4. After the TUI exits (`p.Run()` returns), main.go runs cleanup (session delete, server stop) with a timeout
5. The TUI does not block on cleanup — cleanup is a post-TUI concern

## Flow
```
[Progress View: all 5 steps shown]
  Step 1: ⠋ Starting OpenCode...
  Step 2:   Creating session...
  Step 3:   Generating commit messages...
  Step 4:   Deleting session...
  Step 5:   Stopping OpenCode server...
       ↓ (steps complete one by one)
  Step 1: ✓ Starting OpenCode...
  Step 2: ✓ Creating session...
  Step 3: ✓ Generating commit messages...
  Step 4: ✓ Deleting session...
  Step 5: ✓ Stopping OpenCode server...
       ↓ (300ms delay — auto-transition)
[Message Selection TUI (existing)]

Failure example:
  Step 1: ✓ Starting OpenCode...
  Step 2: ✗ Creating session...
  Step 3:   Generating commit messages...  ← dimmed, never executes
  Step 4:   Deleting session...            ← dimmed, never executes
  Step 5:   Stopping OpenCode server...    ← dimmed, never executes
       Error: failed to create session: connection refused
       Press any key to exit.
```

## Success criteria
- Running `gen-commit-msg` in interactive mode shows the progress view with all 5 steps
- Each step transitions from pending → running (spinner) → done (checkmark) in order
- After step 5 completes, the TUI waits 300ms then auto-advances to message selection
- If any step fails, a failure indicator (✗) + per-step error message is shown; pressing any key exits
- Cleanup (session delete, server stop) runs outside the TUI after `p.Run()` returns
- If the terminal does not support Unicode, checkmarks/crosses fall back to `[OK]`/`[FAIL]`
- Non-TTY mode prints no progress output (unchanged behavior)
- Quiet mode shows no progress view (unchanged behavior)
- Existing tests pass; new tests cover progress state transitions
- `make all` (fmt → vet → test → build) passes

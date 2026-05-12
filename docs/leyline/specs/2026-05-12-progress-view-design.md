# Progress View for Backend Steps - product spec
Date: 2026-05-12
Author: chpock
Surfaces: single-screen-ui

Product spec approved - round 1 - 2026-05-12

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
Add a new `stateProgress` state to the existing TUI model. The progress view displays all 5 steps as a vertical list, each with a status indicator. The TUI starts earlier — before server initialization — and receives step-completion messages via `tea.Cmd` or goroutines. After all steps complete, the TUI transitions to the message selection view.

Trade-offs: Moderate implementation cost; restructures `main.go` orchestration; uses established bubbletea patterns; highly reversible.

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
- **Done**: green checkmark ✓ + dimmed label
- **Failed**: red cross ✗ + dimmed label

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
       ↓ (all done — transition)
[Message Selection TUI (existing)]
```

## Success criteria
- Running `gen-commit-msg` in interactive mode shows the progress view with all 5 steps
- Each step transitions from pending → running (spinner) → done (checkmark) in order
- If any step fails, a failure indicator is shown and the program exits after the error display
- Non-TTY mode prints no progress output (unchanged behavior)
- Quiet mode shows no progress view (unchanged behavior)
- Existing tests pass; new tests cover progress state transitions
- `make all` (fmt → vet → test → build) passes

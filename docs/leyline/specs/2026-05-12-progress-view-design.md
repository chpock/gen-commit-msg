# Progress view for generation steps — product spec

Date: 2026-05-12
Current harness contract: 2026-10-24
Surfaces: inline TUI
Original product approval: round 3, 2026-05-12.
Current authority: `2026-10-24-pi-harness-design.md`, D1–D13 in the implementation plan.

## Purpose

Show the real collection/generation/cleanup sequence before message selection. All steps for the selected harness are visible from the start, rather than displaying only a generation spinner or inventing server/session operations for Pi.

## Architecture

Retain one Bubble Tea program with progress and selection phases. The TUI is a view, with no server/client/process ownership. A shared application operation collects Git context and calls the selected generator in both TTY and non-TTY modes. Each backend declares its steps, executes them, owns resources and completes cleanup before returning.

This supersedes the original fixed OpenCode-only step list and duplicated main-level lifecycle orchestration. Backend lifecycle methods are not exposed to the TUI. Preserve the existing first-frame initialization and 300ms successful progress-to-selection transition.

## Step labels

| Index | OpenCode | Pi |
| --- | --- | --- |
| 0 | Collecting information about current changes... | Collecting information about current changes... |
| 1 | Starting OpenCode... | Starting Pi... |
| 2 | Creating session... | Generating commit messages... |
| 3 | Generating commit messages... | Stopping Pi... |
| 4 | Cleaning up OpenCode resources... | — |

OpenCode startup includes agent setup, server start and health checking. Pi startup includes supported-version verification and correlated RPC readiness. Pi corrections update generation detail, such as `Correcting response format (attempt 2 of 3)`, not the declared step list.

## Status and dependency contract

- Pending: dimmed label.
- Running: spinner and highlighted label.
- Done: checkmark.
- Failed: cross and operation-specific detail.
- Interrupted: interrupted operation, followed by owned cleanup.
- Warning: nonfatal cleanup issue.
- Skipped: prerequisite not satisfied, no operation attempted.

Preserve the existing structural prefixes, colors, Unicode handling and terminal layout. Status must not depend on color alone.

Git collection precedes all backend steps. An unsuccessful prerequisite skips dependent operations. Cleanup runs whenever the backend acquired resources, even after failure or cancellation. Show the actual outcome, not a fabricated successful step. OpenCode session deletion and server termination are combined in the existing cleanup step.

## Errors and completion

Use standard errors at the progress boundary. Preserve a backend's rich rendering when its error provides that capability; neither shared types nor the TUI may require an OpenCode error type. Display the first generation failure rather than replacing it with a subsequent cleanup failure.

Continue accepting progress through cleanup until terminal completion. On failure, show the final error and honor the existing pause/dismissal behavior. On success, provide candidates only after cleanup: one candidate auto-selects, several use the existing inline subjects-only list, and zero candidates produce the existing no-messages error.

A confirmed successful generation remains usable after cleanup warning. Warnings use visible progress, otherwise stderr, and are logged. Quiet mode hides progress but not warnings. If the TUI closes before cleanup ends, preserve a stderr fallback for later cleanup diagnostics. Never put diagnostics into commit-message stdout or the output file.

## Modes and cancellation

- Non-TTY mode has no progress UI and uses the same production operation.
- Quiet mode hides progress, not message selection or error/warning reporting.
- User cancellation stops generation, suppresses output and still waits for backend cleanup.
- The TUI does not itself stop a server, delete a session or close process pipes.
- Generated output is not written before an actual selection.

## Accepted implementation approach

The original single-TUI approach remains selected over separate sequential TUI programs or plain stdout status lines. Its lifecycle orchestration now delegates to the backend-owned generation boundary. Historical discovery/review decisions remain recorded in the existing plan/review history; the harness spec defines the current lifecycle guarantees.

## Verification

- Both complete step lists are rendered initially, with no Pi session/server labels.
- Status transitions, skipped prerequisites, generic/rich errors and interrupted cleanup are covered.
- Cleanup warnings preserve candidates and remain visible without progress.
- Selection/formatting and existing single/multiple/zero-result tests remain valid.
- Actual TTY selection/cancellation and stdout/file routing are exercised for both backends.
- Focused TUI/CLI checks and `make all` pass.

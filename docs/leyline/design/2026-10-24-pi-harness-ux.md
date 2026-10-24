# Pi and OpenCode harness support — UX spec

Date: 2026-10-24
Product: `../specs/2026-10-24-pi-harness-design.md`.
Approved design: D1–D14 in `../plans/2026-10-24-pi-harness-plan.md`.

## CLI and selection

`--harness auto|pi|opencode` and `GCM_HARNESS` use flag > environment > default precedence; default is auto. Auto prefers a Pi executable on PATH and otherwise OpenCode. Once selected, errors never switch backends. Explicit selection is deterministic.

Help identifies `--agent` and `--install-agent` as OpenCode-only. Pi uses its ordinary configured model/authentication and ignores these options; no agent installation occurs. README examples show explicit harness selection, minimum Pi 1.4.0 and native search-helper prerequisites.

Help/version and no-staged-files exits require no harness. Non-TTY multi-subject requests fail before harness startup, preserving the existing suggestion to use `--subject-max 1`. Missing executables, unsupported/unrecognizable Pi versions and runtime failures have actionable English errors. Invalid harness configuration exits 2; runtime/output failures exit 1.

## Progress surfaces

All steps for the selected backend are visible initially. The existing indicator, selection color, keyboard and inline-list behavior remain unchanged.

| Order | OpenCode | Pi |
| --- | --- | --- |
| 1 | Collecting information about current changes... | Collecting information about current changes... |
| 2 | Starting OpenCode... | Starting Pi... |
| 3 | Creating session... | Generating commit messages... |
| 4 | Generating commit messages... | Stopping Pi... |
| 5 | Cleaning up OpenCode resources... | — |

Pi startup includes version verification and a successful readiness exchange. It does not display a server or session step. Corrective prompts update generation detail, for example `Correcting response format (attempt 2 of 3)`, without inventing extra lifecycle steps.

The TUI remains a view. Shared orchestration executes Git collection and the selected backend outside it. Backends own resources and report actual statuses. Failed prerequisites cause skipped steps; existing resources are still cleaned up before the terminal completion message. A generation failure remains the primary displayed error if cleanup also fails.

## Interactive flow

1. Check configuration, repository and staged files; select the executable.
2. Show the selected step list and execute the shared operation.
3. Complete backend cleanup, displaying warnings without losing confirmed candidates.
4. For one candidate, auto-select without a list. For several, show the existing subjects-only inline selection.
5. Enter writes the selected subject/common body to stdout or `--output`. Esc/cancellation produces no message and preserves the destination.

The existing progress-to-selection delay, pause behavior, terminal-width handling and message formatting remain. Do not add a model picker, credentials dialog, new startup preflight for search helpers or configurable tools.

## Non-TTY and quiet

Non-TTY with `--subject-max 1` uses the same collection/generation/cleanup operation without a TUI. Only after cleanup does it write the message. Quiet hides progress, not the subject list, errors or cleanup warnings.

Warnings appear in the progress UI when it is visible and otherwise on stderr; they also enter the log. A canceled or failed TUI must not silently lose diagnostics produced during subsequent cleanup. Never put warnings or Pi protocol/stderr records in the commit-message stdout/file.

## Failures and accepted limitations

- Pi startup has a 30-second budget; each generation/correction gets 120 seconds including native work. Progress does not reset deadlines. Shutdown has a separate five-second grace before forced termination.
- Authentication/provider/protocol failure is an error, not a formatting correction or OpenCode fallback.
- Three invalid completed responses produce an error and no output overwrite.
- Confirmed valid output followed by abnormal/forced shutdown remains selectable, with a cleanup warning. Successful selection/write exits 0; cancellation still suppresses output.
- The direct OpenCode server receives SIGTERM, with up to five seconds for shutdown including waiting/draining. If shutdown remains incomplete, its existing dedicated process group receives SIGKILL; direct-child waiting and output draining complete before presentation (D14). Descendants receive no group-wide graceful SIGTERM. Expected requested-signal termination is not automatically an error.
- Native grep/find require cached/system rg and fd/fdfind. Without them, offline Pi reports ordinary tool errors to the model; generation may still succeed from supplied context. Do not download helpers, reject application startup or silently remove tools.
- Native read-only tools can read outside the repository with the user's permissions. Repository-only access is instructed, not enforced by a sandbox.

## Preserved output contract

The output is the chosen subject, optionally a blank line and common body, with the existing final newline. No success banner or harness framing. Existing output validation remains nondestructive, and overwrite happens only for an actual message. Help/version/logging retain their existing channels and configuration semantics.

## Verification

Test both backend step lists, generic and rich error rendering, skipped/interrupted/warning states, single/multiple selection, quiet/non-TTY warnings and destination preservation. Verify actual application TTY interactions and native Pi helper execution/missing-helper outcomes as delivery gates, not just rendered strings or tool declarations.

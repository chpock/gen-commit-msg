# Pi harness plan — review log

Date: 2026-10-24
Plan: `2026-10-24-pi-harness-plan.md`
Baseline: `48c5309`
Scope: research, architectural decisions and implementation planning only.

## Discovery verification

- Read current project source/specifications rather than relying on prior architecture summaries.
- Read installed Pi 1.4.0 documentation and relevant CLI/resource/package/prompt/RPC/session source.
- A read-only scout returned configuration-isolation evidence. Its uncertain APPEND_SYSTEM and package-startup claims were checked independently against source and controlled native execution.
- `probe_pi_cli.py` ran real Pi against a localhost fixture with disposable configuration and dummy credentials. All twelve final observation checks passed.
- Recorded actual API request counts for RPC success (1), correction (2), exhaustion (3) and provider error (1), with native network retries disabled in the fixture configuration.
- Confirmed instruction sentinels excluded, exact read-only tool declarations, no marker-extension execution, no persisted session JSONL, large stdin delivery, and normal RPC shutdown by stdin EOF.
- Initial probe assumptions about offline package-helper execution and terminal truncation were disproved. Source and runtime observations corrected the conclusions: offline blocks installation but not package-root lookup; native recovery can remove a truncated assistant attempt and leave empty successful print output. The plan does not infer generation success from process exit.

## Operator decisions

- Add auto/Pi/OpenCode selection, preferring Pi.
- Use CLI-level isolation while retaining ordinary global Pi model/auth/runtime settings.
- Auto selection is based on executable presence; no fallback after selected-harness failure.
- Allow two corrective prompts after an invalid model result, not local JSON extraction/repair.
- Use one sessionless RPC process so corrections share the conversation.
- Ignore OpenCode-only agent/install options in Pi mode.
- Accept native read-only tools without enforced repository-path confinement; do not add a sandbox or extension.
- Approve the common generation boundary and require OpenCode refactoring within the same task: shared TTY/non-TTY orchestration, backend-owned resources, harness-specific progress and presentation only after cleanup. This includes the explained non-TTY output-ordering change.
- Approve Pi timeouts: 30 seconds for startup/readiness, 120 seconds for each initial/corrective prompt operation including native Pi work, and five seconds for orderly shutdown before forced termination. Preserve existing OpenCode timeouts. The former combined 120-second proposal is superseded; progress does not renew operation budgets, and cleanup remains independently bounded after cancellation.

- Approve Pi 1.4.0 as the minimum supported version, without backward compatibility for older releases. Version verification precedes generation and Git-context transmission; unsupported/unrecognizable versions fail without harness fallback.

- Approve preservation of confirmed generation results after cleanup failure. For Pi this requires prompt acceptance, successful completed final output, non-aborted settlement and strict result validation. Abnormal/forced shutdown is a visible cleanup warning, not result loss; cleanup/reaping still completes. Successful selection/output exits 0, cancellation suppresses output, and simultaneous generation/cleanup failure preserves the generation cause.

Decisions D1–D13 are approved. Oracle re-read the updated plan and confirmed READY TO IMPLEMENT: O1/O2 are resolved at the design/scope level, with no remaining implementation-readiness blocker from the consultation. Corrections, documentation and implementation checks remain future work. Implementation is not yet authorized.

## Independent plan review

A fresh-context reviewer inspected the actual plan, current project code/tests/specifications, authoritative installed Pi source/docs and supplied runtime evidence.

- **Findings:** none.
- **Verdict at review time:** READY WITH OPEN DECISION; implementation unauthorized.
- **Open decision at that time:** native tool file-access boundary. The operator subsequently accepted the documented native read-only behavior, and the plan was updated without changing its technical design.
- **Implementation watchpoint accepted:** current OpenCode CLI fixtures inherit the host PATH and do not explicitly select a harness. Parent verification confirmed this in `cmd/gen-commit-msg/cli_test.go:46–50,118–131`. The plan now requires pinning OpenCode fixtures and controlling selection-test executables/environment before enabling auto selection or running the affected CLI suite.
- The reviewer did not rerun probes or application tests; supplied evidence was inspected. This is a planning review, not implementation acceptance.

## Validation and limitations

- Controlled native Pi probe: twelve observation cases passed; no paid provider was used.
- Git whitespace checks passed: `git diff --check`, plus `git diff --no-index --check` for both untracked documents. The latter's exit status 1 indicates added-file differences, not a whitespace error; neither document produced diagnostics.
- Application source, configuration, dependencies and runtime behavior were not modified. `make all` was not required or run for this documentation-only planning task; it remains an implementation gate.
- A real authenticated-model smoke test and actual application integration do not exist for the proposed Pi adapter yet. They are explicitly required before delivering implementation.
- No commits or pushes were made for this planning task.

## Final implementation-readiness review

- A fresh-context reviewer read the final plan, relevant current source/tests/specifications, installed Pi 1.4.0 source/docs and supplied probe artifacts after D8–D11 were approved and incorporated.
- **Findings:** none. **Plan verdict:** OK. **Implementation-readiness verdict:** READY, with no blockers.
- Verified consistency of the startup readiness exchange and separate timeout budgets, confirmed-success versus cleanup-warning rules, warning delivery outside visible progress, version/isolation policy and implementation ordering.
- OpenCode timeout preservation includes its existing distinctions: a 120-second HTTP client timeout, 30-second server startup context, and session-deletion deadlines of 10 seconds in TTY versus five seconds in non-TTY. These remain compatibility constraints, not a reason to duplicate orchestration.
- Evidence report: `/home/kot/.local/share/pi/tasks/github.com-chpock-gen-commit-msg-7dbe/2026-10-24-pi-harness-plan/final-readiness-review.md`.
- The reviewer did not run commands, models or tests. Existing native probes predate the new readiness handshake and application cleanup-warning propagation; the plan explicitly requires those checks during implementation.
- No material design questions remain. Specifications and acceptance tests are the first implementation step; application integration, authenticated smoke testing, `make all` and implementation review remain delivery gates.
- The readiness verdict does not authorize implementation, commits or pushes by itself.

## Oracle critique — findings at review time

- A forked oracle challenged the plan against inherited operator decisions, fresh project/Pi/Go source and probe evidence. A focused follow-up separated required correctness work from additional behavior choices. No remediation was applied.
- **Readiness at critique:** the earlier reviewer READY verdict was challenged pending individual operator disposition of O1 and O2. The core harness architecture and D1–D11 were not rejected. Subsequent dispositions are recorded below.
- **O1 — confirmed lifecycle/scope gap:** `ProcessServer.Stop` discards signal/wait/kill errors and can return before its wait goroutine finishes (`internal/server/server.go:201–221`). Its existing caller already accepts an error for warning display. Completed waiting and meaningful diagnostics are necessary to fulfill D8/D11. Separately, context cancellation can race with SIGTERM because the default `exec.CommandContext` cancellation kills the process (`server.go:52–60,203–207`; Go `os/exec/exec.go:488–503`). Graceful-first SIGTERM → existing five-second wait → forced kill if needed → complete waiting is recommended but changes effective behavior and awaits explicit operator approval. Ordinary requested-signal termination must not automatically become a cleanup warning.
- **O2 — confirmed prerequisite/coverage gap:** native Pi grep/find require cached or system rg/fd (including the system name fdfind); offline mode prevents downloading missing helpers. Existing probes verify tool declarations, not execution. Proposed disposition is to document this limitation, test actual execution/missing-helper behavior and accept native tool errors without new startup rejection, downloads or fallback. This awaits operator discussion after O1.
- The parent independently verified the factual basis of both findings. They are pre-existing OpenCode limitations and an omitted Pi deployment prerequisite respectively, not evidence that RPC or CLI isolation should be replaced.
- Reports are under `/home/kot/.local/share/pi/tasks/github.com-chpock-gen-commit-msg-7dbe/2026-10-24-pi-harness-plan/`: `oracle-critique.md`, `oracle-followup.md`, and `oracle-findings-verification.md`.
- Review validation used fresh source inspection and read-only Git/hash checks. No application/model experiments or tests were run for these findings.
- At critique time the implementation plan was unchanged, with SHA-256 `23da0ecb8f92e0899e11db225a8fb614391242f91232ea5ed296a82a35eb5d90`. This section records the original findings, not approval to fix them or start implementation. Subsequent operator decisions are recorded below.

## O1 operator decision — approved as D12

- The operator approved the OpenCode shutdown correction within the OpenCode workflow refactor: SIGTERM, up to the existing five seconds to exit, SIGKILL if necessary, then completed process waiting before return.
- The plan explicitly includes removal of the competing immediate cancellation-driven kill, accurate cleanup diagnostics and classification of expected requested-signal termination. It preserves the other existing timeout values and the D11 candidate/error policy.
- Step 2 now includes the correction and process-fixture checks for graceful/forced shutdown, completed waiting, prior exit and correct error/warning delivery. These checks remain future implementation work; no code or tests have been changed or run.
- **O1 disposition:** scope and shutdown-order decision resolved; correction is planned, not implemented or verified. The compatibility-minimal option retaining the race was not selected.
- **O2 disposition:** still open for individual discussion. No helper-availability policy, downloads, dependency changes or automatic startup rejection has been approved.
- Only the plan, decision/review records and task evidence were updated. This approval is not an instruction to start application implementation or create commits.

## O2 operator decision — approved as D13

- The operator chose to retain Pi's native `read,grep,find,ls` read-only set and document the search-helper dependencies, rather than reducing tools or enabling the default coding set.
- D13 records that grep needs cached/system rg and find needs cached/system fd or the supported system name fdfind. Offline mode remains enabled; missing helpers produce ordinary native tool errors, not new application startup rejection, installation or harness fallback.
- The plan now requires capability-specific dependency documentation and native integration cases that actually call grep/find, verify results with helpers and errors without helpers, and verify no helper download. Earlier probes establish declarations only; the new execution checks remain future delivery evidence.
- **O2 disposition:** policy decision resolved and incorporated into the plan. No helper installation, mandatory application dependency, replacement tool or implicit behavior change was authorized.
- Both oracle findings now have operator-approved plan dispositions. This does not claim either implementation work or runtime verification is complete. Final oracle closure will check the updated dispositions, not start a new architecture interview or authorize implementation.

## Oracle disposition closure — READY TO IMPLEMENT

- The same oracle fresh-read the updated plan and review log after D12/D13 were approved and recorded.
- **O1:** design/scope concern resolved. The plan includes graceful-first OpenCode shutdown, the existing five-second grace, completed waiting, meaningful diagnostics and behavioral verification gates.
- **O2:** prerequisite/policy concern resolved. The plan documents cached/system helpers, preserves offline/native-error semantics and requires actual helper-execution and missing-helper coverage.
- **Verdict:** READY TO IMPLEMENT. No new material contradiction or remaining implementation-readiness blocker was found in the consultation.
- Closure report: `/home/kot/.local/share/pi/tasks/github.com-chpock-gen-commit-msg-7dbe/2026-10-24-pi-harness-plan/oracle-disposition-closure.md`.
- The oracle performed document review only: no code, tests, models or probes were run or changed. Shutdown fixes, final feature documentation, regression/native integration checks and implementation review remain future obligations; this is not implementation acceptance.
- The parent verified the recorded dispositions and repeated whitespace and working-tree checks. Application code/tests remain unchanged, and implementation/commits/pushes still require operator authorization.

## Implementation authorization and baseline

- The operator authorized implementation of the approved plan, with a mandatory stop for new architectural decisions, scope expansion or material trade-offs. No commit/push authorization was given for this work.
- The parent read the approved contracts and affected current source/specifications. The application baseline remains `48c5309`; baseline `make all` passed, including race tests.
- Added the Pi/OpenCode product and UX specifications and updated superseded main/progress/context/output requirements. Historical review entries above remain records of their original scope and authorization state.
- Multi-seam ownership/gates and handoff routing are recorded in `/home/kot/.local/share/pi/tasks/github.com-chpock-gen-commit-msg-7dbe/2026-10-24-pi-harness-plan/implementation-board.md`. Mutation ownership is serial in the current checkout; independent reviews remain required.
- Implementation acceptance, native helper checks, actual TTY checks and authenticated Pi smoke testing are not yet complete.

## OpenCode lifecycle implementation and D14 refinement

- The lifecycle worker implemented D12 within `internal/server`, with only the two mechanical `server.New("opencode")` caller changes. `New(executable string)` retains the selected executable path for direct execution; application orchestration is otherwise unchanged.
- Behavioral tests preceded production edits. Initial RED checks reproduced a missing graceful SIGTERM handshake caused by the cancellation kill race and a discarded abnormal wait result. Removing the lifetime `CommandContext` cancellation race permits direct-server SIGTERM, graceful waiting, forced escalation only if needed, and completed direct-child waiting/output draining before return.
- **New issue verified:** inherited stdout/stderr can remain open in descendants after the direct server exits. The original dedicated process group and Go pipe/wait behavior establish this possibility; it was not observed with a real OpenCode process. A controlled descendant fixture reproduced a blocked stop under direct-process-only forced termination.
- **D14 operator decision:** send graceful SIGTERM only to the direct server. Keep the existing five-second grace, including waiting/draining; when shutdown is still incomplete, send SIGKILL to the existing dedicated process group, then complete direct-child waiting/draining. Do not add group-wide SIGTERM, descendant enumeration or a generic process framework. The plan and new harness product/UX specs now record this refinement.
- The descendant fixture verifies a successful direct-server exit before escalation, no SIGTERM delivered to the descendant, group termination, completed output draining and direct-child reaping. Orphan descendant reaping belongs to the OS adopter, not the server's direct-child Wait.
- Cleanup errors now preserve unexpected already-observed exits, abnormal wait results, forced termination and genuine signal/kill/drain failures. A SIGTERM-compatible wait after an accepted stop request is not automatically failure; the implementation does not assert which sender caused a racing exit. Startup failure/cancellation preserves its original error while joining cleanup diagnostics and waiting for drains before reading stderr. Repeated Stop calls reuse the completed result.
- Initial `make all` runs found two test-only lint issues (error-channel result consumption), then an existing CLI regression expecting the established successful-shutdown log message. The test expression was corrected and the successful log text retained; no CLI test or orchestration change was needed.
- Focused lifecycle race tests passed, including ten repetitions. Full `make all` passed after those corrections. Final verification also covers the selected executable path without a second PATH lookup. Scoped server diagnostics report zero errors/warnings. The durable server handoff records final commands and results.
- This is implementation evidence and the worker's self-check, not an independent-review verdict. Fresh read-only review of this seam and later shared-adapter cleanup-warning/candidate preservation integration remain required. No real harness/model calls, dependencies, commits or pushes were used.
- A further drain regression check first failed when a post-readiness output line exceeded the startup scanner's record limit. After the listen URL is recognized, stdout now switches to unparsed byte draining rather than continuing startup parsing. Startup URL parsing and its existing numeric limits remain unchanged; this prevents drain implementation from terminating a healthy server on unrelated long output.

## OpenCode lifecycle implementation review — S1

Independent source review of D12/D14 found one blocking diagnostic defect: `Stop` clears `os.ErrProcessDone` when the direct child has already been reaped but inherited output pipes are still draining. If draining completes within the grace period after an unrequested zero-status exit, the method incorrectly reports successful cleanup. The parent verified the claim against current `internal/server/server.go` and Go's process/wait implementation. The reviewer labeled it P1; parent triage is P2 because the demonstrated loss is a required cleanup diagnostic, not candidate corruption or abandoned waiting. It remains a required fix, not a deferred improvement.

Disposition: confirmed, correction and behavioral regression pending. Preserve the already-exited observation while completing waiting/draining; do not change D12/D14 signal targets or grace. No architecture, timeout or scope decision is required. Before the correction, parent-run `go test -count=10 -race ./internal/server` and `go test -count=1 -race ./cmd/gen-commit-msg -run '^TestNonInteractiveSuccessCleansUp$'` passed; those existing tests do not cover S1. Review artifact: `server-lifecycle-review.md` in the established task evidence directory. The OpenCode refactor remains in progress.

### S1 closure

S1 is fixed: `os.ErrProcessDone` is retained as a contextual cleanup error, without changing signal targets, the grace period or completed waiting/draining. The new process regression reproduced the prior nil-error result before the fix, then passed 20 race-enabled writer runs; the server suite passed ten runs and `make all` passed. The parent fresh-read the corrected branch and independently reran the new regression ten times with the race detector: PASS. The targeted independent reviewer fresh-read source and tests, found no remaining issues and returned **Merge verdict: OK** (`server-lifecycle-s1-review.md`). S1 is closed. Full OpenCode adapter/TUI/application extraction and its review gate remain pending.

## OpenCode shared-operation integration checkpoint

- The operator authorized one checkpoint commit after the complete OpenCode workflow refactor passes validation and independent review, followed by continued Pi implementation. No push or additional commit is authorized.
- The neutral generation contract, extracted OpenCode generator and harness-neutral TUI are now used by production. Interactive and non-interactive execution share Git collection and generation; each backend completes resource cleanup before returning candidates. `main` exits only after the return-based application entry point finishes its deferred work.
- Preserved the installed OpenCode agent bytes, native SDK request/prompt and decoder acceptance. Existing numeric HTTP/startup/session-delete limits remain, including ten-second TTY versus five-second non-TTY deletion. D12/D14 are the approved shutdown-behavior exception.
- Cleanup warnings have an independent result channel. Non-TTY, quiet and dismissed/failed TUI paths expose them on stderr; ordinary visible progress retains their details. Warnings never enter the commit-message stdout/file or replace the primary failure. Cancellation suppresses candidates, including cancellation during cleanup.
- New tests first reproduced output written before DELETE cleanup, silent non-TTY/quiet cleanup warnings and missing deletion diagnostics on primary failure. Those regressions pass with the shared operation. Behavioral tests cover collection and reporter ordering, cancellation/error precedence, panic recovery, completed cleanup, output preservation and destination separation.
- Actual application PTY tests cover first/second selection, single-candidate auto-selection, Escape preservation of existing/missing output, visible/quiet cleanup warnings and dismissal during a pending provider request followed by completed cleanup. They exposed long-warning clipping by the terminal renderer. A focused model test reproduced it at widths 40 and 80; ANSI-aware wrapping fixes the model and actual-PTY regressions without changing warning policy or list styling.
- The final focused race checks, full `make all` and whitespace checks passed. Scoped application/TUI diagnostics report no errors or warnings. Existing ANSI/sys modules were promoted to direct requirements; no module versions or sum entries changed.
- Integrated independent review is pending. The checkpoint has not yet been committed. Pi selection/RPC, native helper execution, authenticated smoke testing and final feature acceptance remain pending. No real model was invoked.
- Integration evidence: `opencode-integration-handoff.md` in the established task directory. Component evidence remains in the neutral-foundation, server-lifecycle, generator and neutral-TUI handoffs.

### Integrated review — R1

The fresh independent reviewer returned **Merge verdict: BLOCK** for a quiet-TTY pause regression (reviewer P1). When generation fails with quiet mode and pausing enabled, the application prints the generic/rich error but passes an empty instruction to the pause reader; unlike the previous error renderer, that path never displays the Enter hint. The parent verified the current error branch, pause reader and original rendering contract. This is an undisclosed input wait after completed cleanup, not a cleanup deadlock. Disposition: confirmed; restore the instruction only when the quiet path actually pauses, add actual-PTY coverage and recheck the fix. No new architecture, scope or pause policy is required. The checkpoint remains uncommitted. Review evidence: `opencode-checkpoint-review.md` in the established task directory.

R1 correction: quiet errors now display the existing Enter instruction only when pausing is enabled. The new actual-PTY test first failed because the instruction never appeared, then passed for pause on/on-error/off. It checks completed exactly-once cleanup before presentation, continued waiting after a non-Enter byte, exit 1 after Enter, hidden quiet progress and unchanged output. The focused test passed five race-enabled repetitions; `make all`, scoped application diagnostics and staged/unstaged whitespace checks passed. The original reviewer will recheck R1 and its blast radius before the checkpoint commit.

### R1 closure and OpenCode checkpoint gate

The same independent reviewer fresh-read the corrected branch and PTY matrix and returned **Merge verdict: OK**. R1 is closed; no prior actionable findings or new defects remain in the correction's blast radius. The reviewer had no test runner, so runtime evidence is explicitly parent-run; its source review and fresh diagnostics are independent. Closure evidence: `opencode-checkpoint-r1-review.md` in the established task directory.

Step 2 is accepted at its checkpoint: the production OpenCode workflow, shutdown correction, neutral TUI and CLI integration pass their focused checks, full `make all`, real-PTY regressions and independent review. The authorized checkpoint commit may now be created. This acceptance does not claim Pi implementation or final feature validation: Steps 3–6, native helper/missing-helper execution and authenticated Pi smoke testing remain required.

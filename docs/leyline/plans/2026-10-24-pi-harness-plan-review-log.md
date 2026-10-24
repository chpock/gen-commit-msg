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

## Checkpoint commit and next component handoffs

- Created the operator-authorized OpenCode checkpoint commit `98ea46a` (`refactor(opencode): unify generation and cleanup workflow`) after Step 2 checks and independent review closed S1/R1. The working tree was clean afterward. No push occurred; further commits remain unauthorized.
- The configuration/selection component adds the harness flag/environment precedence, invalid-value handling, native PATH discovery and an absolute retained executable path. Existing OpenCode subprocess fixtures are explicitly pinned before application auto selection is enabled. Behavioral lookup tests cover both/either/neither executable, explicit modes, lookup errors, no runtime invocation and retained-path execution after PATH changes. Focused race checks, `make all` and whitespace checks passed.
- The pure Pi validator accepts only the approved object/field/type/count/body/subject contract without changing OpenCode decoding or adding writing-style rejection. Behavior-first tables cover accepted values and 52 invalid cases; focused race checks, `make all`, scoped diagnostics and whitespace checks passed. It does not implement process/RPC behavior or establish final Pi acceptance.
- Parent fresh-source inspection confirmed the selector and validator match their component contracts. Independent component review and Pi process/RPC implementation are next. CLI factory wiring, runtime no-fallback/ignored-option/precheck acceptance, native helper execution and real-model smoke testing remain pending.
- Durable component evidence: `harness-selection-handoff.md` and `pi-validation-handoff.md` in the established task directory. Pi protocol research anchors are in `pi-rpc-implementation-anchors.md`; the actual installed versioned source, not simplified historical probe code, is authoritative for message extraction and lifecycle behavior.

## Pi RPC component and application wiring

The Pi generator component implements the approved version/readiness, one-process RPC, strict completion/correction and independent cleanup policies. Writer-run stress/fixture checks and `make all` passed; the parent fresh-read the production source and reran the Pi race suite successfully. This is implementation evidence, not native/model acceptance.

Fresh independent backend review returned **BLOCK** with two diagnostic findings. The parent verified both against `internal/pi/process.go`/`protocol.go`, the installed Pi initialization path and the native-recovery fixture:

- **P1 (reviewer severity), lost stderr:** process stderr is copied to `io.Discard`, so startup failures before RPC lose their actual explanation even from logs. Confirmed required defect. Retain bounded failure diagnostics for structured logging, without raw stdout/TUI forwarding or changing primary-error/cleanup semantics.
- **P2, lost handled failures:** recovered assistant errors and native retry/tool/compaction failure details disappear when a later response succeeds. Confirmed required logging defect. Log selected actual failure diagnostics without dumping protocol records or turning recoverable tool events into generation failures/corrections.

Both corrections are within the existing logging/UX contract and require behavioral logging regressions plus the same reviewer's narrow recheck. No new architecture, permission, fallback or timeout policy is authorized by these findings. Evidence: `pi-rpc-component-review.md` and `pi-rpc-generator-handoff.md` in the established task directory.

The parent integrated the selector with exactly one backend factory after application prechecks, retaining resolved executable paths and OpenCode deletion budgets. New real-application subprocess/PTY tests first reproduced old OpenCode-only wiring, then passed Pi selection, no-fallback failures, exact correction/lifecycle counters, ignored agent-install side effects, output preservation, signal cancellation and cleanup-before-output. `make all`, focused CLI race checks, scoped diagnostics and whitespace checks pass. Application integration still needs independent review; documentation/packaging and native/model delivery gates remain pending. Evidence: `pi-cli-integration-handoff.md`.

### Pi diagnostics closure and application acceptance

Both diagnostic findings are fixed and the same independent reviewer returned **Merge verdict: OK**, with no new actionable issues. The parent fresh-read the bounded stderr capture and selected native-event logging and reran all focused diagnostic race regressions successfully. Earlier stdout/TUI, acceptance, correction, cancellation and cleanup semantics remain unchanged. Retention is bounded to recent 16 KiB of stderr; truncation/total bytes are explicit, and optional diagnostic metadata is not a new protocol requirement. Evidence: `pi-diagnostics-handoff.md` and `pi-diagnostics-review.md`.

The separate fresh application review also returned bounded **OK**, without findings. Deterministic selection, retained paths, ignored OpenCode options, prechecks, warnings/output/PTY and signal wiring are accepted at their integration gate. This does not substitute for actual native-Pi/model validation.

### Documentation and AUR artifacts

README now describes Pi/OpenCode selection, no fallback, Pi minimum/model/auth/isolation/read-only limitations, helper prerequisites/offline behavior, prompt/time budgets, cleanup warnings and shared workflow. Existing editor integration is preserved. Both AUR templates retain mandatory dependencies and list Pi/OpenCode as alternatives; the Pi entry uses actual `pi-coding-agent-bin>=1.4.0`, verified against authoritative AUR RPC/PKGBUILD (upstream v1.4.0 and `/usr/bin/pi`), not an invented package name.

Both templates pass Bash syntax and native `makepkg --printsrcinfo` checks; the metadata places both harnesses only in optdepends. An initial makepkg invocation used a path outside its required package working directory and was corrected; both proper-directory checks pass. No package was built, installed or published. The exact-tree `make all` and whitespace checks pass after these changes. Fresh documentation/artifact review and native/helper/model validation remain pending.

### Documentation review and actual native-Pi evidence

Fresh independent README/AUR review returned **OK with notes**, without defects; the reviewer verified actual installed Pi contracts and freshly fetched the authoritative AUR PKGBUILD. The remaining notes are validation boundaries, not unresolved documentation findings. README subsequently documents the actual optional native command and its Node/preload/helper prerequisites; that small addition is included in final review.

The optional native fixture now runs the actual application `main()` in an isolated subprocess against installed Pi 1.4.0 and a local API. The parent fresh-read both maintained test/observer files and independently reran all six cases successfully. Actual cached rg/system fd searches, native missing-helper errors, no management/helper download attempt, instruction/resource suppression, same-conversation correction with a deliberately changed Git index, real PTY selection/cancellation and completed processes are observed rather than inferred from declarations. Test-only fetch observation is positively checked and scoped to the installed management/helper global-fetch path; it is not an OS network firewall. Both operator credentials and remote model traffic are excluded.

Per full six-case run: six version and six RPC processes; seven application prompts/acceptances/non-aborted settlements; twelve local API requests; eight real search executions; twenty native tool calls with two expected missing-helper errors; zero persisted sessions, fallback executions, external management/helper fetch attempts or surviving observed PIDs. The fixture-only no-match negative control fails despite unchanged declarations and a later valid answer. Evidence and exact per-case counters: `native-pi-validation-handoff.md`.

The native worker's focused/ordinary race suites, Node syntax, `make all` and whitespace checks pass; parent native race rerun also passes. The final fresh review will inspect these native files with the assembled feature. The authenticated standalone-binary/model smoke remains pending; localhost evidence is not substituted for it.

### Authenticated standalone-binary smoke

The parent ran the built standalone application against a private Git index containing actual changed `main.go`, configuration and selector sources. The ordinary Pi provider/model/auth/thinking settings were preserved; no credentials were inspected, login performed or settings changed. An observation-only Node stdout tap recorded selected metadata while forwarding protocol bytes unchanged. Explicit output/count/pause arguments prevented inherited application settings from routing smoke output to an operator file.

Observed result: exit 0; provider `openai-codex`, model `gpt-6.1-sol`, thinking `high`; one version process, one RPC process, one accepted application prompt, one successful assistant message (`stop`), one non-aborted settlement, empty stderr and no surviving owned native PID. The model produced `feat(cli): add Pi and OpenCode harness selection`, which accurately describes the staged configuration/selection/factory subset. The real Git index remained byte-for-byte unchanged. No application corrective prompt was needed; localhost/native counters are separate from this authenticated smoke. Model usage reported 9037 total tokens; no claim of exhaustive provider/model coverage is made.

Credential-free evidence: `authenticated-smoke/result.json` in the established task directory; controller: `run-authenticated-smoke.py`. The real-model delivery gate is now run and passing. Final independent assembled-feature review and the final exact-tree repository gate remain pending.

### Final assembled review and exact-tree gates

Fresh independent assembled-feature review returned **OK with notes**, with no P0/P1/P2 findings and no new architectural decision. It fresh-read the fourteen untracked feature files, native Go/CJS fixture, integrated contracts and actual authenticated result. Notes explicitly distinguish parent/worker-run tests from independent source/authoritative-contract checks and limit native/model evidence to the exercised installation/model. Evidence: `pi-feature-final-review.md`.

The parent then completed the frozen-tree delivery gates: `make all` (fmt, vet, lint with zero issues, all twelve package race suites and standalone build), tracked/untracked Git whitespace checks, Node observer syntax, empty real staging area and intended-file inventory. The final source has no unresolved implementation or required-validation finding. HEAD remains the sole authorized checkpoint `98ea46a`; later feature changes are uncommitted and no push occurred.

### Additional documentation scope question

Final inventory inspection confirmed that unchanged `AGENTS.md:20–26,31,35–37` still describes an OpenCode-only architecture/dependency and legacy non-TTY flag behavior. These maintainer-guidance sections were not named in the plan's README/help/AUR documentation edits. The parent has not silently expanded the file scope. Operator decision requested: either authorize a narrow factual refresh of Architecture, Config precedence and Runtime dependencies (no development-rule/policy changes), or leave that maintenance outside this completed implementation plan. This is not an application defect or a failed required gate.

### Maintainer-guidance scope decision

The operator authorized the recommended narrow documentation refresh. Updated only `AGENTS.md` Architecture, Config precedence and Runtime dependencies: both backends/shared lifecycle, `GCM_HARNESS`, optional runtime selection/no fallback, and actual non-TTY `--subject-max 1` requirement. Other sections, including commands, development workflow and logging policies, are unchanged. Added this explicit documentation scope to plan Step 5. No runtime behavior, model setting, dependency or permission decision changed. A targeted document-only final recheck remains; all application/native/model results remain applicable because executable code is unchanged.

### Delivery closure

The same final reviewer fresh-read the approved maintainer-guidance update and returned **Document-recheck: OK / Merge verdict: OK**, without factual/instruction defects or remaining actionable findings. The byte-scope and whitespace checks pass. Only documentation and review/status records changed after the last successful full application gate; executable code, native fixture and model/runtime configuration are unchanged, so those results remain valid without redundant model calls. Evidence: `agent-guidance-review.md`.

All plan implementation and required delivery gates are complete. Accepted limitations remain: Pi read-only tools do not enforce repository confinement; ordinary global Pi runtime/configuration effects remain; search helpers are optional and may return native errors; diagnostic retention is bounded. Native verification covers installed Pi 1.4.0 and the authenticated smoke covers the selected `openai-codex/gpt-6.1-sol` setup, not every provider/future release. No real-model OpenCode run or AUR package build/install/publication is claimed; the approved OpenCode process/SDK/application/PTY and AUR artifact gates passed.

Publication state: `98ea46a` is the authorized OpenCode-refactor checkpoint. Later Pi/documentation/test changes are intentionally uncommitted and unstaged. No push, installation, deployment, credential inspection or manual model/auth reconfiguration occurred. No unresolved defect, failed required check or open scope decision remains within this task.

### Model/thinking documentation clarification

README's Pi section and the product configuration contract now explicitly state the existing behavior: no application-specific model/thinking flags or environment variables, no corresponding Pi CLI arguments or RPC changes, ordinary Pi user-level startup selection, and no inheritance from another open session or disabled project settings. Clarified that `enabledModels`, per-model `modelThinkingLevels` and model capabilities also participate, rather than promising exclusive use of `default*` fields. README includes `/thinking` plus `Ctrl+S` for saving a startup level.

Verified against the application invocation/minimal RPC commands and installed Pi 1.4.0 `main.js::buildSessionOptions`, startup settings/model-scope resolution, SDK thinking selection and model/settings documentation. This is a documentation clarification, not a new model/provider policy or runtime change. No model request, application code change, commit or push was made; executable validation results remain applicable.

### Authorized Pi feature commit — 2026-10-24

The operator authorized a local commit of the completed Pi feature, including the explicit model/thinking documentation, after the separately committed OpenCode checkpoint `98ea46a`. Prior independent component, assembled-feature and document review findings remain closed; no runtime code changed after their acceptance.

Fresh pre-commit checks pass: scoped Go diagnostics (zero errors/warnings), `make all` (fmt, vet, lint, complete race-enabled suite and standalone build), `GCM_TEST_NATIVE_PI=1 go test -count=1 -race -v ./cmd/gen-commit-msg -run '^TestNativePi'`, native observer Node syntax, both AUR shell syntax/`makepkg --printsrcinfo` checks and Git whitespace validation. Native checks use the isolated local API without operator credentials or paid requests. The earlier authenticated smoke remains valid on the unchanged executable source and was not repeated.

The commit contains only the reviewed feature source, regression/native fixtures, configuration, README/maintainer guidance, AUR metadata and specification/plan records. No push, package installation or deployment is authorized or performed.

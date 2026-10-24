# Pi harness support — implementation plan

Date: 2026-10-24
Baseline: `48c5309`
Status: implementation authorized and in progress under D1–D14. Oracle confirmed READY TO IMPLEMENT with O1/O2 resolved at the design/scope level. New architecture/scope/trade-off questions require stopping for an operator decision. The operator authorized one verified OpenCode-refactor checkpoint commit followed by continued Pi implementation; pushes and further commits require separate authorization.

## 1. Decision surface and implementation gate

### Decisions already approved

| ID | Decision | Consequence |
| --- | --- | --- |
| D1 | Add `--harness auto|pi|opencode`, default `auto`, and prefer Pi. | Existing installations with both executables change their default harness; `--harness opencode` preserves the former choice. |
| D2 | Use CLI-level configuration isolation, not a separate application-owned Pi configuration directory. | Keep ordinary Pi authentication, model definitions/selection and runtime settings; explicitly disable incidental instructions/resources and project settings. |
| D3 | Availability means an executable found through PATH. | Auto selects Pi when found. Version, authentication and generation failures do not trigger fallback to OpenCode. |
| D4 | Validate Pi results strictly, with at most two corrective generation requests. | Up to three application-level prompts; no extracting JSON from Markdown, silently fixing output, or returning an invalid earlier attempt. |
| D5 | Use one `pi --mode rpc --no-session` process. | Corrections share in-memory conversation state. No create/delete-session API calls or agent installation are needed. |
| D6 | Ignore OpenCode-only agent/install options in Pi mode. | `--agent`, `--install-agent`, `GCM_AGENT` and `GCM_INSTALL_AGENT` do not select Pi instructions or create files. Document their scope and log ignored configuration without polluting normal stdout. |
| D7 | Allow native `read,grep,find,ls` without an OS/path sandbox. | Repository-only reading is an instruction, not an enforced path boundary. The operator explicitly accepted this limitation. |
| D8 | Introduce one harness-neutral generation operation and refactor OpenCode to use it within this task. | TTY and non-TTY share orchestration; each harness owns its resources and completes cleanup before returning. Non-TTY output consequently moves after cleanup, matching the existing interactive ordering. |
| D9 | Pi startup/readiness has a 30-second budget, each prompt operation has 120 seconds, and shutdown has a five-second grace period before forced termination. | Each initial/corrective prompt has its own budget, including native tool work and retries. There is no combined 120-second generation budget. Existing OpenCode timeout values remain unchanged; its shutdown ordering follows D12. No new timeout flag is introduced. |
| D10 | Pi 1.4.0 is the minimum supported version; no backward compatibility with older versions is required. | Verify the selected executable's version before generation. Reject older or unrecognizable versions explicitly, without fallback. Continue validating RPC behavior independently of the version number. |
| D11 | Cleanup failure after confirmed successful generation is a warning, not loss of the generated candidates. | Pi requires a successful final assistant response, non-aborted settlement and valid output before this rule applies. Always terminate/reap owned processes, expose cleanup warnings separately, preserve the primary generation error, and honor cancellation. Successful selection/output still exits 0. |
| D12 | Correct OpenCode shutdown within its workflow refactor: SIGTERM, up to the existing five seconds to exit, SIGKILL if necessary, then complete waiting before return. | Remove the competing immediate context-cancellation kill, report genuine cleanup failures, and distinguish expected requested-signal termination. Timeout values remain unchanged, but graceful-first ordering intentionally changes effective shutdown behavior. |
| D13 | Retain Pi's native read-only set `read,grep,find,ls` and document the cached/system helper requirements for grep/find while keeping offline mode. | Grep requires rg; find requires fd or the supported system name fdfind. Missing helpers produce ordinary native tool errors, not new startup rejection, downloads or fallback. Verify actual tool execution and the missing-helper case, not only declarations. |
| D14 | Refine OpenCode escalation: SIGTERM targets only the direct server; if shutdown including waiting/draining is incomplete after the existing five seconds, SIGKILL targets its existing dedicated process group. | Descendants can retain stdout/stderr after the direct server exits. Complete direct-child waiting and output draining after group escalation; do not add group-wide SIGTERM, descendant enumeration or a generic process framework. This is a source-proven lifecycle possibility, not a reproduced real-OpenCode hang. |

### Approved architecture

- A small harness-neutral generation contract; each implementation owns its lifecycle.
- One shared application orchestration path for Git-context collection and generation, used by both TTY and non-TTY callers.
- Refactoring the existing OpenCode orchestration into that contract is required scope, not an optional follow-up. Retain its HTTP client/server/agent components, with the narrowly scoped server-shutdown correction required by D12.
- Each generation operation completes backend-owned cleanup before returning candidates. Both TTY selection and non-TTY output happen after that return; the non-TTY ordering intentionally changes from the current output-before-deferred-cleanup sequence.
- A native Go subprocess/RPC adapter for Pi; no Node SDK wrapper or new runtime package installed by this application.
- Harness-specific progress steps, not fake Pi server/session operations.

### Approved Pi timeout policy

- Startup and readiness: 30 seconds for the startup phase, including executable/version verification and the RPC readiness exchange. Use a correlated `get_state` command to establish readiness without starting model work; do not infer readiness merely from successful process spawning.
- Generation: 120 seconds for each RPC `prompt` operation, from sending the command through its accepted response and `agent_settled`. Include native Pi tool calls, retries and compaction in that operation's budget; receiving progress does not reset the timer.
- The initial prompt and up to two corrective prompts have independent budgets. Three prompt operations can consume up to 360 seconds, in addition to startup and shutdown; there is no shared 120-second cutoff.
- Shutdown: allow five seconds for orderly shutdown, then force termination as necessary and always wait/reap the process. Cleanup uses a separate budget even when generation was canceled or timed out.
- Preserve existing OpenCode timeout values; D12 explicitly corrects its shutdown ordering and completion/error reporting. The common generation interface does not require identical backend timeout policies.
- These are application constants, not new CLI flags.

### Approved OpenCode shutdown correction

- Correct `ProcessServer.Stop` as part of the OpenCode workflow refactor, not as an unrelated rewrite. Its existing `Stop() error` interface already accommodates reporting failures.
- Request graceful termination with SIGTERM to the direct server only. Allow up to the existing five seconds for shutdown, including process waiting and output draining. If shutdown remains incomplete, send SIGKILL to the existing dedicated process group, then complete direct-child waiting and draining before returning (D14). Descendants can retain output pipes after the direct server exits; this possibility is established by source and a controlled fixture, not a reproduced real-OpenCode hang. Do not add group-wide SIGTERM, descendant enumeration or a generic process framework.
- Do not cancel the `exec.CommandContext` lifetime in a way that races an immediate kill against the graceful phase. The backend still owns shutdown on success, failure and cancellation.
- Preserve meaningful termination/wait failures rather than unconditionally returning success. Ordinary application-requested SIGTERM termination is not automatically a cleanup failure just because the wait result is nonzero. Needing forced termination or observing a genuine shutdown failure produces the D11 cleanup diagnostic.
- Preserve successful candidates after cleanup failure, and retain the primary generation error when both generation and cleanup fail. Do not infer perfect causal ordering when process exit races a stop request.
- Keep the existing numeric limits: 120-second HTTP client timeout, 30-second server startup context, session-deletion deadlines of 10 seconds in TTY and five seconds in non-TTY, and five seconds for server shutdown. Graceful-first ordering can increase actual shutdown waiting compared with the previous cancellation-driven kill; that behavior change is approved.

### Approved Pi version policy

- Pi 1.4.0 is the minimum supported version; older versions are outside the feature's compatibility scope.
- Verify `pi --version` within the startup budget, before starting generation or sending Git context. Reject older or unrecognizable versions with a clear error; do not fall back to OpenCode.
- Version verification does not replace RPC validation or guarantee compatibility with arbitrary future protocol changes. Do not rely on an older CLI rejecting an unknown isolation option.

### Approved generation and cleanup outcome policy

- Distinguish generation failure from failure to clean up after successful generation. Do not discard confirmed candidates solely because subsequent shutdown was abnormal or needed forced termination.
- For Pi, confirmed success requires the matching prompt acceptance, a completed final assistant response with `stopReason: stop`, non-aborted `agent_settled`, and output that passes the Pi validation contract. A JSON-looking intermediate response or exit status 0 alone is insufficient.
- After confirmed success, an abnormal process exit or forced shutdown produces a cleanup warning while preserving candidates. Still complete cleanup, drain output and wait/reap before returning; a warning is not permission to abandon a running child.
- Report cleanup warnings through the progress UI when visible, otherwise stderr, and log them. Never mix them into the commit-message stdout or output file. Quiet mode hides progress, not these warnings.
- Successful message selection and output exit 0 despite a cleanup warning. Output-write failure remains an error; user cancellation still suppresses message output.
- Startup failure, premature exit, generation deadline, provider/protocol failure before confirmed completion, invalid output after exhausted corrections, or cancellation do not become success merely because some candidate text was observed.
- When generation and cleanup both fail, retain the generation failure as primary and include cleanup diagnostics separately. Cleanup must not replace the original cause. OpenCode retains its native result acceptance; this rule does not add Pi's stricter decoder to OpenCode.

### Approved file-access boundary

Allowed tools: `read,grep,find,ls`. They preserve the current agent's useful read/search/list capabilities without shell execution or file writes. However, native Pi tools can access paths outside the repository with the operating-system user's permissions. This does **not** reproduce OpenCode's `external_directory: deny` rule. Repository-only reading remains an instruction, not an enforced path boundary.

The operator chose native read-only tools after considering no tools and separately designed path confinement. Do not add a sandbox, extensions or path-filtering machinery within this feature.

### Approved native tool prerequisites

- `read,grep,find,ls` is Pi's own native read-only set, not custom OpenCode tooling or an extension. Keep the explicit allowlist; Pi's default coding tools also permit shell commands and file mutations.
- Native `grep` requires ripgrep (`rg`); native `find` requires fd (`fd`, or `fdfind` through Pi's supported system lookup). Pi can use its cached helper binaries or executables available through PATH.
- Keep `--offline`. If a required helper is unavailable, Pi does not download it and returns its ordinary tool error to the model. This does not automatically invalidate a generation that later completes successfully and passes strict result validation; supplied Git context may suffice without those tools.
- Document these as prerequisites for the search capabilities, not new mandatory dependencies for every application invocation. Do not add automatic helper installation, startup rejection, substitute tools, a reduced tool set or harness fallback.
- Native integration must exercise actual grep/find calls with available helpers and with missing helpers. Verify returned results/errors, no helper download, and normal completion handling; tool declaration alone does not establish availability or execution.

**Gate:** material design decisions, including the O1/O2 dispositions, are approved, and implementation has been explicitly authorized. The corrections and validation remain delivery obligations, not completed evidence. Stop for new material decisions or scope expansion. One OpenCode-refactor checkpoint commit is authorized after Step 2 checks and independent review; no pushes, further commits or upstream changes are authorized.

## 2. Evidence and headless-mode investigation

### Authoritative sources

Pi references below are versioned to 1.4.0 under:
`https://github.com/earendil-works/pi/tree/v1.4.0/packages/coding-agent`.

| Subject | Evidence |
| --- | --- |
| Mode selection and failure semantics | `docs/cli-integration.md:9–80`; `dist/modes/print-mode.js:96–144` in the installed package. |
| Resource/tool/session/prompt flags | `docs/cli.md:78–240`; `dist/cli/args.js:72–78,87–89,108–125,156–199,229–237`. |
| Context/project trust | `docs/configuration.md`; `docs/security.md:27–82`. Context files load independently of project trust. |
| Appended-prompt suppression | `dist/core/resource-loader.js:465–493`: an explicit append source prevents APPEND_SYSTEM discovery; an empty source resolves to no addition. |
| Package startup side effects | `dist/core/resource-loader.js:368–409`; `dist/core/package-manager.js:704–744,1000–1057,1779–1793`. Package resolution precedes resource suppression. |
| RPC completion and shutdown | `docs/rpc.md:58–95`; `docs/rpc-commands.md:7–42`; `docs/json.md:31–68`; `examples/rpc-client.ts`. |
| Assistant result shape | `docs/message-types.md:119–145`. Terminal assistant state must be inspected, not inferred from process exit. |
| Native retries/compaction | `docs/settings.md:69–80,120–136`; `dist/core/agent-session.js:2391–2472`. |
| Native read-only set and helper prerequisites | `docs/cli.md:111–149`; `dist/core/tools/index.js:109–115`; `dist/core/tools/grep.js:52–55`; `dist/core/tools/find.js:118–125`; `dist/utils/tools-manager.js:19–24,75–92,301–312`. |
| Existing application coupling | `cmd/gen-commit-msg/main.go`; `internal/opencode/opencode.go`; `internal/opencode/runner.go`; `internal/tui/tui.go`; `internal/agent/gen-commit-msg.md`. |

The installed package root used for verification was:
`/home/kot/.pi/agent/install/releases/1.4.0/node_modules/@earendil-works/pi-coding-agent`.

### Mode comparison

| Mode | Fit | Decision |
| --- | --- | --- |
| `--print` | Simplest one-shot final-text interface. Conditional corrections require separate processes or another control channel. | Suitable for the rejected one-pass design; not selected after corrective requests were approved. |
| `--mode json` | One-shot JSONL events with no later input commands. It does not constrain the model's response to a schema. | No advantage over RPC for the approved correction flow. |
| `--mode rpc` | One process, explicit prompt commands, authoritative completed-message events and completion notification. | Selected. Implement only the required subset. |
| TypeScript SDK | Full in-process control in Node/Bun. | Not appropriate for the existing single-binary Go application. |

There is no arbitrary `--agents-file` selector in the inspected CLI. Use app-owned instructions through `--system-prompt`, not AGENTS discovery. Pi still creates internal in-memory session state; `--no-session` prevents conversation persistence, not all configuration reads/writes or process cleanup.

### Controlled runtime evidence

The real installed Pi was run against a localhost OpenAI-compatible fixture, using disposable configuration and dummy credentials. No paid model or operator credential was used.

Evidence directory:
`/home/kot/.local/share/pi/tasks/github.com-chpock-gen-commit-msg-7dbe/2026-10-24-pi-harness-plan/`.

Artifacts: `probe_pi_cli.py`, `cli-probe-results.json`, per-case stdout/stderr, `pi-isolation-evidence.md`, and `grill-session.md`.

Observed results:

- A 160 KB prompt reached the fixture through stdin; no large prompt argv or prompt file was necessary.
- An explicit system prompt alone still admitted a global APPEND_SYSTEM sentinel. Adding an explicit empty append argument removed it.
- Ambient AGENTS/SYSTEM/project APPEND instructions were absent with the selected isolation controls.
- Exactly `read`, `grep`, `find`, `ls` were declared despite conflicting global/project tool settings; marker extensions did not execute. These probes did not issue tool calls; actual helper execution and missing-helper behavior remain implementation validation gates.
- No session JSONL files were created in any case.
- RPC success, correction success and correction exhaustion made exactly 1, 2 and 3 fixture API calls respectively, with native network retries disabled. Each prompt was accepted and reached `agent_settled`.
- A provider authentication failure emitted terminal assistant `stopReason: error` while the RPC process later exited 0. The adapter must reject that result without a format-correction prompt.
- Print mode also returned exit 0 for malformed text. A synthetic truncated response without usage metadata triggered native recovery and then returned empty stdout with exit 0. Process success is never sufficient evidence of valid candidates.
- `--offline` still allowed model requests, prevented package installation, but did not prevent a configured package-manager `root -g` lookup. Without offline mode, even the disabled resource set reached an installation attempt; a controlled trap stopped it before installation.

These are integration observations, not assertions that the application implementation already exists. Initial incorrect probe assumptions and their corrected source explanations are retained in the task ledger.

## 3. Public behavior

### Harness selection

- Add `Config.Harness`, `--harness`, and `GCM_HARNESS`, preserving flag > environment > default precedence.
- Validate the enum during configuration parsing; invalid flag/environment values are configuration errors (exit 2).
- Preserve help/version and no-staged-files behavior: do not require either harness or start a harness process for those exits.
- Preserve the non-TTY multi-subject rejection before starting either generation runtime.
- Resolve executables using `exec.LookPath`; retain and execute the selected path rather than running a shell command.
- `auto`: Pi if found; otherwise OpenCode if found; otherwise a clear error naming both missing alternatives.
- Explicit `pi`/`opencode`: use only the requested executable. Never fall back on authentication, unsupported version, invalid result, timeout or process failure.
- Log requested mode, selected harness and selected executable. Do not inspect credentials or make a paid readiness request for selection.

### Model and legacy options

- Pi uses the user's normal configured model/provider/thinking behavior and credentials. Do not introduce `--model` or duplicate Pi login/configuration in this feature.
- Disabling extensions intentionally excludes extension-registered model providers. Built-in providers and compatible endpoints from `models.json` remain usable.
- OpenCode retains existing named-agent/custom-file/model/frontmatter/install behavior.
- OpenCode agent options are ignored by Pi and described as OpenCode-only in help/README. Do not read an installed OpenCode agent as the Pi instruction source.

### Result and output

Both harnesses continue returning candidates with a common body:

```json
{"subjects":["feat(scope): describe the staged change"],"body":""}
```

Preserve Git context format/order/truncation, subject ranking, single-candidate auto-selection, multi-candidate TUI selection, body formatting, quiet/non-TTY behavior and output-file preservation on cancellation/failure. Do not write a commit or modify the Git index.

Pi boundary validation must require:

- Exactly one JSON object, with no trailing non-whitespace data, Markdown fences or extra keys.
- Present, non-null `subjects` array and `body` string; correctly typed array elements.
- Candidate count inside the inclusive requested range.
- Nonblank, single-line subjects; reject embedded CR/LF rather than silently selecting only one line.
- Empty body when `--body=false`.

Stylistic instructions such as preferred line length remain instructions, not new hard rejection rules. Do not silently clamp counts, drop invalid subjects, fabricate missing fields or extract an embedded JSON fragment.

Apply this new strict external-input validation to Pi. Do not incidentally tighten the existing OpenCode decoder's acceptance behavior during the extraction/refactor; a broader validator change needs separate approval.

## 4. Pi invocation and lifecycle

### Process arguments

Using the approved native read-only tool boundary, the proposed argv is equivalent to:

```text
pi --mode rpc --no-session
   --no-extensions --no-mcp
   --no-skills --no-prompt-templates --no-themes
   --no-context-files --no-approve
   --offline
   --tools read,grep,find,ls
   --system-prompt <embedded application instruction text>
   --append-system-prompt ""
```

Pass each element directly through `exec.CommandContext`; the empty append value must be a real empty argument.

- Set cwd to the same repository working directory used by the current application.
- Do not create/install a Pi agent file, system-prompt file, extension, session file or temporary configuration tree.
- Keep ordinary Pi agent-directory resolution and authentication/proxy environment. Never print credential values or the full environment.
- `--offline` means no automatic startup/package networking or missing-tool helper downloads, not no AI networking. Retain usable configured/cached model catalogs; users refresh catalogs separately when necessary. Search helpers follow the D13 cached/system prerequisite policy.
- CLI isolation does not disable global retry/compaction, model endpoint overrides, credential helper commands, package-root lookup, migrations or OAuth refresh. Do not claim a sandbox or a completely read-only Pi configuration directory.

### Minimal RPC state machine

1. Within the 30-second startup budget, check the selected executable's version, start its RPC process and obtain a successful correlated `get_state` readiness response. Continuously drain stdout and stderr from startup through exit.
2. With a fresh 120-second operation budget, send one JSONL `prompt` command with a unique ID. Its message contains generation parameters, the response schema as text, and the app-collected Git context.
3. Correlate the matching command response and require prompt acceptance. A successful response only acknowledges acceptance; it is not completion.
4. Observe completed assistant messages from `message_end`. Ignore thinking/tool-call blocks when extracting final text; do not reconstruct candidates from streamed deltas.
5. Wait through native retries/recovery until `agent_settled`; `agent_end` alone is insufficient. Select the final completed assistant response for this prompt operation, not an intermediate tool-use turn or earlier failed attempt.
6. Require non-aborted settlement and a successful final assistant stop reason before JSON validation. Provider errors, output-token truncation, process/protocol failure or missing final response are runtime failures, not format-correction opportunities.
7. For a successful assistant response with invalid JSON/schema/content constraints, send a concise corrective prompt with the actual validation errors and the unchanged required schema. Give each corrective prompt its own 120-second operation budget. Use the same in-memory conversation and Git snapshot. Never send a correction before the previous operation has settled.
8. Stop immediately on a valid result, or fail after the third invalid result. Do not perform an extra fourth prompt. Preserve the final validation diagnosis in the error.
9. Close stdin to request normal RPC shutdown, allow a five-second grace period, then force termination if necessary; drain output and wait/reap the process before returning candidates. After confirmed successful generation, preserve candidates and report abnormal/forced shutdown as a cleanup warning. Premature exit before confirmed completion remains a generation error, and cancellation suppresses output.
10. On cancellation/deadline/protocol failure, request shutdown using a separate five-second grace period, then terminate the process/process group as necessary and always wait/reap. No cleanup path may depend on `os.Exit` executing defers.

Implement LF-delimited JSONL reading without the default 64 KB `bufio.Scanner` record limit: echoed context records can exceed that size. Tolerate unrelated event kinds without weakening required command/message/completion validation. Do not forward Pi protocol records or raw stderr into the application's stdout/TUI.

Only startup readiness (`get_state`), prompt control and stream consumption are needed; do not implement session creation, switching, exporting, provider selection, remote shell execution or the full RPC API. An abort command may be used for graceful cancellation if it simplifies the bounded shutdown path; it is not required for ordinary completion.

The maximum of three refers to **application prompt operations**, not total provider requests. Pi may internally perform read-tool turns, network retries and compaction requests. Each prompt's independent deadline bounds its elapsed work; logs must distinguish corrective prompts from native retries.

## 5. Internal boundaries and progress

Proposed layout:

```text
internal/generation/  neutral request/message/progress contracts,
                      shared generation instructions, prompt and schema builders
internal/harness/     mode validation/selection using executable lookup
internal/pi/          subprocess lifecycle, minimal RPC transport, Pi validation
internal/opencode/    existing SDK client plus the production generation adapter
internal/agent/       OpenCode-only frontmatter and installed-agent management
internal/server/      OpenCode server lifecycle with the D12 shutdown correction
internal/tui/         progress/selection view, no concrete harness dependency
cmd/gen-commit-msg/   prechecks, shared orchestration, signal/TUI/output wiring
```

- The neutral request contains subject range, body preference and serialized Git context, not OpenCode session IDs or agent-install options.
- The neutral result preserves existing subject/body semantics and exposes nonfatal cleanup diagnostics separately from the fatal generation error. Do not make progress events the only warning channel: quiet and non-TTY callers must not lose cleanup warnings.
- Use a small `Generator` contract: generation with context cancellation/progress reporting plus declared progress steps. No application-facing Start/CreateSession/DeleteSession/Stop lifecycle interface.
- Keep standard `error` at the progress/TUI boundary. Preserve the existing OpenCode detailed rendering through its error's rendering capability; do not put an `OC` field in common types or make Pi fabricate an OpenCode error.
- Extract the current agent instruction body once; compose the unchanged OpenCode frontmatter plus that body for installation, and pass only the neutral body to Pi. Preserve existing custom agent files and install policy. Test the generated OpenCode default prompt for unintended changes.
- Reuse the current generation prompt/schema construction outside the transport package. Keep OpenCode's native `json_schema` request and `retryCount: 2`; append a textual schema to Pi's prompt because RPC itself has no structured-response switch.
- Replace/migrate the test-only `PipelineRunner` into the real OpenCode adapter rather than retaining another unused pipeline abstraction.

Progress steps, all visible initially for the selected harness:

| Shared first step | OpenCode continuation | Pi continuation |
| --- | --- | --- |
| Collecting information about current changes... | Starting OpenCode... | Starting Pi... |
| | Creating session... | Generating commit messages... |
| | Generating commit messages... | Stopping Pi... |
| | Cleaning up OpenCode resources... | |

Pi startup completes after version verification and a successful RPC readiness response, not merely process spawning. Later RPC/model failures belong to the generation step. Corrective prompts update generation detail, e.g. `Correcting response format (attempt 2 of 3)`, rather than adding fake steps. Preserve cleanup warnings and existing OpenCode progress semantics. Selection appears only after backend-owned cleanup has completed.

## 6. Ordered implementation and verification gates

### Step 1 — Document the approved contracts

- Carry the approved architecture, file-access, timeout, minimum-version, generation/cleanup outcome, OpenCode shutdown and native-tool prerequisite decisions into the specifications. No material operator decisions remain open.
- Add the feature product/UX specifications under `docs/leyline/specs/` and `docs/leyline/design/`.
- Update superseded requirements in the main/progress/context specifications explicitly; do not silently rewrite historical review decisions.
- Write acceptance tests against the approved behavior before production implementation.

Gate: no unresolved material behavior/permission/lifecycle decision. No Pi implementation in this step.

### Step 2 — Refactor OpenCode behind the approved generation boundary

- Preserve existing OpenCode agent, request, response and selection semantics. Approved exceptions are presentation after cleanup (D8), visible nonfatal cleanup diagnostics in quiet/non-TTY modes (D11), and graceful-first completed server shutdown with meaningful diagnostics (D12).
- Extract neutral data/prompt/schema contracts and instruction body.
- Make the OpenCode adapter own agent/server/session lifecycle and expose real progress.
- Correct server shutdown to follow the D12 SIGTERM → five-second grace → SIGKILL if needed → completed waiting sequence. Remove the competing cancellation-driven kill and classify expected termination separately from genuine failures.
- Add behavioral process-fixture coverage for graceful exit, ignored SIGTERM requiring forced termination, observed prior exit, completed waiting and cleanup diagnostics. Include a descendant retaining output pipes after direct-server exit: verify direct-only SIGTERM, group SIGKILL after grace and completed draining (D14). Use explicit process handshakes/counters to verify ordering and completion, not just elapsed-time assertions. Include successful candidates plus a cleanup warning and preservation of the primary error when generation also fails.
- Route both existing CLI modes through the shared orchestration operation; remove concrete OpenCode types from TUI.
- Migrate relevant runner tests to the production-used path.

Gate: focused OpenCode, server, agent, TUI and CLI integration tests; existing exactly-once cleanup and output-preservation regressions remain passing; `make all`. Review this refactor and shutdown correction before building on them.

### Step 3 — Harness configuration and deterministic selection

- Before enabling auto selection or running the affected CLI suite, pin existing OpenCode subprocess fixtures with `--harness opencode`. Their current PATH includes host executables; an installed real Pi must never be selected by an OpenCode fixture.
- Give selection/Pi tests controlled executable paths, configuration and environment; normal tests must not invoke a real model or read operator credentials.
- Implement flag/environment precedence, enum validation and executable resolution.
- Test both executables, either executable, neither executable, explicit choices and ignored OpenCode-only settings in Pi mode.
- Test that selected Pi version/auth/runtime failure does not call OpenCode, and that no-staged/help/version/non-TTY prechecks do not start generation.

Gate: focused config/selection/CLI tests and `make all`.

### Step 4 — Pi transport, validation and conditional correction

- Implement arguments, supported-version check, process ownership, continuous I/O and the minimal RPC state machine.
- Implement strict result validation and at most two feedback prompts, sharing one process/conversation.
- Implement cancellation, timeout, bounded shutdown and diagnostics/logging.

Gate: fixture subprocess tests demonstrate actual required interactions and lifecycle outcomes, not source-presence checks. Cover:

- One valid result: one application prompt.
- Invalid then valid: two prompts, unchanged original Git snapshot.
- Three invalid results: exactly three prompts, error and no output overwrite.
- Provider/authentication error: no corrective prompt and no harness fallback.
- Tool-use/intermediate messages, delayed acceptance, native retry events and multiple `agent_end` before settlement. Ordinary tool errors do not alone invalidate a later confirmed successful response; retain strict final validation and the same prompt/time budgets.
- Error/length/aborted stop reasons, missing final response, premature EOF, malformed protocol, and nonzero exit before confirmed completion: generation error, not a cleanup warning.
- Valid completed response followed by nonzero exit or forced shutdown: candidates preserved, warning visible without corrupting message output, process reaped, successful output exits 0. Cancellation still suppresses output; simultaneous generation/cleanup failure preserves the generation cause.
- Startup readiness without a model call, independent generation budgets for initial/corrective prompts, progress that does not reset a deadline, and a separate shutdown grace period after cancellation. Preserve OpenCode timeout values and its approved D12 shutdown behavior.
- Large records over 64 KB, split pipe writes and Unicode separators inside strings.
- Required fields/types/count/body rules, whitespace-only or multiline subjects, unknown keys, fences and trailing JSON/text.
- SIGINT/SIGTERM, deadline, shutdown during pipe activity, and a child that ignores graceful termination. Verify completion/reaping through explicit fixture counters/handshakes, not timing claims.

Run focused tests, then `make all`.

### Step 5 — UX, documentation and packaging

- Wire declared steps into the existing progress view and retain quiet/single/multiple-result/selection cancellation behavior.
- Add harness help/README examples and Pi authentication/configuration/version requirements. Document the native search helpers (rg for grep; fd/fdfind for find), Pi cache/PATH lookup, disabled offline downloads and ordinary tool-error behavior when helpers are absent. Do not make these helpers new mandatory package dependencies for all application use.
- Update both AUR templates' optional harness dependency descriptions after checking the actual Arch package name for Pi. Do not invent a package name or add Pi as a mandatory dependency.
- Preserve stdout/file output contracts and editor integration examples.

Gate: TUI/CLI integration tests for both harnesses, no-staged and failure paths; output files unchanged until a real message is selected; `make all`.

### Step 6 — Native integration, independent review and delivery

- Promote a focused native-Pi fixture API check into an optional integration test: real Pi 1.4.0, isolated test configuration, conflicting instruction/resource/tool sentinels, exact request/prompt counters and zero session files. Normal unit tests must not require an installed Pi or external service.
- Extend the fixture API to request real native grep/find calls, verify their observable results with available cached/system helpers, and exercise missing-helper tool errors using controlled PATH/cache. Verify no helper download is attempted and a subsequent successful valid answer is accepted. These native checks are required delivery evidence, not a requirement to install helpers during tests or make normal unit suites depend on host Pi/helpers; report unavailable required checks as unrun.
- Verify real TTY selection and cancellation with the actual application for Pi and OpenCode; verify non-TTY stdout and `--output` routing.
- Run a small authenticated real-model smoke test for Pi and confirm useful messages from actual staged changes. Record selected model, results and limitations; do not log credentials. If unavailable, report this required validation as unrun rather than claiming full integration completion.
- Request fresh independent reviews of changed code against the approved spec and authoritative Pi contracts. Verify findings, fix confirmed defects, rerun invalidated checks and record every disposition in this plan's review log.
- Final `make all`, `git diff --check`, clean intended diff. Commit/push only if separately authorized.

## 7. Unknowns taxonomy and scope control

| Category | Current disposition |
| --- | --- |
| Known knowns | D1–D14 are approved; O1/O2 have explicit operator dispositions. Existing Git-context, message-content and selection semantics remain; presentation ordering, cleanup diagnostics, OpenCode shutdown and search-tool prerequisites follow D8/D11/D12/D13. |
| Known unknowns | No unresolved material design decisions. Native integration, real-model smoke testing and implementation review are delivery gates, not completed validation. |
| Unknown knowns | Whether auto should hide misconfigured Pi or legacy options should route to OpenCode was elicited: no fallback, and options do not alter selection. |
| Unknown unknowns discovered | APPEND_SYSTEM survives system replacement; package resolution precedes resource suppression; exit 0 is not candidate success; automatic recovery can add calls; large context exceeds default line-reader/argv limits. Resolved by source and controlled native probes, then reflected in this design. |

Out of scope: a Pi extension, SDK service/wrapper, full RPC client library, persisted Pi sessions, credential management, configurable arbitrary tool lists, an OS sandbox, OpenCode response-hardening unrelated to the extraction, Git-context redesign, new commit execution or editor/hook behavior.

Deviation rule: stop for new public/cross-component contracts, changed permissions, model/provider selection, cost/privacy trade-offs or incompatible CLI evidence. Recheck the authoritative source/runtime and revise the plan with the operator; do not substitute prompt-based workarounds for broken process/protocol contracts. Choose local implementation details within the approved design independently.

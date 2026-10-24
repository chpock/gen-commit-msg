# Pi and OpenCode harness support — product spec

Date: 2026-10-24
Surfaces: CLI, progress TUI, commit-message output
Design authority: D1–D14 in `../plans/2026-10-24-pi-harness-plan.md`.
UX: `../design/2026-10-24-pi-harness-ux.md`.

## Goals and scope

Generate commit-message candidates with either Pi or OpenCode while keeping the existing staged-change context, message ordering, common body, selection, formatting and output-preservation contracts. Introduce a shared production generation operation and refactor OpenCode into it; Pi is not a separate duplicate application workflow.

Each backend owns its resources and finishes cleanup before returning. TTY selection and non-TTY output happen afterward. OpenCode retains its agent files, custom agent behavior, SDK requests, structured-output schema/retry settings and decoder acceptance. Approved changes are cleanup-before-output in non-TTY, visible cleanup warnings in quiet/non-TTY, and corrected OpenCode shutdown.

No commits, index changes, credential management, sandbox, extension, SDK service, full RPC library, arbitrary tool configuration or unrelated OpenCode response hardening are part of this feature.

## Selection and configuration

| Flag | Environment | Values | Default |
| --- | --- | --- | --- |
| `--harness` | `GCM_HARNESS` | `auto`, `pi`, `opencode` | `auto` |

Precedence is explicit flag, nonempty environment value, then default. An invalid selected value is a configuration error (exit 2). Help/version preserve their existing early exits without runtime requirements.

- Resolve executable presence through PATH. Retain and execute the selected path without a shell.
- Auto prefers Pi if present, otherwise OpenCode; report both alternatives if neither exists.
- Explicit selection uses only that executable. Unsupported version, authentication, runtime, timeout or invalid-result errors never cause fallback.
- No-staged-files exits and non-TTY multi-result rejection happen before starting or probing a generation runtime.
- Pi ignores OpenCode-only `--agent`, `--install-agent` and their environment settings, installs no agent and logs the ignored configuration. Document this scope in help/README.
- Preserve ordinary Pi global model/provider/thinking, authentication and runtime settings. Do not add an application model flag or separate Pi configuration directory.

## Shared generation boundary

A neutral request contains subject minimum/maximum, body preference and serialized Git context. A neutral result contains candidates with the existing subject/body semantics and separate nonfatal cleanup diagnostics. Cancellation and a progress reporter are part of the operation. Backends declare their actual progress steps; the TUI holds no server, client or process reference.

Do not expose application-facing session/server lifecycle methods. Preserve OpenCode's rich error rendering through an error capability rather than a concrete OpenCode field in shared types. Both CLI modes use the same Git collection and generation operation.

Extract shared instructions and prompt/schema construction without changing the installed default OpenCode prompt or native `json_schema` request with `retryCount: 2`. Migrate the test-only OpenCode pipeline onto the production path rather than retaining another unused runner.

## Pi execution and isolation

Pi 1.4.0 is the minimum supported version; older versions have no compatibility path. Verify the selected executable with `--version` before starting generation or transmitting Git context. Reject unsupported/unrecognizable versions without fallback.

Use one RPC process for initial generation and any corrections:

```text
pi --mode rpc --no-session
   --no-extensions --no-mcp
   --no-skills --no-prompt-templates --no-themes
   --no-context-files --no-approve --offline
   --tools read,grep,find,ls
   --system-prompt <embedded instruction text>
   --append-system-prompt ""
```

Pass arguments directly, including the genuinely empty append argument. Use the repository working directory. Do not create a Pi agent, instruction file, persisted session, extension or temporary production configuration tree.

These flags isolate discovered instructions/resources and project settings, not all Pi behavior. Global endpoint overrides, retries, compaction, proxy settings, credential helpers, package-root lookup, migrations and credential refresh remain possible. Offline mode blocks automatic catalog/package/helper downloads, not model requests.

Native tools are read-only but not restricted to repository paths by an enforced boundary. Repository-only access is an instruction. Do not add path confinement or a sandbox.

Native grep requires cached/system `rg`; find requires cached/system `fd` or the supported system name `fdfind`. Document these search-capability prerequisites, not mandatory dependencies for every invocation. Missing helpers return ordinary native tool errors without startup rejection, downloading, replacement tools or fallback. A later successful valid final answer remains acceptable.

## RPC completion and correction

1. Drain stdout/stderr continuously. Establish readiness using a correlated successful `get_state` response without a model call.
2. Send a JSONL prompt with a unique ID, generation parameters, textual response schema and the app-collected Git context.
3. Require matching prompt acceptance. Acceptance is not completion.
4. Read completed assistant messages from `message_end`, not text deltas. Ignore thinking/tool-call blocks when extracting final text.
5. Wait through internal retries/recovery until non-aborted `agent_settled`; `agent_end` alone is insufficient. Use the final completed assistant response for that operation, with `stopReason: stop`.
6. Treat provider errors, truncation, premature exit, protocol failure, missing final response and cancellation as runtime failures, not formatting-correction opportunities.
7. Only successful but invalid output permits a corrective prompt. Send validation feedback with the unchanged schema in the same process/conversation/Git snapshot, after settlement. Allow at most two corrections: three application prompts total.
8. Stop on valid output or fail with the final validation diagnosis after the third invalid result. Never salvage an earlier invalid attempt, extract fenced JSON, clamp/drop candidates or fabricate missing fields.

Read LF-delimited records beyond 64 KB, including split pipe writes and Unicode separators inside JSON strings. Tolerate unrelated events while retaining required response/message/completion checks. Keep protocol records and raw stderr out of normal stdout/TUI output.

## Pi output validation

Require exactly one JSON object and no trailing non-whitespace data, fences or extra keys. Require present, non-null correctly typed `subjects` and `body`, string array elements, the inclusive requested candidate count, nonblank single-line subjects without embedded CR/LF, and empty body when body generation is disabled.

Line length, Conventional Commits style and other writing preferences remain instructions, not additional hard rejection rules. Do not apply Pi's stricter decoder to OpenCode.

## Timeouts and cleanup outcomes

- Pi startup: 30 seconds total for version verification, process startup and readiness.
- Each Pi prompt: independent 120 seconds from sending through acceptance and settlement, including native tools/retries/compaction. Progress does not renew the budget. Three prompts may use 360 seconds plus startup/shutdown.
- Pi shutdown: separate five-second grace, then force termination as necessary and always wait/reap, even after cancellation or deadline.
- OpenCode: preserve the 120-second HTTP-client timeout, 30-second startup context and session-deletion deadlines of 10 seconds in TTY versus five seconds in non-TTY.
- Correct OpenCode stop ordering: SIGTERM to the direct server only, up to the existing five seconds for shutdown including waiting/draining, then SIGKILL to its existing dedicated process group if shutdown is incomplete. Complete direct-child waiting and output draining before return, including when descendants retain output pipes after server exit (D14). Do not send group-wide SIGTERM or enumerate descendants. Remove the racing immediate context-cancellation kill. Preserve genuine failures; an expected requested SIGTERM result is not automatically failure.

After confirmed generation, abnormal/forced cleanup is a visible warning and does not discard candidates. Still finish cleanup before returning. Report warnings through visible progress, otherwise stderr, and log them. Quiet does not hide warnings. Successful selection/output exits 0; output-write failure remains an error. Cancellation suppresses output. Retain a primary generation failure when cleanup also fails, with separate cleanup diagnostics.

## Acceptance and delivery gates

- Configuration/selection tests cover precedence, both/either/neither executable, explicit modes, ignored OpenCode options, prechecks and absence of fallback after all selected-Pi failures.
- Production-path OpenCode regressions cover request/prompt compatibility, cleanup ordering/ownership, cancellation, warnings and the shutdown correction. Pin existing OpenCode subprocess fixtures before auto can discover host Pi.
- Pi process fixtures prove acceptance/completion, retries/corrections, independent budgets, all required validation failures, large/split records, errors/cancellation/termination and completed reaping through observable handshakes/counters.
- Native Pi plus a local fixture API proves isolation, zero persisted sessions, real helper-backed tool calls and missing-helper behavior without downloads. Ordinary unit suites require no installed Pi/helpers or external service.
- Exercise actual application TTY selection/cancellation and non-TTY stdout/file output for both backends, preserving output destinations until a real selection.
- Perform a small authenticated Pi smoke test on actual staged changes. Report unavailable required checks as unrun rather than claiming acceptance.
- Run focused checks after changes, `make all`, independent reviews and final diff checks. Record findings/dispositions in the plan review log.

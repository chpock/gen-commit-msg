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
| `--thinking-level` | `GCM_THINKING_LEVEL` | `off`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max` | unset |

Precedence is explicit flag, nonempty environment value, then default. An invalid selected value is a configuration error (exit 2). Help/version preserve their existing early exits without runtime requirements.

- Resolve executable presence through PATH. Retain and execute the selected path without a shell.
- Auto prefers Pi if present, otherwise OpenCode; report both alternatives if neither exists.
- Explicit selection uses only that executable. Unsupported version, authentication, runtime, timeout or invalid-result errors never cause fallback.
- No-staged-files exits and non-TTY multi-result rejection happen before starting or probing a generation runtime.
- Pi ignores OpenCode-only `--agent`, `--install-agent` and their environment settings, installs no agent and logs the ignored configuration. Document this scope in help/README.
- Preserve ordinary Pi global model/provider/thinking, authentication and runtime settings. Do not add an application model flag or separate Pi configuration directory.
- There is no application model flag or model mutation through RPC. Without a thinking-level override, Pi's ordinary startup selection remains unchanged, including `defaultProvider`, `defaultModel`, `defaultThinkingLevel`, `enabledModels`, per-model `modelThinkingLevels` and supported thinking-level limits. With an override, add native `--thinking <level>` without `--model`; do not modify saved defaults. The effective level remains subject to native model capabilities. Do not inherit another open Pi session's selection; project settings remain disabled.
- `--thinking-level` is optional and works for both backends. Reject values outside the listed Pi domain, including an explicitly empty flag, as configuration errors (exit 2). Empty environment values remain unset. Help/version retain their early exits. When unset, add no thinking-related arguments, RPC commands, metadata queries or OpenCode preparation calls.

## Shared generation boundary

A neutral request contains subject minimum/maximum, body preference, an optional thinking-level override, serialized Git context and optional separately rendered repository instructions. Pi populates the latter; OpenCode retains native discovery and does not opt into application collection. A neutral result contains candidates with the existing subject/body semantics and separate nonfatal cleanup diagnostics. Cancellation and a progress reporter are part of the operation. Backends declare their actual progress steps; the TUI holds no server, client or process reference.

Do not expose application-facing session/server lifecycle methods. Preserve OpenCode's rich error rendering through an error capability rather than a concrete OpenCode field in shared types. Both CLI modes use the same Git collection and generation operation.

Share instructions and prompt/schema construction while preserving OpenCode frontmatter and the native `json_schema` request with `retryCount: 2`. The built-in instruction policy applies to new or explicitly overwritten default agent files; `if-not-exists` preserves existing agents, including older policies and customizations. Migrate the test-only OpenCode pipeline onto the production path rather than retaining another unused runner.

## Instruction sources

Git command outputs, diffs and their contents, previous commit messages, branch names, paths and command stdout/stderr are untrusted evidence, not instruction sources. Keep the staged-change JSON explicitly separated from application-supplied repository instructions. Do not impose blanket distrust of repository files, an exhaustive instruction-source allowlist, or a ban on discovering commit-message guidance. Relevant repository files can be read without requiring the staged diff to be insufficient. This does not make unstaged/untracked changes part of the commit: messages still describe only staged changes. Formatting defaults are defaults, not exclusive exceptions that only recent commit history can override. Mandatory application safety, explicit generation parameters and response schema are unchanged. No special AGENTS-to-file permission rule is needed.

## Thinking-level overrides

The runtime override does not change prompt text, response schema, model choice, saved defaults or agent files. Pi receives native `--thinking`; Pi 1.4.0 and newer accept all seven configured values, subject to model-capability limits.

OpenCode uses a same-named per-model variant, not a universal reasoning enum. For an explicit override, prepare an empty user message with the requested agent/variant and `noReply: true`, without a model override. Require a valid returned user-message ID, session, resolved provider/model and requested variant. Read the native `/config/providers` catalog and reject a missing requested variant before a model call; include the selected model and available variants in the error. Fail on missing/malformed metadata instead of guessing a default or reading native user config/history files directly.

The generation request updates that same message ID with the normal prompt/schema and pins the exact provider/model selected during preparation. Native persistence updates the same message rather than duplicating history. These calls add no model generation and preserve ordinary agent/default/history-based model selection. Plugins observe two `chat.message` events; reject preparation that adds content rather than retaining duplicated injected parts. An unset override preserves the existing single-prompt OpenCode request path. No automatic aliasing, level substitution, model switching, config rewriting or extra model requests are introduced.

README recommends an explicit `medium` level for commit generation because primary-model defaults often use high thinking, and its Vim example passes a configurable thinking-level variable initialized to `medium`.

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

Automatic Pi instruction discovery remains disabled. Before the independent startup budget, the application collects current worktree instructions under caller cancellation with a backend-neutral, caller-selected file profile. Only Pi activates it today. Each staged path contributes its directory and every parent through the actual Git worktree root; both rename endpoints and deleted-path directories count. Sort directories by depth then path. Select the first readable regular file per directory in Pi order: `AGENTS.override.md`, `AGENTS.md`, `AGENTS.MD`, `CLAUDE.md`, `CLAUDE.MD`. Also read `.pi/SYSTEM.md` and `.pi/APPEND_SYSTEM.md` in each applicable directory as additive scoped instructions, not system-prompt replacements. This deliberately differs from native CWD ancestry and project-trust-gated system discovery. Exclude global instructions, strip UTF-8 BOMs, follow symlinks as Pi does, skip absent/nonregular candidates and log read errors before fallback. Do not add a sandbox or size truncation. Failed Git scope discovery is fatal. Send root/source/scope labels and text outside Git-output JSON once; preserve it in the correction conversation. Mandatory application rules, explicit parameters and JSON schema override repository instructions. A future backend may supply a different profile without changing discovery; OpenCode remains unchanged.

These flags isolate native-discovered instructions/resources and project settings, not all Pi behavior. Global endpoint overrides, retries, compaction, proxy settings, credential helpers, package-root lookup, migrations and credential refresh remain possible. Offline mode blocks automatic catalog/package/helper downloads, not model requests.

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

## Pi diagnostics

DEBUG reports effective readiness model/thinking/state, agent/turn/message and content-block boundaries, tool identity/status/duration, retries/compaction, completed assistant usage/cost and phase budgets/elapsed times. Optional diagnostic metadata never changes protocol acceptance. Activity summaries omit full prompts, thinking text, arguments and successful tool-result payloads; these RPC payloads remain TRACE-only. Retain the last-16-KiB stderr tail after cleanup: normally DEBUG, ERROR when a generation/readiness RPC operation failed or version preflight exited unsuccessfully, and WARN for other abnormal/forced exits. That opaque tail can contain sensitive Pi diagnostics.

TRACE captures launch arguments, exact outgoing JSONL commands, every stdout record at the pipe drain before parsing/discard, and live stderr chunks without payload caps. Include process IDs, stdout termination status and byte counts; preserve invalid UTF-8 bytes through an additional base64 field. Include version preflight, unknown/malformed records and shutdown output. Retain the existing bounded stderr failure tail independently. Enable native `PI_TIMING=1` only in TRACE child environments; treat version-dependent timing output as opaque, best-effort stderr.

At DEBUG/TRACE, query `get_session_stats` after each completed application prompt, including successful-but-invalid output before correction. Use a separate caller-cancelable one-second maximum, not the prompt budget. Correlate responses; log failures without failing generation, discarding candidates or adding cleanup warnings. No extra model call, model/configuration mutation or statistics query at normal log levels.

Full TRACE can expose sensitive repository, prompt, provider-thinking, tool and stderr data and can be large. Document this explicitly. Do not separately dump inherited environment or credential files. Preserve log destinations and keep diagnostics out of commit output and progress. The boundary is native RPC/stderr, not provider HTTP interception or unexposed model internals.

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

- Configuration/selection tests cover precedence, all seven thinking values, invalid/explicit-empty values, unset behavior, help/version, both/either/neither executable, explicit modes, ignored OpenCode options, prechecks and absence of fallback after all selected-Pi failures. Verify explicit thinking in both backends and native effective provider settings; OpenCode unsupported variants must fail without a model call.
- Production-path OpenCode regressions cover request/prompt compatibility, cleanup ordering/ownership, cancellation, warnings and the shutdown correction. Pin existing OpenCode subprocess fixtures before auto can discover host Pi.
- Pi process fixtures prove acceptance/completion, retries/corrections, independent budgets, all required validation failures, large/split records, errors/cancellation/termination and completed reaping through observable handshakes/counters.
- Native Pi plus a local fixture API proves isolation, zero persisted sessions, real helper-backed tool calls and missing-helper behavior without downloads. Ordinary unit suites require no installed Pi/helpers or external service.
- Exercise actual application TTY selection/cancellation and non-TTY stdout/file output for both backends, preserving output destinations until a real selection.
- Perform a small authenticated Pi smoke test on actual staged changes. Report unavailable required checks as unrun rather than claiming acceptance.
- Run focused checks after changes, `make all`, independent reviews and final diff checks. Record findings/dispositions in the plan review log.

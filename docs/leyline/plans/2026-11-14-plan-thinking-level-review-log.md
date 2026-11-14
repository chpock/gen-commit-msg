# Thinking-level override — review log

Task: `2026-11-14-plan-thinking-level`.

## Contract and implementation

- Optional `--thinking-level` / `GCM_THINKING_LEVEL`, with flag > nonempty environment > unset precedence. Accept exactly `off`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`; reject other values and explicitly empty flags with exit 2. Preserve help/version early exits.
- Carry the override through the existing shared generation request for TTY and non-TTY. Unset Pi arguments and OpenCode requests remain unchanged.
- Pi uses native `--thinking`, without selecting a model or changing saved defaults. Native capability limits remain in force.
- OpenCode uses same-named model variants. The operator approved failure before generation for an unavailable variant, and native `noReply` preparation/catalog lookup to preserve native model selection. The generation request updates the prepared message ID and pins the exact resolved provider/model. Plugins observe two `chat.message` events; added preparation content or malformed metadata fails rather than being silently retained.
- README, current product/UX specs, CLI help and the Vim variable/argument are updated. README recommends a lower custom level such as `low` because primary-model defaults often use higher thinking.

## Primary evidence and consultations

- Pi 1.4.0 already accepts all seven values: tagged `packages/coding-agent/src/cli/args.ts` in `earendil-works/pi`. Its session thinking setter requires explicit persistence to update saved defaults. The existing minimum version remains unchanged.
- OpenCode v1.19.5 `packages/opencode/src/session/prompt.ts` provides independent optional model/variant fields, native agent/default/history model selection and `noReply` before the generation loop.
- OpenCode v1.19.5 `packages/opencode/src/session/llm/request.ts` merges the selected model variant after base/model/agent options. Missing variant names otherwise silently no-op.
- OpenCode v1.19.5 `packages/core/src/session/projector.ts` updates an existing message on ID conflict; preparation and generation do not create duplicate user messages.
- Pre-implementation second opinions identified the unsupported-variant policy gap and the need to pin the prepared model. Both were independently checked against tagged source and then resolved by operator decisions. Blind forwarding with silent no-op was rejected; validating only after a model call was also rejected.

## Validation

Passed:

```text
GCM_TEST_NATIVE_PI=1 GCM_TEST_NATIVE_OPENCODE=1 make all
  go fmt ./...
  go vet ./...
  golangci-lint run ./...: 0 issues
  go test -count=1 -race ./...: all 13 packages passed
  go build -ldflags "-X main.version=dev" -o gen-commit-msg ./cmd/gen-commit-msg
```

Focused configuration/backend regressions cover the full value domain, default/precedence/invalid inputs, help/version behavior, unchanged output on invalid CLI configuration, Pi corrective-prompt lifecycle and all seven startup overrides, OpenCode omitted fields when unset, native model/message reuse and failure before generation for unsupported or malformed metadata.

Controlled native checks use disposable repositories/configuration and a local model API with dummy credentials, not operator credentials or paid provider generation:

| Scenario | Observable result |
| --- | --- |
| Pi 1.5.2, inherited thinking | Provider `reasoning_effort: high`; two native tool-turn API requests; one application prompt; saved default remains high |
| Pi 1.5.2, explicit low | Provider `reasoning_effort: low`; same request/prompt counts; saved default remains high |
| Pi 1.5.2, explicit low with TTY | Native tools and interactive second-candidate selection succeed with low thinking |
| OpenCode 1.19.5, inherited thinking | One provider request; thinking budget 8192; one plugin message event; native history-selected model retained |
| OpenCode 1.19.5, explicit low | One provider request; thinking budget 2048; preparation/final plugin events share one message ID; selected model retained |
| OpenCode 1.19.5, explicit low with agent-selected model and TTY | Agent model wins over competing configured/history defaults; one provider request, budget 2048, same-message events and interactive second-candidate file output |
| OpenCode 1.19.5, unavailable minimal variant | Zero provider requests; clear runtime error; only preparation plugin event |

OpenCode saved configuration and customized agent files remain unchanged. Native Pi's existing tools, missing-helper, correction, isolation and selection/cancellation scenarios also passed in the full suite.

A repeated verbose native OpenCode run passed and recorded explicit counters: inherited `provider requests=1, thinking budget=8192, plugin events=1`; low/history and low/agent/TTY each `provider requests=1, thinking budget=2048, plugin events=2`; unsupported variant `provider requests=0, plugin events=1`.

The built binary's help lists the flag and choices; an invalid value exits 2 without stdout. The exact README Vim snippet loaded successfully in clean headless Vim; this checks script loading, not a live Vim-to-provider editor session.

## Findings and dispositions

1. **Initial native OpenCode fixture selected the built-in agent's explicit `openai/gpt-5.6-luna` model instead of fixture history.** Confirmed test-isolation issue, not a production selection bug. Use a disposable customized agent without a model for history-selection coverage, and a separate explicit agent model for precedence coverage. Both paths passed.
2. **Native OpenCode added a missing `$schema` property to fixture configuration.** Confirmed ordinary native behavior, not thinking-level persistence. Supply the schema in the controlled fixture before comparing saved configuration. Native production configuration behavior is unchanged.
3. **Staticcheck QF1003 in new malformed-parts regression setup.** Confirmed and fixed using a tagged switch. The final native-enabled `make all` passed after the correction.
4. **Repowise large/complex test-function signals.** The live whole-change scorer flags the explicit multi-scenario OpenCode fixtures. Accepted as test-maintainability signals, rejected as evidence of critical functional defects: the branches set up independent API failure modes and assert required interactions. Independent review found no obscured correctness defect. No metric-driven fixture abstraction was introduced.
5. **Stored health freshness.** `get_health` remains commit-based and does not include the new `thinking.go`, despite refreshed working-tree graph facts. No fresh before/after stored-health improvement is claimed. Review and validation use current source and native request observations.

## Independent implementation review

- Fresh-context `reviewer`: no issues found; exact validation, omitted behavior, native preparation/model pinning, shared propagation, README/Vim changes and regression assertions checked against current files. It also inspected tagged upstream behavior. No shell/tests were run by the reviewer; validation above is the primary agent's execution evidence.
- Fresh-context `second-opinion`: no functional defect found. Its factual checks covered configuration, propagation, omitted wire fields, native preparation/model selection and variant merge order. The primary agent independently checked its material concerns as follows:
  1. **Different variant domains:** accepted; this is the approved failure policy and is documented. Rejected the stronger claim that `off` can never work in OpenCode: tagged `provider.ts` defines arbitrary string-keyed variants and merges configured `model.variants` into generated variants. A configured same-named `off` variant is therefore possible; the application does not automatically alias it to `none`.
  2. **Shipped agent pins a model and medium reasoning:** accepted against `internal/agent/frontmatter.md`. It is unchanged. Native fixtures test the same native agent-priority behavior with a disposable model; the shipped model's paid provider was not exercised.
  3. **Version dependency:** accepted as an API-capability/verification limitation, not evidence of a known numeric version floor. README now states the required native fields and the verified OpenCode 1.19.5 version. Older-tag behavior was not established; no unsupported minimum-version claim or new version gate is introduced.
  4. **Aggregate elapsed time:** accepted qualitatively: additional preparation/catalog requests use the existing 120-second per-request client timeout. README discloses that total elapsed time can increase. No overall deadline was promised or changed, and no exact total-time ceiling is claimed.
  5. **Test complexity:** accepted as optional maintainability feedback, not an established requirement violation; both independent inspections found no obscured correctness issue.
  6. **AGENTS key-environment list:** optional, not a missing public parameter document; it is a non-exhaustive list. The parameter is documented in README, help and both current product specs; no unrelated instruction-file edit is needed.

## Remaining limitations and status

Implementation, documentation, required native-enabled checks, both independent implementation inspections and final whitespace/status verification pass. Their material concerns have been checked and disposed of above. At the initial implementation handoff, changes were left unstaged; no commit, push or deployment had been performed.

OpenCode variants remain model/configuration-specific; there is no promise that all Pi names exist on every OpenCode model or map to identical provider budgets. Pi retains its native model-capability limits. Native execution was checked on installed Pi 1.5.2 and OpenCode 1.19.5 with controlled local APIs; all provider/model combinations, other OpenCode versions and paid external-provider generation were not exercised. These are verification boundaries, not silently accepted test failures.

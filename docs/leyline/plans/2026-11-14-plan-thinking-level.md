# 2026-11-14-plan-thinking-level

## Goal and approved contract

Add optional `--thinking-level` to customize the current generation's thinking without selecting a different model. Accept exactly Pi's current values: `off`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`; reject other explicitly supplied values as configuration errors. The default is unset. Preserve the previous behavior and runtime requests when unset. Follow existing flag/environment precedence with `GCM_THINKING_LEVEL`.

When set, apply the parameter to both Pi and OpenCode, including interactive and non-interactive generation. Keep existing model selection, timeouts, cleanup, response validation and instruction isolation. Do not change saved thinking/model defaults or customized agent files.

The operator explicitly chose a clear error when the selected OpenCode model has no matching variant, rather than native silent ignoring. No automatic substitution or model switch is approved. Pi retains its native model-capability limits.

The operator also approved pre-generation validation through native `noReply` preparation. Prepare an empty message with the requested agent and variant but no model override; inspect the natively selected model and its catalog variants; then update the same message ID with the actual prompt and pin the exact model selected by OpenCode. This adds metadata/preparation API calls but no extra model request. User plugins observe two `chat.message` events for the same message. The alternative of validating after a potentially paid generation was explicitly rejected.

Document the parameter, explain that primary-model defaults often use substantial thinking and recommend a lower explicit level for commit generation. Add a thinking-level variable and its flag to the README Vim example.

## Evidence and consequential questions

- Both Pi 1.4.0 and 1.5.2 CLI parsers accept the seven listed values independently of `--model`. Pi 1.4.0's `setThinkingLevel` persists global defaults only with explicit `options.persist`; the default is false. The minimum supported version remains 1.4.0.
- OpenCode 1.19.5 accepts independent optional top-level `variant` and `model` fields in `SessionPrompt.PromptInput`. Model selection is `input.model ?? agent.model ?? currentModel(sessionID)`.
- OpenCode's `LLMRequestPrep.prepare` looks up variants in the selected model and merges their options. A missing variant is silently ignored; native transport alone cannot enforce the approved error policy.
- The HTTP prompt route forwards the same schema and request fields. The pinned Go SDK can carry extra JSON fields with its existing `option.WithJSONSet`, already used for structured output.
- Native `noReply` returns the resolved user-message model without starting the generation loop. The message projector inserts by message ID and uses `onConflictDoUpdate` for that ID; the second phase can update the same user message without duplicating it. Catalog variants are exposed by `/config/providers`. Reusing these native interfaces avoids copying config/history-based model selection.

Primary upstream sources at tag `v1.19.5`: `packages/opencode/src/session/prompt.ts`, `session/llm/request.ts`, `provider/provider.ts`, and `server/routes/instance/httpapi/{groups,handlers}/session.ts` in `https://github.com/anomalyco/opencode`.

## Scope and impact

Change configuration, the shared generation request, CLI wiring and both backend adapters, plus focused tests and maintained documentation. The shared request has many consumers, including the CLI, Pi and OpenCode; preserve prompt/schema content and existing zero-value behavior. CLI wiring is a high-churn area, so avoid unrelated refactoring. Add no dependencies, global settings edits, automatic model changes, commits or publication.

## Stages and checks

1. Resolve remaining native-API/version questions; independently assess material trade-offs. Record operator decisions.
2. Add regression tests for all accepted/rejected values, explicit empty input, defaults, precedence, help/version behavior and CLI propagation. Implement backend behavior, preserving omitted fields/arguments when unset. Verify selected-model handling and unsupported-level errors in OpenCode without generating candidates under an ignored override.
3. Update README, current product/UX specifications, help and Vim example. Keep historical decision records clearly distinguished from the new contract.
4. Run focused tests, `make all`, native-backend checks with a controlled local API, independent implementation review and final diff checks. Verify effective provider request settings, not merely transport-field presence. Record findings and dispositions in `2026-11-14-plan-thinking-level-review-log.md`.

## Completion criteria

- Valid levels reach both backends through the shared production workflow; invalid inputs fail clearly.
- Unset behavior is unchanged, and normal model selection is preserved.
- OpenCode does not silently accept a missing selected-model variant.
- Documentation and Vim example meet the requested guidance.
- Required validation and independent reviews pass, with any unrun checks or remaining limitations explicitly reported.

## Status

Implementation and documentation are complete. Configuration regression tests were added first and failed before `Config.ThinkingLevel` existed. The final `GCM_TEST_NATIVE_PI=1 GCM_TEST_NATIVE_OPENCODE=1 make all` passed, including expanded native TTY and agent-priority checks. Pi inherited high versus explicit low was verified in provider requests. OpenCode provider budgets were 8192 inherited and 2048 low, with one provider call per successful generation, same-message plugin events, unchanged saved configuration/customized agents and zero provider calls for an unsupported variant. A repeated verbose native OpenCode run confirmed these counters. Both fresh-context independent implementation inspections found no functional defects; concerns and verification boundaries are recorded in `2026-11-14-plan-thinking-level-review-log.md`. Final whitespace/status checks passed. All implementation completion criteria are fulfilled. At the initial implementation handoff, changes were left unstaged and no commit or push had been performed.

## Pre-implementation second opinion

Accepted: OpenCode variant availability is model-specific; silent no-op versus failure requires an operator decision; Pi capability clamping is a different native behavior; update the existing no-thinking-flag statements and preserve flag/environment parity. The operator resolved the missing-variant policy in favor of failure, authorizing the necessary capability check. Rejected as the chosen implementation contract: blind forwarding with a log-only no-op caveat. Pi 1.4.0 enum and persistence compatibility were subsequently verified against its tagged source; no version gate is needed.

Second consultation: accepted native `noReply` preparation and catalog lookup as the supported pre-generation path, subject to operator approval of extra API/plugin events. Independently closed the consultation's projector evidence gap: `packages/core/src/session/projector.ts` keys `MessageTable` by the returned message ID and updates on conflict. The SDK does not type newer variant metadata; use its existing extensible JSON request/response facilities rather than assuming generated fields exist. Keep the exact natively resolved model in the final request so validation and generation target the same model. Do not accept the consultation's blanket characterization of plugin effects as benign: the operator was explicitly informed of duplicate plugin events and approved that trade-off.

# 2026-11-14-plan-pi-trace-logging

Status: completed; the operator authorized the local Git commit. Findings and check results are recorded in `2026-11-14-plan-pi-trace-logging-review-log.md`.

## Goal and boundary

Expose the Pi work currently hidden between prompt acceptance and completion. The operator requests all available native diagnostics at TRACE and readable activity at DEBUG. Preserve generation acceptance, correction, cancellation, process cleanup, read-only isolation, minimum Pi 1.4.0 and commit-output contracts from the Pi harness specification.

## Evidence and approach

- Previously `internal/pi/protocol.go` ignored most session events and `diagnostics.go` logged selected failures/recovery only.
- Previously `internal/pi/process.go` retained only a 16 KiB stderr tail and reported it after cleanup; full live TRACE now complements that retained tail.
- Pi's RPC/JSON references expose message/thinking/text/tool-call streaming, tools, turns, retries, compaction, usage and cost. Capture all incoming records at the pipe reader, including unknown/malformed records and shutdown output, without relying on the protocol consumer.
- TRACE records outgoing commands and launch arguments, complete incoming records and live stderr chunks without payload truncation. Do not log the inherited environment or separately read credentials.
- DEBUG reports lifecycle, effective model/thinking from existing readiness state, content-block boundaries, tool identity/status/duration and usage without full prompt, thinking or tool-result payloads. Preserve the existing bounded stderr tail at DEBUG on success or WARN/ERROR on failure.
- Native `PI_TIMING=1` enables startup-phase stderr diagnostics in installed Pi 1.5.2; `--verbose` is interactive-only. Enable timing only for TRACE child processes; retain ordinary environment otherwise.
- The operator approved `get_session_stats` after each completed application prompt at DEBUG/TRACE: a separate maximum one-second wait, derived from caller cancellation, with log-only failure and no loss of candidates. Existing generation deadlines remain unchanged.

## Stages and completion

1. Verify native capabilities and independently challenge the logging/diagnostic-query design; resolve consequential policy changes with the operator.
2. Add behavioral regression tests first, then transport capture and activity summaries. Update product/UX specifications and README for the TRACE sensitivity/volume and supported diagnostics.
3. Pass focused race tests, `make all`, actual Pi against the local model API, independent fresh-file review and second opinion, then verify each material finding and record dispositions in the companion review log.

Completion requires full TRACE capture even for large/split/unknown records and shutdown output, useful DEBUG activity with payload-level separation, unchanged success/failure/cleanup outcomes, and clean required checks. Raw model HTTP traffic or model-internal reasoning not exposed by Pi cannot be manufactured; document these limits. No OpenCode changes or new dependencies. The operator subsequently authorized the local Git commit; no push or deployment is authorized.

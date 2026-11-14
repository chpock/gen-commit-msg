# Repository instructions — review log

## Design consultation and primary verification

- The first second-opinion run exceeded 240 seconds after source inspection; the same run was resumed with a 600-second budget and completed. No implementation preceded its assessment.
- Accepted: preserve the independent 30-second Pi version/startup/readiness budget. Repository collection uses the caller context before that phase, like existing application Git collection. Rejected the suggested warning-and-empty fallback for failed Git discovery: Git is required, and unknown root/staged paths cannot establish the requested instruction scope. File-read fallback remains native WARN/try-next behavior.
- Accepted: existing Pi subprocess fixtures need real Git and a repository; initialize them explicitly rather than masking collection failures. A logged anomaly is not automatically a result cleanup warning, contrary to one assertion in the opinion.
- Accepted: distinct global/project/resource markers and assertions across actual model-facing messages are required. Existing native tests inspect only the system prompt and reuse markers.
- Accepted: repository instructions stay outside Git-output JSON. Added a prompt-output regression. Rejected the claim that appending a section necessarily removes the old BuildPrompt output as a substring: appended output preserves that prefix; actual assertions remain subject to execution.
- Accepted: scope is each staged path's directory and every parent through the worktree root, ordered by depth then path. Deleted paths and both rename ends count. Git queries belong in internal/git. Repository SYSTEM/APPEND are deliberately additive instructions without native trust gating; no executable resources are enabled.
- Exact source anchors: installed Pi 1.5.2 core/resource-loader.js:129-149 (candidate names, priority, BOM, regular-file/read fallback), 179-209 (native ancestry), 959-979 (SYSTEM/APPEND locations). Operator-approved deviations are staged-path ancestry, Git-root boundary, excluded global resources, and additive per-directory SYSTEM/APPEND handling.

## Commit 1 verification

Test-first collector/prompt regressions failed before implementation (undefined collector/data model). Implemented the neutral collector/profile/renderer and Pi-only transport. `make all` passed before review and again after fixes (fmt, vet, lint: 0 issues, race-enabled full tests, build). Actual Pi 1.5.2/local API integration passed all eight scenarios, including missing helpers, correction, TTY selection/cancellation and output preservation. No external fetches, persisted sessions or owned processes remained. Scoped authoritative Go diagnostics: 0 errors/warnings. `git diff --check` passed.

### Independent findings and dispositions

1. Reviewer: missing-executable test no longer reached Pi after collection was introduced. **Accepted and fixed.** Strengthening its assertion first reproduced a failure identifying Git discovery, not Pi. Use an initialized Git fixture and assert `pi version preflight`; the full suite now passes through the intended missing-executable path.
2. Second opinion: read-error fallback lacked direct coverage. **Accepted and fixed.** Permission-denied regular-file regression verifies fallback, WARN output and absence of instruction bodies at DEBUG. It ran and passed as a non-root user, not skipped. File-read errors are always logged, including a file disappearing after stat.
3. Second opinion: nested-worktree test only proved root isolation with an empty index. **Accepted and strengthened.** Stage a deep path and assert both nested-root and deep rules from subdirectory CWD, excluding main-worktree rules.
4. Second opinion: temporary directories could discover an enclosing repository, and subprocess fixtures inherited global Git config. **Accepted for test isolation.** Bound negative discovery with Git ceiling directories and disable global/system Git config in the Pi fixture helper. No production Git configuration policy changes.
5. Second opinion: raw collection error is shown in the existing starting step. **Intended behavior**, already specified; no new UI step or wrapper requirement.
6. Second opinion: separate Git exec helper duplicates some plumbing. **No change:** existing command-spec execution truncates output and lacks a working-directory argument; reusing it would corrupt NUL-delimited paths. Shared execution-argument hardening is reused. `--name-only` does not invoke external diff drivers.
7. Intermediate file-distrust conflict: **accepted sequencing, not completed model behavior**; commit 2 remains mandatory. Actual local API tests prove delivery only.
8. Structural risk analysis flagged existing large native-test methods and required per-file filesystem I/O. **Not defects:** current additions are bounded assertions/setup; the I/O is the requested discovery itself. All inferred affected tests were covered by `make all`. No unrelated complexity refactor is authorized.

Both independent opinions were assessed against fresh source and the approved contract. No unresolved commit-1 production defect remains. Boundary validation of impossible standard-Git path forms was inspected directly; adversarial replacement Git executables are not an added test requirement.

## Commit 2 verification

Pending after commit 1. Native/local fixture API checks demonstrate delivery and actual tool mechanics, not autonomous model interpretation; model-following claims require a real-model smoke.

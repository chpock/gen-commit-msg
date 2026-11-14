# 2026-11-14-plan-repository-instructions

Status: commit-1 collection/transport implemented, verified and independently reviewed; commit-2 policy correction pending. Two local commits requested; no push.

## Contract and approved decisions

1. Collect repository instructions in the application and inject them outside Git-output JSON. Activate only for Pi; leave OpenCode native discovery and transport unchanged. Keep the collector backend-neutral with caller-selected file sets for future reuse and different harness profiles.
2. Search the repository root and ancestor directories of staged paths, not just CWD. Stop at the actual Git worktree root. Read current worktree files, including local instructions, rather than index blobs.
3. Match inspected Pi 1.5.2 `resource-loader.js` per-directory context precedence: `AGENTS.override.md`, `AGENTS.md`, `AGENTS.MD`, `CLAUDE.md`, `CLAUDE.MD`; select the first readable regular file. Also include `.pi/SYSTEM.md` and `.pi/APPEND_SYSTEM.md` in applicable directories as additive scoped guidance, never as replacements for mandatory application instructions. Their approved application-side inclusion intentionally bypasses native project-trust gating for these text files only; executable/configuration resources remain disabled. This additional per-staged-path scope is operator-approved and intentionally differs from native CWD-only ancestry.
4. Preserve native BOM stripping, regular-file checks, symlink following and logged read-error fallback. The directory-walk boundary is not a filesystem sandbox; no new file-target confinement or size truncation policy is introduced.
5. Keep Pi discovery/project settings disabled and retain ordinary user model/auth/runtime configuration. No extension, configuration tree, SDK bridge or new dependency.
6. Correct trust instructions in the second commit: Git output, diffs, previous commit messages and metadata are evidence, never instruction sources. Do not prohibit instructions from repository files, exhaustive-allowlist their names, or specially encode AGENTS-to-file-X permission. Preserve read-only tools, staged-only change scope, explicit generation parameters and JSON schema; repository conventions can override style defaults, not mandatory constraints.

## Commit sequence

### Commit 1 — repository context collection

Add reusable file-set selection, exact Git worktree/staged-path discovery (NUL-safe, both ends of renames via a non-rename path listing), deterministic depth-then-path directory union and scoped prompt rendering. Each staged path contributes its own directory and all parents through the worktree root. Collect under the caller context before the independent 30-second Pi startup/version/readiness budget, like other application-side Git context. Git discovery failure is fatal because the required repository context cannot be established; individual file read failures retain native logged fallback. Initialize a real Git repository and expose the Git executable in existing Pi subprocess fixtures. Capture once for all corrective prompts. An empty repository-instruction field preserves the existing OpenCode prompt. Add collector, real-Git, subprocess prompt and actual-Pi/local-API regressions with distinct global/project sentinels. Update README and product/UX specifications.

This commit establishes transport; the requested instruction-policy correction intentionally follows in commit 2. Do not claim the complete model behavior before both commits.

### Commit 2 — instruction trust policy

Remove broad file-content distrust, the exhaustive instruction-source allowlist and the prohibition on discovering commit-message guidance from the shared embedded instructions and prompt builder. Keep Git evidence/metadata distrust and required output/safety constraints. Update prompt-output regression coverage and native tool interaction coverage for a project rule referring to another file. Verify installed default/custom OpenCode-agent behavior is not silently overwritten.

## Verification and delivery

Use test-first focused regressions, `make all` after code changes, actual Pi with the local fixture API, and authenticated staged-change smoke where available. Native/local API tests prove transport and actual tool execution; they do not prove model understanding. Independently review each nontrivial commit target with reviewer and fresh second opinion in parallel; verify findings directly and record dispositions in `2026-11-14-plan-repository-instructions-review-log.md`. Commit only after the relevant required checks pass. Final delivery lists both commit IDs, limitations and unrun required checks. No unrelated OpenCode refactor or automatic agent-file overwrite.

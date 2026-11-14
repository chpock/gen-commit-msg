# GitHub Actions upgrade review log

## Scope and status

Upgrade all Actions used by CI and Release to current stable release lines and remove the Node 20 deprecation warning and Ubuntu-label migration notice. At the pre-push review cutoff, implementation, local validation and both final independent reviews were complete; hosted execution of the updated workflows was unverified. The operator subsequently authorized committing, pushing the branch and monitoring CI. Release publication and AUR deployment remain outside that authorization.

Only action references and runner labels change in the workflows. Triggers, permissions, Go selection, golangci-lint version, architecture matrix, shell commands, action inputs, artifact names and release/AUR outputs remain unchanged.

## Versions checked on 2026-11-14

| Action | Previous ref | Updated ref | Current stable release |
| --- | --- | --- | --- |
| actions/checkout | v4 | v7 | v7.0.1 |
| actions/setup-go | v5 | v7 | v7.0.0 |
| golangci/golangci-lint-action | v8 | v9 | v9.3.0 |
| actions/upload-artifact | v4 | v7 | v7.0.2 |
| actions/download-artifact | v4 | v8 | v8.0.2 |
| softprops/action-gh-release | v2 | v3 | v3.0.4 |
| KSXGitHub/github-actions-deploy-aur | v4.1.3 | v4.2.0 | v4.2.0 |

Official latest-release API responses and Git tag resolution confirm that the selected floating major tags currently resolve to these stable releases. The existing major-tag policy is preserved; AUR retains an exact release tag. The six JavaScript action manifests use Node 24; AUR is a Docker action.

## Compatibility assessment

- All four jobs now use `ubuntu-26.04`. The previous successful [CI run 43614758984](https://github.com/chpock/gen-commit-msg/actions/runs/43614758984) already used Ubuntu 26.04.1 with runner 2.338.2. The fixed label retains that OS and removes the migration ambiguity, rather than downgrading to 24.04. Future Ubuntu major upgrades require updating these labels.
- Checkout requires runner >=2.327.1 for Node 24 and >=2.329.0 for authenticated Git inside Docker. The observed hosted runner meets both. V7's fork protections affect `pull_request_target` and `workflow_run`; neither workflow uses those events.
- Setup-go v6 changed toolchain precedence; v7 is an internal ESM migration. `go.mod` contains only `go 1.26.2`, without a toolchain directive. Go selection remains unchanged; cache-key changes can cause a cold cache.
- The linter action is upgraded, but golangci-lint itself stays at v2.11.4.
- Upload v7 defaults to archived uploads. Keep that default: `archive: false` would ignore the custom artifact name and break the existing download pattern. Download v8 retains pattern/merge/path behavior and now fails on digest mismatch; this integrity protection is intentionally not weakened.
- Release v3 retains the existing `files`, `body_path`, token and permission contract.
- AUR v4.2.0 adds optional `asset_dir` synchronization. Its default is empty; the existing pkgbuild-only copy/staging path remains in use. The action still uses a rolling `archlinux:base` container, as before.

The successful baseline job's `Set up job` log includes these values (timestamps and grouping markers omitted):

```text
Current runner version: '2.338.2'
Ubuntu
26.04.1
LTS
Image: ubuntu-26.04
Version: 20261111.82.6
Image Release: https://github.com/actions/runner-images/releases/tag/ubuntu26%2F20261111.82
```

Primary sources: tagged `action.yml` and READMEs for the releases in the table; their official release notes; AUR `Dockerfile`, `entrypoint.sh` and `build.sh` at v4.1.3/v4.2.0; [official Ubuntu migration issue](https://github.com/actions/runner-images/issues/14748).

## Validation

- Editor diagnostics reported no problems in the two workflow files. This is an auxiliary check, not evidence of GitHub Actions runtime compatibility; workflow validation is performed by actionlint below.
- Initial actionlint 1.7.12 validation rejected `ubuntu-26.04` because the latest released validator does not yet recognize this hosted label. The label is independently verified as generally available in GitHub's official runner-image notice and actual hosted CI. Revalidation registers only that label through an external validator configuration; no project configuration or warning suppression is added.
- Actionlint 1.7.12/ShellCheck and `git diff --check`: passed, with only the independently verified runner label added to the external validator configuration.
- Parsed-workflow comparison against HEAD: passed; only action refs and runner labels differ.
- Exact release shell snippets: valid `v1.2.3` exports the expected version; four malformed tag forms fail before exporting a version.
- Exact release build commands with Go 1.26.2: passed for Linux amd64 and arm64. Both outputs have the expected ELF architecture and no dynamic-loader dependency. The amd64 binary executes and reports `gen-commit-msg 1.2.3`; ARM64 is inspected, not executed.
- `make all` with Go 1.26.2 and the official SHA256-verified golangci-lint v2.11.4 binary: passed (format, vet, lint, race-enabled tests across all 13 packages, build).
- Repowise treats workflow YAML as history-only, with no applicable code-health measurements or before/after comparison. Its live change analysis requests the full test suite; no code-health verdict is inferred for YAML.

## Independent review and dispositions

- Preflight second opinion: no blocker. An initial 180-second timeout was resumed with a longer limit; the consultation subsequently completed.
- Accepted and verified: preserve archived uploads and naming, retain hard digest checking, keep the pinned analyzer, distinguish setup-go v6 toolchain changes from v7 ESM changes, and disclose remote-execution gaps.
- Optional broader changes are outside this update: new triggers, a throwaway publication repository, Dependabot configuration and a new SHA-pinning policy. None is required to fix the reported warnings.
- Final reviewer: no issues found; local checks and compatibility assessment supported, with the documented remote-validation limits.
- Fresh final second opinion: no material defect found; versions, tag resolution, manifests, artifact pairing and AUR compatibility independently checked against primary upstream sources.
- Accepted evidence-quality concerns: include the baseline runner-log excerpt and exact actionlint label registration; both are recorded here. Editor diagnostics are explicitly auxiliary. The stronger claim that no diagnostics provider exists was not established: the tool reported authoritative diagnostics for both files.
- Accepted maintenance concern: fixed Ubuntu labels require future major-version updates, already documented above.
- Not accepted as a new defect or an operator decision: softprops v3.0.4 was published on November 5, but recency alone establishes no regression. A throwaway-repository rehearsal was not performed within this scope; the operator did not reject such a rehearsal or accept its absence as proof. Release-runtime verification remains open rather than being labelled an accepted or passed check.
- Non-blocking checkout patch-note precision does not affect the conclusion: neither protected event is used. No implementation changes were required by either final review.

To reproduce configured actionlint validation, run `actionlint -config-file FILE` with a local file containing exactly:

```yaml
self-hosted-runner:
  labels:
    - ubuntu-26.04
```

This adds only the independently verified hosted label to the validator's known-label set. No repository configuration or diagnostic suppression is required.

## Remaining evidence gaps

At the pre-push review cutoff, new Action versions had not executed on GitHub-hosted runners. Local checks cannot prove cloud artifact transport, release API publication or AUR deployment. Hosted CI is a follow-up to the authorized branch push; the tag-only Release workflow needs a separately authorized release. ARM64 artifact execution is not available locally. This pre-push review does not claim successful remote execution or publication.

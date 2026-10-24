# Cleanup and output preservation — review log

Date: 2026-10-24

## Scope and decisions

- Restore non-interactive successful session/server cleanup.
- Make early output-path validation non-destructive.
- Keep the two bug fixes in separate commits.
- The operator approved an additional prerequisite commit for two existing lint failures. Both failures were reproduced on untouched baseline `9ad5d09859a65f672b62808b1b4287a8cd48c618`; no lint checks are disabled.

## Prerequisite: baseline lint repairs

### Changes

- Use a completion-only channel when waiting for the OpenCode process. The process exit error remains intentionally ignored, as before, because termination is requested during shutdown.
- Replace the `reflect.Ptr` alias with `reflect.Pointer`.

### Independent review

- A fresh-context reviewer inspected both changes and reported no findings: **OK**.
- Parent verification confirmed that completion is still signaled only after `Wait` returns, the timeout remains unchanged, and `Ptr` aliases `Pointer` in the installed Go standard library.

### Validation

- `make lint` on the untouched baseline: failed with the two reported `errcheck` and `govet` issues.
- `make all` on an isolated baseline worktree with only these production changes: passed (format, vet, lint, race-enabled tests, build).
- `make all` in the main worktree with the pending cleanup regression test and fix: passed.

### Limitations

- The server shutdown protocol and treatment of expected process termination errors are unchanged.

## Non-interactive successful cleanup

### Changes

- Return normally after successful non-interactive output so deferred session deletion, server shutdown, and log closure execute.
- Add subprocess regression coverage using the actual CLI entry point, real Git fixtures, an HTTP fixture server, and a spawned fake OpenCode executable. Check stdout and file output, exactly one session deletion, and completed server shutdown.

### Independent review and disposition

- Initial review confirmed the production fix and found **P1**: fixture Git commands inherited `GIT_*` variables and could affect an external repository/index.
- **Accepted and fixed:** use the same filtered environment for fixture Git commands and the CLI subprocess. A regression supplies disposable external Git paths and verifies they remain untouched while the fixture receives its own staged index.
- A fresh-context follow-up read the repaired helper and retained the production approval: **OK**, no remaining findings.
- Non-TTY `--pause` is a no-op under the UX contract; removing the previous incidental success pause prompt is consistent with it.

### Validation

- Cleanup regression before the fix: failed for both stdout and file output, with no HTTP DELETE or completed shutdown.
- Fixture-isolation regression before its repair: failed while staging against the supplied external Git environment.
- `go test -count=1 -race -run '^Test(NonInteractiveSuccessCleansUp|CLIRepositoryIgnoresGitEnvironment)$' ./cmd/gen-commit-msg/`: passed.
- `make all`: passed after the fixture repair.
- `git diff --check`: passed.

### Limitations

- The subprocess tests exercise the CLI, SDK HTTP transport, and process lifecycle with controlled fixtures; they do not invoke a real OpenCode installation or model.

## Output preservation

### Decisions

- The operator explicitly included the related TUI cancellation bug in the output-preservation commit.
- The operator approved early rejection of output symlinks whose target does not exist. Existing links to writable files remain supported. Validation preserves rejected dangling links and does not create their targets.
- Historical deferred item D5 is resolved; the original review log links this follow-up. The product spec's stale test-file reference (F8) is also corrected.

### Changes

- Open existing output files without truncation during validation. Preserve their contents, permissions, and identity, including symlinks and hard links.
- Probe absent destinations using exclusive creation and remove only successfully created probes. Report close/removal errors.
- Move opening/writing/closing into the existing `writeSelectedMessage` helper so empty selection is checked before any destructive open. Both CLI paths share the same behavior.
- Preserve overwrite semantics for real messages and propagate both write and close failures. Stdout remains open.
- Update the output product and UX specifications.

### Validation

- Before the fix, preservation regressions failed for regular files, symlinks, hard links, and CLI exits with no staged changes, no repository, or an invalid non-interactive subject range.
- Focused race-enabled config and CLI tests passed after the fix. Coverage includes absent probe removal, permissions, unchanged existing output, absent new output on cancellation, successful overwrite, and open/write errors.
- A controlled real-PTY experiment ran the actual binaries with a fake OpenCode HTTP process, waited for the two-subject selector, and sent Escape or Enter:

  | Scenario | Baseline | Fixed |
  | --- | --- | --- |
  | Cancel with an existing output file | File emptied | Original bytes preserved |
  | Cancel with an absent output file | Empty file created | File remains absent |
  | Select with an existing output file | Message overwritten correctly | Message overwritten correctly |

- All six PTY runs exited with code 0 and made exactly one health, create-session, generation, and delete-session request, in that order. The initial PTY driver failed to recognize ANSI-colored subjects and timed out; matching the rendered description text corrected the driver before collecting the comparison above.
- PTY evidence and its scratch driver are in the task directory `2026-10-24-cleanup-output-fixes`: `before-tty-results.json`, `after-tty-results.json`, `tty_probe.py`, and the corresponding terminal/application logs.
- `go test -count=1 -race -run '^TestValidateOutputPath' ./internal/config/`: passed after adding the approved dangling-symlink regression.
- Final `make all`: passed (format, vet, lint, all race-enabled tests, build).
- `git diff --check`: passed.

### Independent review

- A fresh-context reviewer inspected the complete eight-file change, specifications, tests, PTY driver, and before/after evidence: **OK**, no findings.
- A separate fresh-context follow-up verified the subsequently approved dangling-symlink contract, regression test, and documentation: **OK**, no findings; prior approval retained.

### Limitations

- No real OpenCode installation or model was invoked; generation responses came from existing repository fixtures.
- Maintained tests cover output behavior without Python or a PTY dependency. Interactive selection/cancellation was additionally exercised by the scratch real-PTY experiment.
- Close/removal-failure propagation is checked by source review, not fault injection. Atomic output replacement and protection against concurrent external pathname replacement are not guarantees introduced by this change.
- Dangling output symlinks are rejected by the operator-approved contract.

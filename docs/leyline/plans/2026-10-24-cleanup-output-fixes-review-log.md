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

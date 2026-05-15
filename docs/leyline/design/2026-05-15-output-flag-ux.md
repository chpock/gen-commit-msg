# --output flag — UX spec

Date: 2026-05-15
Product spec: docs/leyline/specs/2026-05-15-output-flag-design.md
Surfaces: cli-only

## Commands enumerated

| Command/flag | Status | Description |
|---|---|---|
| `--output` / `-o <path>` | **new** | Write the selected commit message to `<path>` instead of stdout |
| All existing flags | unchanged | No behavior change |

### Environment variables

| Variable | Equivalent flag | Description |
|---|---|---|
| `GCM_OUTPUT` | `--output` | Write the selected commit message to file instead of stdout |

## Help / usage text

The new flag appears in `--help` output as:

```
-o, --output string   write commit message to file instead of stdout
```

Shorthand `-o` matches common CLI convention. The description uses imperative mood consistent with all existing flag descriptions.

## Error and progress output formatting

| Condition | Output channel | Message |
|---|---|---|
| File cannot be created | stderr | `Error: failed to open output file "path": <os error>` |
| File write fails | stderr | `Error: failed to write output file "path": <os error>` |
| Output successful | file | (silent — same as current stdout success) |

Progress output during TUI mode (spinner, step status) is unaffected — it renders to `/dev/tty`, not stdout. The `--quiet` flag suppresses it regardless of `--output`.

## Voice and tone

- **Error:** `Error: failed to open output file "/bad/path": permission denied`
- **Success:** (no output to stdout/stderr — the commit message is in the file)
- **Empty (no selection):** (unchanged — no output at all, as with stdout mode when user cancels)

Existing voice conventions: errors go to stderr, may use `col.RedText` when TTY, format is `Error: <reason>`. The new error messages follow this pattern exactly.

## Output accessibility

- **Color independence:** error messages include the file path and OS error text — no meaning is conveyed by color alone.
- **Screen-reader-friendly:** all errors are plain text on stderr, readable by any terminal screen reader.
- **Terminal width:** not applicable — output goes to a file, not the terminal.

## Exit codes and their meanings

| Code | Meaning |
|---|---|
| 0 | Success (message written to file or stdout; or no staged files) |
| 1 | Runtime error (git repo not found, server failed, output file unwritable, generation failed) |
| 2 | Usage error (invalid flags, invalid config) |

The new file-write failures exit with code 1, consistent with all other runtime errors.

## Non-goals

- Progress output on where the file was saved (current stdout mode is silent on success too)
- Directory auto-creation (the parent directory must exist)
- Multiple `--output` values or concurrent writes
- Template variables in the output path (e.g., `--output /tmp/msg-%d.txt`)

## Approvals

UX spec approved - round 1 - 2026-05-15
UX spec updated - round 2 - 2026-05-15 (added GCM_OUTPUT env var table)

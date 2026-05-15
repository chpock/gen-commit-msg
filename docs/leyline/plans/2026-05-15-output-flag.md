# output-flag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `leyline:subagent-driven-development` (recommended) or `leyline:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--output`/`-o` flag and `GCM_OUTPUT` env var to write the selected commit message to a file instead of stdout.

**Architecture:** Three-layer change: config flag definition + early validation, output-writer selection helpers in main, and integration into both TUI and non-interactive code paths. The `ValidateOutputPath()` method catches unwritable paths before server start; `resolveOutputWriter()` returns the correct `io.WriteCloser` for the rest of main to use.

**Tech Stack:** Go 1.x, stdlib only (`os`, `io`, `fmt`), `github.com/spf13/pflag` (already used).

**Spec references:**
- Product spec: `docs/leyline/specs/2026-05-15-output-flag-design.md` (round 2)
- UX spec: `docs/leyline/design/2026-05-15-output-flag-ux.md`
- Baseline: `docs/leyline/plans/2026-05-15-output-flag-baseline.md`

**Surfaces:** cli-only

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/gen-commit-msg/main.go`
- Test: `internal/config/config_test.go`
- Test: `cmd/gen-commit-msg/output_test.go`

---

### Task 1: Config - Add Output field and --output flag

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [x] **Step 1: Write the failing test**

In `internal/config/config_test.go`, add:

```go
func TestParseFlagsOutputFlag(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--output", "/tmp/test-msg.txt"}
	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "/tmp/test-msg.txt" {
		t.Errorf("Output = %q, want /tmp/test-msg.txt", cfg.Output)
	}
}

func TestParseFlagsOutputEnvVar(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}
	_ = os.Setenv("GCM_OUTPUT", "/tmp/env-msg.txt")
	t.Cleanup(func() { _ = os.Unsetenv("GCM_OUTPUT") })

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "/tmp/env-msg.txt" {
		t.Errorf("Output = %q, want /tmp/env-msg.txt", cfg.Output)
	}
}

func TestParseFlagsOutputDefault(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}
	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "" {
		t.Errorf("Output = %q, want empty string (default)", cfg.Output)
	}
}

func TestParseFlagsOutputShortFlag(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "-o", "/tmp/short-msg.txt"}
	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "/tmp/short-msg.txt" {
		t.Errorf("Output = %q, want /tmp/short-msg.txt", cfg.Output)
	}
}
```

- [x] **Step 2: Run the test, confirm failure**

```
go test -count=1 -run 'TestParseFlagsOutput' ./internal/config/
# Expected: compilation error — Config has no field Output
```

- [x] **Step 3: Implement minimal code**

In `internal/config/config.go`:

1. Add field to `Config` struct (after `InstallAgent`, before `Version`):

```go
Output       string
```

2. Add flag in `initFlags()` (after `pause` flag):

```go
flags.StringP("output", "o", "", "write commit message to file instead of stdout")
```

3. Resolve in `ParseFlags()` (after `InstallAgent` resolution):

```go
cfg.Output = getStringFlagOrEnv(flags, "output", "GCM_OUTPUT", "")
```

4. Update the `slog.Debug` call at the start of `main()` (near line 48) to include output — done in Task 3.

- [x] **Step 4: Run tests, confirm pass**

```
go test -count=1 -run 'TestParseFlagsOutput' ./internal/config/ -v
# Expected: 4 passing tests
```

- [x] **Step 5: Commit**

```
git add internal/config/config.go internal/config/config_test.go && git commit -m "feat(config): add --output flag and GCM_OUTPUT env var"
```

---

### Task 2: Config - Add ValidateOutputPath method

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [x] **Step 1: Write the failing test**

In `internal/config/config_test.go`, add:

```go
func TestValidateOutputPathEmpty(t *testing.T) {
	cfg := &Config{Output: ""}
	if err := cfg.ValidateOutputPath(); err != nil {
		t.Errorf("expected nil error for empty output path, got: %v", err)
	}
}

func TestValidateOutputPathWritableFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/commit-msg.txt"
	cfg := &Config{Output: path}
	if err := cfg.ValidateOutputPath(); err != nil {
		t.Errorf("expected nil error for writable path, got: %v", err)
	}
}

func TestValidateOutputPathNonExistentParent(t *testing.T) {
	cfg := &Config{Output: "/nonexistent-dir-xyz/commit-msg.txt"}
	err := cfg.ValidateOutputPath()
	if err == nil {
		t.Fatal("expected error for non-existent parent directory")
	}
}

func TestValidateOutputPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Output: dir}
	err := cfg.ValidateOutputPath()
	if err == nil {
		t.Fatal("expected error when output path is a directory")
	}
}
```

- [x] **Step 2: Run the test, confirm failure**

```
go test -count=1 -run 'TestValidateOutputPath' ./internal/config/
# Expected: compilation error — Config has no method ValidateOutputPath
```

- [x] **Step 3: Implement minimal code**

In `internal/config/config.go`, add after `ParseFlags()`:

```go
func (c *Config) ValidateOutputPath() error {
	if c.Output == "" {
		return nil
	}
	dir := filepath.Dir(c.Output)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", dir)
		}
		return fmt.Errorf("cannot access output directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output parent is not a directory: %s", dir)
	}
	f, err := os.OpenFile(c.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("cannot write to output file %s: %w", c.Output, err)
	}
	_ = f.Close()
	_ = os.Remove(c.Output)
	return nil
}
```

Add imports at top of file: `"os"` and `"path/filepath"`.

- [x] **Step 4: Run tests, confirm pass**

```
go test -count=1 -run 'TestValidateOutputPath' ./internal/config/ -v
# Expected: 4 passing tests
```

- [x] **Step 5: Commit**

```
git add internal/config/config.go internal/config/config_test.go && git commit -m "feat(config): add ValidateOutputPath for early file-write check"
```

---

### Task 3: Main - Add output writer helpers with tests

**Files:**
- Modify: `cmd/gen-commit-msg/main.go`
- Test: `cmd/gen-commit-msg/output_test.go`

- [x] **Step 1: Write the failing test**

In `cmd/gen-commit-msg/output_test.go`, add after existing tests:

```go
func TestResolveOutputWriterStdout(t *testing.T) {
	w, closer := resolveOutputWriter("")
	if w != os.Stdout {
		t.Error("expected os.Stdout when output path is empty")
	}
	closer() // must not panic
}

func TestResolveOutputWriterFile(t *testing.T) {
	path := t.TempDir() + "/out.txt"
	w, closer := resolveOutputWriter(path)
	if w == os.Stdout {
		t.Error("expected file writer, got os.Stdout")
	}
	_, err := fmt.Fprintln(w, "hello")
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	closer()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("file content = %q, want %q", string(data), "hello\n")
	}
}
```

- [x] **Step 2: Run the test, confirm failure**

```
go test -count=1 -run 'TestResolveOutputWriter' ./cmd/gen-commit-msg/
# Expected: compilation error — resolveOutputWriter is not defined
```

- [x] **Step 3: Implement resolveOutputWriter**

In `cmd/gen-commit-msg/main.go`, add before `isTerminal()`:

```go
func resolveOutputWriter(path string) (io.WriteCloser, func()) {
	if path == "" {
		return os.Stdout, func() {}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		slog.Error("failed to open output file", "path", path, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to open output file %q: %v\n", path, err)
		os.Exit(1)
	}
	return f, func() { _ = f.Close() }
}
```

- [x] **Step 4: Run tests, confirm pass**

```
go test -count=1 -run 'TestResolveOutputWriter' ./cmd/gen-commit-msg/ -v
# Expected: 2 passing tests
```

- [x] **Step 5: Commit**

```
git add cmd/gen-commit-msg/main.go cmd/gen-commit-msg/output_test.go && git commit -m "feat(main): add resolveOutputWriter helper for --output flag"
```

---

### Task 4: Main - Integrate output writer into TUI and non-interactive paths

**Files:**
- Modify: `cmd/gen-commit-msg/main.go`

- [x] **Step 1: Write the failing test**

No standalone failing test possible for `main()` integration. Verification is through the test suite and `make build`:

```
go vet ./... && make build
```

The existing tests in `output_test.go` (Task 3) already verify the helper logic. This task wires them into main.

- [x] **Step 2: Run the test, confirm failure**

```
make build
# Expected: succeeds (config.Output is unused, but that's a vet warning, not a build failure)
```

- [x] **Step 3: Implement integration**

In `cmd/gen-commit-msg/main.go`:

**Early validation (both paths):** After `cfg, err := config.ParseFlags()` and before `isTerminal()`, add:

```go
if err := cfg.ValidateOutputPath(); err != nil {
    slog.Error("output path validation failed", "error", err)
    fmtError("Error: %v\n", err)
    pauseExit(1, true)
}
```

**TUI path (line ~301):** Replace:

```go
wrote, writeErr := writeSelectedMessage(os.Stdout, selected)
```

With:

```go
writer, closeWriter := resolveOutputWriter(cfg.Output)
wrote, writeErr := writeSelectedMessage(writer, selected)
closeWriter()
```

**Non-interactive path (line ~377):** Replace:

```go
fmt.Println(formatMessageFromOC(messages[0]))
```

With:

```go
writer, closeWriter := resolveOutputWriter(cfg.Output)
fmt.Fprintln(writer, formatMessageFromOC(messages[0]))
closeWriter()
```

Note: `resolveOutputWriter` calls `os.Exit(1)` on failure, which skips deferred cleanup in the non-interactive path. However, `ValidateOutputPath()` is called early (before server start), so the file-open at write time should succeed. The early validation catches the error case.

- [x] **Step 4: Run tests, confirm pass**

```
go vet ./... && go test -count=1 ./... && make build
# Expected: all packages pass, build succeeds
```

- [x] **Step 5: Commit**

```
git add cmd/gen-commit-msg/main.go && git commit -m "feat(main): wire --output flag into TUI and non-interactive paths"
```

---

### Task 5: CLI Output Surface - UX Task

**Surface:** CLI output (`--output` flag, error messages)
**Artifact reference:** `docs/leyline/design/2026-05-15-output-flag-ux.md#commands-enumerated`

- [x] **Step 1:** Confirm the artifact section is current (DRAW step)

- [x] **Step 2:** Implement per the artifact (BUILD step) — completed in Tasks 1-4.

- [x] **Step 3:** Trigger each state from the UX spec:
  - Error (file cannot be created): Run `gen-commit-msg ./gen-commit-msg --output /nonexistent-dir/msg.txt` — expect `Error: output directory does not exist: /nonexistent-dir` on stderr, exit code 1
  - Error (file write fails): Run `gen-commit-msg --output /dev/full` (Linux only) — expect `Error: failed to open output file "/dev/full": ...` on stderr, exit code 1
  - Success: Run with staged changes and `--output /tmp/test-msg.txt` — expect message written to file, stdout silent, exit code 0
  - Empty (no selection): Run in TUI with staged changes, cancel — expect no file created, exit code 0

- [x] **Step 4:** Run the accessibility verification:
  - Color independence: error messages include path and OS error text — no meaning conveyed by color alone
  - Screen-reader-friendly: all errors are plain text on stderr
  - Verify with `gen-commit-msg --output /nonexistent/msg.txt 2>&1` — all output is plain text

- [x] **Step 5:** Side-by-side reconciliation (RECONCILE step):
  - Error format matches UX spec: `Error: failed to open output file "path": <os error>`
  - Exit codes match UX spec: code 1 for runtime errors, code 0 for success
  - Help text displays `-o, --output string   write commit message to file instead of stdout`
  - No divergence expected — if any found, fix code OR update UX spec and loop back

- [x] **Step 6:** Commit

```
git commit --allow-empty -m "ux(cli): verify --output flag UX alignment"
```

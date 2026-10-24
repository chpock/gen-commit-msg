package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectMin != 1 {
		t.Errorf("SubjectMin = %d, want 1", cfg.SubjectMin)
	}
	if cfg.SubjectMax != 5 {
		t.Errorf("SubjectMax = %d, want 5", cfg.SubjectMax)
	}
	if !cfg.Body {
		t.Errorf("Body = false, want true")
	}
	if cfg.Quiet {
		t.Errorf("Quiet = true, want false")
	}
	if cfg.Agent != "gen-commit-msg" {
		t.Errorf("Agent = %q, want gen-commit-msg", cfg.Agent)
	}
	if cfg.LogLevel != "none" {
		t.Errorf("LogLevel = %q, want none", cfg.LogLevel)
	}
	if cfg.Pause != "on-error" {
		t.Errorf("Pause = %q, want on-error", cfg.Pause)
	}
	if cfg.InstallAgent != "if-not-exists" {
		t.Errorf("InstallAgent = %q, want if-not-exists", cfg.InstallAgent)
	}
}

func TestParseFlagsEnvVars(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}
	_ = os.Setenv("GCM_SUBJECT_MIN", "2")
	_ = os.Setenv("GCM_SUBJECT_MAX", "8")
	_ = os.Setenv("GCM_BODY", "false")
	t.Cleanup(func() {
		_ = os.Unsetenv("GCM_SUBJECT_MIN")
		_ = os.Unsetenv("GCM_SUBJECT_MAX")
		_ = os.Unsetenv("GCM_BODY")
	})

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectMin != 2 {
		t.Errorf("SubjectMin = %d, want 2", cfg.SubjectMin)
	}
	if cfg.SubjectMax != 8 {
		t.Errorf("SubjectMax = %d, want 8", cfg.SubjectMax)
	}
	if cfg.Body != false {
		t.Errorf("Body = true, want false")
	}
}

func TestParseFlagsVersionEarlyReturn(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--version"}
	_ = os.Setenv("GCM_SUBJECT_MIN", "100")
	_ = os.Setenv("GCM_SUBJECT_MAX", "200")
	t.Cleanup(func() {
		_ = os.Unsetenv("GCM_SUBJECT_MIN")
		_ = os.Unsetenv("GCM_SUBJECT_MAX")
	})

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Version {
		t.Error("Version should be true when --version flag is set")
	}
	if cfg.SubjectMin != 0 {
		t.Errorf("SubjectMin = %d, want 0 (env should not be read on early return)", cfg.SubjectMin)
	}
	if cfg.SubjectMax != 0 {
		t.Errorf("SubjectMax = %d, want 0 (env should not be read on early return)", cfg.SubjectMax)
	}
	if cfg.LogLevel != "none" {
		t.Errorf("LogLevel = %q, want none on early return", cfg.LogLevel)
	}
}

func TestParseFlagsCLIOverridesEnv(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-min", "2", "--subject-max", "8"}
	_ = os.Setenv("GCM_SUBJECT_MIN", "1")
	_ = os.Setenv("GCM_SUBJECT_MAX", "3")
	t.Cleanup(func() {
		_ = os.Unsetenv("GCM_SUBJECT_MIN")
		_ = os.Unsetenv("GCM_SUBJECT_MAX")
	})

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectMin != 2 {
		t.Errorf("SubjectMin = %d, want 2 (CLI overrides env)", cfg.SubjectMin)
	}
	if cfg.SubjectMax != 8 {
		t.Errorf("SubjectMax = %d, want 8 (CLI overrides env)", cfg.SubjectMax)
	}
}

func TestParseFlagsUnknownFlag(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--nonexistent"}
	_, err := ParseFlags()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseFlagsInvalidValue(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-min", "abc"}
	_, err := ParseFlags()
	if err == nil {
		t.Fatal("expected error for invalid flag value")
	}
}

func TestParseFlagsValidationSubjectMinLessThanOne(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-min", "0"}
	_, err := ParseFlags()
	if err == nil {
		t.Fatal("expected error for subject-min < 1")
	}
	if !strings.Contains(err.Error(), "subject-min must be at least 1") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseFlagsValidationSubjectMaxLessThanMin(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-min", "5", "--subject-max", "3"}
	_, err := ParseFlags()
	if err == nil {
		t.Fatal("expected error for subject-max < subject-min")
	}
	if !strings.Contains(err.Error(), "subject-max") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseFlagsSubjectMinOnlyExceedsDefaultMax(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-min", "10"}
	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectMin != 10 {
		t.Errorf("SubjectMin = %d, want 10", cfg.SubjectMin)
	}
	if cfg.SubjectMax != 10 {
		t.Errorf("SubjectMax = %d, want 10 (auto-adjusted from default 5)", cfg.SubjectMax)
	}
}

func TestParseFlagsValidationSubjectMaxGreaterThan20(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-max", "21"}
	_, err := ParseFlags()
	if err == nil {
		t.Fatal("expected error for subject-max > 20")
	}
	if !strings.Contains(err.Error(), "subject-max must not exceed 20") {
		t.Errorf("unexpected error: %v", err)
	}
}

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
	t.Setenv("GCM_OUTPUT", "")
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

func TestParseFlagsOutputCLIOverridesEnv(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--output", "/tmp/cli-msg.txt"}
	_ = os.Setenv("GCM_OUTPUT", "/tmp/env-msg.txt")
	t.Cleanup(func() { _ = os.Unsetenv("GCM_OUTPUT") })

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "/tmp/cli-msg.txt" {
		t.Errorf("Output = %q, want /tmp/cli-msg.txt (CLI overrides env)", cfg.Output)
	}
}

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
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("validation left an output file: %v", err)
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

func TestParseFlagsValidationSubjectMinMaxEqual(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-min", "3", "--subject-max", "3"}
	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error for subject-min == subject-max: %v", err)
	}
	if cfg.SubjectMin != 3 || cfg.SubjectMax != 3 {
		t.Errorf("SubjectMin=%d SubjectMax=%d, want both 3", cfg.SubjectMin, cfg.SubjectMax)
	}
}

func TestValidateOutputPathPreservesExistingFile(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "existing.txt")
			const original = "previous commit message\n"
			if err := os.WriteFile(target, []byte(original), 0640); err != nil {
				t.Fatal(err)
			}
			path := target
			if kind != "regular" {
				path = filepath.Join(dir, "output.txt")
				link := os.Link
				if kind == "symlink" {
					link = os.Symlink
				}
				if err := link(target, path); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg := &Config{Output: path}
			if err := cfg.ValidateOutputPath(); err != nil {
				t.Fatal(err)
			}
			after, err := os.Lstat(path)
			if err != nil {
				t.Fatalf("validation removed the output path: %v", err)
			}
			if !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Error("validation replaced the output path or changed its permissions")
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != original {
				t.Fatalf("existing content = %q, error = %v; want %q", data, err, original)
			}
		})
	}
}

func TestValidateOutputPathPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass file permissions")
	}
	t.Run("existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "message.txt")
		const original = "keep this message\n"
		if err := os.WriteFile(path, []byte(original), 0444); err != nil {
			t.Fatal(err)
		}
		cfg := &Config{Output: path}
		if err := cfg.ValidateOutputPath(); !errors.Is(err, os.ErrPermission) {
			t.Errorf("expected permission error, got %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original {
			t.Fatalf("read-only content = %q, error = %v", data, err)
		}
	})
	t.Run("parent directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
		cfg := &Config{Output: filepath.Join(dir, "message.txt")}
		if err := cfg.ValidateOutputPath(); !errors.Is(err, os.ErrPermission) {
			t.Errorf("expected permission error, got %v", err)
		}
	})
}

func TestValidateOutputPathRejectsDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")
	const target = "missing.txt"
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Output: path}
	if err := cfg.ValidateOutputPath(); err == nil {
		t.Fatal("expected early rejection of an output symlink with an absent target")
	}
	if got, err := os.Readlink(path); err != nil || got != target {
		t.Fatalf("validation changed the symlink: target=%q error=%v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(dir, target)); !os.IsNotExist(err) {
		t.Fatalf("validation created the absent symlink target: %v", err)
	}
}

package config

import (
	"os"
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

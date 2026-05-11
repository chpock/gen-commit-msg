package config

import (
	"os"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectCount != 5 {
		t.Errorf("SubjectCount = %d, want 5", cfg.SubjectCount)
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
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want error", cfg.LogLevel)
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
	os.Setenv("GCM_SUBJECT_COUNT", "3")
	os.Setenv("GCM_BODY", "false")
	t.Cleanup(func() {
		os.Unsetenv("GCM_SUBJECT_COUNT")
		os.Unsetenv("GCM_BODY")
	})

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectCount != 3 {
		t.Errorf("SubjectCount = %d, want 3", cfg.SubjectCount)
	}
	if cfg.Body != false {
		t.Errorf("Body = true, want false")
	}
}

func TestParseFlagsCLIOverridesEnv(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-count", "7"}
	os.Setenv("GCM_SUBJECT_COUNT", "3")
	t.Cleanup(func() { os.Unsetenv("GCM_SUBJECT_COUNT") })

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectCount != 7 {
		t.Errorf("SubjectCount = %d, want 7 (CLI overrides env)", cfg.SubjectCount)
	}
}

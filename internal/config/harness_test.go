package config

import (
	"os"
	"strings"
	"testing"
)

// Existing configuration tests must not inherit the operator's harness choice.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("GCM_HARNESS"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestParseFlagsHarness(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		args []string
		want string
	}{
		{name: "default", want: "auto"},
		{name: "environment pi", env: "pi", want: "pi"},
		{name: "environment opencode", env: "opencode", want: "opencode"},
		{name: "environment auto", env: "auto", want: "auto"},
		{name: "flag pi overrides environment", env: "opencode", args: []string{"--harness", "pi"}, want: "pi"},
		{name: "flag opencode overrides environment", env: "pi", args: []string{"--harness=opencode"}, want: "opencode"},
		{name: "flag auto overrides invalid environment", env: "invalid", args: []string{"--harness", "auto"}, want: "auto"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GCM_HARNESS", tc.env)
			oldArgs := os.Args
			t.Cleanup(func() { os.Args = oldArgs })
			os.Args = append([]string{"gen-commit-msg"}, tc.args...)
			cfg, err := ParseFlags()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Harness != tc.want {
				t.Errorf("Harness = %q, want %q", cfg.Harness, tc.want)
			}
		})
	}
}

func TestParseFlagsHarnessInvalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		args []string
	}{
		{name: "invalid environment", env: "unknown"},
		{name: "case sensitive environment", env: "Pi"},
		{name: "whitespace environment", env: " pi "},
		{name: "invalid flag", env: "pi", args: []string{"--harness", "unknown"}},
		{name: "empty explicit flag", env: "pi", args: []string{"--harness="}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GCM_HARNESS", tc.env)
			oldArgs := os.Args
			t.Cleanup(func() { os.Args = oldArgs })
			os.Args = append([]string{"gen-commit-msg"}, tc.args...)
			cfg, err := ParseFlags()
			if err == nil || !strings.Contains(err.Error(), "harness") || !strings.Contains(err.Error(), "auto, pi, opencode") {
				t.Fatalf("config=%+v, error=%v; want actionable harness configuration error", cfg, err)
			}
		})
	}
}

func TestParseFlagsHarnessEarlyExit(t *testing.T) {
	for _, early := range []string{"--help", "--version"} {
		t.Run(early, func(t *testing.T) {
			t.Setenv("GCM_HARNESS", "invalid")
			t.Setenv("PATH", t.TempDir())
			oldArgs := os.Args
			t.Cleanup(func() { os.Args = oldArgs })
			os.Args = []string{"gen-commit-msg", early, "--harness", "invalid"}
			cfg, err := ParseFlags()
			if err != nil {
				t.Fatal(err)
			}
			if (early == "--help" && !cfg.Help) || (early == "--version" && !cfg.Version) {
				t.Fatalf("early exit flag lost: %+v", cfg)
			}
		})
	}
}

func TestHarnessFlagHelp(t *testing.T) {
	flags := initFlags()
	if f := flags.Lookup("harness"); f == nil || f.DefValue != "auto" || !strings.Contains(f.Usage, "auto, pi, opencode") {
		t.Fatalf("harness flag missing supported choices/default: %+v", f)
	}
	for _, name := range []string{"agent", "install-agent"} {
		if usage := flags.Lookup(name).Usage; !strings.Contains(usage, "OpenCode-only") {
			t.Errorf("%s help does not identify its scope: %q", name, usage)
		}
	}
}

func TestParseFlagsHarnessPreservesLegacyOptions(t *testing.T) {
	t.Setenv("GCM_HARNESS", "pi")
	t.Setenv("GCM_AGENT", "custom/path.md")
	t.Setenv("GCM_INSTALL_AGENT", "legacy-unvalidated-value")
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"gen-commit-msg"}
	cfg, err := ParseFlags()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent != "custom/path.md" || cfg.InstallAgent != "legacy-unvalidated-value" {
		t.Fatalf("legacy options were changed during configuration parsing: %+v", cfg)
	}
}

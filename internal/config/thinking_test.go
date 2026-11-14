package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseFlagsThinkingLevel(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		args []string
		want string
	}{
		{name: "unset"},
		{name: "environment", env: "medium", want: "medium"},
		{name: "flag overrides environment", env: "high", args: []string{"--thinking-level", "low"}, want: "low"},
		{name: "flag overrides invalid environment", env: "invalid", args: []string{"--thinking-level=off"}, want: "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GCM_THINKING_LEVEL", tc.env)
			oldArgs := os.Args
			t.Cleanup(func() { os.Args = oldArgs })
			os.Args = append([]string{"gen-commit-msg"}, tc.args...)
			cfg, err := ParseFlags()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ThinkingLevel != tc.want {
				t.Fatalf("ThinkingLevel = %q, want %q", cfg.ThinkingLevel, tc.want)
			}
		})
	}
}

func TestParseFlagsThinkingLevelChoices(t *testing.T) {
	for _, level := range []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		for _, source := range []string{"flag", "environment"} {
			t.Run(source+"/"+level, func(t *testing.T) {
				t.Setenv("GCM_THINKING_LEVEL", "")
				oldArgs := os.Args
				t.Cleanup(func() { os.Args = oldArgs })
				os.Args = []string{"gen-commit-msg"}
				if source == "flag" {
					os.Args = append(os.Args, "--thinking-level", level)
				} else {
					t.Setenv("GCM_THINKING_LEVEL", level)
				}
				cfg, err := ParseFlags()
				if err != nil {
					t.Fatal(err)
				}
				if cfg.ThinkingLevel != level {
					t.Fatalf("ThinkingLevel = %q, want %q", cfg.ThinkingLevel, level)
				}
			})
		}
	}
}

func TestParseFlagsThinkingLevelInvalid(t *testing.T) {
	for _, value := range []string{"", "none", "auto", "default", "LOW", " low", "high ", "ultra"} {
		for _, source := range []string{"flag", "environment"} {
			if source == "environment" && value == "" {
				continue
			}
			t.Run(source+"/"+value, func(t *testing.T) {
				t.Setenv("GCM_THINKING_LEVEL", "")
				oldArgs := os.Args
				t.Cleanup(func() { os.Args = oldArgs })
				os.Args = []string{"gen-commit-msg"}
				if source == "flag" {
					os.Args = append(os.Args, "--thinking-level="+value)
				} else {
					t.Setenv("GCM_THINKING_LEVEL", value)
				}
				cfg, err := ParseFlags()
				if err == nil || !strings.Contains(err.Error(), "thinking-level") || !strings.Contains(err.Error(), "off, minimal, low, medium, high, xhigh, max") {
					t.Fatalf("config=%+v, error=%v; want supported thinking levels in the error", cfg, err)
				}
			})
		}
	}
}

func TestParseFlagsThinkingLevelEarlyExit(t *testing.T) {
	for _, early := range []string{"--help", "--version"} {
		t.Run(early, func(t *testing.T) {
			t.Setenv("GCM_THINKING_LEVEL", "invalid")
			oldArgs := os.Args
			t.Cleanup(func() { os.Args = oldArgs })
			os.Args = []string{"gen-commit-msg", early, "--thinking-level", "invalid"}
			cfg, err := ParseFlags()
			if err != nil {
				t.Fatal(err)
			}
			if (early == "--help" && !cfg.Help) || (early == "--version" && !cfg.Version) || cfg.ThinkingLevel != "" {
				t.Fatalf("early exit changed: %+v", cfg)
			}
		})
	}
}

func TestThinkingLevelFlagHelp(t *testing.T) {
	f := initFlags().Lookup("thinking-level")
	if f == nil || f.DefValue != "" || !strings.Contains(f.Usage, "off, minimal, low, medium, high, xhigh, max") {
		t.Fatalf("thinking-level help missing supported choices or unset default: %+v", f)
	}
}

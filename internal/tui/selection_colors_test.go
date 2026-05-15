package tui

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func TestResolveSelectionColorMode(t *testing.T) {
	tests := []struct {
		name       string
		noColor    string
		toggle     string
		capability capabilityClass
		wantMode   selectionColorMode
		wantWarn   bool
		wantNorm   string
	}{
		{name: "no color wins", noColor: "1", toggle: "1", capability: capabilityANSI, wantMode: modeDisabledNoColor, wantWarn: false, wantNorm: "1"},
		{name: "toggle zero disables", toggle: "0", capability: capabilityANSI, wantMode: modeDisabledEnv, wantWarn: false, wantNorm: "0"},
		{name: "trimmed zero disables", toggle: " 0 ", capability: capabilityANSI, wantMode: modeDisabledEnv, wantWarn: false, wantNorm: "0"},
		{name: "toggle one is invalid", toggle: "1", capability: capabilityANSI, wantMode: modeEnabledInvalidEnv, wantWarn: true, wantNorm: "1"},
		{name: "invalid non-empty enables with warn", toggle: "false", capability: capabilityANSI, wantMode: modeEnabledInvalidEnv, wantWarn: true, wantNorm: "false"},
		{name: "unicode whitespace around zero is invalid", toggle: "\u00a00\u00a0", capability: capabilityANSI, wantMode: modeEnabledInvalidEnv, wantWarn: true, wantNorm: "\u00a00\u00a0"},
		{name: "unset enables", toggle: "", capability: capabilityANSI, wantMode: modeEnabled, wantWarn: false, wantNorm: ""},
		{name: "no-color capability disables", capability: capabilityNoColor, wantMode: modeDisabledCapability, wantWarn: false, wantNorm: ""},
		{name: "degraded capability disables", capability: capabilityDegraded, wantMode: modeDisabledCapability, wantWarn: false, wantNorm: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSelectionColorMode(tc.noColor, tc.toggle, tc.capability)
			if got.mode != tc.wantMode {
				t.Fatalf("mode=%q want=%q", got.mode, tc.wantMode)
			}
			if got.warnInvalidToggle != tc.wantWarn {
				t.Fatalf("warnInvalidToggle=%v want=%v", got.warnInvalidToggle, tc.wantWarn)
			}
			if got.envNormalized != tc.wantNorm {
				t.Fatalf("envNormalized=%q want=%q", got.envNormalized, tc.wantNorm)
			}
		})
	}
}

func TestConventionalPrefixMatch(t *testing.T) {
	tests := []struct {
		subject string
		match   bool
	}{
		{subject: "fix: bug", match: true},
		{subject: "fix(scope): bug", match: true},
		{subject: "fix(scope)!: bug", match: true},
		{subject: "Fix(scope): bug", match: false},
		{subject: "fix(scope_name): bug", match: false},
		{subject: "fix(scope.name): bug", match: false},
		{subject: "prefix fix: bug", match: false},
	}

	for _, tc := range tests {
		if got := conventionalPrefixMatch(tc.subject); got != tc.match {
			t.Fatalf("subject=%q match=%v want=%v", tc.subject, got, tc.match)
		}
	}
}

func TestRenderSelectedSubjectColorizedPrefix(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	subject := "fix(scope)!: parser: v2 (fast)!"
	out := renderSelectedSubject(subject, true)

	if out == subject {
		t.Fatal("expected rendered output to be styled when enabled")
	}

	if !strings.Contains(out, "\x1b[") {
		t.Fatal("expected ANSI styling sequences when enabled")
	}

	if !strings.Contains(out, "parser: v2 (fast)!") {
		t.Fatal("expected remainder punctuation to stay plain and contiguous")
	}
}

func TestRenderSelectedSubjectFallbackPlainText(t *testing.T) {
	subject := "fix(scope)!: parser"
	out := renderSelectedSubject(subject, false)
	if out != subject {
		t.Fatalf("got %q want %q", out, subject)
	}
}

func TestLogSelectionColorDecisionFields(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	d := selectionColorDecision{
		mode:              modeDisabledCapability,
		capability:        capabilityDegraded,
		envRawPresent:     true,
		envNormalized:     "invalid",
		envRecognized:     false,
		warnInvalidToggle: true,
	}

	logSelectionColorDecision(logger, d)

	if len(h.records) != 1 {
		t.Fatalf("records=%d want=1", len(h.records))
	}

	got := map[string]any{}
	h.records[0].Attrs(func(a slog.Attr) bool {
		got[a.Key] = a.Value.Any()
		return true
	})

	if got["mode"] != string(d.mode) {
		t.Fatalf("mode=%v want=%q", got["mode"], d.mode)
	}
	if got["capability"] != string(d.capability) {
		t.Fatalf("capability=%v want=%q", got["capability"], d.capability)
	}
	if got["env_raw_present"] != d.envRawPresent {
		t.Fatalf("env_raw_present=%v want=%v", got["env_raw_present"], d.envRawPresent)
	}
	if got["env_normalized"] != d.envNormalized {
		t.Fatalf("env_normalized=%v want=%q", got["env_normalized"], d.envNormalized)
	}
	if got["env_recognized"] != d.envRecognized {
		t.Fatalf("env_recognized=%v want=%v", got["env_recognized"], d.envRecognized)
	}
	if got["warn_invalid_toggle"] != d.warnInvalidToggle {
		t.Fatalf("warn_invalid_toggle=%v want=%v", got["warn_invalid_toggle"], d.warnInvalidToggle)
	}
}

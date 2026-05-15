package tui

import "testing"

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

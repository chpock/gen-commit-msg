package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	reSimple = regexp.MustCompile(`^[a-z]+:`)
	reScope  = regexp.MustCompile(`^[a-z]+\([a-z0-9-]+\):`)
	reBang   = regexp.MustCompile(`^[a-z]+\([a-z0-9-]+\)!:`)
)

type capabilityClass string

const (
	capabilityANSI     capabilityClass = "ansi_capable"
	capabilityNoColor  capabilityClass = "no_color"
	capabilityDegraded capabilityClass = "degraded_or_partial"
)

type selectionColorMode string

const (
	modeEnabled            selectionColorMode = "enabled"
	modeEnabledInvalidEnv  selectionColorMode = "enabled_invalid_env"
	modeDisabledNoColor    selectionColorMode = "disabled_no_color"
	modeDisabledEnv        selectionColorMode = "disabled_env"
	modeDisabledCapability selectionColorMode = "disabled_capability"
)

type selectionColorDecision struct {
	mode              selectionColorMode
	capability        capabilityClass
	envRawPresent     bool
	envNormalized     string
	envRecognized     bool
	warnInvalidToggle bool
}

func resolveSelectionColorMode(noColorValue, toggleValue string, capability capabilityClass) selectionColorDecision {
	normalized := trimASCIISpace(toggleValue)
	if trimASCIISpace(noColorValue) != "" {
		return selectionColorDecision{mode: modeDisabledNoColor, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: normalized == "0" || normalized == ""}
	}
	if capability == capabilityNoColor || capability == capabilityDegraded {
		return selectionColorDecision{mode: modeDisabledCapability, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: normalized == "0" || normalized == ""}
	}
	if normalized == "0" {
		return selectionColorDecision{mode: modeDisabledEnv, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: true}
	}
	if normalized == "" {
		return selectionColorDecision{mode: modeEnabled, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: true}
	}
	return selectionColorDecision{mode: modeEnabledInvalidEnv, capability: capability, envRawPresent: true, envNormalized: normalized, envRecognized: false, warnInvalidToggle: true}
}

func trimASCIISpace(s string) string {
	start := 0
	for start < len(s) && isASCIISpace(s[start]) {
		start++
	}

	end := len(s)
	for end > start && isASCIISpace(s[end-1]) {
		end--
	}

	return s[start:end]
}

func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

func conventionalPrefixMatch(subject string) bool {
	return reBang.MatchString(subject) || reScope.MatchString(subject) || reSimple.MatchString(subject)
}

func renderSelectedSubject(subject string, enableColors bool) string {
	if !enableColors || !conventionalPrefixMatch(subject) {
		return subject
	}
	punctGray := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	punctRed := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	r := subject
	r = strings.ReplaceAll(r, "!", punctRed.Render("!"))
	r = strings.ReplaceAll(r, "(", punctGray.Render("("))
	r = strings.ReplaceAll(r, ")", punctGray.Render(")"))
	r = strings.ReplaceAll(r, ":", punctGray.Render(":"))
	return selected.Render(r)
}

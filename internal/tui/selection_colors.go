package tui

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

func detectCapabilityClass() capabilityClass {
	profile := lipgloss.ColorProfile()
	if profile == termenv.Ascii {
		return capabilityNoColor
	}
	if profile == termenv.ANSI || profile == termenv.ANSI256 || profile == termenv.TrueColor {
		return capabilityANSI
	}
	return capabilityDegraded
}

func logSelectionColorDecision(logger *slog.Logger, d selectionColorDecision) {
	if logger == nil {
		return
	}
	logger.Info(
		"selection color mode decision",
		"mode", string(d.mode),
		"source", "delegate_render",
		"selected_row_styling", d.mode == modeEnabled || d.mode == modeEnabledInvalidEnv,
		"capability_class", string(d.capability),
		"env_raw_present", d.envRawPresent,
		"env_normalized_value", d.envNormalized,
		"env_recognized_toggle", d.envRecognized,
	)
	if d.warnInvalidToggle {
		logger.Warn(
			"selection color toggle value is not recognized; using default behavior",
			"env_normalized_value", d.envNormalized,
		)
	}
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
	_, ok := conventionalPrefixEnd(subject)
	return ok
}

func conventionalPrefixEnd(subject string) (int, bool) {
	for _, re := range []*regexp.Regexp{reBang, reScope, reSimple} {
		if idx := re.FindStringIndex(subject); idx != nil {
			return idx[1], true
		}
	}

	return 0, false
}

func renderSelectedSubject(subject string, enableColors bool) string {
	prefixEnd, ok := conventionalPrefixEnd(subject)
	if !enableColors || !ok {
		return subject
	}
	punctGray := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	punctRed := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	r := subject[:prefixEnd]
	r = strings.ReplaceAll(r, "!", punctRed.Render("!"))
	r = strings.ReplaceAll(r, "(", punctGray.Render("("))
	r = strings.ReplaceAll(r, ")", punctGray.Render(")"))
	r = strings.ReplaceAll(r, ":", punctGray.Render(":"))
	return selected.Render(r + subject[prefixEnd:])
}

# Selection List Colors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `leyline:subagent-driven-development` (recommended) or `leyline:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement deterministic selected-row color styling in the commit selection list, including Conventional-like punctuation coloring, runtime disable controls, capability fallback, and mode-decision logging.

**Architecture:** Add a small `internal/tui` helper module for color-mode resolution (env normalization, precedence, capability mapping) and prefix token rendering. Keep list behavior unchanged and apply styling only in delegate rendering for the selected row. Drive behavior with deterministic unit tests covering grammar, fallback, logging safety, and marker invariants.

**Tech Stack:** Go, bubbletea/bubbles list delegate, lipgloss, slog

**Spec references:**
- Product spec: `docs/leyline/specs/2026-05-15-selection-list-colors-design.md` (round 6)
- UX spec: `docs/leyline/design/2026-05-15-selection-list-colors-ux.md` (round 6)
- Baseline: `docs/leyline/plans/2026-05-15-selection-list-colors-baseline.md`

**Surfaces:** single-screen-ui

**Files:**
- Create: `internal/tui/selection_colors.go` - mode resolution, capability seam, prefix token styling helpers
- Create: `internal/tui/selection_colors_test.go` - deterministic unit tests for env rules, capability mapping, and prefix rendering
- Modify: `internal/tui/tui.go` - delegate uses selection-color helpers and emits mode-decision logs
- Modify: `internal/tui/tui_test.go` - integration checks for delegate rendering invariants and unstyled non-selected rows

---

### Task 1: Add failing tests for mode resolution and normalization

**Files:**
- Create: `internal/tui/selection_colors_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/selection_colors_test.go`:

```go
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
		{name: "invalid non-empty enables with warn", toggle: "false", capability: capabilityANSI, wantMode: modeEnabledInvalidEnv, wantWarn: true, wantNorm: "false"},
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
```

- [ ] **Step 2: Run the test, confirm failure**

```sh
go test -count=1 -race ./internal/tui -run TestResolveSelectionColorMode
# Expected: compile failure for undefined types/functions (capabilityClass, selectionColorMode, resolveSelectionColorMode)
```

- [ ] **Step 3: Implement minimal code**

Create `internal/tui/selection_colors.go`:

```go
package tui

import "strings"

type capabilityClass string

const (
	capabilityANSI     capabilityClass = "ansi_capable"
	capabilityNoColor  capabilityClass = "no_color"
	capabilityDegraded capabilityClass = "degraded_or_partial"
)

type selectionColorMode string

const (
	modeEnabled           selectionColorMode = "enabled"
	modeEnabledInvalidEnv selectionColorMode = "enabled_invalid_env"
	modeDisabledNoColor   selectionColorMode = "disabled_no_color"
	modeDisabledEnv       selectionColorMode = "disabled_env"
	modeDisabledCapability selectionColorMode = "disabled_capability"
)

type selectionColorDecision struct {
	mode             selectionColorMode
	capability       capabilityClass
	envRawPresent    bool
	envNormalized    string
	envRecognized    bool
	warnInvalidToggle bool
}

func resolveSelectionColorMode(noColorValue, toggleValue string, capability capabilityClass) selectionColorDecision {
	normalized := strings.TrimSpace(toggleValue)
	if strings.TrimSpace(noColorValue) != "" {
		return selectionColorDecision{mode: modeDisabledNoColor, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: normalized == "0" || normalized == "1" || normalized == ""}
	}
	if capability == capabilityNoColor || capability == capabilityDegraded {
		return selectionColorDecision{mode: modeDisabledCapability, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: normalized == "0" || normalized == "1" || normalized == ""}
	}
	if normalized == "0" {
		return selectionColorDecision{mode: modeDisabledEnv, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: true}
	}
	if normalized == "" || normalized == "1" {
		return selectionColorDecision{mode: modeEnabled, capability: capability, envRawPresent: toggleValue != "", envNormalized: normalized, envRecognized: true}
	}
	return selectionColorDecision{mode: modeEnabledInvalidEnv, capability: capability, envRawPresent: true, envNormalized: normalized, envRecognized: false, warnInvalidToggle: true}
}
```

- [ ] **Step 4: Run tests, confirm pass**

```sh
go test -count=1 -race ./internal/tui -run TestResolveSelectionColorMode
# Expected: passing
```

- [ ] **Step 5: Commit**

```sh
git add internal/tui/selection_colors.go internal/tui/selection_colors_test.go && git commit -m "test(tui): codify selection color mode resolution"
```

---

### Task 2: Add failing tests for prefix grammar and token rendering

**Files:**
- Modify: `internal/tui/selection_colors_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/selection_colors_test.go`:

```go
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
	out := renderSelectedSubject("fix(scope)!: parser", true)
	for _, token := range []string{"(", ")", ":", "!"} {
		if !strings.Contains(out, token) {
			t.Fatalf("expected token %q in rendered output", token)
		}
	}
	if !strings.Contains(out, "parser") {
		t.Fatal("expected remainder text preserved")
	}
}

func TestRenderSelectedSubjectFallbackPlainText(t *testing.T) {
	subject := "fix(scope)!: parser"
	out := renderSelectedSubject(subject, false)
	if out != subject {
		t.Fatalf("got %q want %q", out, subject)
	}
}
```

- [ ] **Step 2: Run tests, confirm failure**

```sh
go test -count=1 -race ./internal/tui -run "TestConventionalPrefixMatch|TestRenderSelectedSubject"
# Expected: compile failure for undefined conventionalPrefixMatch/renderSelectedSubject
```

- [ ] **Step 3: Implement minimal code**

In `internal/tui/selection_colors.go`, update the import block to include `regexp`
and `github.com/charmbracelet/lipgloss` (keep the existing `strings` import),
then add:

```go

var (
	reSimple = regexp.MustCompile(`^[a-z]+:`)
	reScope  = regexp.MustCompile(`^[a-z]+\([a-z0-9-]+\):`)
	reBang   = regexp.MustCompile(`^[a-z]+\([a-z0-9-]+\)!:`)
)

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
```

- [ ] **Step 4: Run tests, confirm pass**

```sh
go test -count=1 -race ./internal/tui -run "TestConventionalPrefixMatch|TestRenderSelectedSubject"
# Expected: passing
```

- [ ] **Step 5: Commit**

```sh
git add internal/tui/selection_colors.go internal/tui/selection_colors_test.go && git commit -m "feat(tui): add conventional prefix token renderer"
```

---

### Task 3: Integrate mode decision and safe logging into delegate

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/selection_colors_test.go`

- [ ] **Step 1: Write failing tests for mode logging and safety**

In `internal/tui/selection_colors_test.go`, update imports to include `context`
and `log/slog`, then add:

```go
func TestLogSelectionColorDecisionFields(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	decision := selectionColorDecision{
		mode:          modeDisabledEnv,
		capability:    capabilityANSI,
		envRawPresent: true,
		envNormalized: "0",
		envRecognized: true,
	}
	logSelectionColorDecision(logger, decision)

	if len(h.records) == 0 {
		t.Fatal("expected at least one log record")
	}
}

type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }
```

- [ ] **Step 2: Run tests, confirm failure**

```sh
go test -count=1 -race ./internal/tui -run TestLogSelectionColorDecisionFields
# Expected: compile failure for undefined logSelectionColorDecision
```

- [ ] **Step 3: Implement minimal code**

In `internal/tui/selection_colors.go`, update imports to include `log/slog`, then
add:

```go
func logSelectionColorDecision(logger *slog.Logger, d selectionColorDecision) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("selection color mode decision",
		"mode", string(d.mode),
		"source", "delegate_render",
		"selected_row_styling", d.mode == modeEnabled || d.mode == modeEnabledInvalidEnv,
		"capability_class", string(d.capability),
		"env_raw_present", d.envRawPresent,
		"env_normalized_value", d.envNormalized,
		"env_recognized_toggle", d.envRecognized,
	)
	if d.warnInvalidToggle {
		logger.Warn("invalid GCM_TUI_SELECTION_COLORS value; enabling with warning", "env_normalized_value", d.envNormalized)
	}
}
```

In `internal/tui/tui.go`, update delegate construction and render path:

```go
func newCommitDelegate() list.ItemDelegate {
	d := list.NewDefaultDelegate()
	d.SetSpacing(0)

	decision := resolveSelectionColorMode(
		os.Getenv("NO_COLOR"),
		os.Getenv("GCM_TUI_SELECTION_COLORS"),
		detectCapabilityClass(),
	)
	logSelectionColorDecision(slog.Default(), decision)

	return &commitItemDelegate{DefaultDelegate: d, decision: decision}
}

type commitItemDelegate struct {
	list.DefaultDelegate
	decision selectionColorDecision
}
```

- [ ] **Step 4: Run tests, confirm pass**

```sh
go test -count=1 -race ./internal/tui -run TestLogSelectionColorDecisionFields
# Expected: passing
```

- [ ] **Step 5: Commit**

```sh
git add internal/tui/tui.go internal/tui/selection_colors.go internal/tui/selection_colors_test.go && git commit -m "feat(tui): log selection color mode decisions"
```

---

### Task 4: Apply selected-row styling and verify fallback invariants

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing integration tests for delegate render output**

Add to `internal/tui/tui_test.go`:

```go
func TestCommitDelegateSelectedAndUnselectedRendering(t *testing.T) {
	d, ok := newCommitDelegate().(*commitItemDelegate)
	if !ok {
		t.Fatal("delegate type assertion failed")
	}

	items := []list.Item{
		CommitItem{Subject: "fix(scope)!: parser"},
		CommitItem{Subject: "docs: readme"},
	}
	m := list.New(items, d, 80, 2)
	m.Select(0)

	var selected bytes.Buffer
	d.Render(&selected, m, 0, items[0])
	if !strings.HasPrefix(stripANSI(selected.String()), "> ") {
		t.Fatalf("selected row must keep plain-text marker prefix, got %q", selected.String())
	}

	var unselected bytes.Buffer
	d.Render(&unselected, m, 1, items[1])
	if strings.Contains(unselected.String(), "\x1b[") {
		t.Fatalf("unselected row must remain unstyled, got %q", unselected.String())
	}
}

func TestCommitDelegateNoColorFallbackIsPlainText(t *testing.T) {
	d := &commitItemDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
		decision: selectionColorDecision{mode: modeDisabledNoColor},
	}
	item := CommitItem{Subject: "fix(scope)!: parser"}
	m := list.New([]list.Item{item}, d, 80, 1)

	var out bytes.Buffer
	d.Render(&out, m, 0, item)
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("fallback should not emit ANSI escapes, got %q", got)
	}
	if !strings.HasPrefix(got, "> ") {
		t.Fatalf("fallback must preserve marker prefix, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests, confirm failure**

```sh
go test -count=1 -race ./internal/tui -run "TestCommitDelegateSelectedAndUnselectedRendering|TestCommitDelegateNoColorFallbackIsPlainText"
# Expected: failures because delegate currently bolds selected row only and does not enforce new fallback/token behavior
```

- [ ] **Step 3: Implement minimal code**

In `internal/tui/tui.go`, replace `Render` with:

```go
func (d commitItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ci, ok := item.(CommitItem)
	if !ok {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}

	if index != m.Index() {
		_, _ = fmt.Fprint(w, "  "+ci.Subject)
		return
	}

	marker := lipgloss.NewStyle().Bold(true).Render("> ")
	if d.decision.mode == modeDisabledNoColor || d.decision.mode == modeDisabledEnv || d.decision.mode == modeDisabledCapability {
		_, _ = fmt.Fprint(w, "> "+ci.Subject)
		return
	}

	renderedSubject := renderSelectedSubject(ci.Subject, true)
	_, _ = fmt.Fprint(w, marker+renderedSubject)
}
```

Also add ANSI stripper helper in `internal/tui/tui_test.go`:

```go
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}
```

- [ ] **Step 4: Run tests, confirm pass**

```sh
go test -count=1 -race ./internal/tui -run "TestCommitDelegateSelectedAndUnselectedRendering|TestCommitDelegateNoColorFallbackIsPlainText"
# Expected: passing
```

- [ ] **Step 5: Commit**

```sh
git add internal/tui/tui.go internal/tui/tui_test.go && git commit -m "feat(tui): colorize selected commit row with safe fallback"
```

---

### Task 5: Message selection view - UX Task

**Surface:** Message selection view
**Artifact reference:** `docs/leyline/design/2026-05-15-selection-list-colors-ux.md#user-flows`

- [ ] **Step 1:** Confirm artifact sections are current (`User flows`, `Accessibility targets`, `Platform / harness constraints`, `Operational mitigation`) and match implementation intent.
- [ ] **Step 2:** Implement the surface behavior per artifact (selected marker/text split color, punctuation token colors only for anchored conventional-like selected prefixes, unstyled non-selected rows).
- [ ] **Step 3:** Trigger each state from the UX state matrix and observe:
  - Empty: N/A - zero messages do not enter this view
  - Loading: N/A - this view is shown after generation
  - Error: N/A - errors handled in progress/error flow before this view
  - Success: Selected row has ANSI 39 bold marker and ANSI 14 text when colorization is enabled; non-selected rows use terminal default; punctuation highlighting applies only on selected row when pattern matches; if `NO_COLOR` or `GCM_TUI_SELECTION_COLORS=0`, added colorization is disabled
  - Permission-denied: N/A - no permission-gated action in this view
  - Offline: N/A - no network action in this view
- [ ] **Step 4:** Run accessibility verification procedure:
  - Keyboard flow: arrow keys move selection, Enter confirms, Esc exits.
  - Screen-reader/plain-text check: selected subject remains complete text; marker remains visible prefix.
  - Color independence check: with `NO_COLOR=1`, with `GCM_TUI_SELECTION_COLORS=0`, and with capability fallback forced, selected row still unambiguous by `> ` prefix and row position.
  - Diagnostics safety check: mode logs include mode metadata only and no subject/full-row content.
- [ ] **Step 5:** Reconcile implementation with UX artifact; if behavior diverges, either fix code to match artifact or update UX spec and re-approve before continuing.
- [ ] **Step 6:** Commit

```sh
git add docs/leyline/design/2026-05-15-selection-list-colors-ux.md docs/leyline/specs/2026-05-15-selection-list-colors-design.md internal/tui/tui.go internal/tui/tui_test.go internal/tui/selection_colors.go internal/tui/selection_colors_test.go && git commit -m "feat(tui): implement approved selection list color UX"
```

---

### Task 6: Final verification gate

**Files:**
- Modify: none

- [ ] **Step 1:** Exception: formatting task - no failing test. Verification: run `make all` and confirm fmt/vet/lint/test/build all pass.
- [ ] **Step 1:** Exception: doc-only task - no failing test. Verification: run `make all` and confirm fmt/vet/lint/test/build all pass, then record evidence in the review log.

```sh
make all
# Expected: all stages pass; binary builds successfully
```

- [ ] **Step 2:** Record verification evidence for Stage 6 overlays (`verification-before-completion` and `accessibility-verification`) in task notes/review log.

- [ ] **Step 3:** Commit verification note (if review log updated).

```sh
git add docs/leyline/plans/2026-05-15-selection-list-colors-review-log.md && git commit -m "docs(plan): record selection list colors verification evidence"
```

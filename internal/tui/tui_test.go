package tui

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestModelInit(t *testing.T) {
	m := NewModel(5, false)
	if m.state != stateProgress {
		t.Error("initial state should be progress")
	}
	if m.subjectMax != 5 {
		t.Errorf("subjectMax = %d, want 5", m.subjectMax)
	}
	if m.quiet {
		t.Error("quiet should be false")
	}
}

func TestModelInitQuiet(t *testing.T) {
	m := NewModel(5, true)
	if m.state != stateProgress {
		t.Error("initial state should be progress")
	}
	if !m.quiet {
		t.Error("quiet should be true")
	}
}

func TestQuietInitSkipsSpinner(t *testing.T) {
	m := NewModel(5, true)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init with quiet should return nil, skipping spinner")
	}
}

func TestModelInitMsg(t *testing.T) {
	m := NewModel(3, false)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

func TestModelQuitOnCtrlC(t *testing.T) {
	m := NewModel(3, false)
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updated, _ := m.Update(msg)
	if updated.(Model).quitting != true {
		t.Error("Ctrl+C should set quitting to true")
	}
}

func TestModelQuitOnEsc(t *testing.T) {
	m := NewModel(3, false)
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	if updated.(Model).quitting != true {
		t.Error("Esc should set quitting to true")
	}
}

func TestSetMessagesReturnsGenerationResultMsg(t *testing.T) {
	msg := SetMessages([]CommitItem{{Subject: "test", Body: ""}})
	gr, ok := msg.(generationResultMsg)
	if !ok {
		t.Fatal("SetMessages should return generationResultMsg")
	}
	if len(gr.messages) != 1 || gr.messages[0].Subject != "test" {
		t.Error("messages not set correctly")
	}
}

func TestSetErrorReturnsGenerationResultMsg(t *testing.T) {
	msg := SetError(fmt.Errorf("something broke"))
	gr, ok := msg.(generationResultMsg)
	if !ok {
		t.Fatal("SetError should return generationResultMsg")
	}
	if gr.err == nil || gr.err.Error() != "something broke" {
		t.Error("error not set correctly")
	}
}

func TestSingleMessageAutoSelect(t *testing.T) {
	m := NewModel(1, false)
	msg := SetMessages([]CommitItem{{Subject: "feat: done", Body: ""}})
	updated, cmd := m.Update(msg)
	if updated.(Model).selected != "feat: done" {
		t.Errorf("selected = %q, want %q", updated.(Model).selected, "feat: done")
	}
	if cmd == nil {
		t.Error("should return tea.Quit for single message")
	}
}

func TestMultiMessageGoesToResultState(t *testing.T) {
	m := NewModel(3, false)
	msg := SetMessages([]CommitItem{
		{Subject: "feat: a", Body: ""},
		{Subject: "feat: b", Body: ""},
	})
	updated, _ := m.Update(msg)
	if updated.(Model).state != stateResult {
		t.Errorf("state = %v, want stateResult", updated.(Model).state)
	}
}

func TestZeroMessagesGoesToErrorState(t *testing.T) {
	m := NewModel(3, false)
	msg := SetMessages([]CommitItem{})
	updated, _ := m.Update(msg)
	if updated.(Model).state != stateError {
		t.Errorf("state = %v, want stateError", updated.(Model).state)
	}
	if updated.(Model).err == nil || updated.(Model).err.Error() != "no commit messages generated" {
		t.Error("expected 'no commit messages generated' error")
	}
}

func TestErrorMsgGoesToErrorState(t *testing.T) {
	m := NewModel(3, false)
	msg := SetError(fmt.Errorf("server crash"))
	updated, _ := m.Update(msg)
	if updated.(Model).state != stateError {
		t.Errorf("state = %v, want stateError", updated.(Model).state)
	}
	if updated.(Model).err == nil || updated.(Model).err.Error() != "server crash" {
		t.Error("error not stored")
	}
}

func TestFormatMessageSubjectOnly(t *testing.T) {
	got := formatMessage(CommitItem{Subject: "  feat: add  ", Body: ""})
	if got != "feat: add" {
		t.Errorf("got %q, want %q", got, "feat: add")
	}
}

func TestFormatMessageWithBody(t *testing.T) {
	got := formatMessage(CommitItem{
		Subject: "fix: bug  ",
		Body:    "  the details  ",
	})
	want := "fix: bug\n\nthe details"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSpinnerView(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateSpinner
	v := m.View()
	if !contains(v, "Generating commit messages") {
		t.Errorf("spinner view missing label: %q", v)
	}
}

func TestSpinnerViewQuiet(t *testing.T) {
	m := NewModel(5, true)
	m.state = stateSpinner
	v := m.View()
	if contains(v, "Generating commit messages") {
		t.Errorf("quiet spinner view should suppress label, got: %q", v)
	}
}

func TestErrorView(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateError
	m.err = fmt.Errorf("test error")
	v := m.RenderError()
	if !contains(v, "Error: test error") {
		t.Errorf("error view missing error text: %q", v)
	}
	if !contains(v, "Press Enter to exit") {
		t.Errorf("error view missing exit prompt: %q", v)
	}
}

func TestSelectedMessage(t *testing.T) {
	m := NewModel(5, false)
	m.selected = "feat: test"
	if m.SelectedMessage() != "feat: test" {
		t.Error("SelectedMessage() mismatch")
	}
}

func TestErrorAccessor(t *testing.T) {
	m := NewModel(5, false)
	err := fmt.Errorf("oops")
	m.err = err
	if m.Error() != err {
		t.Error("Error() mismatch")
	}
}

func TestShouldQuit(t *testing.T) {
	m := NewModel(5, false)
	if m.ShouldQuit() {
		t.Error("ShouldQuit should be false initially")
	}
	m.quitting = true
	if !m.ShouldQuit() {
		t.Error("ShouldQuit should be true after setting quitting")
	}
}

func TestCommitItemSatisfiesListItem(t *testing.T) {
	ci := CommitItem{Subject: "feat: x", Body: "details"}
	if ci.Title() != "feat: x" {
		t.Error("Title() mismatch")
	}
	if ci.Description() != "details" {
		t.Error("Description() mismatch")
	}
	if ci.FilterValue() != "feat: x" {
		t.Error("FilterValue() mismatch")
	}
}

func TestStepStatusValues(t *testing.T) {
	if StepPending != 0 {
		t.Error("StepPending should be 0 (zero value)")
	}
	if StepRunning != 1 {
		t.Error("StepRunning should be 1")
	}
	if StepDone != 2 {
		t.Error("StepDone should be 2")
	}
	if StepFailed != 3 {
		t.Error("StepFailed should be 3")
	}
	if StepWarning != 4 {
		t.Error("StepWarning should be 4")
	}
	if StepSkipped != 5 {
		t.Error("StepSkipped should be 5")
	}
}

func TestStepLabels(t *testing.T) {
	labels := stepLabels()
	if len(labels) != 5 {
		t.Fatalf("expected 5 step labels, got %d", len(labels))
	}
	if labels[0] != "Starting OpenCode..." {
		t.Errorf("step 0 label = %q", labels[0])
	}
	if labels[1] != "Creating session..." {
		t.Errorf("step 1 label = %q", labels[1])
	}
	if labels[2] != "Generating commit messages..." {
		t.Errorf("step 2 label = %q", labels[2])
	}
	if labels[3] != "Deleting session..." {
		t.Errorf("step 3 label = %q", labels[3])
	}
	if labels[4] != "Stopping OpenCode server..." {
		t.Errorf("step 4 label = %q", labels[4])
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestProgressStateInit(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init in progress state should return spinner tick")
	}
}

func TestProgressViewShowsAllSteps(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepPending}
	}
	v := m.View()
	for _, label := range labels {
		if !contains(v, label) {
			t.Errorf("progress view missing label: %q", label)
		}
	}
}

func TestProgressViewDoesNotEndWithExtraBlankLine(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepDone}
	}

	v := m.View()
	if strings.HasSuffix(v, "\n") {
		t.Fatalf("progress view should not end with trailing newline, got: %q", v)
	}
}

func TestProgressViewAddsTrailingNewlineWhenQuitting(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepRunning}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	v := updated.(Model).View()
	if !strings.HasSuffix(v, "\n") {
		t.Fatalf("progress view should end with newline while quitting, got: %q", v)
	}
}

func TestStepUpdateChangesStatus(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepPending}
	}
	msg := StepUpdateMsg{Index: 0, Status: StepRunning}
	updated, _ := m.Update(msg)
	if updated.(Model).steps[0].status != StepRunning {
		t.Errorf("step 0 status = %v, want StepRunning", updated.(Model).steps[0].status)
	}
}

func TestProgressDoneAllStepsTransitionsToResult(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepDone}
	}
	msg := allStepsDoneMsg{}
	updated, _ := m.Update(msg)
	if updated.(Model).state != stateResult {
		t.Errorf("state = %v, want stateResult after all steps done", updated.(Model).state)
	}
}

func TestStepFailureShowsError(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepPending}
	}
	msg := StepUpdateMsg{Index: 2, Status: StepFailed, Detail: "connection refused"}
	updated, _ := m.Update(msg)
	if updated.(Model).steps[2].status != StepFailed {
		t.Error("step 2 should be failed")
	}
	if updated.(Model).stepDetail != "connection refused" {
		t.Errorf("stepDetail = %q, want %q", updated.(Model).stepDetail, "connection refused")
	}
	if updated.(Model).err == nil || updated.(Model).err.Error() != "connection refused" {
		t.Error("m.err should be set to the failure detail")
	}
	if updated.(Model).state != stateProgress {
		t.Errorf("state = %v, want stateProgress (error state deferred to allStepsDoneMsg)", updated.(Model).state)
	}
	v := updated.(Model).View()
	if !contains(v, "connection refused") {
		t.Errorf("progress view missing error detail: %q", v)
	}
}

func TestProgressViewQuiet(t *testing.T) {
	m := NewModel(5, true)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepPending}
	}
	v := m.View()
	if v != "" {
		t.Errorf("quiet progress view should be empty, got %q", v)
	}
}

func TestAllStepsDoneMsg(t *testing.T) {
	msg := AllStepsDone()
	_, ok := msg.(allStepsDoneMsg)
	if !ok {
		t.Fatal("AllStepsDone should return allStepsDoneMsg")
	}
}

func TestKeyQuitOnFailedStep(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepPending}
	}
	m.steps[0].status = StepFailed
	m.err = fmt.Errorf("test error")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.(Model).quitting {
		t.Error("should set quitting on keypress with failed step")
	}
	if cmd == nil {
		t.Error("should return tea.Quit command")
	}
}

func TestStepUpdateOutOfBoundsIsIgnored(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepPending}
	}
	msg := StepUpdateMsg{Index: 99, Status: StepRunning}
	updated, _ := m.Update(msg)
	for _, s := range updated.(Model).steps {
		if s.status != StepPending {
			t.Errorf("out-of-bounds update should not change any step, got %v", s.status)
		}
	}
	if updated.(Model).stepDetail != "" {
		t.Errorf("out-of-bounds update should not set stepDetail, got %q", updated.(Model).stepDetail)
	}

	msg2 := StepUpdateMsg{Index: -1, Status: StepFailed, Detail: "should not appear"}
	updated2, _ := m.Update(msg2)
	if updated2.(Model).stepDetail != "" {
		t.Errorf("out-of-bounds update with Detail should not set stepDetail, got %q", updated2.(Model).stepDetail)
	}
}

func TestSpinnerTickInProgressState(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepPending}
	}
	// Simulate a spinner tick arriving in progress state.
	cmd := m.spinner.Tick
	tickMsg := cmd()
	updated, _ := m.Update(tickMsg)
	// Should not panic; model should remain in progress state.
	if updated.(Model).state != stateProgress {
		t.Errorf("state should remain stateProgress after tick, got %v", updated.(Model).state)
	}
}

func TestStepSkippedValue(t *testing.T) {
	if StepSkipped != 5 {
		t.Errorf("StepSkipped = %v, want 5", StepSkipped)
	}
}

func TestStepFailureDoesNotAutoSkipDependentSteps(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepPending}
	}
	msg := StepUpdateMsg{Index: 2, Status: StepFailed, Detail: "connection refused"}
	updated, _ := m.Update(msg)
	if updated.(Model).steps[2].status != StepFailed {
		t.Error("step 2 should be StepFailed")
	}
	// Subsequent steps are NOT auto-skipped — the goroutine explicitly controls skip status.
	if updated.(Model).steps[3].status != StepPending {
		t.Errorf("step 3 status = %v, want StepPending (not auto-skipped)", updated.(Model).steps[3].status)
	}
	if updated.(Model).steps[4].status != StepPending {
		t.Errorf("step 4 status = %v, want StepPending (not auto-skipped)", updated.(Model).steps[4].status)
	}
	// Steps before the failure should be unaffected.
	if updated.(Model).steps[0].status != StepPending {
		t.Errorf("step 0 status = %v, want StepPending", updated.(Model).steps[0].status)
	}
	if updated.(Model).steps[1].status != StepPending {
		t.Errorf("step 1 status = %v, want StepPending", updated.(Model).steps[1].status)
	}
	// Error should be set but state stays in progress.
	if updated.(Model).err == nil || updated.(Model).err.Error() != "connection refused" {
		t.Error("m.err should be set to the failure detail")
	}
	if updated.(Model).state != stateProgress {
		t.Errorf("state = %v, want stateProgress (error state deferred to allStepsDoneMsg)", updated.(Model).state)
	}
}

func TestErrorViewShowsSteps(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateError
	m.err = fmt.Errorf("connection refused")
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: StepPending}
	}
	m.steps[0].status = StepDone
	m.steps[1].status = StepDone
	m.steps[2].status = StepFailed
	m.steps[3].status = StepSkipped
	m.steps[4].status = StepSkipped
	v := m.RenderError()
	for _, label := range labels {
		if !contains(v, label) {
			t.Errorf("error view missing step label: %q", label)
		}
	}
	if !contains(v, "Error: connection refused") {
		t.Errorf("error view missing error text: %q", v)
	}
	if !contains(v, "Press Enter to exit") {
		t.Errorf("error view missing exit prompt: %q", v)
	}
}

func TestAllStepsDoneWithFailureTransitionsToError(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepDone}
	}
	m.steps[1].status = StepFailed
	m.err = fmt.Errorf("session creation failed")
	msg := allStepsDoneMsg{}
	updated, _ := m.Update(msg)
	if updated.(Model).state != stateError {
		t.Errorf("state = %v, want stateError when a step failed", updated.(Model).state)
	}
}

func TestStepUpdateAcceptedAfterFailure(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepPending}
	}
	failMsg := StepUpdateMsg{Index: 1, Status: StepFailed, Detail: "session error"}
	updated, _ := m.Update(failMsg)
	if updated.(Model).state != stateProgress {
		t.Errorf("state = %v, want stateProgress after failure", updated.(Model).state)
	}
	doneMsg := StepUpdateMsg{Index: 4, Status: StepDone}
	updated2, _ := updated.(Model).Update(doneMsg)
	if updated2.(Model).steps[4].status != StepDone {
		t.Errorf("step 4 status = %v, want StepDone (update after failure should be accepted)", updated2.(Model).steps[4].status)
	}
	if updated2.(Model).state != stateProgress {
		t.Errorf("state = %v, still want stateProgress", updated2.(Model).state)
	}
	if updated2.(Model).err == nil || updated2.(Model).err.Error() != "session error" {
		t.Error("m.err should still hold the first failure detail")
	}
}

func TestExplicitStepSkippedAcceptedAfterFailure(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepPending}
	}
	failMsg := StepUpdateMsg{Index: 1, Status: StepFailed, Detail: "session error"}
	updated, _ := m.Update(failMsg)
	skipMsg := StepUpdateMsg{Index: 2, Status: StepSkipped}
	updated2, _ := updated.(Model).Update(skipMsg)
	skipMsg2 := StepUpdateMsg{Index: 3, Status: StepSkipped}
	updated3, _ := updated2.(Model).Update(skipMsg2)
	doneMsg := StepUpdateMsg{Index: 4, Status: StepDone}
	updated4, _ := updated3.(Model).Update(doneMsg)
	if updated4.(Model).steps[1].status != StepFailed {
		t.Error("step 1 should be StepFailed")
	}
	if updated4.(Model).steps[2].status != StepSkipped {
		t.Error("step 2 should be StepSkipped (explicitly sent by goroutine)")
	}
	if updated4.(Model).steps[3].status != StepSkipped {
		t.Error("step 3 should be StepSkipped (explicitly sent by goroutine)")
	}
	if updated4.(Model).steps[4].status != StepDone {
		t.Error("step 4 should be StepDone (cleanup ran)")
	}
	allDone := allStepsDoneMsg{}
	updated5, _ := updated4.(Model).Update(allDone)
	if updated5.(Model).state != stateError {
		t.Errorf("state = %v, want stateError (step 1 failed)", updated5.(Model).state)
	}
}

func TestAllStepsDoneWithoutErrorGoesToResult(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: StepDone}
	}
	msg := allStepsDoneMsg{}
	updated, _ := m.Update(msg)
	if updated.(Model).state != stateResult {
		t.Errorf("state = %v, want stateResult when no failures", updated.(Model).state)
	}
}

func TestMultiMessageSetsListHeight(t *testing.T) {
	m := NewModel(3, false)
	msg := SetMessages([]CommitItem{
		{Subject: "feat: a", Body: ""},
		{Subject: "feat: b", Body: ""},
	})
	updated, _ := m.Update(msg)
	if updated.(Model).list.Height() != 2 {
		t.Errorf("list height = %d, want 2", updated.(Model).list.Height())
	}
}

func TestEnterInResultStateSetsStateDone(t *testing.T) {
	m := NewModel(3, false)
	msg := SetMessages([]CommitItem{
		{Subject: "feat: a", Body: ""},
		{Subject: "feat: b", Body: ""},
	})
	updated, _ := m.Update(msg)

	selectMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updated2, cmd := updated.(Model).Update(selectMsg)

	if updated2.(Model).state != stateDone {
		t.Errorf("state = %v, want stateDone after Enter in stateResult", updated2.(Model).state)
	}
	if cmd == nil {
		t.Error("should return tea.Quit after Enter selection")
	}
}

func TestEscInResultStateClearsListWithoutSelection(t *testing.T) {
	m := NewModel(3, false)
	msg := SetMessages([]CommitItem{
		{Subject: "feat: a", Body: ""},
		{Subject: "feat: b", Body: ""},
	})
	updated, _ := m.Update(msg)

	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updated2, cmd := updated.(Model).Update(escMsg)

	if updated2.(Model).state != stateDone {
		t.Errorf("state = %v, want stateDone after Esc in stateResult", updated2.(Model).state)
	}
	if updated2.(Model).selected != "" {
		t.Errorf("selected = %q, want empty selection on Esc", updated2.(Model).selected)
	}
	if v := updated2.(Model).View(); v != "" {
		t.Errorf("view = %q, want empty view after Esc", v)
	}
	if cmd == nil {
		t.Error("should return tea.Quit after Esc in stateResult")
	}
}

func TestCommitItemDelegateHeightIsOne(t *testing.T) {
	d := newCommitDelegate()
	if d.Height() != 1 {
		t.Errorf("delegate height = %d, want 1", d.Height())
	}
}

func TestCommitDelegateSelectedAndUnselectedRendering(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GCM_TUI_SELECTION_COLORS", "1")
	t.Setenv("CLICOLOR_FORCE", "1")

	d := commitItemDelegate{decision: selectionColorDecision{mode: modeEnabled}}
	m := list.New([]list.Item{
		CommitItem{Subject: "feat: one"},
		CommitItem{Subject: "feat: two"},
	}, d, 40, 2)
	m.Select(1)

	var unselected bytes.Buffer
	d.Render(&unselected, m, 0, CommitItem{Subject: "feat: one"})
	if got := unselected.String(); got != "  feat: one" {
		t.Fatalf("unselected row = %q, want plain %q", got, "  feat: one")
	}

	var selected bytes.Buffer
	d.Render(&selected, m, 1, CommitItem{Subject: "feat: two"})
	selectedRaw := selected.String()
	if !strings.Contains(selectedRaw, "\x1b[") {
		t.Fatalf("selected row should include ANSI styling, got %q", selectedRaw)
	}
	if got := stripANSI(selectedRaw); got != "> feat: two" {
		t.Fatalf("selected row text = %q, want %q", got, "> feat: two")
	}
}

func TestCommitDelegateNoColorFallbackIsPlainText(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")

	d := commitItemDelegate{decision: selectionColorDecision{mode: modeDisabledNoColor}}
	m := list.New([]list.Item{CommitItem{Subject: "fix: fallback"}}, d, 40, 1)
	m.Select(0)

	var selected bytes.Buffer
	d.Render(&selected, m, 0, CommitItem{Subject: "fix: fallback"})

	got := selected.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("fallback row should be plain text without ANSI, got %q", got)
	}
	if got != "> fix: fallback" {
		t.Fatalf("fallback row = %q, want %q", got, "> fix: fallback")
	}
}

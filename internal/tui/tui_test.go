package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInit(t *testing.T) {
	m := NewModel(5, false)
	if m.state != stateSpinner {
		t.Error("initial state should be spinner")
	}
	if m.subjectCount != 5 {
		t.Errorf("subjectCount = %d, want 5", m.subjectCount)
	}
	if m.quiet {
		t.Error("quiet should be false")
	}
}

func TestModelInitQuiet(t *testing.T) {
	m := NewModel(5, true)
	if m.state != stateSpinner {
		t.Error("initial state should be spinner")
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
	v := m.View()
	if !contains(v, "Generating commit messages") {
		t.Errorf("spinner view missing label: %q", v)
	}
}

func TestSpinnerViewQuiet(t *testing.T) {
	m := NewModel(5, true)
	v := m.View()
	if contains(v, "Generating commit messages") {
		t.Errorf("quiet spinner view should suppress label, got: %q", v)
	}
}

func TestErrorView(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateError
	m.err = fmt.Errorf("test error")
	v := m.View()
	if !contains(v, "Error: test error") {
		t.Errorf("error view missing error text: %q", v)
	}
	if !contains(v, "Press any key to exit") {
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
	if stepPending != 0 {
		t.Error("stepPending should be 0 (zero value)")
	}
	if stepRunning != 1 {
		t.Error("stepRunning should be 1")
	}
	if stepDone != 2 {
		t.Error("stepDone should be 2")
	}
	if stepFailed != 3 {
		t.Error("stepFailed should be 3")
	}
	if stepWarning != 4 {
		t.Error("stepWarning should be 4")
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

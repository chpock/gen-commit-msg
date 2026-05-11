package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInit(t *testing.T) {
	m := NewModel(5)
	if m.state != stateSpinner {
		t.Error("initial state should be spinner")
	}
	if m.subjectCount != 5 {
		t.Errorf("subjectCount = %d, want 5", m.subjectCount)
	}
}

func TestModelInitMsg(t *testing.T) {
	m := NewModel(3)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

func TestModelQuitOnCtrlC(t *testing.T) {
	m := NewModel(3)
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updated, _ := m.Update(msg)
	if updated.(Model).quitting != true {
		t.Error("Ctrl+C should set quitting to true")
	}
}

func TestModelQuitOnEsc(t *testing.T) {
	m := NewModel(3)
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
	m := NewModel(1)
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
	m := NewModel(3)
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
	m := NewModel(3)
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
	m := NewModel(3)
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
	m := NewModel(5)
	v := m.View()
	if !contains(v, "Generating commit messages") {
		t.Errorf("spinner view missing label: %q", v)
	}
}

func TestErrorView(t *testing.T) {
	m := NewModel(5)
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
	m := NewModel(5)
	m.selected = "feat: test"
	if m.SelectedMessage() != "feat: test" {
		t.Error("SelectedMessage() mismatch")
	}
}

func TestErrorAccessor(t *testing.T) {
	m := NewModel(5)
	err := fmt.Errorf("oops")
	m.err = err
	if m.Error() != err {
		t.Error("Error() mismatch")
	}
}

func TestShouldQuit(t *testing.T) {
	m := NewModel(5)
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

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

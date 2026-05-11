package tui

import (
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

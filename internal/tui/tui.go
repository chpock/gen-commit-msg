package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateSpinner state = iota
	stateResult
	stateDone
)

type commitItem struct {
	subject string
	body    string
}

func (i commitItem) Title() string       { return i.subject }
func (i commitItem) Description() string { return i.body }
func (i commitItem) FilterValue() string { return i.subject }

type Model struct {
	state        state
	spinner      spinner.Model
	list         list.Model
	messages     []commitItem
	selected     string
	quitting     bool
	err          error
	subjectCount int
	width        int
	height       int
}

func NewModel(subjectCount int) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return Model{
		state:        stateSpinner,
		spinner:      s,
		list:         l,
		subjectCount: subjectCount,
	}
}

type generationResultMsg struct {
	messages []commitItem
	err      error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, generateMessages(m.subjectCount))
}

func generateMessages(count int) tea.Cmd {
	return func() tea.Msg {
		// Actual generation is done externally; this is a placeholder
		// The real messages are set before the TUI starts via SetMessages()
		return generationResultMsg{}
	}
}

func SetMessages(messages []commitItem) tea.Cmd {
	return func() tea.Msg {
		return generationResultMsg{messages: messages}
	}
}

func SetError(err error) tea.Cmd {
	return func() tea.Msg {
		return generationResultMsg{err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil
	case generationResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateDone
			return m, tea.Quit
		}
		m.messages = msg.messages
		if len(m.messages) == 1 {
			m.selected = formatMessage(m.messages[0])
			m.state = stateDone
			return m, tea.Quit
		}
		items := make([]list.Item, len(m.messages))
		for i, cm := range m.messages {
			items[i] = cm
		}
		m.list.SetItems(items)
		m.state = stateResult
		return m, nil
	}

	switch m.state {
	case stateSpinner:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case stateResult:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEnter {
				if item, ok := m.list.SelectedItem().(commitItem); ok {
					m.selected = formatMessage(item)
					m.state = stateDone
					return m, tea.Quit
				}
			}
		}
		return m, cmd
	}
	return m, nil
}

func formatMessage(item commitItem) string {
	if item.body == "" {
		return strings.TrimSpace(item.subject)
	}
	return strings.TrimSpace(item.subject) + "\n\n" + strings.TrimSpace(item.body)
}

func (m Model) View() string {
	switch m.state {
	case stateSpinner:
		return fmt.Sprintf("\n  %s Generating commit messages...\n", m.spinner.View())
	case stateResult:
		return m.list.View()
	case stateDone:
		return ""
	}
	return ""
}

func (m Model) SelectedMessage() string {
	return m.selected
}

func (m Model) Error() error {
	return m.err
}

func (m Model) ShouldQuit() bool {
	return m.quitting
}

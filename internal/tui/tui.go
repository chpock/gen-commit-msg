package tui

import (
	"fmt"
	"io"
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
	stateError
)

type CommitItem struct {
	Subject string
	Body    string
}

func (i CommitItem) Title() string       { return i.Subject }
func (i CommitItem) Description() string { return i.Body }
func (i CommitItem) FilterValue() string { return i.Subject }

type Model struct {
	state        state
	spinner      spinner.Model
	list         list.Model
	messages     []CommitItem
	selected     string
	quitting     bool
	err          error
	subjectCount int
	quiet        bool
	width        int
	height       int
}

func NewModel(subjectCount int, quiet bool) Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot

	delegate := newCommitDelegate()
	l := list.New([]list.Item{}, delegate, 40, 10)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	return Model{
		state:        stateSpinner,
		spinner:      s,
		list:         l,
		subjectCount: subjectCount,
		quiet:        quiet,
	}
}

type generationResultMsg struct {
	messages []CommitItem
	err      error
}

func (m Model) Init() tea.Cmd {
	if m.quiet {
		return nil
	}
	return m.spinner.Tick
}

func SetMessages(messages []CommitItem) tea.Msg {
	return generationResultMsg{messages: messages}
}

func SetError(err error) tea.Msg {
	return generationResultMsg{err: err}
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
		w := msg.Width
		if w < 40 {
			w = 40
		}
		h := msg.Height - 2
		if h < 1 {
			h = 1
		}
		m.list.SetSize(w, h)
		return m, nil
	case generationResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.messages = msg.messages
		switch len(m.messages) {
		case 0:
			m.err = fmt.Errorf("no commit messages generated")
			m.state = stateError
			return m, nil
		case 1:
			m.selected = formatMessage(m.messages[0])
			return m, tea.Quit
		default:
			items := make([]list.Item, len(m.messages))
			for i, cm := range m.messages {
				items[i] = cm
			}
			m.list.SetItems(items)
			m.state = stateResult
			return m, nil
		}
	}

	switch m.state {
	case stateSpinner:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case stateResult:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			if item, ok := m.list.SelectedItem().(CommitItem); ok {
				m.selected = formatMessage(item)
				return m, tea.Quit
			}
		}
		return m, cmd
	case stateError:
		if _, ok := msg.(tea.KeyMsg); ok {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func formatMessage(item CommitItem) string {
	if item.Body == "" {
		return strings.TrimSpace(item.Subject)
	}
	return strings.TrimSpace(item.Subject) + "\n\n" + strings.TrimSpace(item.Body)
}

func (m Model) View() string {
	switch m.state {
	case stateSpinner:
		if m.quiet {
			return ""
		}
		return fmt.Sprintf("\n  %s Generating commit messages...\n", m.spinner.View())
	case stateResult:
		return m.list.View()
	case stateError:
		return fmt.Sprintf("\n  Error: %s\n\n  Press any key to exit.\n", m.err)
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

type commitItemDelegate struct {
	list.DefaultDelegate
}

func newCommitDelegate() list.ItemDelegate {
	d := list.NewDefaultDelegate()
	d.SetSpacing(0)
	return &commitItemDelegate{DefaultDelegate: d}
}

func (d commitItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ci, ok := item.(CommitItem)
	if !ok {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}

	var (
		titleStyle, descStyle lipgloss.Style
		prefix                string
	)

	if index == m.Index() {
		prefix = "> "
		titleStyle = lipgloss.NewStyle().Bold(true)
		descStyle = lipgloss.NewStyle()
	} else {
		prefix = "  "
		titleStyle = lipgloss.NewStyle()
		descStyle = lipgloss.NewStyle()
	}

	s := titleStyle.Render(prefix + ci.Subject)
	if ci.Body != "" {
		s += "\n" + descStyle.Render("    " + ci.Body)
	}
	fmt.Fprint(w, s)
}

func (d commitItemDelegate) Height() int {
	return 3
}

func (d commitItemDelegate) Spacing() int {
	return 0
}

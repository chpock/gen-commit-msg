package tui

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateProgress state = iota
	stateSpinner
	stateResult
	stateError
)

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
	StepWarning
)

type stepItem struct {
	label  string
	status StepStatus
}

type StepUpdateMsg struct {
	Index  int
	Status StepStatus
	Detail string
}

func stepLabels() [5]string {
	return [5]string{
		"Starting OpenCode...",
		"Creating session...",
		"Generating commit messages...",
		"Deleting session...",
		"Stopping OpenCode server...",
	}
}

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
	steps        []stepItem
	stepDetail   string
	logPath      string
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

	labels := stepLabels()
	steps := make([]stepItem, 5)
	for i := range steps {
		steps[i] = stepItem{label: labels[i], status: StepPending}
	}

	return Model{
		state:        stateProgress,
		spinner:      s,
		list:         l,
		steps:        steps,
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
	if m.state == stateProgress {
		return m.spinner.Tick
	}
	return nil
}

func SetMessages(messages []CommitItem) tea.Msg {
	return generationResultMsg{messages: messages}
}

func SetError(err error) tea.Msg {
	return generationResultMsg{err: err}
}

type setLogPathMsg struct {
	path string
}

type allStepsDoneMsg struct{}

func SetLogPath(path string) tea.Msg {
	return setLogPathMsg{path: path}
}

func AllStepsDone() tea.Msg {
	return allStepsDoneMsg{}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == stateProgress && m.err != nil {
			m.quitting = true
			return m, tea.Quit
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if msg.Width < 40 {
			m.err = fmt.Errorf("terminal too narrow: %d columns. Minimum width: 40 columns", msg.Width)
			m.quitting = true
			return m, tea.Quit
		}
		w := msg.Width
		h := msg.Height - 2
		if h < 1 {
			h = 1
		}
		m.list.SetSize(w, h)
		return m, nil
	case generationResultMsg:
		if msg.err != nil {
			slog.Error("generation failed", "error", msg.err)
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		slog.Debug("received messages", "count", len(msg.messages))
		m.messages = msg.messages
		switch len(m.messages) {
		case 0:
			slog.Warn("no commit messages generated")
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
			slog.Debug("switched to result state", "item_count", len(items))
			return m, nil
		}
	case setLogPathMsg:
		m.logPath = msg.path
		return m, nil
	case StepUpdateMsg:
		if m.state == stateProgress {
			if msg.Index >= 0 && msg.Index < len(m.steps) {
				m.steps[msg.Index].status = msg.Status
				m.stepDetail = msg.Detail
			} else {
				slog.Warn("step update with out-of-bounds index", "index", msg.Index, "len", len(m.steps))
			}
			if msg.Status == StepFailed {
				m.err = fmt.Errorf("%s", msg.Detail)
				m.state = stateError
				slog.Debug("step failure", "index", msg.Index, "detail", msg.Detail)
				return m, nil
			}
			if msg.Status == StepWarning {
				slog.Debug("step warning", "index", msg.Index, "detail", msg.Detail)
				return m, nil
			}
			return m, m.spinner.Tick
		}
	case allStepsDoneMsg:
		if m.state == stateProgress {
			m.state = stateResult
			return m, nil
		}
	}

	switch m.state {
	case stateProgress:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
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
	case stateProgress:
		if m.quiet {
			return ""
		}
		bold := lipgloss.NewStyle().Bold(true)
		faint := lipgloss.NewStyle().Faint(true)
		doneIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render
		failIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render
		warnIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render
		var b strings.Builder
		for _, s := range m.steps {
			b.WriteString("\n  ")
			switch s.status {
			case StepPending:
				b.WriteString(faint.Render("  "))
				b.WriteString(" ")
				b.WriteString(faint.Render(s.label))
				continue
			case StepRunning:
				b.WriteString(m.spinner.View())
				b.WriteString(" ")
				b.WriteString(bold.Render(s.label))
				continue
			case StepDone:
				b.WriteString(doneIcon("✓"))
			case StepFailed:
				b.WriteString(failIcon("✗"))
			case StepWarning:
				b.WriteString(warnIcon("⚠"))
			}
			b.WriteString(" ")
			b.WriteString(faint.Render(s.label))
		}
		if m.stepDetail != "" {
			b.WriteString("\n\n  ")
			b.WriteString(m.stepDetail)
		}
		if m.logPath != "" {
			b.WriteString("\n  Details: ")
			b.WriteString(m.logPath)
		}
		b.WriteString("\n")
		return b.String()
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
		s += "\n" + descStyle.Render("    "+ci.Body)
	}
	fmt.Fprint(w, s)
}

func (d commitItemDelegate) Height() int {
	return 3
}

func (d commitItemDelegate) Spacing() int {
	return 0
}

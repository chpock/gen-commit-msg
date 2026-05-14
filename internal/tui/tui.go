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

	"github.com/chpock/gen-commit-msg/internal/color"
	"github.com/chpock/gen-commit-msg/internal/opencode"
)

type state int

const (
	stateProgress state = iota
	stateSpinner
	stateResult
	stateError
	stateDone
)

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
	StepWarning
	StepSkipped
)

type stepItem struct {
	label  string
	status StepStatus
}

type StepUpdateMsg struct {
	Index  int
	Status StepStatus
	Detail string
	Err    *opencode.AppError
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
	state      state
	spinner    spinner.Model
	list       list.Model
	messages   []CommitItem
	selected   string
	quitting   bool
	err        error
	appErr     *opencode.AppError
	subjectMax int
	quiet      bool
	width      int
	height     int
	steps      []stepItem
	stepDetail string
}

func NewModel(subjectMax int, quiet bool) Model {
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
		state:      stateProgress,
		spinner:    s,
		list:       l,
		steps:      steps,
		subjectMax: subjectMax,
		quiet:      quiet,
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

type allStepsDoneMsg struct{}

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
		case tea.KeyCtrlC:
			slog.Info("received SIGINT, initiating graceful shutdown")
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
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
			m.quitting = true
			return m, tea.Quit
		}
		slog.Debug("received messages", "count", len(msg.messages))
		m.messages = msg.messages
		switch len(m.messages) {
		case 0:
			slog.Warn("no commit messages generated")
			m.err = fmt.Errorf("no commit messages generated")
			m.state = stateError
			m.quitting = true
			return m, tea.Quit
		case 1:
			m.selected = formatMessage(m.messages[0])
			m.state = stateDone
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
				if msg.Err != nil {
					m.appErr = msg.Err
				}
				slog.Debug("step failure", "index", msg.Index, "detail", msg.Detail)
			}
			if msg.Status == StepWarning {
				slog.Debug("step warning", "index", msg.Index, "detail", msg.Detail)
			}
			return m, m.spinner.Tick
		}
	case allStepsDoneMsg:
		if m.state == stateProgress {
			if m.err != nil {
				m.state = stateError
				m.quitting = true
				return m, tea.Quit
			} else {
				m.state = stateResult
			}
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
	}
	return m, nil
}

func formatMessage(item CommitItem) string {
	if item.Body == "" {
		return strings.TrimSpace(item.Subject)
	}
	return strings.TrimSpace(item.Subject) + "\n\n" + strings.TrimSpace(item.Body)
}

func (m Model) renderSteps(b *strings.Builder) {
	bold := lipgloss.NewStyle().Bold(true)
	faint := lipgloss.NewStyle().Faint(true)
	doneIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render
	failIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render
	warnIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render
	skipIcon := lipgloss.NewStyle().Faint(true).Render
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
		case StepSkipped:
			b.WriteString(skipIcon("-"))
		}
		b.WriteString(" ")
		b.WriteString(faint.Render(s.label))
	}
}

func (m Model) View() string {
	switch m.state {
	case stateProgress:
		if m.quiet {
			return ""
		}
		var b strings.Builder
		m.renderSteps(&b)
		if m.stepDetail != "" {
			b.WriteString("\n\n  ")
			b.WriteString(m.stepDetail)
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
		return ""
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

func (m Model) RenderError() string {
	var b strings.Builder
	m.renderSteps(&b)

	b.WriteString("\n\n")

	if m.appErr != nil {
		b.WriteString(m.appErr.Render())
	} else {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
		detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

		b.WriteString(errStyle.Render("Error:"))
		b.WriteString(" ")

		errText := m.err.Error()
		if idx := strings.Index(errText, "\n"); idx >= 0 {
			firstLine := errText[:idx]
			rest := errText[idx+1:]
			b.WriteString(detailStyle.Render(firstLine))
			b.WriteString("\n")
			if jsonIdx := strings.LastIndex(rest, "\n{"); jsonIdx >= 0 {
				detailBlock := rest[:jsonIdx+1]
				jsonPart := rest[jsonIdx+1:]
				b.WriteString(color.Indent(color.ColorizeKeyValueBlock(detailBlock), 4))
				b.WriteString(color.Indent(color.ColorizeJSON(jsonPart), 4))
			} else if jsonIdx := strings.LastIndex(rest, "\n["); jsonIdx >= 0 {
				detailBlock := rest[:jsonIdx+1]
				jsonPart := rest[jsonIdx+1:]
				b.WriteString(color.Indent(color.ColorizeKeyValueBlock(detailBlock), 4))
				b.WriteString(color.Indent(color.ColorizeJSON(jsonPart), 4))
			} else if strings.HasPrefix(strings.TrimSpace(rest), "{") || strings.HasPrefix(strings.TrimSpace(rest), "[") {
				b.WriteString(color.Indent(color.ColorizeJSON(rest), 4))
			} else {
				b.WriteString(color.Indent(rest, 4))
			}
		} else {
			b.WriteString(detailStyle.Render(errText))
		}
	}

	b.WriteString("\nPress Enter to exit.")
	return b.String()
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
	_, _ = fmt.Fprint(w, s)
}

func (d commitItemDelegate) Height() int {
	return 3
}

func (d commitItemDelegate) Spacing() int {
	return 0
}

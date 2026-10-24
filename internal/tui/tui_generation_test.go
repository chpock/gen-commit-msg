package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/opencode"
)

func harnessStepLists() map[string][]string {
	return map[string][]string{
		"opencode": openCodeStepLabels(),
		"pi": {
			"Collecting information about current changes...",
			"Starting Pi...",
			"Generating commit messages...",
			"Stopping Pi...",
		},
	}
}

func updateModel(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestModelDeclaredSteps(t *testing.T) {
	for name, labels := range harnessStepLists() {
		t.Run(name, func(t *testing.T) {
			m := NewModel(3, false, labels, nil)
			view := stripANSI(m.View())
			rows := strings.Split(view, "\n")
			if len(rows) != len(labels) {
				t.Fatalf("initial rows = %d, want %d: %q", len(rows), len(labels), view)
			}
			for i, label := range labels {
				if rows[i] != "  - "+label {
					t.Errorf("initial row %d = %q, want pending %q", i, rows[i], label)
				}
			}
			labels[0] = "mutated caller storage"
			if strings.Contains(m.View(), labels[0]) {
				t.Fatal("caller mutation changed declared labels")
			}
		})
	}
}

func TestModelGenericAndRichErrors(t *testing.T) {
	rich := &opencode.AppError{Op: "generate_messages", Message: "provider refused", OC: &opencode.OCError{Kind: opencode.OCErrAPI, Code: "provider_error", Message: "provider detail", RequestType: "generate"}}
	for _, err := range []error{
		errors.New("Pi protocol failed\nresponse id: prompt-1"),
		rich,
		fmt.Errorf("wrapped generation: %w", rich),
	} {
		for _, terminal := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/terminal=%t", err.Error(), terminal), func(t *testing.T) {
				m := NewModel(3, false, harnessStepLists()["pi"], nil)
				if terminal {
					m = updateModel(m, SetError(err))
				} else {
					m = updateModel(m, generation.Update{Index: 2, Status: generation.StepFailed, Detail: "Generation failed", Err: err})
					if m.state != stateProgress {
						t.Fatal("failure stopped progress before cleanup")
					}
					m = updateModel(m, AllStepsDone())
				}
				if m.Error() != err || m.state != stateError {
					t.Fatalf("terminal error/state = %v/%v, want original error/error state", m.Error(), m.state)
				}
				view := stripANSI(m.RenderError())
				var renderer interface{ Render() string }
				if errors.As(err, &renderer) {
					if !strings.Contains(view, stripANSI(renderer.Render())) {
						t.Errorf("rich rendering lost: %q", view)
					}
				} else if !strings.Contains(view, "Pi protocol failed") || !strings.Contains(view, "response id: prompt-1") {
					t.Errorf("generic error lost: %q", view)
				}
			})
		}
	}
}

func TestModelProgressStatusesAndCleanupWarning(t *testing.T) {
	for name, labels := range harnessStepLists() {
		t.Run(name, func(t *testing.T) {
			m := NewModel(3, false, labels, nil)
			m = updateModel(m, generation.Update{Index: 1, Status: generation.StepInterrupted, Detail: "Interrupted by user"})
			m = updateModel(m, generation.Update{Index: 2, Status: generation.StepSkipped})
			last := len(labels) - 1
			m = updateModel(m, generation.Update{Index: last, Status: generation.StepRunning})
			warning := errors.New("forced termination required")
			m = updateModel(m, generation.Update{Index: last, Status: generation.StepWarning, Err: warning})
			view := stripANSI(m.View())
			for _, text := range []string{"✗ " + labels[1], "- " + labels[2], "⚠ " + labels[last], "Interrupted by user", "Cleanup issue: " + warning.Error()} {
				if !strings.Contains(view, text) {
					t.Errorf("progress missing %q: %q", text, view)
				}
			}
			if m.Error() != nil {
				t.Errorf("interruption/warning created fatal error: %v", m.Error())
			}
			m = updateModel(m, AllStepsDone())
			if !m.ShouldQuit() || m.SelectedMessage() != "" {
				t.Fatal("interrupted generation must quit without selection")
			}
		})
	}
}

func TestModelPrimaryErrorSurvivesCleanupWarning(t *testing.T) {
	for name, labels := range harnessStepLists() {
		t.Run(name, func(t *testing.T) {
			m := NewModel(3, false, labels, nil)
			primary := errors.New("provider authentication failed")
			m = updateModel(m, generation.Update{Index: 1, Status: generation.StepFailed, Err: primary})
			m = updateModel(m, generation.Update{Index: len(labels) - 1, Status: generation.StepWarning, Detail: "Cleanup issue: shutdown timed out"})
			m = updateModel(m, generation.Update{Index: 0, Status: generation.StepDone, Detail: "Other progress detail"})
			if !strings.Contains(m.View(), "Cleanup issue: shutdown timed out") {
				t.Fatal("later progress erased cleanup warning detail")
			}
			m = updateModel(m, AllStepsDone())
			view := stripANSI(m.RenderError())
			if m.Error() != primary || !strings.Contains(view, primary.Error()) || !strings.Contains(view, "Cleanup issue: shutdown timed out") {
				t.Fatalf("primary or warning lost: error=%v, view=%q", m.Error(), view)
			}
		})
	}
}

func TestModelFirstFailureRemainsPrimary(t *testing.T) {
	m := NewModel(3, false, harnessStepLists()["pi"], nil)
	primary := errors.New("generation failed")
	secondary := errors.New("cleanup failed")
	m = updateModel(m, generation.Update{Index: 2, Status: generation.StepFailed, Err: primary})
	m = updateModel(m, generation.Update{Index: 3, Status: generation.StepFailed, Err: secondary})
	m = updateModel(m, SetError(secondary))
	if m.Error() != primary || !strings.Contains(m.RenderError(), primary.Error()) {
		t.Fatalf("terminal failure replaced primary: %v", m.Error())
	}
}

func TestModelMinimumTerminalWidth(t *testing.T) {
	for _, width := range []int{39, 40, 80} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := NewModel(3, false, harnessStepLists()["pi"], nil)
			m = updateModel(m, tea.WindowSizeMsg{Width: width, Height: 1})
			if width < 40 {
				if m.Error() == nil || !m.ShouldQuit() || !strings.Contains(m.Error().Error(), "Minimum width: 40 columns") {
					t.Fatalf("narrow terminal not rejected: %v", m.Error())
				}
				return
			}
			if m.Error() != nil || m.ShouldQuit() || m.list.Width() != width || m.list.Height() != 1 {
				t.Fatalf("valid terminal rejected or wrong layout: error=%v size=%dx%d", m.Error(), m.list.Width(), m.list.Height())
			}
		})
	}
}

func TestModelWarningPreservesSelection(t *testing.T) {
	for name, labels := range harnessStepLists() {
		for _, quiet := range []bool{false, true} {
			for _, count := range []int{1, 2} {
				t.Run(fmt.Sprintf("%s/quiet=%t/count=%d", name, quiet, count), func(t *testing.T) {
					m := NewModel(3, quiet, labels, nil)
					warning := "Cleanup issue: forced shutdown"
					m = updateModel(m, generation.Update{Index: len(labels) - 1, Status: generation.StepWarning, Detail: warning})
					if !quiet && !strings.Contains(m.View(), warning) {
						t.Fatal("visible progress lost warning")
					}
					if quiet && m.View() != "" {
						t.Fatal("quiet progress must remain hidden")
					}
					messages := []CommitItem{{Subject: "feat: first", Body: "common body"}, {Subject: "fix: second", Body: "common body"}}
					m = updateModel(m, SetMessages(messages[:count]))
					if m.Error() != nil {
						t.Fatalf("cleanup warning blocked candidates: %v", m.Error())
					}
					want := "feat: first\n\ncommon body"
					if count > 1 {
						view := stripANSI(m.View())
						if !strings.Contains(view, "feat: first") || !strings.Contains(view, "fix: second") || strings.Contains(view, "common body") || strings.Contains(view, warning) {
							t.Fatalf("selection must remain subjects-only: %q", view)
						}
						m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
						m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
						want = "fix: second\n\ncommon body"
					}
					if m.SelectedMessage() != want || m.View() != "" {
						t.Fatalf("selection = %q, view = %q", m.SelectedMessage(), m.View())
					}
				})
			}
		}
	}
}

func TestModelCancellationSuppressesLateCandidates(t *testing.T) {
	for name, labels := range harnessStepLists() {
		for _, key := range []tea.KeyType{tea.KeyCtrlC, tea.KeyEsc} {
			t.Run(fmt.Sprintf("%s/key=%d", name, key), func(t *testing.T) {
				calls := 0
				m := NewModel(3, false, labels, func() { calls++ })
				m = updateModel(m, tea.KeyMsg{Type: key})
				if key == tea.KeyCtrlC && calls != 1 {
					t.Fatalf("cancellation callbacks = %d, want 1", calls)
				}
				m = updateModel(m, generation.Update{Index: len(labels) - 1, Status: generation.StepWarning, Detail: "Cleanup issue: stopped forcibly"})
				m = updateModel(m, SetMessages([]CommitItem{{Subject: "must not escape"}}))
				m = updateModel(m, AllStepsDone())
				if m.SelectedMessage() != "" || !m.ShouldQuit() {
					t.Fatalf("cancellation leaked candidate: %q, quit=%t", m.SelectedMessage(), m.ShouldQuit())
				}
			})
		}
	}
}

func TestModelDoesNotInvokeHarnessOrInstallAgents(t *testing.T) {
	bin := t.TempDir()
	configDir := t.TempDir()
	marker := filepath.Join(bin, "invoked")
	for _, name := range []string{"pi", "opencode"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho invoked > '"+marker+"'\nexit 1\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", configDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)
	for _, labels := range harnessStepLists() {
		m := NewModel(1, false, labels, nil)
		_ = m.Init()
		for i := range labels {
			m = updateModel(m, generation.Update{Index: i, Status: generation.StepDone})
		}
		m = updateModel(m, SetMessages([]CommitItem{{Subject: "feat: view only"}}))
		if m.SelectedMessage() != "feat: view only" {
			t.Fatal("view-only flow failed")
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("harness invocation marker: %v", err)
	}
	entries, err := os.ReadDir(configDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unexpected configuration writes: %v, %v", entries, err)
	}
}

func TestModelCleanupWarningFitsTerminalWidth(t *testing.T) {
	for _, width := range []int{40, 80} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			m := NewModel(2, false, openCodeStepLabels(), nil)
			m = updateModel(m, tea.WindowSizeMsg{Width: width, Height: 24})
			warning := errors.New("delete OpenCode session: DELETE http://127.0.0.1:12345/session/" + strings.Repeat("longidentifier", 5) + ": provider rejected session cleanup")
			m = updateModel(m, generation.Update{Index: len(m.steps) - 1, Status: generation.StepWarning, Err: warning})
			view := stripANSI(m.View())
			start := strings.Index(view, "Cleanup issue:")
			if start < 0 {
				t.Fatalf("cleanup warning missing: %q", view)
			}
			for _, line := range strings.Split(view[start:], "\n") {
				if len(line) > width {
					t.Errorf("warning exceeds terminal width %d: %q", width, line)
				}
			}
			if !strings.Contains(strings.Join(strings.Fields(view), " "), "provider rejected session cleanup") || m.Error() != nil {
				t.Fatalf("warning lost its cause or became fatal: error=%v, view=%q", m.Error(), view)
			}
		})
	}
}

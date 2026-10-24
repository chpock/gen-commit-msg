package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

type testTerminal struct {
	master  *os.File
	command *exec.Cmd
	mu      sync.Mutex
	output  bytes.Buffer
	changed chan struct{}
	done    chan struct{}
	drained chan struct{}
	err     error
}

func startTestTerminal(t *testing.T, command *exec.Cmd) *testTerminal {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			t.Skipf("PTY unavailable: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = master.Close() })
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
		t.Fatal(err)
	}
	command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	// A bare PTY cannot answer OSC color queries; multiplexers skip those queries.
	command.Env = append(command.Env, "TERM=screen-256color", "NO_COLOR=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()
	terminal := &testTerminal{master: master, command: command, changed: make(chan struct{}, 1), done: make(chan struct{}), drained: make(chan struct{})}
	go func() {
		defer close(terminal.drained)
		buffer := make([]byte, 4096)
		for {
			count, err := master.Read(buffer)
			if count != 0 {
				terminal.mu.Lock()
				_, _ = terminal.output.Write(buffer[:count])
				terminal.mu.Unlock()
				select {
				case terminal.changed <- struct{}{}:
				default:
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		terminal.err = command.Wait()
		close(terminal.done)
	}()
	t.Cleanup(func() {
		select {
		case <-terminal.done:
		default:
			_ = command.Process.Kill()
		}
		select {
		case <-terminal.done:
		case <-time.After(5 * time.Second):
			t.Error("TTY process did not finish during test cleanup")
		}
		_ = master.Close()
		<-terminal.drained
	})
	return terminal
}

func (terminal *testTerminal) text() string {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	return ansi.Strip(terminal.output.String())
}

func (terminal *testTerminal) await(t *testing.T, text string) {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for !strings.Contains(terminal.text(), text) {
		select {
		case <-terminal.changed:
		case <-terminal.drained:
			t.Fatalf("TTY closed before %q appeared:\n%s", text, terminal.text())
		case <-timer.C:
			t.Fatalf("TTY never displayed %q:\n%s", text, terminal.text())
		}
	}
}

func (terminal *testTerminal) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := terminal.master.WriteString(keys); err != nil {
		t.Fatal(err)
	}
}

func (terminal *testTerminal) wait(t *testing.T) {
	t.Helper()
	terminal.waitExitCode(t, 0)
}

func (terminal *testTerminal) waitExitCode(t *testing.T, want int) {
	t.Helper()
	select {
	case <-terminal.done:
	case <-time.After(15 * time.Second):
		t.Fatalf("TTY process did not finish:\n%s", terminal.text())
	}
	select {
	case <-terminal.drained:
	case <-time.After(5 * time.Second):
		t.Fatalf("TTY output did not drain:\n%s", terminal.text())
	}
	if state := terminal.command.ProcessState; state == nil || state.ExitCode() != want {
		t.Fatalf("TTY process state=%v, error=%v; want exit %d\n%s", state, terminal.err, want, terminal.text())
	}
}

func TestCLIWorkflowTTYSelectionAndCancellation(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		keys     string
		selected string
		existing bool
		quiet    bool
		warning  bool
	}{
		{name: "first", keys: "\r", selected: "fix: preserve the first candidate"},
		{name: "second", keys: "\x1b[B\r", selected: "fix: preserve the second candidate"},
		{name: "cancel existing", keys: "\x1b", existing: true},
		{name: "cancel missing", keys: "\x1b"},
		{name: "visible warning", keys: "\r", selected: "fix: preserve the first candidate", warning: true},
		{name: "quiet warning", keys: "\r", selected: "fix: preserve the first candidate", quiet: true, warning: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			fixture := newWorkflowFixture(t, workflowScenario{subjects: []string{"fix: preserve the first candidate", "fix: preserve the second candidate"}, cleanupError: scenario.warning})
			path := filepath.Join(t.TempDir(), "message.txt")
			const previous = "previous message\n"
			if scenario.existing {
				if err := os.WriteFile(path, []byte(previous), 0644); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--subject-max", "2", "--output", path}
			if scenario.quiet {
				args = append(args, "--quiet")
			}
			terminal := startTestTerminal(t, fixture.command(t, newCLIRepository(t, true), args...))
			terminal.await(t, "fix: preserve the second candidate")
			terminal.send(t, scenario.keys)
			terminal.wait(t)
			data, err := os.ReadFile(path)
			switch {
			case scenario.selected != "":
				if want := scenario.selected + "\n\n" + workflowBody + "\n"; err != nil || string(data) != want {
					t.Fatalf("selected output=%q, error=%v, want %q", data, err, want)
				}
			case scenario.existing:
				if err != nil || string(data) != previous {
					t.Fatalf("cancellation changed output=%q, %v", data, err)
				}
			default:
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("cancellation created output=%q, %v", data, err)
				}
			}
			if scenario.quiet && strings.Contains(terminal.text(), generation.CollectContextStep) {
				t.Errorf("quiet TTY displayed progress:\n%s", terminal.text())
			}
			if scenario.warning {
				prefix := "Cleanup issue:"
				if scenario.quiet {
					prefix = "Warning:"
				}
				visible := strings.Join(strings.Fields(terminal.text()), " ")
				if !strings.Contains(visible, prefix) || !strings.Contains(visible, "fixture cleanup failure") {
					t.Errorf("TTY lost cleanup warning:\n%s", terminal.text())
				}
			}
			fixture.assertCleanup(t)
		})
	}
}

func TestCLIWorkflowClosedTTYStillReportsCleanupWarning(t *testing.T) {
	started := make(chan struct{})
	fixture := newWorkflowFixture(t, workflowScenario{cleanupError: true, promptStarted: started, waitForCancellation: true})
	path := filepath.Join(t.TempDir(), "message.txt")
	const previous = "preserve this existing message\n"
	if err := os.WriteFile(path, []byte(previous), 0644); err != nil {
		t.Fatal(err)
	}
	terminal := startTestTerminal(t, fixture.command(t, newCLIRepository(t, true), "--subject-max", "2", "--output", path))
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatalf("generation did not start:\n%s", terminal.text())
	}
	terminal.send(t, "\x1b")
	terminal.wait(t)
	if !strings.Contains(terminal.text(), "Warning:") || !strings.Contains(terminal.text(), "fixture cleanup failure") {
		t.Errorf("dismissed TTY lost subsequent cleanup warning:\n%s", terminal.text())
	}
	if strings.Contains(terminal.text(), "forced termination required") {
		t.Errorf("dismissal unnecessarily forced the fixture server to stop:\n%s", terminal.text())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != previous {
		t.Errorf("dismissal changed output=%q, %v", data, err)
	}
	fixture.assertCleanup(t)
}

func TestCLIWorkflowTTYAutoSelectsSingleCandidate(t *testing.T) {
	fixture := newWorkflowFixture(t, workflowScenario{subjects: []string{"fix: auto-select the only candidate"}})
	path := filepath.Join(t.TempDir(), "message.txt")
	terminal := startTestTerminal(t, fixture.command(t, newCLIRepository(t, true), "--subject-max", "1", "--output", path))
	terminal.wait(t)
	data, err := os.ReadFile(path)
	const want = "fix: auto-select the only candidate\n\n" + workflowBody + "\n"
	if err != nil || string(data) != want {
		t.Fatalf("automatic selection output=%q, error=%v, want %q", data, err, want)
	}
	fixture.assertCleanup(t)
}

func TestCLIWorkflowQuietTTYErrorShowsPauseInstruction(t *testing.T) {
	for _, pause := range []string{"on", "on-error", "off"} {
		t.Run(pause, func(t *testing.T) {
			fixture := newWorkflowFixture(t, workflowScenario{generationError: true})
			path := filepath.Join(t.TempDir(), "message.txt")
			logPath := filepath.Join(t.TempDir(), "workflow.log")
			const previous = "retain this message after failure\n"
			if err := os.WriteFile(path, []byte(previous), 0644); err != nil {
				t.Fatal(err)
			}
			command := fixture.command(t, newCLIRepository(t, true), "--quiet", "--pause", pause, "--subject-max", "1", "--output", path, "--log-level", "debug", "--log-file", logPath)
			// Do not mistake the race runtime's post-exit sleep for waiting on user input.
			command.Env = append(command.Env, "GORACE=atexit_sleep_ms=0")
			terminal := startTestTerminal(t, command)
			if pause == "off" {
				terminal.waitExitCode(t, 1)
				if strings.Contains(terminal.text(), "Press Enter") {
					t.Errorf("disabled pause displayed an input instruction:\n%s", terminal.text())
				}
			} else {
				terminal.await(t, "Press Enter to exit.")
			}
			fixture.assertCleanup(t)
			logs, err := os.ReadFile(logPath)
			if err != nil || strings.Count(string(logs), `msg="opencode process exited cleanly"`) != 1 {
				t.Fatalf("cleanup did not complete exactly once before presentation: %v\n%s", err, logs)
			}
			if pause != "off" {
				terminal.send(t, "x")
				select {
				case <-terminal.done:
					t.Fatalf("pause finished without Enter: %v\n%s", terminal.err, terminal.text())
				case <-time.After(150 * time.Millisecond):
				}
				terminal.send(t, "\r")
				terminal.waitExitCode(t, 1)
			}
			if strings.Contains(terminal.text(), generation.CollectContextStep) {
				t.Errorf("quiet error displayed hidden progress:\n%s", terminal.text())
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != previous {
				t.Errorf("failed generation changed output=%q, %v", data, err)
			}
		})
	}
}

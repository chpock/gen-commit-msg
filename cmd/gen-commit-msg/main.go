package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"golang.org/x/term"

	"github.com/chpock/gen-commit-msg/internal/agent"
	col "github.com/chpock/gen-commit-msg/internal/color"
	"github.com/chpock/gen-commit-msg/internal/config"
	"github.com/chpock/gen-commit-msg/internal/git"
	"github.com/chpock/gen-commit-msg/internal/logging"
	"github.com/chpock/gen-commit-msg/internal/opencode"
	"github.com/chpock/gen-commit-msg/internal/server"
	"github.com/chpock/gen-commit-msg/internal/tui"
)

var version = "dev"

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		fmtError("Error: %v\n", err)
		os.Exit(2)
	}

	closeLog, err := logging.SetupFromConfig(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		fmtError("Error: failed to configure logging: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = closeLog() }()

	slog.Debug("gen-commit-msg starting", "version", version,
		"subject_min", cfg.SubjectMin, "subject_max", cfg.SubjectMax, "body", cfg.Body,
		"quiet", cfg.Quiet, "agent", cfg.Agent, "log_level", cfg.LogLevel,
		"log_file", cfg.LogFile, "pause", cfg.Pause, "install_agent", cfg.InstallAgent,
		"output", cfg.Output)

	isTTY := isTerminal()
	slog.Debug("terminal check", "is_tty", isTTY)

	pauseExit := func(code int, isError bool) {
		shouldPause := cfg.Pause == "on" || (isError && cfg.Pause == "on-error")
		if shouldPause {
			pauseMsg := "Press Enter to exit..."
			if isError {
				pauseMsg = "An error occurred. Press Enter to exit..."
			}
			pauseWithEnter(isTTY, pauseMsg)
		}
		os.Exit(code)
	}

	if err := cfg.ValidateOutputPath(); err != nil {
		slog.Error("output path validation failed", "error", err)
		fmtError("Error: %v\n", err)
		pauseExit(1, true)
	}

	if cfg.Version {
		fmt.Printf("gen-commit-msg %s\n", version)
		return
	}

	if cfg.Help {
		config.Usage()
		return
	}

	if !git.IsRepo() {
		slog.Error("not a git repository")
		fmtError("Error: not a git repository\n")
		pauseExit(1, true)
	}

	hasStaged, err := git.HasStagedFiles()
	if err != nil {
		slog.Error("failed to check staged files", "error", err)
		fmtError("Error: %v\n", err)
		pauseExit(1, true)
	}
	if !hasStaged {
		slog.Info("no staged files, exiting")
		return
	}

	repoDir, err := os.Getwd()
	if err != nil {
		slog.Error("failed to get current directory", "error", err)
		fmtError("Error: failed to get current directory: %s\n", err.Error())
		pauseExit(1, true)
	}
	slog.Debug("repository directory", "dir", repoDir)
	if !isTTY && cfg.SubjectMax > 1 {
		slog.Error("non-TTY with subject max > 1",
			"subject_max", cfg.SubjectMax, "is_tty", isTTY)
		fmtError("Error: subject range requiring > 1 result needs an interactive terminal. Use --subject-max 1 for non-interactive mode.\n")
		pauseExit(1, true)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, initiating shutdown", "signal", sig.String())
		cancel()
	}()

	if isTTY {
		m := tui.NewModel(int(cfg.SubjectMax), cfg.Quiet)
		tty, closeTTY := openTTY()
		defer closeTTY()
		p := tea.NewProgram(m, tea.WithOutput(tty), tea.WithContext(ctx))

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			var srv *server.ProcessServer
			var oc *opencode.Client
			var sessionID string
			var baseURL string
			var messages []opencode.CommitMessage
			cleanupDone := false
			defer func() {
				if cleanupDone {
					return
				}
				if sessionID != "" && oc != nil {
					delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer delCancel()
					if err := oc.DeleteSession(delCtx, sessionID); err != nil {
						slog.Warn("cleanup: failed to delete session", "session_id", sessionID, "error", err)
					}
				}
				if srv != nil {
					if err := srv.Stop(); err != nil {
						slog.Warn("cleanup: failed to stop server", "error", err)
					}
				}
			}()

			time.Sleep(50 * time.Millisecond)

			// Step 0: Starting OpenCode (agent + server + healthcheck).
			p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepRunning})
			step0OK := true
			if err := agent.Ensure(cfg.Agent, cfg.InstallAgent); err != nil {
				slog.Error("failed to ensure agent", "error", err)
				p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "Agent setup failed", Err: &opencode.AppError{Op: "agent_setup", Message: err.Error(), Err: err}})
				step0OK = false
			}
			if step0OK {
				srv = server.New()
				var startErr error
				baseURL, startErr = srv.Start(ctx)
				if startErr != nil {
					slog.Error("failed to start server", "error", startErr)
					p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "OpenCode server failed to start", Err: &opencode.AppError{Op: "server_start", Message: startErr.Error(), Err: startErr}})
					step0OK = false
				} else {
					slog.Info("opencode server started", "url", baseURL)
					p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepDone})
				}
			}

			// Step 1: Creating session (depends on step 0).
			if !step0OK {
				p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepSkipped})
			} else {
				p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepRunning})
				oc = opencode.NewClient(baseURL, repoDir, cfg.Agent)
				var createErr error
				sessionID, createErr = oc.CreateSession(ctx, cfg.Agent)
				if createErr != nil {
					if errors.Is(createErr, context.Canceled) {
						slog.Debug("session creation cancelled", "agent", cfg.Agent)
					} else {
						slog.Error("failed to create session", "agent", cfg.Agent, "error", createErr)
					}
					var createAppErr *opencode.AppError
					if !errors.As(createErr, &createAppErr) {
						createAppErr = &opencode.AppError{Op: "create_session", Message: createErr.Error(), Err: createErr}
					}
					p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepFailed, Detail: "Failed to create session", Err: createAppErr})
				} else {
					p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepDone})
				}
			}

			// Step 2: Generating commit messages (depends on step 1).
			sessionOK := step0OK && sessionID != ""
			if !sessionOK {
				p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepSkipped})
			} else {
				p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepRunning})
				genParams := opencode.GenerateParams{
					SubjectMin: int(cfg.SubjectMin),
					SubjectMax: int(cfg.SubjectMax),
					Body:       cfg.Body,
				}
				var genErr error
				messages, genErr = oc.GenerateMessages(ctx, sessionID, genParams)
				if genErr != nil {
					if errors.Is(genErr, context.Canceled) {
						slog.Debug("message generation cancelled")
					} else {
						slog.Error("failed to generate messages", "error", genErr)
					}
					var genAppErr *opencode.AppError
					if !errors.As(genErr, &genAppErr) {
						genAppErr = &opencode.AppError{Op: "generate_messages", Message: genErr.Error(), Err: genErr}
					}
					p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepFailed, Detail: "Failed to generate commit messages", Err: genAppErr})
				} else {
					p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepDone})
				}
			}

			// Step 3: Deleting session (depends on step 1 — session must exist).
			if sessionID != "" && oc != nil {
				p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepRunning})
				delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer delCancel()
				if err := oc.DeleteSession(delCtx, sessionID); err != nil {
					slog.Warn("failed to delete session", "session_id", sessionID, "error", err)
					p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepWarning, Detail: "Cleanup issue: " + err.Error()})
				} else {
					slog.Info("session deleted", "id", sessionID)
					p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepDone})
				}
			} else {
				p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepSkipped})
			}

			// Step 4: Stopping OpenCode server (depends on step 0 — srv must exist).
			if srv != nil {
				p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepRunning})
				if err := srv.Stop(); err != nil {
					slog.Warn("failed to stop server", "error", err)
					p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepWarning, Detail: "Cleanup issue: " + err.Error()})
				} else {
					slog.Info("server stopped")
					p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepDone})
				}
			} else {
				p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepSkipped})
			}

			cleanupDone = true

			if ctx.Err() != nil {
				return
			}

			time.Sleep(300 * time.Millisecond)

			items := make([]tui.CommitItem, len(messages))
			for i, msg := range messages {
				items[i] = tui.CommitItem{Subject: msg.Subject, Body: msg.Body}
			}
			p.Send(tui.SetMessages(items))

			p.Send(tui.AllStepsDone())
		}()

		finalModel, err := p.Run()
		cancel()
		slog.Info("initiating graceful shutdown")
		wg.Wait()
		slog.Info("graceful shutdown complete")
		if err != nil {
			slog.Error("TUI initialization failed", "error", err)
			fmtError("Error: TUI initialization failed: %v\n", err)
			closeTTY()
			pauseExit(1, true)
		}

		m = finalModel.(tui.Model)
		if m.Error() != nil {
			slog.Error("TUI ended with error", "error", m.Error())
			closeTTY()
			fmt.Fprint(os.Stderr, m.RenderError())
			if isTTY {
				pauseWithEnter(isTTY, "")
			}
			fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}
		selected := m.SelectedMessage()
		writer, closeWriter := resolveOutputWriter(cfg.Output)
		if writer == nil {
			slog.Error("failed to open output file", "path", cfg.Output)
			fmtError("Error: failed to open output file %q: %v\n", cfg.Output, closeWriter())
			pauseExit(1, true)
		}
		wrote, writeErr := writeSelectedMessage(writer, selected)
		if writeErr != nil {
			slog.Error("failed to write output file", "path", cfg.Output, "error", writeErr)
			fmtError("Error: failed to write output file %q: %v\n", cfg.Output, writeErr)
			_ = closeWriter()
			pauseExit(1, true)
		}
		if err := closeWriter(); err != nil {
			slog.Error("failed to close output file", "path", cfg.Output, "error", err)
			fmtError("Error: failed to write output file %q: %v\n", cfg.Output, err)
			pauseExit(1, true)
		}
		if wrote {
			slog.Info("message selected", "message", truncateString(selected, 80))
		} else {
			slog.Info("selection canceled, no message printed")
		}

		return
	}

	// Ensure agent prompt file exists.
	if err := agent.Ensure(cfg.Agent, cfg.InstallAgent); err != nil {
		slog.Error("failed to ensure agent", "error", err)
		fmtError("Error: failed to ensure agent: %s\n", err.Error())
		pauseExit(1, true)
	}

	srv := server.New()
	baseURL, err := srv.Start(ctx)
	if err != nil {
		slog.Error("failed to start server", "error", err)
		printServerError(err)
		pauseExit(1, true)
	}
	slog.Info("opencode server started", "url", baseURL)

	oc := opencode.NewClient(baseURL, repoDir, cfg.Agent)
	sessionID, err := oc.CreateSession(ctx, cfg.Agent)
	cleanup := func() {
		slog.Debug("cleaning up session and server", "session_id", sessionID)
		delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer delCancel()
		if err := oc.DeleteSession(delCtx, sessionID); err != nil {
			slog.Warn("failed to delete session during cleanup", "session_id", sessionID, "error", err)
		}
		if err := srv.Stop(); err != nil {
			slog.Warn("failed to stop server during cleanup", "error", err)
		}
		slog.Info("server stopped")
	}
	defer cleanup()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Debug("session creation cancelled", "agent", cfg.Agent)
		} else {
			slog.Error("failed to create session", "agent", cfg.Agent, "error", err)
		}
		fmt.Fprintln(os.Stderr, formatOpenCodeError(err))
		cleanup()
		pauseExit(1, true)
	}
	genParams := opencode.GenerateParams{
		SubjectMin: int(cfg.SubjectMin),
		SubjectMax: int(cfg.SubjectMax),
		Body:       cfg.Body,
	}

	if !isTTY && cfg.SubjectMax == 1 {
		slog.Debug("non-interactive mode", "subject_min", cfg.SubjectMin, "subject_max", cfg.SubjectMax)
		messages, err := oc.GenerateMessages(ctx, sessionID, genParams)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Debug("message generation cancelled")
			} else {
				slog.Error("failed to generate messages", "error", err)
			}
			fmt.Fprintln(os.Stderr, formatOpenCodeError(err))
			cleanup()
			pauseExit(1, true)
		}
		if len(messages) > 0 {
			writer, closeWriter := resolveOutputWriter(cfg.Output)
			if writer == nil {
				slog.Error("failed to open output file", "path", cfg.Output)
				fmtError("Error: failed to open output file %q: %v\n", cfg.Output, closeWriter())
				cleanup()
				pauseExit(1, true)
			}
			fmt.Fprintln(writer, formatMessageFromOC(messages[0]))
			if err := closeWriter(); err != nil {
				slog.Error("failed to close output file", "path", cfg.Output, "error", err)
				fmtError("Error: failed to write output file %q: %v\n", cfg.Output, err)
				cleanup()
				pauseExit(1, true)
			}
		}
		pauseExit(0, false)
	}

}

func resolveOutputWriter(path string) (io.WriteCloser, func() error) {
	if path == "" {
		return os.Stdout, func() error { return nil }
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, func() error { return err }
	}
	return f, func() error { return f.Close() }
}

func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

func formatMessageFromOC(msg opencode.CommitMessage) string {
	if msg.Body == "" {
		return strings.TrimSpace(msg.Subject)
	}
	return strings.TrimSpace(msg.Subject) + "\n\n" + strings.TrimSpace(msg.Body)
}

func writeSelectedMessage(out io.Writer, selected string) (bool, error) {
	if strings.TrimSpace(selected) == "" {
		return false, nil
	}
	_, err := fmt.Fprintln(out, selected)
	if err != nil {
		return false, err
	}
	return true, nil
}

func printServerError(err error) {
	isTTY := isatty.IsTerminal(os.Stderr.Fd())
	var msg string
	switch {
	case errors.Is(err, server.ErrOpenCodeNotFound):
		msg = "Error: opencode not found. Is it installed?"
	case errors.Is(err, server.ErrServerTimeout):
		msg = "Error: opencode server failed to start (no response after 30s)"
	case errors.Is(err, server.ErrServerExited):
		msg = "Error: opencode server exited unexpectedly"
	default:
		msg = fmt.Sprintf("Error: %v", err)
	}
	if isTTY {
		msg = col.RedText(msg)
	}
	fmt.Fprintln(os.Stderr, msg)
}

func fmtError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if isatty.IsTerminal(os.Stderr.Fd()) {
		msg = col.RedText(msg)
	}
	fmt.Fprint(os.Stderr, msg)
}

func formatOpenCodeError(err error) string {
	isTTY := isatty.IsTerminal(os.Stderr.Fd())

	var appErr *opencode.AppError
	if errors.As(err, &appErr) {
		if isTTY {
			return appErr.Render()
		}
		return "Error: " + appErr.Error()
	}

	if isTTY {
		return col.Red + "Error: " + col.Reset + err.Error()
	}
	return "Error: " + err.Error()
}

func openTTY() (*os.File, func()) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return os.Stdout, func() {}
	}
	return f, func() { _ = f.Close() }
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func pauseWithEnter(isTTY bool, message string) {
	slog.Debug("pausing before exit", "message", message)
	if message != "" {
		fmt.Fprintf(os.Stderr, "\n%s", message)
	}
	if !isTTY {
		return
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer func() { _ = tty.Close() }()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	var buf [1]byte
	for {
		n, _ := tty.Read(buf[:])
		if n == 0 {
			break
		}
		if buf[0] == '\r' || buf[0] == '\n' || buf[0] == 3 { // 3 = Ctrl+C
			break
		}
	}
}

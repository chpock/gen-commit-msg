package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

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

	if err := logging.SetupFromConfig(cfg.LogLevel, cfg.LogFile); err != nil {
		fmtError("Error: failed to configure logging: %v\n", err)
		os.Exit(2)
	}

	slog.Debug("gen-commit-msg starting", "version", version,
		"subject_count", cfg.SubjectCount, "body", cfg.Body,
		"quiet", cfg.Quiet, "agent", cfg.Agent, "log_level", cfg.LogLevel,
		"log_file", cfg.LogFile, "pause", cfg.Pause, "install_agent", cfg.InstallAgent)

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
	if !isTTY && cfg.SubjectCount > 1 {
		slog.Error("non-TTY with subject count > 1",
			"subject_count", cfg.SubjectCount, "is_tty", isTTY)
		fmtError("Error: --subject-count > 1 requires an interactive terminal. Use --subject-count 1 for non-interactive mode.\n")
		pauseExit(1, true)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Debug("signal received", "signal", sig)
		cancel()
	}()

	if isTTY {
		m := tui.NewModel(int(cfg.SubjectCount), cfg.Quiet)
		tty, closeTTY := openTTY()
		defer closeTTY()
		p := tea.NewProgram(m, tea.WithOutput(tty))

		logPath := logFilePath(cfg.LogFile)
		if logPath != "" {
			p.Send(tui.SetLogPath(logPath))
		}

		go func() {
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
				p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "Agent setup failed: " + err.Error()})
				step0OK = false
			}
			if step0OK {
				srv = server.New()
				var startErr error
				baseURL, startErr = srv.Start(ctx)
				if startErr != nil {
					slog.Error("failed to start server", "error", startErr)
					p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "OpenCode server failed to start: " + startErr.Error()})
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
					slog.Error("failed to create session", "agent", cfg.Agent, "error", createErr)
					p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepFailed, Detail: "Failed to create session: " + createErr.Error()})
				} else {
					slog.Info("session created", "id", sessionID, "agent", cfg.Agent)
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
					SubjectCount: int(cfg.SubjectCount),
					Body:         cfg.Body,
				}
				var genErr error
				messages, genErr = oc.GenerateMessages(ctx, sessionID, genParams)
				if genErr != nil {
					slog.Error("failed to generate messages", "error", genErr)
					p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepFailed, Detail: "Failed to generate commit messages: " + genErr.Error()})
				} else {
					slog.Info("messages generated", "count", len(messages))
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

			time.Sleep(300 * time.Millisecond)

			if len(messages) > 0 {
				items := make([]tui.CommitItem, len(messages))
				for i, msg := range messages {
					items[i] = tui.CommitItem{Subject: msg.Subject, Body: msg.Body}
				}
				p.Send(tui.SetMessages(items))
			}

			p.Send(tui.AllStepsDone())
		}()

		finalModel, err := p.Run()
		if err != nil {
			slog.Error("TUI initialization failed", "error", err)
			fmtError("Error: TUI initialization failed: %v\n", err)
			closeTTY()
			pauseExit(1, true)
		}

		m = finalModel.(tui.Model)
		if m.Error() != nil {
			slog.Error("TUI ended with error", "error", m.Error())
			os.Exit(1)
		}
		selected := m.SelectedMessage()
		slog.Info("message selected", "message", truncateString(selected, 80))
		fmt.Println(selected)

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
		slog.Error("failed to create session", "agent", cfg.Agent, "error", err)
		fmt.Fprintln(os.Stderr, formatOpenCodeError(err))
		cleanup()
		pauseExit(1, true)
	}
	slog.Info("session created", "id", sessionID, "agent", cfg.Agent)

	genParams := opencode.GenerateParams{
		SubjectCount: int(cfg.SubjectCount),
		Body:         cfg.Body,
	}

	if !isTTY && cfg.SubjectCount == 1 {
		slog.Debug("non-interactive mode", "subject_count", cfg.SubjectCount)
		messages, err := oc.GenerateMessages(ctx, sessionID, genParams)
		if err != nil {
			slog.Error("failed to generate messages", "error", err)
			fmt.Fprintln(os.Stderr, formatOpenCodeError(err))
			cleanup()
			pauseExit(1, true)
		}
		slog.Info("messages generated", "count", len(messages))
		if len(messages) > 0 {
			fmt.Println(formatMessageFromOC(messages[0]))
		}
		pauseExit(0, false)
	}

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
	msg := fmt.Sprintf("Error: failed to generate commit message: %v", err)
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		return msg
	}
	return formatErrorColorized(msg)
}

func formatErrorColorized(msg string) string {
	idx := strings.Index(msg, ": ")
	if idx < 0 {
		return col.Red + msg + col.Reset
	}

	label := msg[:idx+2]
	rest := msg[idx+2:]

	var b strings.Builder
	b.WriteString(col.Red)
	b.WriteString(label)
	b.WriteString(col.Reset)

	// If rest contains JSON, colorize it.
	if jsonStart := strings.Index(rest, "\n{") + 1; jsonStart > 0 {
		prefix := rest[:jsonStart]
		jsonPart := rest[jsonStart:]
		b.WriteString(prefix)
		b.WriteString(col.ColorizeJSON(jsonPart))
	} else if jsonStart := strings.Index(rest, "\n["); jsonStart >= 0 {
		prefix := rest[:jsonStart]
		jsonPart := rest[jsonStart:]
		b.WriteString(prefix)
		b.WriteString(col.ColorizeJSON(jsonPart))
	} else {
		b.WriteString(rest)
	}
	return b.String()
}

func openTTY() (*os.File, func()) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return os.Stdout, func() {}
	}
	return f, func() { _ = f.Close() }
}

func logFilePath(logFile string) string {
	if logFile == "" || logFile == "-" {
		return ""
	}
	return logFile
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func pauseWithEnter(isTTY bool, message string) {
	slog.Debug("pausing before exit", "message", message)
	fmt.Fprintf(os.Stderr, "\n%s", message)
	if isTTY {
		tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
		if err != nil {
			return
		}
		defer func() { _ = tty.Close() }()
		var buf [1]byte
		for {
			n, _ := tty.Read(buf[:])
			if n == 0 {
				break
			}
			if buf[0] == '\n' {
				break
			}
		}
	}
	fmt.Fprintln(os.Stderr)
}

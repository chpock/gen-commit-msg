package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"

	col "github.com/chpock/gen-commit-msg/internal/color"
	"github.com/chpock/gen-commit-msg/internal/config"
	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/git"
	"github.com/chpock/gen-commit-msg/internal/logging"
	"github.com/chpock/gen-commit-msg/internal/opencode"
)

var version = "dev"

func main() {
	os.Exit(run())
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

func writeSelectedMessage(path, selected string) (bool, error) {
	if strings.TrimSpace(selected) == "" {
		return false, nil
	}
	writer, closeWriter := resolveOutputWriter(path)
	if writer == nil {
		return false, fmt.Errorf("failed to open output file %q: %w", path, closeWriter())
	}
	_, writeErr := fmt.Fprintln(writer, selected)
	if err := errors.Join(writeErr, closeWriter()); err != nil {
		return false, fmt.Errorf("failed to write output file %q: %w", path, err)
	}
	return true, nil
}

func fmtError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if isatty.IsTerminal(os.Stderr.Fd()) {
		msg = col.RedText(msg)
	}
	fmt.Fprint(os.Stderr, msg)
}

func formatGenerationError(err error) string {
	if isatty.IsTerminal(os.Stderr.Fd()) {
		var rich interface{ Render() string }
		if errors.As(err, &rich) {
			return rich.Render()
		}
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

func run() (exitCode int) {
	cfg, err := config.ParseFlags()
	if err != nil {
		fmtError("Error: %v\n", err)
		return 2
	}
	closeLog, err := logging.SetupFromConfig(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		fmtError("Error: failed to configure logging: %v\n", err)
		return 2
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("application panicked", "panic", recovered, "stack", string(debug.Stack()))
			fmtError("Error: application failed unexpectedly: %v\n", recovered)
			exitCode = 1
		}
		_ = closeLog()
	}()

	slog.Debug("gen-commit-msg starting", "version", version,
		"subject_min", cfg.SubjectMin, "subject_max", cfg.SubjectMax, "body", cfg.Body,
		"quiet", cfg.Quiet, "agent", cfg.Agent, "log_level", cfg.LogLevel,
		"log_file", cfg.LogFile, "pause", cfg.Pause, "install_agent", cfg.InstallAgent,
		"output", cfg.Output)
	isTTY := isTerminal()
	slog.Debug("terminal check", "is_tty", isTTY)
	finish := func(code int, isError bool) int {
		if cfg.Pause == "on" || (isError && cfg.Pause == "on-error") {
			message := "Press Enter to exit..."
			if isError {
				message = "An error occurred. Press Enter to exit..."
			}
			pauseWithEnter(isTTY, message)
		}
		return code
	}
	if err := cfg.ValidateOutputPath(); err != nil {
		slog.Error("output path validation failed", "error", err)
		fmtError("Error: %v\n", err)
		return finish(1, true)
	}
	if cfg.Version {
		fmt.Printf("gen-commit-msg %s\n", version)
		return 0
	}
	if cfg.Help {
		config.Usage()
		return 0
	}
	if !git.IsRepo() {
		slog.Error("not a git repository")
		fmtError("Error: not a git repository\n")
		return finish(1, true)
	}
	hasStaged, err := git.HasStagedFiles()
	if err != nil {
		slog.Error("failed to check staged files", "error", err)
		fmtError("Error: %v\n", err)
		return finish(1, true)
	}
	if !hasStaged {
		slog.Info("no staged files, exiting")
		return 0
	}
	repoDir, err := os.Getwd()
	if err != nil {
		slog.Error("failed to get current directory", "error", err)
		fmtError("Error: failed to get current directory: %s\n", err.Error())
		return finish(1, true)
	}
	slog.Debug("repository directory", "dir", repoDir)
	if !isTTY && cfg.SubjectMax > 1 {
		slog.Error("non-TTY with subject max > 1", "subject_max", cfg.SubjectMax)
		fmtError("Error: subject range requiring > 1 result needs an interactive terminal. Use --subject-max 1 for non-interactive mode.\n")
		return finish(1, true)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case received := <-signals:
			slog.Info("received signal, initiating shutdown", "signal", received.String())
			cancel()
		case <-ctx.Done():
		}
	}()

	deleteTimeout := 5 * time.Second
	if isTTY {
		deleteTimeout = 10 * time.Second
	}
	backend := opencode.NewGenerator(opencode.GeneratorConfig{
		Executable: "opencode", RepoDir: repoDir, Agent: cfg.Agent,
		InstallAgent: cfg.InstallAgent, SessionDeleteTimeout: deleteTimeout,
	})
	request := generation.Request{SubjectMin: int(cfg.SubjectMin), SubjectMax: int(cfg.SubjectMax), Body: cfg.Body}
	var selected string
	if isTTY {
		model, err := runInteractiveGeneration(ctx, backend, request, cfg.Quiet)
		if err != nil {
			slog.Error("TUI failed", "error", err)
			fmtError("Error: TUI failed: %v\n", err)
			return finish(1, true)
		}
		if model.Error() != nil {
			slog.Error("TUI ended with error", "error", model.Error())
			if cfg.Quiet {
				fmt.Fprintln(os.Stderr, formatGenerationError(model.Error()))
			} else {
				fmt.Fprint(os.Stderr, model.RenderError())
			}
			if cfg.Pause == "on" || cfg.Pause == "on-error" {
				message := ""
				if cfg.Quiet {
					message = "Press Enter to exit."
				}
				pauseWithEnter(isTTY, message)
			}
			fmt.Fprintln(os.Stderr)
			return 1
		}
		selected = model.SelectedMessage()
		if ctx.Err() != nil {
			selected = ""
		}
	} else {
		slog.Debug("non-interactive mode", "subject_min", cfg.SubjectMin, "subject_max", cfg.SubjectMax)
		result, err := generateMessages(ctx, backend, request, nil)
		writeWarnings(os.Stderr, result.Warnings)
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, formatGenerationError(err))
			return finish(1, true)
		}
		selected = result.Messages[0].Format()
	}

	wrote, err := writeSelectedMessage(cfg.Output, selected)
	if err != nil {
		slog.Error("failed to write selected message", "path", cfg.Output, "error", err)
		fmtError("Error: %v\n", err)
		return finish(1, true)
	}
	if wrote {
		slog.Info("message selected", "message", truncateString(selected, 80))
	} else {
		slog.Info("selection canceled, no message printed")
	}
	return 0
}

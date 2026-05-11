package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"github.com/chpock/gen-commit-msg/internal/agent"
	"github.com/chpock/gen-commit-msg/internal/config"
	"github.com/chpock/gen-commit-msg/internal/git"
	"github.com/chpock/gen-commit-msg/internal/opencode"
	"github.com/chpock/gen-commit-msg/internal/server"
	"github.com/chpock/gen-commit-msg/internal/tui"
)

var version = "dev"

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}

	hasStaged, err := git.HasStagedFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !hasStaged {
		return
	}

	if err := agent.Ensure(cfg.Agent, cfg.InstallAgent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	isTTY := isTerminal()
	if !isTTY && cfg.SubjectCount > 1 {
		fmt.Fprintln(os.Stderr, "Error: --subject-count > 1 requires an interactive terminal. Use --subject-count 1 for non-interactive mode.")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	srv := server.New()
	baseURL, err := srv.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer srv.Stop()

	oc := opencode.NewClient(baseURL)
	sessionID, err := oc.CreateSession(ctx, cfg.Agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer delCancel()
		oc.DeleteSession(delCtx, sessionID)
	}()

	if !isTTY && cfg.SubjectCount == 1 {
		messages, err := oc.GenerateMessages(ctx, sessionID, opencode.GenerateParams{
			SubjectCount: int(cfg.SubjectCount),
			Body:         cfg.Body,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(messages) > 0 {
			fmt.Println(formatMessageFromOC(messages[0]))
		}
		return
	}

	m := tui.NewModel(int(cfg.SubjectCount))
	p := tea.NewProgram(m)

	go func() {
		messages, err := oc.GenerateMessages(ctx, sessionID, opencode.GenerateParams{
			SubjectCount: int(cfg.SubjectCount),
			Body:         cfg.Body,
		})
		if err != nil {
			p.Send(tui.SetError(err))
			return
		}
		items := make([]tui.CommitItem, len(messages))
		for i, msg := range messages {
			items[i] = tui.CommitItem{Subject: msg.Subject, Body: msg.Body}
		}
		p.Send(tui.SetMessages(items))
	}()

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m = finalModel.(tui.Model)
	fmt.Println(m.SelectedMessage())

	if cfg.Pause == "on" || (cfg.Pause == "on-error" && m.Error() != nil) {
		fmt.Fprintf(os.Stderr, "\nPress any key to exit...")
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

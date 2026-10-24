package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/tui"
)

func runInteractiveGeneration(parent context.Context, backend generation.Generator, request generation.Request, quiet bool) (tui.Model, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	labels := append([]string{generation.CollectContextStep}, backend.Steps()...)
	model := tui.NewModel(request.SubjectMax, quiet, labels, cancel)
	tty, closeTTY := openTTY()
	defer closeTTY()
	program := tea.NewProgram(model, tea.WithOutput(tty), tea.WithContext(context.Background()))

	var result generation.Result
	var operation sync.WaitGroup
	operation.Add(1)
	go func() {
		defer operation.Done()
		waitForProgress(ctx, 50*time.Millisecond)
		var err error
		result, err = generateMessages(ctx, backend, request, func(update generation.Update) { program.Send(update) })
		if ctx.Err() != nil {
			program.Send(tui.AllStepsDone())
			return
		}
		if err != nil {
			program.Send(tui.SetError(err))
			program.Send(tui.AllStepsDone())
			return
		}
		if waitForProgress(ctx, 300*time.Millisecond) {
			items := make([]tui.CommitItem, len(result.Messages))
			for index, message := range result.Messages {
				items[index] = tui.CommitItem{Subject: message.Subject, Body: message.Body}
			}
			program.Send(tui.SetMessages(items))
		}
		program.Send(tui.AllStepsDone())
	}()

	finalModel, err := program.Run()
	cancel()
	slog.Info("waiting for generation cleanup")
	operation.Wait()
	slog.Info("generation cleanup complete")
	if err != nil {
		writeWarnings(os.Stderr, result.Warnings)
		return model, err
	}
	model = finalModel.(tui.Model)
	if quiet || model.SelectedMessage() == "" {
		writeWarnings(os.Stderr, result.Warnings)
	}
	return model, nil
}

func waitForProgress(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

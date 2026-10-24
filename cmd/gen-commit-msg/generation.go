package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"

	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/git"
)

func generateMessages(ctx context.Context, backend generation.Generator, request generation.Request, report generation.Reporter) (result generation.Result, err error) {
	var reportedWarnings []error
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("generation failed unexpectedly: %v", recovered)
			// A panic can prevent the backend from returning its cleanup diagnostics.
			result.Warnings = append(result.Warnings, reportedWarnings...)
			slog.Error("generation panicked", "panic", recovered, "stack", string(debug.Stack()))
		}
		if err != nil {
			result.Messages = nil
		}
	}()

	report.Emit(generation.Update{Index: 0, Status: generation.StepRunning})
	err = ctx.Err()
	if err == nil {
		var snapshot git.CommitMessageContext
		snapshot, err = git.CollectCommitMessageContext(ctx)
		if err == nil {
			request.ContextJSON, err = snapshot.Marshal()
		}
	}
	if err != nil {
		status := generation.StepFailed
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
			status = generation.StepInterrupted
		}
		err = fmt.Errorf("failed to collect information about current changes: %w", err)
		slog.Error("failed to prepare commit message context", "error", err)
		report.Emit(generation.Update{Index: 0, Status: status, Err: err})
		for index := range backend.Steps() {
			report.Emit(generation.Update{Index: index + 1, Status: generation.StepSkipped})
		}
		return result, err
	}
	report.Emit(generation.Update{Index: 0, Status: generation.StepDone})

	result, err = backend.Generate(ctx, request, func(update generation.Update) {
		if update.Status == generation.StepWarning && update.Err != nil {
			reportedWarnings = append(reportedWarnings, update.Err)
		}
		update.Index++
		report.Emit(update)
	})
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && len(result.Messages) == 0 {
		err = errors.New("no commit messages generated")
	}
	if err != nil {
		slog.Error("commit message generation failed", "error", err)
	}
	return result, err
}

func writeWarnings(destination io.Writer, warnings []error) {
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(destination, "Warning: %v\n", warning); err != nil {
			slog.Error("failed to report cleanup warning", "error", err)
		}
	}
}

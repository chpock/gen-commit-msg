package opencode

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/chpock/gen-commit-msg/internal/agent"
	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/server"
)

type GeneratorConfig struct {
	Executable           string
	RepoDir              string
	Agent                string
	InstallAgent         string
	SessionDeleteTimeout time.Duration
}

type Generator struct {
	config      GeneratorConfig
	ensureAgent func(string, string) error
	newServer   func(string) server.Server
	newClient   func(string, string, string) *Client
}

func NewGenerator(config GeneratorConfig) *Generator {
	return &Generator{
		config:      config,
		ensureAgent: agent.Ensure,
		newServer:   func(executable string) server.Server { return server.New(executable) },
		newClient:   NewClient,
	}
}

func (g *Generator) Steps() []string {
	return []string{
		"Starting OpenCode...",
		"Creating session...",
		"Generating commit messages...",
		"Cleaning up OpenCode resources...",
	}
}

func (g *Generator) Generate(ctx context.Context, request generation.Request, report generation.Reporter) (result generation.Result, err error) {
	slog.Info("OpenCode generation starting", "executable", g.config.Executable, "repo", g.config.RepoDir,
		"agent", g.config.Agent, "install_agent", g.config.InstallAgent)
	var srv server.Server
	var client *Client
	var sessionID string
	step := 0
	defer func() {
		for i := step + 1; i < 3; i++ {
			report.Emit(generation.Update{Index: i, Status: generation.StepSkipped})
		}
		result.Warnings = g.cleanup(srv, client, sessionID, report)
		// Cancellation during independent cleanup still suppresses presentation.
		if ctx.Err() != nil && err == nil {
			err = ctx.Err()
		}
		if err != nil {
			result.Messages = nil
			slog.Error("OpenCode generation failed", "error", err)
		} else {
			slog.Info("OpenCode generation completed", "message_count", len(result.Messages), "warning_count", len(result.Warnings))
		}
	}()

	report.Emit(generation.Update{Index: step, Status: generation.StepRunning})
	var baseURL string
	srv, baseURL, err = g.start(ctx)
	if err = finishStep(ctx, report, step, err); err != nil {
		return result, err
	}

	step = 1
	report.Emit(generation.Update{Index: step, Status: generation.StepRunning})
	if err = ctx.Err(); err == nil {
		client = g.newClient(baseURL, g.config.RepoDir, g.config.Agent)
		sessionID, err = client.CreateSession(ctx, g.config.Agent)
	}
	if err = finishStep(ctx, report, step, err); err != nil {
		return result, err
	}

	step = 2
	report.Emit(generation.Update{Index: step, Status: generation.StepRunning})
	if err = ctx.Err(); err == nil {
		result.Messages, err = client.GenerateMessages(ctx, sessionID, request)
	}
	err = finishStep(ctx, report, step, err)
	return result, err
}

func (g *Generator) start(ctx context.Context) (server.Server, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if err := g.ensureAgent(g.config.Agent, g.config.InstallAgent); err != nil {
		return nil, "", &AppError{Op: "agent_setup", Message: "failed to ensure agent", Err: err}
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	srv := g.newServer(g.config.Executable)
	baseURL, err := srv.Start(ctx)
	if err != nil {
		return srv, "", &AppError{Op: "server_start", Message: "failed to start OpenCode", Err: err}
	}
	return srv, baseURL, nil
}

func finishStep(ctx context.Context, report generation.Reporter, index int, err error) error {
	if err == nil {
		err = ctx.Err()
	}
	status := generation.StepDone
	if err != nil {
		status = generation.StepFailed
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = generation.StepInterrupted
		}
		slog.Error("OpenCode step failed", "step", index, "error", err)
	}
	report.Emit(generation.Update{Index: index, Status: status, Err: err})
	return err
}

func (g *Generator) cleanup(srv server.Server, client *Client, sessionID string, report generation.Reporter) []error {
	if srv == nil {
		report.Emit(generation.Update{Index: 3, Status: generation.StepSkipped})
		return nil
	}
	report.Emit(generation.Update{Index: 3, Status: generation.StepRunning})
	var warnings []error
	if sessionID != "" {
		// Session deletion must remain possible after the generation context ends.
		ctx, cancel := context.WithTimeout(context.Background(), g.config.SessionDeleteTimeout)
		if err := client.DeleteSession(ctx, sessionID); err != nil {
			warnings = append(warnings, fmt.Errorf("delete OpenCode session: %w", err))
		}
		cancel()
	}
	// Start may already have stopped a partially started server; Stop is idempotent
	// and returns that cleanup diagnostic again without repeating process cleanup.
	if err := srv.Stop(); err != nil {
		warnings = append(warnings, fmt.Errorf("stop OpenCode server: %w", err))
	}
	if len(warnings) != 0 {
		err := errors.Join(warnings...)
		slog.Warn("OpenCode cleanup produced warnings", "error", err)
		report.Emit(generation.Update{Index: 3, Status: generation.StepWarning, Err: err})
	} else {
		slog.Info("OpenCode resources cleaned up")
		report.Emit(generation.Update{Index: 3, Status: generation.StepDone})
	}
	return warnings
}

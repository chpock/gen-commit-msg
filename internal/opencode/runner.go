package opencode

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

type AgentEnsurer func(agent string, install bool) error

type ServerStarter func(ctx context.Context) (baseURL string, err error)

type ServerStopper func() error

type ClientFactory func(baseURL, repoDir, agent string) *Client

type PipelineRunner struct {
	EnsureAgent  AgentEnsurer
	StartServer  ServerStarter
	StopServer   ServerStopper
	NewClient    ClientFactory
	RepoDir      string
	Agent        string
	InstallAgent bool
}

type PipelineResult struct {
	SessionID string
	Messages  []CommitMessage
	Error     error
}

func (r *PipelineRunner) Run(ctx context.Context, genParams GenerateParams) PipelineResult {
	slog.Debug("pipeline starting", "agent", r.Agent, "repo", r.RepoDir)

	if err := r.EnsureAgent(r.Agent, r.InstallAgent); err != nil {
		slog.Error("pipeline: agent setup failed", "error", err)
		return PipelineResult{Error: &AppError{Op: "agent_setup", Message: err.Error(), Err: err}}
	}

	baseURL, err := r.StartServer(ctx)
	if err != nil {
		slog.Error("pipeline: server start failed", "error", err)
		return PipelineResult{Error: &AppError{Op: "server_start", Message: err.Error(), Err: err}}
	}

	oc := r.NewClient(baseURL, r.RepoDir, r.Agent)
	sessionID, err := oc.CreateSession(ctx, r.Agent)
	if err != nil {
		slog.Error("pipeline: session creation failed", "error", err)
		_ = r.StopServer()
		return PipelineResult{Error: err}
	}

	messages, err := oc.GenerateMessages(ctx, sessionID, genParams)
	if err != nil {
		slog.Error("pipeline: message generation failed", "error", err)
		delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer delCancel()
		_ = oc.DeleteSession(delCtx, sessionID)
		_ = r.StopServer()
		return PipelineResult{SessionID: sessionID, Error: err}
	}

	delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer delCancel()
	_ = oc.DeleteSession(delCtx, sessionID)

	if err := r.StopServer(); err != nil {
		slog.Warn("pipeline: server stop produced warning", "error", err)
	}

	slog.Info("pipeline completed", "session_id", sessionID, "message_count", len(messages))
	return PipelineResult{SessionID: sessionID, Messages: messages}
}

func (r *PipelineRunner) Stop() {
	if err := r.StopServer(); err != nil {
		slog.Warn("pipeline: cleanup stop failed", "error", err)
	}
}

func formatMessageFromOC(msg CommitMessage) string {
	if msg.Body == "" {
		return strings.TrimSpace(msg.Subject)
	}
	return strings.TrimSpace(msg.Subject) + "\n\n" + strings.TrimSpace(msg.Body)
}

func FormatMessages(messages []CommitMessage) []string {
	result := make([]string, len(messages))
	for i, msg := range messages {
		result[i] = formatMessageFromOC(msg)
	}
	return result
}

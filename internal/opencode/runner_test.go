package opencode

import (
	"context"
	"errors"
	"testing"
)

func TestPipelineRunner_Success(t *testing.T) {
	mockSess := newMockSession()
	svrBaseURL := "http://127.0.0.1:9999"

	runner := &PipelineRunner{
		RepoDir:      "/tmp/repo",
		Agent:        "test-agent",
		InstallAgent: false,
		EnsureAgent: func(agent string, install bool) error {
			if agent != "test-agent" {
				t.Errorf("unexpected agent: %q", agent)
			}
			return nil
		},
		StartServer: func(ctx context.Context) (string, error) {
			return svrBaseURL, nil
		},
		StopServer: func() error {
			return nil
		},
		NewClient: func(baseURL, repoDir, agent string) *Client {
			if baseURL != svrBaseURL {
				t.Errorf("unexpected baseURL: %q", baseURL)
			}
			return newClientWithSession(mockSess, repoDir, agent)
		},
	}

	result := runner.Run(context.Background(), GenerateParams{
		SubjectMin: 1,
		SubjectMax: 3,
		Body:       true,
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.SessionID != "mock_sess_001" {
		t.Errorf("expected session ID 'mock_sess_001', got %q", result.SessionID)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Subject != "feat(config): add --subject-count flag" {
		t.Errorf("unexpected subject: %q", result.Messages[0].Subject)
	}
}

func TestPipelineRunner_AgentSetupError(t *testing.T) {
	agentErr := errors.New("agent file not found")

	runner := &PipelineRunner{
		RepoDir:      "/tmp/repo",
		Agent:        "test-agent",
		InstallAgent: false,
		EnsureAgent: func(agent string, install bool) error {
			return agentErr
		},
		StartServer: func(ctx context.Context) (string, error) {
			t.Error("StartServer should not be called")
			return "", nil
		},
		StopServer: func() error { return nil },
		NewClient: func(baseURL, repoDir, agent string) *Client {
			t.Error("NewClient should not be called")
			return nil
		},
	}

	result := runner.Run(context.Background(), GenerateParams{SubjectMin: 1, SubjectMax: 1})
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(result.Error, &appErr) {
		t.Fatalf("expected *AppError, got %T", result.Error)
	}
	if appErr.Op != "agent_setup" {
		t.Errorf("expected op 'agent_setup', got %q", appErr.Op)
	}
}

func TestPipelineRunner_ServerStartError(t *testing.T) {
	svrErr := errors.New("opencode not found")

	runner := &PipelineRunner{
		RepoDir:      "/tmp/repo",
		Agent:        "test-agent",
		InstallAgent: false,
		EnsureAgent: func(agent string, install bool) error {
			return nil
		},
		StartServer: func(ctx context.Context) (string, error) {
			return "", svrErr
		},
		StopServer: func() error { return nil },
		NewClient: func(baseURL, repoDir, agent string) *Client {
			t.Error("NewClient should not be called")
			return nil
		},
	}

	result := runner.Run(context.Background(), GenerateParams{SubjectMin: 1, SubjectMax: 1})
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(result.Error, &appErr) {
		t.Fatalf("expected *AppError, got %T", result.Error)
	}
	if appErr.Op != "server_start" {
		t.Errorf("expected op 'server_start', got %q", appErr.Op)
	}
}

func TestPipelineRunner_SessionCreateError(t *testing.T) {
	mockSess := newMockSession().WithNewError()
	svrBaseURL := "http://127.0.0.1:9999"

	stopCalled := false
	runner := &PipelineRunner{
		RepoDir:      "/tmp/repo",
		Agent:        "test-agent",
		InstallAgent: false,
		EnsureAgent:  func(agent string, install bool) error { return nil },
		StartServer:  func(ctx context.Context) (string, error) { return svrBaseURL, nil },
		StopServer: func() error {
			stopCalled = true
			return nil
		},
		NewClient: func(baseURL, repoDir, agent string) *Client {
			return newClientWithSession(mockSess, repoDir, agent)
		},
	}

	result := runner.Run(context.Background(), GenerateParams{SubjectMin: 1, SubjectMax: 1})
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if !stopCalled {
		t.Error("expected StopServer to be called for cleanup")
	}
}

func TestPipelineRunner_GenerateError(t *testing.T) {
	mockSess := newMockSession().WithPromptFixture("prompt_error_no_structured.json")
	svrBaseURL := "http://127.0.0.1:9999"

	stopCalled := false
	runner := &PipelineRunner{
		RepoDir:      "/tmp/repo",
		Agent:        "test-agent",
		InstallAgent: false,
		EnsureAgent:  func(agent string, install bool) error { return nil },
		StartServer:  func(ctx context.Context) (string, error) { return svrBaseURL, nil },
		StopServer: func() error {
			stopCalled = true
			return nil
		},
		NewClient: func(baseURL, repoDir, agent string) *Client {
			return newClientWithSession(mockSess, repoDir, agent)
		},
	}

	result := runner.Run(context.Background(), GenerateParams{SubjectMin: 1, SubjectMax: 1})
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.SessionID != "mock_sess_001" {
		t.Errorf("expected session ID 'mock_sess_001', got %q", result.SessionID)
	}
	if !stopCalled {
		t.Error("expected StopServer to be called for cleanup")
	}
	if !mockSess.DeleteCalled() {
		t.Error("expected DeleteSession to be called for cleanup")
	}
	var appErr *AppError
	if !errors.As(result.Error, &appErr) {
		t.Fatalf("expected *AppError, got %T", result.Error)
	}
	if appErr.OC == nil || appErr.OC.Kind != OCErrNoStructuredOutput {
		t.Errorf("expected no_structured_output error, got %v", appErr.OC)
	}
}

func TestPipelineRunner_StopAfterSuccess(t *testing.T) {
	mockSess := newMockSession()
	stopCalled := false

	runner := &PipelineRunner{
		RepoDir:      "/tmp/repo",
		Agent:        "test-agent",
		InstallAgent: false,
		EnsureAgent:  func(agent string, install bool) error { return nil },
		StartServer: func(ctx context.Context) (string, error) {
			return "http://127.0.0.1:9999", nil
		},
		StopServer: func() error {
			stopCalled = true
			return nil
		},
		NewClient: func(baseURL, repoDir, agent string) *Client {
			return newClientWithSession(mockSess, repoDir, agent)
		},
	}

	result := runner.Run(context.Background(), GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !stopCalled {
		t.Error("expected StopServer to be called")
	}
}

func TestFormatMessages(t *testing.T) {
	messages := []CommitMessage{
		{Subject: "feat: add feature", Body: "Added a new feature."},
		{Subject: "fix: bug", Body: ""},
	}
	formatted := FormatMessages(messages)
	if len(formatted) != 2 {
		t.Fatalf("expected 2 formatted messages, got %d", len(formatted))
	}
	if formatted[0] != "feat: add feature\n\nAdded a new feature." {
		t.Errorf("unexpected formatted[0]: %q", formatted[0])
	}
	if formatted[1] != "fix: bug" {
		t.Errorf("unexpected formatted[1]: %q", formatted[1])
	}
}

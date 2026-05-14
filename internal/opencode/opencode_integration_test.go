package opencode

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClient_CreateSession_Success(t *testing.T) {
	mock := newMockSession()
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	sessionID, err := client.CreateSession(context.Background(), "test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != "mock_sess_001" {
		t.Errorf("expected session ID 'mock_sess_001', got %q", sessionID)
	}
}

func TestClient_CreateSession_Error(t *testing.T) {
	mock := newMockSession().WithNewError()
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.CreateSession(context.Background(), "test-agent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.Op != "create_session" {
		t.Errorf("expected op 'create_session', got %q", appErr.Op)
	}
}

func TestClient_GenerateMessages_Success_Basic(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_success_basic.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	msgs, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 3,
		Body:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Subject != "feat(config): add --subject-count flag" {
		t.Errorf("unexpected subject: %q", msgs[0].Subject)
	}
	if !strings.Contains(msgs[0].Body, "Added a --subject-count flag") {
		t.Errorf("unexpected body: %q", msgs[0].Body)
	}
}

func TestClient_GenerateMessages_Success_NoBody(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_success_no_body.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	msgs, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
		Body:       false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Subject != "fix(server): handle opencode process exit" {
		t.Errorf("unexpected subject: %q", msgs[0].Subject)
	}
	if msgs[0].Body != "" {
		t.Errorf("expected empty body, got %q", msgs[0].Body)
	}
}

func TestClient_GenerateMessages_Success_ManySubjects(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_success_many_subjects.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	msgs, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 5,
		Body:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}
	expectedSubjects := []string{
		"feat(tui): add spinner animation",
		"fix(config): validate subject count bounds",
		"refactor(server): simplify healthcheck",
		"docs(readme): add installation guide",
		"chore(deps): bump bubbletea to v1.3.10",
	}
	for i, exp := range expectedSubjects {
		if msgs[i].Subject != exp {
			t.Errorf("message %d: expected subject %q, got %q", i, exp, msgs[i].Subject)
		}
	}
	for i, msg := range msgs {
		if msg.Body != "Various improvements and additions across the codebase." {
			t.Errorf("message %d: unexpected body: %q", i, msg.Body)
		}
	}
}

func TestClient_GenerateMessages_Success_StructuredOutput(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_success_structured_output.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	msgs, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
		Body:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Subject != "feat(api): add endpoint" {
		t.Errorf("unexpected subject: %q", msgs[0].Subject)
	}
}

func TestClient_GenerateMessages_APIError(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_api.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 3,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.Op != "generate_messages" {
		t.Errorf("expected op 'generate_messages', got %q", appErr.Op)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field to be populated")
	}
	if appErr.OC.Kind != OCErrAPI {
		t.Errorf("expected Kind=OCErrAPI, got %v", appErr.OC.Kind)
	}
	if appErr.OC.Code != "APIError" {
		t.Errorf("expected Code=APIError, got %q", appErr.OC.Code)
	}
	if !strings.Contains(appErr.OC.Message, "deepseek-reasoner does not support") {
		t.Errorf("unexpected message: %q", appErr.OC.Message)
	}
	if appErr.OC.Status != 400 {
		t.Errorf("expected Status=400, got %d", appErr.OC.Status)
	}
}

func TestClient_GenerateMessages_ProviderAuthError(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_provider_auth.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field")
	}
	if appErr.OC.Code != "ProviderAuthError" {
		t.Errorf("expected Code=ProviderAuthError, got %q", appErr.OC.Code)
	}
	if !strings.Contains(appErr.OC.Message, "invalid API key") {
		t.Errorf("unexpected message: %q", appErr.OC.Message)
	}
}

func TestClient_GenerateMessages_MessageAbortedError(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_message_aborted.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field")
	}
	if appErr.OC.Code != "MessageAbortedError" {
		t.Errorf("expected Code=MessageAbortedError, got %q", appErr.OC.Code)
	}
}

func TestClient_GenerateMessages_UnknownError(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_unknown.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field")
	}
	if appErr.OC.Code != "UnknownError" {
		t.Errorf("expected Code=UnknownError, got %q", appErr.OC.Code)
	}
}

func TestClient_GenerateMessages_NoStructuredOutput(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_no_structured.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field")
	}
	if appErr.OC.Kind != OCErrNoStructuredOutput {
		t.Errorf("expected Kind=OCErrNoStructuredOutput, got %v", appErr.OC.Kind)
	}
	if appErr.OC.Code != "no_structured_output" {
		t.Errorf("expected Code=no_structured_output, got %q", appErr.OC.Code)
	}
}

func TestClient_GenerateMessages_EmptyStructured(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_empty_structured.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field")
	}
	if appErr.OC.Kind != OCErrNoStructuredOutput {
		t.Errorf("expected Kind=OCErrNoStructuredOutput, got %v", appErr.OC.Kind)
	}
}

func TestClient_GenerateMessages_NullStructured(t *testing.T) {
	mock := newMockSession().WithPromptFixture("prompt_error_null_structured.json")
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC == nil {
		t.Fatal("expected OC field")
	}
	if appErr.OC.Kind != OCErrNoStructuredOutput {
		t.Errorf("expected Kind=OCErrNoStructuredOutput, got %v", appErr.OC.Kind)
	}
}

func TestClient_GenerateMessages_TransportError(t *testing.T) {
	mock := newMockSession().WithPromptError()
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	_, err := client.GenerateMessages(context.Background(), "sess_001", GenerateParams{
		SubjectMin: 1,
		SubjectMax: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.OC != nil {
		t.Errorf("expected nil OC for non-HTTP transport error, got %v", appErr.OC)
	}
	if appErr.Op != "generate_messages" {
		t.Errorf("expected op 'generate_messages', got %q", appErr.Op)
	}
}

func TestClient_DeleteSession_Success(t *testing.T) {
	mock := newMockSession()
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	err := client.DeleteSession(context.Background(), "sess_001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteSession_Error(t *testing.T) {
	mock := newMockSession().WithDeleteError()
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	err := client.DeleteSession(context.Background(), "sess_001")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errMockDeleteSession) {
		t.Errorf("expected errMockDeleteSession, got %v", err)
	}
}

func TestClient_FullLifecycle(t *testing.T) {
	mock := newMockSession()
	client := newClientWithSession(mock, "/tmp/repo", "test-agent")

	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, "test-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	msgs, err := client.GenerateMessages(ctx, sessionID, GenerateParams{
		SubjectMin: 1,
		SubjectMax: 3,
		Body:       true,
	})
	if err != nil {
		t.Fatalf("generate messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}

	err = client.DeleteSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("delete session: %v", err)
	}
}

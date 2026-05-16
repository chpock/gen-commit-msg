package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	opencode "github.com/sst/opencode-sdk-go"
)

func TestGetStructuredJSON_nil(t *testing.T) {
	_, err := getStructuredJSON(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

func TestGetStructuredJSON_structured(t *testing.T) {
	raw := `{
		"info": {
			"id": "msg_1",
			"cost": 0.01,
			"mode": "chat",
			"modelID": "claude-3",
			"parentID": "parent_1",
			"path": {"cwd": "/tmp", "root": "/tmp"},
			"providerID": "anthropic",
			"role": "assistant",
			"sessionID": "sess_1",
			"system": [],
			"time": {"created": 1000, "completed": 2000},
			"tokens": {"input": 100, "output": 50},
			"structured": "{\"subjects\":[\"feat: add thing\"],\"body\":\"details\"}"
		},
		"parts": []
	}`
	var res opencode.SessionPromptResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	got, err := getStructuredJSON(&res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct {
		Subjects []string `json:"subjects"`
		Body     string   `json:"body"`
	}
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Subjects) != 1 || result.Subjects[0] != "feat: add thing" {
		t.Errorf("unexpected subjects: %v", result.Subjects)
	}
	if result.Body != "details" {
		t.Errorf("unexpected body: %q", result.Body)
	}
}

func TestGetStructuredJSON_structured_output(t *testing.T) {
	raw := `{
		"info": {
			"id": "msg_2",
			"cost": 0.02,
			"mode": "chat",
			"modelID": "gpt-4",
			"parentID": "parent_2",
			"path": {"cwd": "/app", "root": "/app"},
			"providerID": "openai",
			"role": "assistant",
			"sessionID": "sess_2",
			"system": [],
			"time": {"created": 2000, "completed": 3000},
			"tokens": {"input": 200, "output": 100},
			"structured_output": "{\"subjects\":[\"fix: bug\"],\"body\":\"fixed\"}"
		},
		"parts": []
	}`
	var res opencode.SessionPromptResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	got, err := getStructuredJSON(&res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct {
		Subjects []string `json:"subjects"`
		Body     string   `json:"body"`
	}
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Subjects) != 1 || result.Subjects[0] != "fix: bug" {
		t.Errorf("unexpected subjects: %v", result.Subjects)
	}
	if result.Body != "fixed" {
		t.Errorf("unexpected body: %q", result.Body)
	}
}

func TestGetStructuredJSON_missing(t *testing.T) {
	raw := `{
		"info": {
			"id": "msg_3",
			"cost": 0.03,
			"mode": "chat",
			"modelID": "gpt-4",
			"parentID": "parent_3",
			"path": {"cwd": "/app", "root": "/app"},
			"providerID": "openai",
			"role": "assistant",
			"sessionID": "sess_3",
			"system": [],
			"time": {"created": 3000, "completed": 4000},
			"tokens": {"input": 300, "output": 200}
		},
		"parts": []
	}`
	var res opencode.SessionPromptResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	_, err := getStructuredJSON(&res)
	if err == nil {
		t.Fatal("expected error for missing structured output")
	}
	var noStr *noStructuredOutputError
	if !errors.As(err, &noStr) {
		t.Fatalf("expected *noStructuredOutputError, got: %T", err)
	}
	if !strings.Contains(noStr.RawJSON, "sess_3") {
		t.Errorf("RawJSON should contain session ID, got: %s", noStr.RawJSON)
	}
}

func TestGetStructuredJSON_empty(t *testing.T) {
	raw := `{
		"info": {
			"id": "msg_4",
			"cost": 0.04,
			"mode": "chat",
			"modelID": "gpt-4",
			"parentID": "parent_4",
			"path": {"cwd": "/app", "root": "/app"},
			"providerID": "openai",
			"role": "assistant",
			"sessionID": "sess_4",
			"system": [],
			"time": {"created": 4000, "completed": 5000},
			"tokens": {"input": 400, "output": 300},
			"structured": ""
		},
		"parts": []
	}`
	var res opencode.SessionPromptResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	_, err := getStructuredJSON(&res)
	if err == nil {
		t.Fatal("expected error for empty structured output")
	}
	var noStr *noStructuredOutputError
	if !errors.As(err, &noStr) {
		t.Fatalf("expected *noStructuredOutputError, got: %T", err)
	}
	if !strings.Contains(noStr.RawJSON, "sess_4") {
		t.Errorf("RawJSON should contain session ID, got: %s", noStr.RawJSON)
	}
}

func TestGetStructuredJSON_null(t *testing.T) {
	raw := `{
		"info": {
			"id": "msg_5",
			"cost": 0.05,
			"mode": "chat",
			"modelID": "gpt-4",
			"parentID": "parent_5",
			"path": {"cwd": "/app", "root": "/app"},
			"providerID": "openai",
			"role": "assistant",
			"sessionID": "sess_5",
			"system": [],
			"time": {"created": 5000, "completed": 6000},
			"tokens": {"input": 500, "output": 400},
			"structured": null
		},
		"parts": []
	}`
	var res opencode.SessionPromptResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	_, err := getStructuredJSON(&res)
	if err == nil {
		t.Fatal("expected error for null structured output")
	}
	var noStr *noStructuredOutputError
	if !errors.As(err, &noStr) {
		t.Fatalf("expected *noStructuredOutputError, got: %T", err)
	}
	if !strings.Contains(noStr.RawJSON, "sess_5") {
		t.Errorf("RawJSON should contain session ID, got: %s", noStr.RawJSON)
	}
}

func TestAppError_http_Render(t *testing.T) {
	appErr := &AppError{
		Op:      "generate_messages",
		Message: "OpenCode prompt request failed",
		OC: &OCError{
			Kind:        OCErrHTTP,
			RequestType: "prompt",
			SessionID:   "sess_1",
			Agent:       "my-agent",
			Code:        "500 POST",
			Message:     "http://127.0.0.1:4096/session/sess_1/message",
			Status:      500,
			RawJSON:     `{"error":"internal server error"}`,
		},
	}
	rendered := appErr.Render()
	if !strings.Contains(rendered, "generate_messages") {
		t.Errorf("render should contain operation, got: %v", rendered)
	}
	if !strings.Contains(rendered, "500") || !strings.Contains(rendered, "POST") {
		t.Errorf("render should contain status code and method, got: %v", rendered)
	}
	if !strings.Contains(rendered, "sess_1") {
		t.Errorf("render should contain session ID, got: %v", rendered)
	}
	if !strings.Contains(rendered, "my-agent") {
		t.Errorf("render should contain agent name, got: %v", rendered)
	}
	if !strings.Contains(rendered, "internal server error") {
		t.Errorf("render should contain response body, got: %v", rendered)
	}
}

func TestNoStructuredOutputError(t *testing.T) {
	err := &noStructuredOutputError{RawJSON: `{"info":{"sessionID":"sess_x"}}`}
	if !strings.Contains(err.Error(), "structured output was not found") {
		t.Errorf("error text mismatch, got: %v", err.Error())
	}
	if !strings.Contains(err.RawJSON, "sess_x") {
		t.Errorf("RawJSON should contain session ID, got: %s", err.RawJSON)
	}
}

func TestAppError_plain_error(t *testing.T) {
	plain := errors.New("connection refused")
	appErr := &AppError{
		Op:      "generate_messages",
		Message: "OpenCode prompt request failed",
		Err:     plain,
	}
	msg := appErr.Error()
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("error should contain wrapped error, got: %v", msg)
	}
	if !strings.Contains(msg, "generate_messages") {
		t.Errorf("error should contain operation, got: %v", msg)
	}
}

func TestBuildHTTPOCError_nonHTTP(t *testing.T) {
	plain := errors.New("connection refused")
	oc := buildHTTPOCError(plain, "prompt", "s1", "agent")
	if oc != nil {
		t.Error("non-HTTP error should return nil OCError")
	}
}

func TestBuildAPIOCError_nil(t *testing.T) {
	oc := buildAPIOCError(nil, "prompt", "s1", "agent")
	if oc != nil {
		t.Errorf("expected nil for nil message, got: %v", oc)
	}
}

func TestBuildAPIOCError_noError(t *testing.T) {
	raw := `{
		"id": "msg_1",
		"sessionID": "sess_1",
		"role": "assistant",
		"mode": "chat",
		"modelID": "gpt-4",
		"providerID": "openai",
		"parentID": "parent_1",
		"path": {"cwd": "/tmp", "root": "/tmp"},
		"time": {"created": 1000, "completed": 2000},
		"tokens": {"input": 100, "output": 50, "reasoning": 0, "cache": {"read": 0, "write": 0}},
		"cost": 0.01
	}`
	var msg opencode.AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	oc := buildAPIOCError(&msg, "prompt", "sess_1", "agent")
	if oc != nil {
		t.Errorf("expected nil for message without error, got: %v", oc)
	}
}

func TestBuildAPIOCError_apiError(t *testing.T) {
	raw := `{
		"id": "msg_1",
		"sessionID": "sess_1",
		"role": "assistant",
		"mode": "gen-commit-msg",
		"modelID": "deepseek-v4-pro",
		"providerID": "opencode-go",
		"parentID": "parent_1",
		"path": {"cwd": "/tmp", "root": "/tmp"},
		"time": {"created": 1000, "completed": 2000},
		"tokens": {"input": 0, "output": 0, "reasoning": 0, "cache": {"read": 0, "write": 0}},
		"cost": 0,
		"error": {
			"name": "APIError",
			"data": {
				"message": "Error from provider (DeepSeek): deepseek-reasoner does not support this tool_choice",
				"statusCode": 400,
				"isRetryable": false
			}
		}
	}`
	var msg opencode.AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	oc := buildAPIOCError(&msg, "prompt", "sess_1", "my-agent")
	if oc == nil {
		t.Fatal("expected OCError for message with APIError")
	}
	if oc.Kind != OCErrAPI {
		t.Errorf("expected Kind=OCErrAPI, got: %v", oc.Kind)
	}
	if oc.RequestType != "prompt" {
		t.Errorf("expected RequestType=prompt, got: %v", oc.RequestType)
	}
	if oc.SessionID != "sess_1" {
		t.Errorf("expected SessionID=sess_1, got: %v", oc.SessionID)
	}
	if oc.Agent != "my-agent" {
		t.Errorf("expected Agent=my-agent, got: %v", oc.Agent)
	}
	if oc.Code != "APIError" {
		t.Errorf("expected Code=APIError, got: %v", oc.Code)
	}
	if !strings.Contains(oc.Message, "deepseek-reasoner does not support this tool_choice") {
		t.Errorf("expected specific message, got: %v", oc.Message)
	}
	if oc.Status != 400 {
		t.Errorf("expected Status=400, got: %v", oc.Status)
	}
	if oc.RawJSON == "" {
		t.Error("expected non-empty RawJSON")
	}
}

func TestBuildAPIOCError_providerAuthError(t *testing.T) {
	raw := `{
		"id": "msg_2",
		"sessionID": "sess_2",
		"role": "assistant",
		"mode": "chat",
		"modelID": "gpt-4",
		"providerID": "openai",
		"parentID": "parent_2",
		"path": {"cwd": "/tmp", "root": "/tmp"},
		"time": {"created": 1000, "completed": 2000},
		"tokens": {"input": 0, "output": 0, "reasoning": 0, "cache": {"read": 0, "write": 0}},
		"cost": 0,
		"error": {
			"name": "ProviderAuthError",
			"data": {
				"message": "invalid API key",
				"providerID": "openai"
			}
		}
	}`
	var msg opencode.AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	oc := buildAPIOCError(&msg, "prompt", "sess_2", "my-agent")
	if oc == nil {
		t.Fatal("expected OCError for message with ProviderAuthError")
	}
	if oc.Code != "ProviderAuthError" {
		t.Errorf("expected ProviderAuthError, got: %v", oc.Code)
	}
	if !strings.Contains(oc.Message, "invalid API key") {
		t.Errorf("expected auth error message, got: %v", oc.Message)
	}
}

func TestBuildAPIOCError_unknownError(t *testing.T) {
	raw := `{
		"id": "msg_3",
		"sessionID": "sess_3",
		"role": "assistant",
		"mode": "chat",
		"modelID": "gpt-4",
		"providerID": "openai",
		"parentID": "parent_3",
		"path": {"cwd": "/tmp", "root": "/tmp"},
		"time": {"created": 1000, "completed": 2000},
		"tokens": {"input": 0, "output": 0, "reasoning": 0, "cache": {"read": 0, "write": 0}},
		"cost": 0,
		"error": {
			"name": "UnknownError",
			"data": {
				"message": "something went wrong"
			}
		}
	}`
	var msg opencode.AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	oc := buildAPIOCError(&msg, "prompt", "sess_3", "my-agent")
	if oc == nil {
		t.Fatal("expected OCError for message with UnknownError")
	}
	if oc.Code != "UnknownError" {
		t.Errorf("expected UnknownError, got: %v", oc.Code)
	}
	if !strings.Contains(oc.Message, "something went wrong") {
		t.Errorf("expected error message, got: %v", oc.Message)
	}
}

func TestBuildAPIOCError_messageAbortedError(t *testing.T) {
	raw := `{
		"id": "msg_4",
		"sessionID": "sess_4",
		"role": "assistant",
		"mode": "chat",
		"modelID": "gpt-4",
		"providerID": "openai",
		"parentID": "parent_4",
		"path": {"cwd": "/tmp", "root": "/tmp"},
		"time": {"created": 1000, "completed": 2000},
		"tokens": {"input": 0, "output": 0, "reasoning": 0, "cache": {"read": 0, "write": 0}},
		"cost": 0,
		"error": {
			"name": "MessageAbortedError",
			"data": {
				"message": "message was aborted"
			}
		}
	}`
	var msg opencode.AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	oc := buildAPIOCError(&msg, "prompt", "sess_4", "my-agent")
	if oc == nil {
		t.Fatal("expected OCError for message with MessageAbortedError")
	}
	if oc.Code != "MessageAbortedError" {
		t.Errorf("expected MessageAbortedError, got: %v", oc.Code)
	}
	if !strings.Contains(oc.Message, "message was aborted") {
		t.Errorf("expected error message, got: %v", oc.Message)
	}
}

func TestOCError_RenderDetails(t *testing.T) {
	oc := &OCError{
		Kind:        OCErrAPI,
		RequestType: "prompt",
		SessionID:   "sess_xyz",
		Agent:       "my-agent",
		Code:        "APIError",
		Message:     "something failed",
		Status:      500,
		RawJSON:     `{"error":{"name":"APIError"}}`,
	}
	rendered := oc.RenderDetails()
	if !strings.Contains(rendered, "prompt") {
		t.Errorf("render should contain request type, got: %v", rendered)
	}
	if !strings.Contains(rendered, "sess_xyz") {
		t.Errorf("render should contain session ID, got: %v", rendered)
	}
	if !strings.Contains(rendered, "my-agent") {
		t.Errorf("render should contain agent name, got: %v", rendered)
	}
	if !strings.Contains(rendered, "APIError") {
		t.Errorf("render should contain error name, got: %v", rendered)
	}
	if !strings.Contains(rendered, "something failed") {
		t.Errorf("render should contain message text, got: %v", rendered)
	}
	if !strings.Contains(rendered, "500") {
		t.Errorf("render should contain status code, got: %v", rendered)
	}
	if !strings.Contains(rendered, "APIError") {
		t.Errorf("render should contain JSON content, got: %v", rendered)
	}
}

func TestOCError_RenderDetails_noStatusCode(t *testing.T) {
	oc := &OCError{
		Kind:        OCErrAPI,
		RequestType: "prompt",
		SessionID:   "sess_xyz",
		Agent:       "my-agent",
		Code:        "UnknownError",
		Message:     "something unknown",
		RawJSON:     `{}`,
	}
	rendered := oc.RenderDetails()
	if strings.Contains(rendered, "StatusCode") {
		t.Errorf("render should not contain StatusCode line when 0, got: %v", rendered)
	}
}

func TestAppError_Render_noOC(t *testing.T) {
	appErr := &AppError{
		Op:      "agent_setup",
		Message: "agent file not found",
		Err:     errors.New("file does not exist"),
	}
	rendered := appErr.Render()
	if !strings.Contains(rendered, "agent_setup") {
		t.Errorf("render should contain operation, got: %v", rendered)
	}
	if !strings.Contains(rendered, "file does not exist") {
		t.Errorf("render should contain error text, got: %v", rendered)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	appErr := &AppError{Op: "test", Err: inner}
	if !errors.Is(appErr, inner) {
		t.Error("errors.Is should find inner error via Unwrap")
	}
}

type captureHandler struct {
	records  []slog.Record
	preAttrs []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	r.AddAttrs(h.preAttrs...)
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.preAttrs = append([]slog.Attr(nil), h.preAttrs...)
	h2.preAttrs = append(h2.preAttrs, attrs...)
	return &h2
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureHandler) assertRecord(t *testing.T, idx int, msg string, wantAttrs map[string]string) {
	t.Helper()
	if idx >= len(h.records) {
		t.Fatalf("record %d not found (total %d records)", idx, len(h.records))
	}
	r := h.records[idx]
	if r.Message != msg {
		t.Errorf("record[%d].Message = %q, want %q", idx, r.Message, msg)
	}
	r.Attrs(func(a slog.Attr) bool {
		if want, ok := wantAttrs[a.Key]; ok {
			got := a.Value.String()
			if got != want {
				t.Errorf("record[%d].%s = %q, want %q", idx, a.Key, got, want)
			}
			delete(wantAttrs, a.Key)
		}
		return true
	})
	for k, v := range wantAttrs {
		t.Errorf("record[%d] missing attr %s (wanted %q)", idx, k, v)
	}
}

func TestLogResponseParts_AllTypes(t *testing.T) {
	fixtureName := "prompt_success_parts.json"
	res, err := loadPromptFixture(fixtureName)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", fixtureName, err)
	}

	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), res.Info.SessionID, res.Parts)

	if len(h.records) == 0 {
		t.Fatal("expected at least one log record")
	}

	partRecords := 0
	summaryFound := false
	for _, r := range h.records {
		if r.Message == "opencode response part" {
			partRecords++
		}
		if r.Message == "opencode response parts summary" {
			summaryFound = true
		}
	}

	if partRecords != len(res.Parts) {
		t.Errorf("expected %d part records, got %d", len(res.Parts), partRecords)
	}
	if !summaryFound {
		t.Error("expected summary record")
	}
}

func TestLogResponseParts_Empty(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), "sess_empty", nil)

	if len(h.records) != 0 {
		t.Errorf("expected 0 records for nil parts, got %d", len(h.records))
	}

	logResponseParts(context.Background(), "sess_empty", []opencode.Part{})

	if len(h.records) != 0 {
		t.Errorf("expected 0 records for empty parts, got %d", len(h.records))
	}
}

func TestLogResponseParts_ToolPart(t *testing.T) {
	parts := []opencode.Part{
		{
			Type:   opencode.PartType("tool"),
			Tool:   "StructuredOutput",
			CallID: "call_test",
			State: opencode.ToolPartState{
				Status: opencode.ToolPartStateStatus("completed"),
			},
		},
	}

	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), "sess_tool", parts)

	if len(h.records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(h.records))
	}

	wantAttrs := map[string]string{
		"session_id": "sess_tool",
		"part_type":  "tool",
		"tool":       "StructuredOutput",
		"status":     "completed",
		"call_id":    "call_test",
	}
	h.assertRecord(t, 0, "opencode response part", wantAttrs)
}

func TestLogResponseParts_ReasoningPart(t *testing.T) {
	parts := []opencode.Part{
		{
			Type: opencode.PartType("reasoning"),
			Text: "This is a reasoning text that explains the model's thinking process.",
			Time: opencode.ReasoningPartTime{Start: 100, End: 400},
		},
	}

	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), "sess_reason", parts)

	if len(h.records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(h.records))
	}

	wantAttrs := map[string]string{
		"session_id": "sess_reason",
		"part_type":  "reasoning",
	}
	h.assertRecord(t, 0, "opencode response part", wantAttrs)
}

func TestLogResponseParts_ReasoningTextTruncated(t *testing.T) {
	longText := ""
	for i := 0; i < 300; i++ {
		longText += "x"
	}
	parts := []opencode.Part{
		{
			Type: opencode.PartType("reasoning"),
			Text: longText,
			Time: opencode.ReasoningPartTime{Start: 100, End: 500},
		},
	}

	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), "sess_trunc", parts)

	r := h.records[0]
	var textFound string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "text" {
			textFound = a.Value.String()
		}
		return true
	})
	if len(textFound) <= 200 {
		t.Error("expected text longer than 200 chars (before truncation check)")
	}
	if !strings.HasSuffix(textFound, "...") && len(longText) > 200 {
		t.Error("expected text to be truncated with '...' suffix")
	}
}

func TestLogResponseParts_StepStartAndFinish(t *testing.T) {
	parts := []opencode.Part{
		{
			Type: opencode.PartType("step-start"),
		},
		{
			Type:   opencode.PartType("step-finish"),
			Reason: "tool-calls",
			Tokens: opencode.StepFinishPartTokens{
				Input:     100,
				Output:    50,
				Reasoning: 200,
				Cache:     opencode.StepFinishPartTokensCache{Read: 0, Write: 0},
			},
		},
	}

	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), "sess_steps", parts)

	if len(h.records) < 3 {
		t.Fatalf("expected at least 3 records (2 parts + summary), got %d", len(h.records))
	}

	h.assertRecord(t, 0, "opencode response part", map[string]string{
		"session_id": "sess_steps",
		"part_type":  "step-start",
	})
	h.assertRecord(t, 1, "opencode response part", map[string]string{
		"session_id": "sess_steps",
		"part_type":  "step-finish",
		"reason":     "tool-calls",
	})
}

func TestLogResponseParts_Summary(t *testing.T) {
	parts := []opencode.Part{
		{
			Type: opencode.PartType("step-start"),
		},
		{
			Type: opencode.PartType("reasoning"),
			Text: "Analyzing...",
			Time: opencode.ReasoningPartTime{Start: 100, End: 300},
		},
		{
			Type:   opencode.PartType("tool"),
			Tool:   "StructuredOutput",
			CallID: "call_x",
			State: opencode.ToolPartState{
				Status: opencode.ToolPartStateStatus("completed"),
			},
		},
		{
			Type:   opencode.PartType("step-finish"),
			Reason: "tool-calls",
			Tokens: opencode.StepFinishPartTokens{
				Input:     50,
				Output:    20,
				Reasoning: 100,
				Cache:     opencode.StepFinishPartTokensCache{Read: 0, Write: 0},
			},
		},
	}

	h := &captureHandler{}
	logger := slog.New(h)
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	logResponseParts(context.Background(), "sess_summary", parts)

	lastIdx := len(h.records) - 1
	h.assertRecord(t, lastIdx, "opencode response parts summary", map[string]string{
		"session_id": "sess_summary",
	})

	r := h.records[lastIdx]
	var totalParts int
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "total_parts" {
			totalParts = int(a.Value.Int64())
		}
		return true
	})
	if totalParts != 4 {
		t.Errorf("summary total_parts = %d, want 4", totalParts)
	}
}

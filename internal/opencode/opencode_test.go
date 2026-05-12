package opencode

import (
	"encoding/json"
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
}

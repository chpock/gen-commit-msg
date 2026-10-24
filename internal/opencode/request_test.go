package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGenerationRequestCompatibility(t *testing.T) {
	response, err := os.ReadFile("../../testdata/fixtures/prompt_success_basic.json")
	if err != nil {
		t.Fatal(err)
	}
	type request struct {
		Agent string `json:"agent"`
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
		Format json.RawMessage `json:"format"`
	}
	requests := make(chan request, 1)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session/session-compat/message" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("directory"); got != "/repo" {
			t.Errorf("request directory = %q, want /repo", got)
		}
		var got request
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		requests <- got
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer api.Close()

	const snapshot = `{"format_version":"1.0","staged_changes":{"outputs":[]},"marker":"snapshot"}`
	client := NewClient(api.URL, "/repo", "custom-agent")
	messages, err := client.GenerateMessages(context.Background(), "session-compat", GenerateParams{
		SubjectMin: 2, SubjectMax: 3, Body: false, ContextJSON: snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	// OpenCode acceptance is intentionally not tightened to Pi's validation rules.
	if len(messages) != 1 || messages[0].Body == "" {
		t.Fatalf("existing OpenCode response acceptance changed: %+v", messages)
	}
	got := <-requests
	if got.Agent != "custom-agent" || len(got.Parts) != 1 || got.Parts[0].Type != "text" {
		t.Fatalf("request routing changed: %+v", got)
	}
	prompt := got.Parts[0].Text
	for _, part := range []string{
		"- Minimum subjects: 2\n", "- Maximum subjects: 3\n", "- Include body: false\n",
		"<commit_message_context_json>\n" + snapshot + "\n</commit_message_context_json>",
	} {
		if strings.Count(prompt, part) != 1 {
			t.Errorf("prompt does not preserve %q exactly once: %s", part, prompt)
		}
	}
	const wantFormat = `{"type":"json_schema","retryCount":2,"schema":{"type":"object","additionalProperties":false,"properties":{"subjects":{"type":"array","minItems":2,"maxItems":3,"description":"Candidate subjects for a Git commit message.","items":{"type":"string"}},"body":{"type":"string","description":"Detailed commit message body. Empty string if not needed."}},"required":["subjects","body"]}}`
	var gotJSON, wantJSON any
	if err := json.Unmarshal(got.Format, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(wantFormat), &wantJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("OpenCode format changed: %s; want %s", got.Format, wantFormat)
	}
}

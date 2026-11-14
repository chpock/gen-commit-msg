package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGenerationThinkingLevel(t *testing.T) {
	for _, level := range []string{"", "off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("level/"+level, func(t *testing.T) {
			runThinkingRequest(t, level, "supported")
		})
	}
	for _, scenario := range []string{
		"unsupported", "no variants", "provider absent", "model absent", "catalog failure",
		"preparation failure", "wrong role", "missing message", "wrong session", "missing model", "changed variant", "injected parts", "missing parts", "null parts",
	} {
		t.Run(scenario, func(t *testing.T) {
			runThinkingRequest(t, "low", scenario)
		})
	}
}

func runThinkingRequest(t *testing.T, level, scenario string) {
	t.Helper()
	response, err := os.ReadFile("../../testdata/fixtures/prompt_success_basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var answer map[string]any
	if err := json.Unmarshal(response, &answer); err != nil {
		t.Fatal(err)
	}
	answer["info"].(map[string]any)["providerID"] = "fixture-provider"
	answer["info"].(map[string]any)["modelID"] = "fixture-model"
	events := make(chan string, 16)
	const snapshot = `{"marker":"original staged context"}`
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if directory := r.URL.Query().Get("directory"); directory != "/repo" {
			t.Errorf("request directory = %q, want /repo", directory)
		}
		if r.Method == http.MethodGet && r.URL.Path == "/config/providers" {
			events <- "catalog"
			if scenario == "catalog failure" {
				http.Error(w, "catalog unavailable", http.StatusBadRequest)
				return
			}
			variants := map[string]any{"high": map[string]any{"reasoningEffort": "high"}}
			if scenario != "unsupported" {
				variants[level] = map[string]any{"reasoningEffort": level}
			}
			if scenario == "no variants" {
				variants = nil
			}
			providerID, modelID := "fixture-provider", "fixture-model"
			if scenario == "provider absent" {
				providerID = "another-provider"
			}
			if scenario == "model absent" {
				modelID = "another-model"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"providers": []any{map[string]any{
					"id":     providerID,
					"models": map[string]any{modelID: map[string]any{"variants": variants}},
				}},
			})
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/session/session-thinking/message" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if string(body["agent"]) != `"custom-agent"` {
			t.Errorf("agent changed: %s", body["agent"])
		}
		if string(body["noReply"]) == "true" {
			events <- "prepare"
			if scenario == "preparation failure" {
				http.Error(w, "preparation unavailable", http.StatusBadRequest)
				return
			}
			if _, ok := body["model"]; ok {
				t.Errorf("preparation overrides native model selection: %s", body["model"])
			}
			if string(body["parts"]) != "[]" || string(body["variant"]) != fmt.Sprintf("%q", level) {
				t.Errorf("preparation must be empty and use requested level: %s", body)
			}
			model := map[string]any{"providerID": "fixture-provider", "modelID": "fixture-model", "variant": level}
			info := map[string]any{"id": "msg_prepared", "sessionID": "session-thinking", "role": "user", "model": model}
			parts := []any{}
			switch scenario {
			case "wrong role":
				info["role"] = "assistant"
			case "missing message":
				delete(info, "id")
			case "wrong session":
				info["sessionID"] = "another-session"
			case "missing model":
				delete(model, "modelID")
			case "changed variant":
				model["variant"] = "high"
			case "injected parts":
				parts = append(parts, map[string]any{"type": "text", "text": "injected preparation content"})
			}
			prepared := map[string]any{"info": info, "parts": parts}
			switch scenario {
			case "missing parts":
				delete(prepared, "parts")
			case "null parts":
				prepared["parts"] = nil
			}
			_ = json.NewEncoder(w).Encode(prepared)
			return
		}
		events <- "generate"
		if level == "" {
			for _, field := range []string{"variant", "model", "messageID", "noReply"} {
				if _, ok := body[field]; ok {
					t.Errorf("unset thinking level adds %s: %s", field, body[field])
				}
			}
		} else {
			if string(body["variant"]) != fmt.Sprintf("%q", level) || string(body["messageID"]) != `"msg_prepared"` {
				t.Errorf("generation lost requested variant or prepared message: %s", body)
			}
			var model map[string]string
			if err := json.Unmarshal(body["model"], &model); err != nil {
				t.Error(err)
			} else if !reflect.DeepEqual(model, map[string]string{"providerID": "fixture-provider", "modelID": "fixture-model"}) {
				t.Errorf("generation does not retain native selected model: %v", model)
			}
		}
		if strings.Count(string(body["parts"]), "original staged context") != 1 || len(body["format"]) == 0 {
			t.Errorf("generation lost context/schema or duplicated context: %s", body)
		}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	defer api.Close()

	client := NewClient(api.URL, "/repo", "custom-agent")
	messages, err := client.GenerateMessages(context.Background(), "session-thinking", GenerateParams{
		SubjectMin: 1, SubjectMax: 1, Body: true, ThinkingLevel: level, ContextJSON: snapshot,
	})
	var calls []string
	for len(events) > 0 {
		calls = append(calls, <-events)
	}
	if scenario == "supported" {
		if err != nil || len(messages) != 1 {
			t.Fatalf("messages=%+v error=%v calls=%v", messages, err, calls)
		}
		want := []string{"prepare", "catalog", "generate"}
		if level == "" {
			want = []string{"generate"}
		}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls=%v, want %v", calls, want)
		}
		return
	}
	if err == nil || len(messages) != 0 {
		t.Fatalf("messages=%+v error=%v; want failure before generation (calls=%v)", messages, err, calls)
	}
	for _, call := range calls {
		if call == "generate" {
			t.Fatalf("invalid thinking override generated candidates: calls=%v error=%v", calls, err)
		}
	}
	if scenario == "unsupported" || scenario == "no variants" {
		for _, detail := range []string{"low", "fixture-provider/fixture-model"} {
			if !strings.Contains(err.Error(), detail) {
				t.Errorf("error omits %q: %v", detail, err)
			}
		}
	}
}

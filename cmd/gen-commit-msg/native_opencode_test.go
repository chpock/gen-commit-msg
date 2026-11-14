package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

func TestNativeOpenCodeThinkingLevel(t *testing.T) {
	if os.Getenv("GCM_TEST_NATIVE_OPENCODE") != "1" {
		t.Skip("set GCM_TEST_NATIVE_OPENCODE=1 for actual OpenCode/local API integration")
	}
	opencode := os.Getenv("GCM_TEST_NATIVE_OPENCODE_PATH")
	if opencode == "" {
		opencode = nativeExecutable(t, "opencode")
	}
	opencode, err := filepath.Abs(opencode)
	if err != nil {
		t.Fatal(err)
	}
	git := nativeExecutable(t, "git")
	for _, tc := range []struct {
		name, level string
		budget      int
		unsupported bool
		agentModel  bool
		tty         bool
	}{
		{name: "override-history-selected-model", level: "low", budget: 2048},
		{name: "override-agent-selected-model-tty", level: "low", budget: 2048, agentModel: true, tty: true},
		{name: "inherit-thinking", budget: 8192},
		{name: "unsupported-before-model", level: "minimal", unsupported: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			repo, bin := filepath.Join(root, "repo"), filepath.Join(root, "bin")
			for _, dir := range []string{repo, bin} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			for name, target := range map[string]string{"git": git, "opencode": opencode} {
				if err := os.Symlink(target, filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			var mu sync.Mutex
			var requests []nativeOpenCodeRequest
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request nativeOpenCodeRequest
				if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" || json.NewDecoder(r.Body).Decode(&request) != nil {
					t.Errorf("unexpected native model request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected model request", http.StatusBadRequest)
					return
				}
				mu.Lock()
				requests = append(requests, request)
				mu.Unlock()
				nativeOpenCodeAnswer(w, tc.tty)
			}))
			defer api.Close()
			catalogPath := filepath.Join(root, "models.json")
			nativeJSON(t, catalogPath, map[string]any{})
			configPath := filepath.Join(root, "config", "opencode", "opencode.json")
			eventsPath := filepath.Join(root, "plugin-events")
			pluginPath := filepath.Join(root, "observe.mjs")
			nativeWrite(t, pluginPath, fmt.Sprintf(`import fs from 'node:fs';
export default async () => ({
  'chat.message': async (_input, output) => {
    fs.appendFileSync(%q, JSON.stringify({id: output.message.id, model: output.message.model, parts: output.parts.length}) + '\n');
  }
});
`, eventsPath), 0600)
			thinking := func(budget int) map[string]any {
				return map[string]any{"thinking": map[string]any{"type": "enabled", "budgetTokens": budget}}
			}
			model := func(date string, low bool) map[string]any {
				variants := map[string]any{"high": thinking(8192)}
				if low {
					variants["low"] = thinking(2048)
				}
				return map[string]any{
					"name": "Native fixture", "reasoning": true, "release_date": date,
					"limit":   map[string]int{"context": 128000, "output": 16384},
					"options": thinking(8192), "variants": variants,
				}
			}
			configuration := map[string]any{
				"$schema":           "https://opencode.ai/config.json",
				"enabled_providers": []string{"gcm-native-fixture"},
				"plugin":            []string{"file://" + pluginPath},
				"provider": map[string]any{"gcm-native-fixture": map[string]any{
					"npm": "@ai-sdk/anthropic", "name": "Native fixture",
					"options": map[string]string{"baseURL": api.URL + "/v1", "apiKey": "dummy-localhost-only"},
					"models": map[string]any{
						"native-fixture": model("2026-01-01", true),
						"unused-fixture": model("2026-12-31", false),
					},
				}},
			}
			agentHeader := "---\ndescription: Native integration fixture\nmode: subagent\nhidden: true\nsteps: 12\n"
			if tc.agentModel {
				configuration["model"] = "gcm-native-fixture/unused-fixture"
				agentHeader += "model: gcm-native-fixture/native-fixture\n"
			}
			nativeJSON(t, configPath, configuration)
			agentPath := filepath.Join(root, "config", "opencode", "agents", "gen-commit-msg.md")
			agentContent := agentHeader + "---\n" + generation.Instructions
			nativeWrite(t, agentPath, agentContent, 0600)
			originalConfig, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			historyModel := "native-fixture"
			if tc.agentModel {
				historyModel = "unused-fixture"
			}
			nativeJSON(t, filepath.Join(root, "state", "opencode", "model.json"), map[string]any{
				"recent": []any{map[string]string{"providerID": "gcm-native-fixture", "modelID": historyModel}},
			})
			env := []string{
				"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + root, "TMPDIR=" + root,
				"XDG_CONFIG_HOME=" + filepath.Join(root, "config"), "XDG_CACHE_HOME=" + filepath.Join(root, "cache"),
				"XDG_DATA_HOME=" + filepath.Join(root, "data"), "XDG_STATE_HOME=" + filepath.Join(root, "state"),
				"OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_MODELS_FETCH=1", "OPENCODE_MODELS_PATH=" + catalogPath,
				"OPENCODE_DISABLE_DEFAULT_PLUGINS=1", "OPENCODE_DISABLE_EXTERNAL_SKILLS=1", "OPENCODE_DISABLE_LSP_DOWNLOAD=1",
				"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
				"GCM_TEST_CLI_PROCESS=1", "GORACE=atexit_sleep_ms=0", "NO_COLOR=1",
			}
			for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "Native Fixture"}, {"config", "user.email", "native@example.invalid"}} {
				command := exec.Command(git, args...)
				command.Dir, command.Env = repo, env
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v %s", args, err, output)
				}
			}
			nativeWrite(t, filepath.Join(repo, "change.txt"), "ORIGINAL_GIT_SNAPSHOT\n", 0600)
			stage := exec.Command(git, "add", "change.txt")
			stage.Dir, stage.Env = repo, env
			if output, err := stage.CombinedOutput(); err != nil {
				t.Fatalf("git add: %v %s", err, output)
			}
			logPath := filepath.Join(root, "application.log")
			args := []string{"--harness", "opencode", "--pause", "off", "--body=false", "--subject-max", "1", "--log-level", "debug", "--log-file", logPath}
			if tc.level != "" {
				args = append(args, "--thinking-level", tc.level)
			}
			output := filepath.Join(root, "message")
			if tc.tty {
				args = append(args, "--subject-max", "2", "--output", output)
			}
			cmd := newCLICommand(t, repo, args...)
			cmd.Env = env
			var stdout, stderr bytes.Buffer
			if tc.tty {
				terminal := startTestTerminal(t, cmd)
				terminal.await(t, nativeSecond)
				terminal.send(t, "\x1b[B\r")
				terminal.wait(t)
				if data, err := os.ReadFile(output); err != nil || string(data) != nativeSecond+"\n" {
					t.Fatalf("TTY output=%q error=%v", data, err)
				}
			} else {
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err = cmd.Run()
			}
			if tc.unsupported {
				if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "thinking-level") || !strings.Contains(stderr.String(), "minimal") {
					t.Fatalf("unsupported level: error=%v stdout=%q stderr=%q", err, &stdout, &stderr)
				}
			} else if !tc.tty && (err != nil || stdout.String() != nativeFirst+"\n" || stderr.Len() != 0) {
				log, _ := os.ReadFile(logPath)
				t.Fatalf("application: %v\nstdout=%s\nstderr=%s\nlog=%s", err, &stdout, &stderr, log)
			}
			mu.Lock()
			calls := append([]nativeOpenCodeRequest(nil), requests...)
			mu.Unlock()
			wantCalls := 1
			if tc.unsupported {
				wantCalls = 0
			}
			if len(calls) != wantCalls {
				t.Fatalf("native provider requests=%d, want %d", len(calls), wantCalls)
			}
			for _, request := range calls {
				if request.Model != "native-fixture" || request.Thinking.BudgetTokens != tc.budget {
					t.Errorf("model=%q thinking budget=%d, want native-fixture/%d", request.Model, request.Thinking.BudgetTokens, tc.budget)
				}
				messages, _ := json.Marshal(request.Messages)
				if strings.Count(string(messages), "ORIGINAL_GIT_SNAPSHOT") != 1 {
					t.Errorf("staged prompt duplicated or missing: %s", messages)
				}
			}
			data, err := os.ReadFile(eventsPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
			wantEvents := 2
			if tc.unsupported || tc.level == "" {
				wantEvents = 1
			}
			if len(lines) != wantEvents {
				t.Fatalf("plugin events=%d, want %d: %s", len(lines), wantEvents, data)
			}
			if tc.level != "" && !tc.unsupported {
				var first, second map[string]any
				if err := json.Unmarshal(lines[0], &first); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(lines[1], &second); err != nil {
					t.Fatal(err)
				}
				if first["id"] != second["id"] || first["parts"] != float64(0) || second["parts"] != float64(1) {
					t.Errorf("preparation duplicated the user message or content: %s", data)
				}
			}
			if config, err := os.ReadFile(configPath); err != nil || !bytes.Equal(config, originalConfig) {
				t.Fatalf("thinking override changed saved configuration: %s %v", config, err)
			}
			if agent, err := os.ReadFile(agentPath); err != nil || string(agent) != agentContent {
				t.Fatalf("thinking override changed customized agent: %s %v", agent, err)
			}
			t.Logf("provider requests=%d, thinking budget=%d, plugin events=%d, native model retained", len(calls), tc.budget, len(lines))
		})
	}
}

type nativeOpenCodeRequest struct {
	Model    string             `json:"model"`
	Messages []nativeAPIMessage `json:"messages"`
	Thinking struct {
		BudgetTokens int `json:"budget_tokens"`
	} `json:"thinking"`
}

func nativeOpenCodeAnswer(w http.ResponseWriter, multiple bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	subjects := []string{nativeFirst}
	if multiple {
		subjects = append(subjects, nativeSecond)
	}
	answer, _ := json.Marshal(map[string]any{"subjects": subjects, "body": ""})
	for _, event := range []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": "msg_native", "type": "message", "role": "assistant", "model": "native-fixture", "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]int{"input_tokens": 100, "output_tokens": 0}}},
		{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": "tool_native", "name": "StructuredOutput", "input": map[string]any{}}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(answer)}},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]any{"stop_reason": "tool_use", "stop_sequence": nil}, "usage": map[string]int{"output_tokens": 30}},
		{"type": "message_stop"},
	} {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}

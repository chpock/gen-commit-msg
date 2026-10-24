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
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

// Native integration is deliberately opt-in: ordinary suites need no Pi, Node,
// search helpers, credentials, or model service. GCM_TEST_NATIVE_PI_PATH may name
// a specific installed Pi executable; all model traffic goes to the local API.
func TestNativePi(t *testing.T) {
	if os.Getenv("GCM_TEST_NATIVE_PI") != "1" {
		t.Skip("set GCM_TEST_NATIVE_PI=1 for actual Pi/local API integration")
	}
	pi := os.Getenv("GCM_TEST_NATIVE_PI_PATH")
	if pi == "" {
		pi = nativeExecutable(t, "pi")
	}
	absolute, err := filepath.Abs(pi)
	if err != nil {
		t.Fatal(err)
	}
	pi = absolute
	node, git := nativeExecutable(t, "node"), nativeExecutable(t, "git")
	rg, fd := nativeExecutable(t, "rg"), ""
	for _, name := range []string{"fd", "fdfind"} {
		if path, err := exec.LookPath(name); err == nil {
			fd = path
			break
		}
	}
	if fd == "" {
		t.Fatal("native delivery check requires installed fd or fdfind")
	}
	for _, tc := range []struct {
		name                                             string
		missing, correction, file, tty, cancel, existing bool
	}{
		{name: "cached-rg-system-fd-stdout"},
		{name: "missing-helpers-file", missing: true, file: true},
		{name: "format-correction", correction: true},
		{name: "tty-second-selection", tty: true},
		{name: "tty-cancel-existing", tty: true, cancel: true, existing: true},
		{name: "tty-cancel-missing", tty: true, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newNativePiFixture(t, pi, node, git, rg, fd, tc.missing, tc.correction, tc.tty)
			output := filepath.Join(f.root, "message")
			if tc.existing {
				nativeWrite(t, output, "previous message\n", 0600)
			}
			args := []string{"--harness", "auto", "--pause", "off", "--body=false", "--subject-min", "1", "--subject-max", "1"}
			if tc.tty {
				args = append(args, "--subject-max", "2")
			}
			if tc.file || tc.tty {
				args = append(args, "--output", output)
			}
			cmd := newCLICommand(t, f.repo, args...)
			cmd.Env = append(f.env, "GCM_TEST_CLI_PROCESS=1")
			if tc.tty {
				terminal := startTestTerminal(t, cmd)
				terminal.await(t, nativeSecond)
				f.assertProcessesGone(t) // Selection must not be offered while Pi is alive.
				before, err := os.ReadFile(output)
				if tc.existing {
					if err != nil || string(before) != "previous message\n" {
						t.Fatalf("output changed before selection: %q %v", before, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("output created before selection: %q %v", before, err)
				}
				if tc.cancel {
					terminal.send(t, "\x1b")
				} else {
					terminal.send(t, "\x1b[B\r")
				}
				terminal.wait(t)
				if strings.Contains(terminal.text(), `"type":"agent_settled"`) {
					t.Error("protocol leaked into terminal")
				}
			} else {
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("application: %v\nstdout=%s\nstderr=%s\nevents=%s", err, &stdout, &stderr, f.eventData(t))
				}
				want := nativeFirst + "\n"
				if tc.file {
					want = ""
				}
				if stdout.String() != want || stderr.Len() != 0 {
					t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
				}
			}
			if tc.file || tc.tty {
				data, err := os.ReadFile(output)
				switch {
				case tc.cancel && tc.existing:
					if err != nil || string(data) != "previous message\n" {
						t.Fatalf("cancel changed existing output: %q %v", data, err)
					}
				case tc.cancel:
					if !os.IsNotExist(err) {
						t.Fatalf("cancel created output: %q %v", data, err)
					}
				default:
					want := nativeFirst
					if tc.tty {
						want = nativeSecond
					}
					if err != nil || string(data) != want+"\n" {
						t.Fatalf("file=%q error=%v", data, err)
					}
				}
			}
			f.assertEvidence(t)
		})
	}
}

const nativeFirst = "fix(pi): preserve native fixture staged change"
const nativeSecond = "refactor(pi): clarify native fixture staged change"
const nativeMarker = "NATIVE_CONFLICTING_RESOURCE_MARKER"

type nativeAPIMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCallID string          `json:"tool_call_id"`
}
type nativeAPIRequest struct {
	Model    string             `json:"model"`
	Messages []nativeAPIMessage `json:"messages"`
	Tools    []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}
type nativeObservation struct {
	Kind    string   `json:"kind"`
	PID     int      `json:"pid"`
	Child   int      `json:"child"`
	Code    int      `json:"code"`
	Argv    []string `json:"argv"`
	Args    []string `json:"args"`
	Helper  string   `json:"helper"`
	URL     string   `json:"url"`
	Version string   `json:"version"`
	Record  struct {
		Type    string          `json:"type"`
		Command string          `json:"command"`
		Success bool            `json:"success"`
		Message json.RawMessage `json:"message"`
		Aborted bool            `json:"aborted"`
		IsError bool            `json:"isError"`
	} `json:"record"`
}
type nativePiFixture struct {
	root, repo                    string
	env                           []string
	missing, correction, multiple bool
	mu                            sync.Mutex
	requests                      []nativeAPIRequest
}

func nativeExecutable(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("native delivery prerequisite %s: %v", name, err)
	}
	return p
}
func nativeWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
func nativeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, path, string(data), 0600)
}

func newNativePiFixture(t *testing.T, pi, node, git, rg, fd string, missing, correction, multiple bool) *nativePiFixture {
	t.Helper()
	f := &nativePiFixture{root: t.TempDir(), missing: missing, correction: correction, multiple: multiple}
	f.repo = filepath.Join(f.root, "repo")
	bin, agent := filepath.Join(f.root, "path"), filepath.Join(f.root, "agent")
	for _, dir := range []string{f.repo, bin, agent} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{"node": node, "git": git} {
		if err := os.Symlink(target, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	// Launch the selected native executable unchanged, with test-only observation.
	nativeWrite(t, filepath.Join(bin, "pi"), fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", pi), 0700)
	nativeWrite(t, filepath.Join(bin, "opencode"), fmt.Sprintf("#!/bin/sh\nprintf invoked > %q\nexit 61\n", filepath.Join(f.root, "fallback")), 0700)
	if !missing {
		if err := os.MkdirAll(filepath.Join(agent, "bin"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(rg, filepath.Join(agent, "bin", "rg")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(fd, filepath.Join(bin, "fd")); err != nil {
			t.Fatal(err)
		}
	}
	preload, err := filepath.Abs("testdata/native-pi/observe.cjs")
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(api.Close)
	// An allowlisted environment, not the operator's environment with a few keys
	// removed: no auth, proxies, Git overrides, NODE_OPTIONS, or Pi paths survive.
	f.env = []string{"PATH=" + bin, "HOME=" + f.root, "XDG_CONFIG_HOME=" + filepath.Join(f.root, "config"), "XDG_CACHE_HOME=" + filepath.Join(f.root, "cache"), "XDG_DATA_HOME=" + filepath.Join(f.root, "data"), "TMPDIR=" + f.root, "PI_CODING_AGENT_DIR=" + agent, "PI_SKIP_VERSION_CHECK=1", "PI_TELEMETRY=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GORACE=atexit_sleep_ms=0", "NODE_OPTIONS=--require=" + preload, "GCM_NATIVE_EVENTS=" + filepath.Join(f.root, "events"), "GCM_NATIVE_ORIGIN=" + api.URL, "NO_COLOR=1"}
	f.git(t, "init", "--quiet")
	f.git(t, "config", "user.name", "Native Fixture")
	f.git(t, "config", "user.email", "native@example.invalid")
	nativeWrite(t, filepath.Join(f.repo, "change.txt"), "ORIGINAL_GIT_SNAPSHOT\n", 0600)
	f.git(t, "add", "change.txt")
	nativeWrite(t, filepath.Join(f.repo, "native-evidence", "needle.txt"), "NATIVE_TOOL_SENTINEL\n", 0600)
	settings := map[string]any{"defaultProvider": "gcm-native-fixture", "defaultModel": "native-fixture", "defaultTools": []string{"bash", "write"}, "retry": map[string]any{"enabled": false}}
	nativeJSON(t, filepath.Join(agent, "settings.json"), settings)
	nativeJSON(t, filepath.Join(f.repo, ".pi", "settings.json"), map[string]any{"defaultProvider": "invalid-project-provider", "defaultModel": "invalid-project-model", "defaultTools": []string{"bash", "write"}})
	nativeJSON(t, filepath.Join(agent, "models.json"), map[string]any{"providers": map[string]any{"gcm-native-fixture": map[string]any{"baseUrl": api.URL + "/v1", "api": "openai-completions", "apiKey": "dummy-localhost-only", "models": []any{map[string]any{"id": "native-fixture", "name": "native-fixture", "reasoning": false, "input": []string{"text"}, "cost": map[string]int{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0}, "contextWindow": 128000, "maxTokens": 4096}}}}})
	for _, base := range []string{agent, filepath.Join(f.repo, ".pi")} {
		for _, name := range []string{"AGENTS.md", "SYSTEM.md", "APPEND_SYSTEM.md", "prompts/marker.md", "skills/marker/SKILL.md"} {
			nativeWrite(t, filepath.Join(base, name), "---\nname: native-marker\ndescription: "+nativeMarker+"\n---\n"+nativeMarker, 0600)
		}
		nativeWrite(t, filepath.Join(base, "extensions/marker.mjs"), fmt.Sprintf("import fs from 'node:fs'; fs.writeFileSync(%q,'executed'); export default function() {}\n", filepath.Join(f.root, "extension-executed")), 0600)
		nativeJSON(t, filepath.Join(base, "mcp.json"), map[string]any{"mcpServers": map[string]any{"marker": map[string]any{"command": "/bin/sh", "args": []string{"-c", fmt.Sprintf("printf executed > %q", filepath.Join(f.root, "mcp-executed"))}}}})
	}
	nativeWrite(t, filepath.Join(f.repo, "AGENTS.md"), nativeMarker, 0600)
	return f
}

func (f *nativePiFixture) git(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(f.root, "path", "git"), args...)
	cmd.Dir = f.repo
	cmd.Env = f.env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
}

func (f *nativePiFixture) serve(w http.ResponseWriter, r *http.Request) {
	var request nativeAPIRequest
	if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" || json.NewDecoder(r.Body).Decode(&request) != nil {
		http.Error(w, "unexpected request", 400)
		return
	}
	f.mu.Lock()
	f.requests = append(f.requests, request)
	index := len(f.requests)
	f.mu.Unlock()
	delta := map[string]any{"role": "assistant"}
	finish := "stop"
	if index == 1 && !f.correction {
		calls := []any{}
		for i, tool := range []struct {
			name string
			args any
		}{
			{"read", map[string]string{"path": "native-evidence/needle.txt"}},
			{"grep", map[string]string{"pattern": "NATIVE_TOOL_SENTINEL", "path": "native-evidence"}},
			{"find", map[string]string{"pattern": "*.txt", "path": "native-evidence"}},
			{"ls", map[string]string{"path": "native-evidence"}},
		} {
			data, _ := json.Marshal(tool.args)
			calls = append(calls, map[string]any{"index": i, "id": fmt.Sprintf("native_%d", i), "type": "function", "function": map[string]any{"name": tool.name, "arguments": string(data)}})
		}
		delta["tool_calls"] = calls
		finish = "tool_calls"
	} else if index == 1 && f.correction {
		delta["content"] = "not valid JSON"
		// Change the index after context delivery. The corrective conversation must
		// retain the original snapshot rather than collecting this replacement.
		_ = os.WriteFile(filepath.Join(f.repo, "change.txt"), []byte("REPLACED_GIT_SNAPSHOT\n"), 0600)
		cmd := exec.Command(filepath.Join(f.root, "path", "git"), "add", "change.txt")
		cmd.Dir = f.repo
		cmd.Env = f.env
		if err := cmd.Run(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else {
		subjects := []string{nativeFirst}
		if f.multiple {
			subjects = append(subjects, nativeSecond)
		}
		answer, _ := json.Marshal(map[string]any{"subjects": subjects, "body": ""})
		delta["content"] = string(answer)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for _, part := range []struct {
		delta  any
		finish any
	}{{delta, nil}, {map[string]any{}, finish}} {
		data, _ := json.Marshal(map[string]any{"id": "native-response", "object": "chat.completion.chunk", "created": 1, "model": "native-fixture", "choices": []any{map[string]any{"index": 0, "delta": part.delta, "finish_reason": part.finish}}, "usage": map[string]int{"prompt_tokens": 100, "completion_tokens": 30, "total_tokens": 130}})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}

func (f *nativePiFixture) eventData(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, "events"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func (f *nativePiFixture) observations(t *testing.T) []nativeObservation {
	t.Helper()
	var out []nativeObservation
	for _, line := range bytes.Split(f.eventData(t), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var event nativeObservation
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		out = append(out, event)
	}
	return out
}
func (f *nativePiFixture) assertProcessesGone(t *testing.T) {
	t.Helper()
	for _, event := range f.observations(t) {
		pid := event.PID
		if event.Kind == "helper-pid" {
			pid = event.Child
		}
		if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
			t.Errorf("owned native process %d still exists: %v", pid, err)
		}
	}
}

func (f *nativePiFixture) assertEvidence(t *testing.T) {
	t.Helper()
	f.assertProcessesGone(t)
	f.mu.Lock()
	requests := append([]nativeAPIRequest(nil), f.requests...)
	f.mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("API requests=%d; want 2", len(requests))
	}
	for _, request := range requests {
		if request.Model != "native-fixture" {
			t.Errorf("global model default lost: %s", request.Model)
		}
		names := []string{}
		for _, tool := range request.Tools {
			names = append(names, tool.Function.Name)
		}
		sort.Strings(names)
		if !reflect.DeepEqual(names, []string{"find", "grep", "ls", "read"}) {
			t.Errorf("native tools=%v", names)
		}
		system := ""
		for _, message := range request.Messages {
			if message.Role == "system" || message.Role == "developer" {
				var text string
				if err := json.Unmarshal(message.Content, &text); err != nil {
					t.Fatal(err)
				}
				system += text
			}
		}
		if !strings.Contains(system, generation.Instructions) || strings.Contains(system, nativeMarker) {
			t.Errorf("embedded instructions missing or conflicting resource included: %s", system)
		}
	}
	toolResults := map[string]string{}
	for _, message := range requests[1].Messages {
		if message.Role == "tool" {
			toolResults[message.ToolCallID] = string(message.Content)
		}
	}
	if !f.correction {
		if len(toolResults) != 4 {
			t.Errorf("native tool results=%v", toolResults)
		}
		for _, id := range []string{"native_0", "native_3"} {
			want := "needle.txt"
			if id == "native_0" {
				want = "NATIVE_TOOL_SENTINEL"
			}
			if !strings.Contains(toolResults[id], want) {
				t.Errorf("tool %s result=%s", id, toolResults[id])
			}
		}
		for _, tc := range []struct{ id, want string }{{"native_1", "NATIVE_TOOL_SENTINEL"}, {"native_2", "needle.txt"}} {
			want := tc.want
			if f.missing {
				want = "not available and could not be downloaded"
			}
			if !strings.Contains(toolResults[tc.id], want) {
				t.Errorf("native search result %s=%s; want %q", tc.id, toolResults[tc.id], want)
			}
		}
	} else {
		for _, request := range requests {
			data, _ := json.Marshal(request.Messages)
			if !bytes.Contains(data, []byte("ORIGINAL_GIT_SNAPSHOT")) || bytes.Contains(data, []byte("REPLACED_GIT_SNAPSHOT")) {
				t.Error("correction lost original Git snapshot")
			}
		}
		if len(requests[1].Messages) != 4 {
			t.Errorf("correction conversation messages=%d; want system,user,assistant,user", len(requests[1].Messages))
		}
	}
	counts := map[string]int{}
	helpers := map[string]int{}
	var prompts []string
	versions := 0
	rpc := 0
	accepted := 0
	settled := 0
	toolErrors := 0
	readiness := 0
	actualVersion := ""
	for _, event := range f.observations(t) {
		counts[event.Kind]++
		if event.Kind == "version" {
			actualVersion = event.Version
		}
		if event.Kind == "command" && event.Record.Type == "get_state" {
			readiness++
		}
		if event.Kind == "preload" {
			if reflect.DeepEqual(event.Argv, []string{"--version"}) {
				versions++
			} else {
				rpc++
				instructions := ""
				for i, arg := range event.Argv {
					if arg == "--system-prompt" && i+1 < len(event.Argv) {
						instructions = event.Argv[i+1]
					}
				}
				if instructions != generation.Instructions {
					t.Error("native launch changed embedded instruction bytes")
				}
			}
		}
		if event.Kind == "exit" && event.Code != 0 {
			t.Errorf("native exit code=%d", event.Code)
		}
		if event.Kind == "command" && event.Record.Type == "prompt" {
			var prompt string
			if err := json.Unmarshal(event.Record.Message, &prompt); err != nil {
				t.Fatal(err)
			}
			prompts = append(prompts, prompt)
		}
		if event.Kind == "event" && event.Record.Type == "response" && event.Record.Command == "prompt" && event.Record.Success {
			accepted++
		}
		if event.Kind == "event" && event.Record.Type == "agent_settled" && !event.Record.Aborted {
			settled++
		}
		if event.Kind == "event" && event.Record.Type == "tool_execution_end" && event.Record.IsError {
			toolErrors++
		}
		if event.Kind == "external-fetch" && event.URL != "https://native-fixture.invalid/preload-check" {
			t.Errorf("external management/helper fetch attempted: %s", event.URL)
		}
		if event.Kind == "helper" {
			kind := "search"
			if reflect.DeepEqual(event.Args, []string{"--version"}) {
				kind = "version"
			}
			helpers[event.Helper+"-"+kind]++
		}
	}
	wantPrompts := 1
	if f.correction {
		wantPrompts = 2
	}
	if actualVersion == "" || readiness != 1 || versions != 1 || rpc != 1 || len(prompts) != wantPrompts || accepted != wantPrompts || settled != wantPrompts || counts["exit"] != 2 || counts["stdin-eof"] != 1 || counts["network-check"] != 2 || counts["external-fetch"] != 2 {
		t.Errorf("version=%d rpc=%d prompts=%d accepted=%d settled=%d observations=%v", versions, rpc, len(prompts), accepted, settled, counts)
	}
	if len(prompts) > 0 && !strings.Contains(prompts[0], "ORIGINAL_GIT_SNAPSHOT") {
		t.Error("application did not supply original Git context")
	}
	if f.correction && len(prompts) == 2 && (!strings.Contains(prompts[1], "subjects") || !strings.Contains(prompts[1], "body")) {
		t.Error("correction omitted required response schema")
	}
	if !f.correction {
		if f.missing {
			if helpers["rg-search"] != 0 || helpers["fd-search"] != 0 || helpers["fdfind-search"] != 0 || helpers["rg-version"] < 1 || helpers["fd-version"] < 1 || helpers["fdfind-version"] < 1 || toolErrors != 2 {
				t.Errorf("missing-helper evidence: %v errors=%d", helpers, toolErrors)
			}
		} else if helpers["rg-search"] != 1 || helpers["fd-search"] != 1 || helpers["fd-version"] < 1 || helpers["rg-version"] != 0 || toolErrors != 0 {
			t.Errorf("available-helper evidence: %v errors=%d", helpers, toolErrors)
		}
	}
	sessions := 0
	err := filepath.WalkDir(f.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
			sessions++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Errorf("persisted session files=%d", sessions)
	}
	for _, name := range []string{"extension-executed", "mcp-executed", "fallback"} {
		if _, err := os.Stat(filepath.Join(f.root, name)); !os.IsNotExist(err) {
			t.Errorf("unexpected %s execution: %v", name, err)
		}
	}
	t.Logf("actual-version=%s version-probes=%d RPC=%d readiness=%d app-prompts=%d accepted=%d settled=%d API=%d helpers=%v tool-errors=%d sessions=%d exits=%d EOF=%d network-checks=%d external-fetches=0 owned-processes=0", actualVersion, versions, rpc, readiness, len(prompts), accepted, settled, len(requests), helpers, toolErrors, sessions, counts["exit"], counts["stdin-eof"], counts["network-check"])
}

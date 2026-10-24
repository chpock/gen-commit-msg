package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

const fixtureAnswer = `{"subjects":["fix(pi): preserve confirmed candidates"],"body":""}`

type fixtureRecord struct {
	Kind    string
	PID     int
	Args    []string
	Command map[string]any
}

func fixtureMark(kind string, command map[string]any) {
	f, err := os.OpenFile(filepath.Join(os.Getenv("GCM_PI_FIXTURE"), "events"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if err := json.NewEncoder(f).Encode(fixtureRecord{kind, os.Getpid(), args, command}); err != nil {
		panic(err)
	}
}

// The only executable used here is this Go test binary, never an installed Pi.
func TestPiFixture(t *testing.T) {
	dir := os.Getenv("GCM_PI_FIXTURE")
	if dir == "" {
		return
	}
	data, _ := os.ReadFile(filepath.Join(dir, "scenario"))
	scenario := string(data)
	args := os.Args
	version := args[len(args)-1] == "--version"
	if scenario == "descendant-child" {
		fixtureMark("descendant", nil)
		for {
			time.Sleep(time.Hour)
		}
	}
	if version {
		fixtureMark("version", nil)
		switch scenario {
		case "old":
			fmt.Println("1.3.9")
		case "unknown-version":
			fmt.Println("unrecognized")
		case "version-error":
			os.Exit(4)
		case "startup-budget":
			time.Sleep(90 * time.Millisecond)
			fmt.Println("1.4.0")
		case "version-hang":
			for {
				time.Sleep(time.Hour)
			}
		default:
			fmt.Println("1.4.0")
		}
		os.Exit(0)
	}
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fixtureMark("rpc", map[string]any{"cwd": cwd, "runtime_marker": os.Getenv("GCM_PI_RUNTIME_MARKER")})
	emit := func(v any) {
		b, _ := json.Marshal(v)
		b = []byte(strings.ReplaceAll(strings.ReplaceAll(string(b), `\u2028`, "\u2028"), `\u2029`, "\u2029"))
		if scenario == "split" {
			for len(b) > 0 {
				n := min(79, len(b))
				_, _ = os.Stdout.Write(b[:n])
				b = b[n:]
			}
		} else {
			_, _ = os.Stdout.Write(b)
		}
		fmt.Print("\r\n")
	}
	response := func(cmd map[string]any, success bool, disposition string) {
		emit(map[string]any{"type": "response", "id": cmd["id"], "command": cmd["type"], "success": success, "data": map[string]any{"disposition": disposition}, "error": "fixture preflight failure"})
	}
	assistant := func(text, stop string) {
		split := len(text) / 2
		for split > 0 && !utf8.RuneStart(text[split]) {
			split--
		}
		emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "thinking", "thinking": "not output"}, map[string]any{"type": "text", "text": text[:split]}, map[string]any{"type": "text", "text": text[split:]}}, "stopReason": stop, "errorMessage": "fixture provider failure"}})
	}
	switch scenario {
	case "startup-stderr", "startup-stderr-zero":
		fmt.Fprintln(os.Stderr, "fixture RPC initialization: selected model is unavailable")
		fixtureMark("stderr-written", nil)
		if scenario == "startup-stderr-zero" {
			os.Exit(0)
		}
		os.Exit(9)
	case "startup-stderr-large", "stderr-success", "stderr-force":
		fmt.Fprintln(os.Stderr, "fixture discarded stderr prefix")
		chunk := strings.Repeat("d", 8192)
		for range 1024 {
			fmt.Fprint(os.Stderr, chunk)
		}
		fmt.Fprintln(os.Stderr, "\nfixture retained stderr tail: model catalog initialization diagnostic")
		fixtureMark("stderr-written", map[string]any{"payload_bytes": 8 << 20})
		if scenario == "startup-stderr-large" {
			os.Exit(9)
		}
	}
	if scenario == "blocked-start" {
		for {
			time.Sleep(time.Hour)
		}
	}
	r := bufio.NewReader(os.Stdin)
	count := 0
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				panic(err)
			}
			break
		}
		var cmd map[string]any
		if err := json.Unmarshal(line, &cmd); err != nil {
			panic(err)
		}
		fixtureMark("command", cmd)
		if cmd["type"] == "get_state" {
			if scenario == "startup-budget" {
				fixtureMark("readiness-delay", nil)
				time.Sleep(90 * time.Millisecond)
			}
			if scenario == "readiness-no-data" {
				emit(map[string]any{"type": "response", "id": cmd["id"], "command": "get_state", "success": true})
				continue
			}
			if scenario == "readiness-hang" {
				for {
					time.Sleep(time.Hour)
				}
			}
			if scenario == "readiness-rejected" {
				response(cmd, false, "")
				continue
			}
			if scenario == "readiness-wrong-id" {
				emit(map[string]any{"type": "response", "id": "other", "command": "get_state", "success": true, "data": map[string]any{}})
				continue
			}
			response(cmd, true, "")
			if scenario == "duplex-backpressure" {
				fixtureMark("duplex-start", nil)
				for range 200 {
					emit(map[string]any{"type": "ignored", "data": strings.Repeat("x", 2048)})
				}
				fixtureMark("duplex-drained", nil)
			}
			if scenario == "blocked-stdin" {
				fixtureMark("not-reading", nil)
				for {
					time.Sleep(time.Hour)
				}
			}
			continue
		}
		count++
		if scenario == "reject" {
			response(cmd, false, "")
			continue
		}
		switch scenario {
		case "queued":
			response(cmd, true, "queued")
		case "unrelated-response":
			emit(map[string]any{"type": "response", "id": "irrelevant", "command": "get_state", "success": true})
			response(cmd, true, "started")
		}
		if scenario == "handled" {
			response(cmd, true, "handled")
			continue
		}
		if scenario == "acceptance-missing" {
			assistant(fixtureAnswer, "stop")
			emit(map[string]any{"type": "agent_settled", "aborted": false})
			continue
		}
		if scenario == "acceptance-no-data" {
			emit(map[string]any{"type": "response", "id": cmd["id"], "command": "prompt", "success": true})
			continue
		}
		if scenario == "acceptance-no-id" {
			emit(map[string]any{"type": "response", "command": "prompt", "success": true, "data": map[string]any{"disposition": "started"}})
			continue
		}
		if scenario == "parse-response" {
			emit(map[string]any{"type": "response", "command": "parse", "success": false, "error": "bad command"})
			continue
		}
		if scenario == "acceptance-malformed" {
			emit(map[string]any{"type": "response", "id": cmd["id"], "command": "prompt", "success": nil})
			continue
		}
		if scenario == "wrong-command" {
			emit(map[string]any{"type": "response", "id": cmd["id"], "command": "get_state", "success": true})
			continue
		}
		if scenario == "delayed-acceptance" {
			assistant(fixtureAnswer, "stop")
			emit(map[string]any{"type": "agent_settled", "aborted": false})
			response(cmd, true, "started")
			continue
		}
		if scenario != "queued" && scenario != "unrelated-response" {
			response(cmd, true, "started")
		}
		switch scenario {
		case "malformed":
			fmt.Println("not JSON")
			continue
		case "no-type":
			fmt.Println(`{"arbitrary":true}`)
			continue
		case "array-record":
			fmt.Println(`[]`)
			continue
		case "message-missing":
			emit(map[string]any{"type": "message_end"})
			continue
		case "role-missing":
			emit(map[string]any{"type": "message_end", "message": map[string]any{"content": []any{}}})
			continue
		case "content-malformed":
			emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "content": "text", "stopReason": "stop"}})
			continue
		case "text-missing":
			emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text"}}, "stopReason": "stop"}})
			continue
		case "settlement-missing-field":
			assistant(fixtureAnswer, "stop")
			emit(map[string]any{"type": "agent_settled"})
			continue
		case "settlement-null":
			assistant(fixtureAnswer, "stop")
			emit(map[string]any{"type": "agent_settled", "aborted": nil})
			continue
		case "missing-response":
			emit(map[string]any{"type": "agent_settled", "aborted": false})
			continue
		case "signal-int", "signal-term":
			fixtureMark("signal", nil)
			signal := syscall.SIGINT
			if scenario == "signal-term" {
				signal = syscall.SIGTERM
			}
			_ = syscall.Kill(os.Getpid(), signal)
			for {
				time.Sleep(time.Hour)
			}
		case "premature-exit":
			os.Exit(6)
		case "partial-record":
			fmt.Print(`{"type":"agent_settled"}`)
			os.Exit(0)
		case "failure-force":
			assistant("provider failed", "error")
			emit(map[string]any{"type": "agent_settled", "aborted": false})
			continue
		case "queue-after-failure":
			fmt.Println("not JSON")
			for range 2000 {
				emit(map[string]any{"type": "ignored", "data": strings.Repeat("x", 2048)})
			}
			fixtureMark("queue-drained", nil)
			continue
		case "progress-forever":
			fixtureMark("native-progress", nil)
			for {
				emit(map[string]any{"type": "auto_retry_start"})
				time.Sleep(5 * time.Millisecond)
			}
		case "native-recovery":
			emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "user", "content": "echo"}})
			emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "custom", "content": "ignored"}})
			assistant(fixtureAnswer, "toolUse")
			emit(map[string]any{"type": "agent_end", "willRetry": true})
			emit(map[string]any{"type": "auto_retry_start", "attempt": 1, "maxAttempts": 3, "delayMs": 10, "errorMessage": "fixture transient provider overload"})
			emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "stopReason": "toolUse", "content": []any{map[string]any{"type": "toolCall", "id": "native-read", "name": "read", "arguments": map[string]any{"path": "file.go"}}}}})
			emit(map[string]any{"type": "tool_execution_end", "toolName": "grep", "toolCallId": "native-grep-1", "isError": true, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "fixture native grep: rg helper missing"}, map[string]any{"type": "image", "data": "fixture-private-payload"}}, "details": map[string]any{"private": "fixture-private-payload"}}})
			assistant("failed native attempt", "error")
			emit(map[string]any{"type": "agent_end", "willRetry": true})
			emit(map[string]any{"type": "compaction_start", "reason": "overflow"})
			emit(map[string]any{"type": "compaction_end", "reason": "overflow", "aborted": false, "willRetry": false, "errorMessage": "fixture compaction: overflow recovery failed", "private": "fixture-private-payload"})
			emit(map[string]any{"type": "compaction_end", "reason": "threshold", "aborted": true})
			emit(map[string]any{"type": "summarization_retry_scheduled", "attempt": 1, "maxAttempts": 3, "delayMs": 10, "errorMessage": "fixture summarization connection reset"})
			emit(map[string]any{"type": "auto_retry_end", "success": false, "attempt": 1, "finalError": "fixture retry attempt exhausted"})
			emit(map[string]any{"type": "auto_retry_end", "success": true, "attempt": 2})
		case "native-diagnostic-optional":
			for _, detail := range []any{nil, 42, map[string]any{"unexpected": "fixture-private-payload"}} {
				emit(map[string]any{"type": "auto_retry_start", "errorMessage": detail, "attempt": "invalid optional metadata"})
				emit(map[string]any{"type": "summarization_retry_scheduled", "errorMessage": detail})
				emit(map[string]any{"type": "compaction_end", "errorMessage": detail})
				emit(map[string]any{"type": "auto_retry_end", "success": false, "finalError": detail})
				emit(map[string]any{"type": "tool_execution_end", "isError": true, "result": detail})
			}
		}
		text := fixtureAnswer
		if scenario == "correct-two" && count == 1 || scenario == "correct-three" && count < 3 {
			text = `{"subjects":[],"body":""}`
		}
		if scenario == "invalid-three" {
			text = fmt.Sprintf(`{"subjects":[],"body":"","attempt%d":true}`, count)
		}
		if scenario == "independent-prompts" {
			time.Sleep(90 * time.Millisecond)
			if count < 3 {
				text = `{"subjects":[],"body":""}`
			}
		}
		if scenario == "split" {
			emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "user", "content": strings.Repeat("x", 150000) + "\u2028\u2029"}})
			text = `{"subjects":["fix: literal ` + "\u2028\u2029" + ` separators"],"body":""}`
		}
		stop := "stop"
		switch scenario {
		case "provider", "latest-error":
			stop = "error"
		case "length":
			stop = "length"
		case "aborted":
			stop = "aborted"
		case "deferred":
			stop = "deferred"
		case "pending":
			stop = "pending"
		case "stop-missing":
			stop = ""
		}
		if scenario == "latest-error" {
			assistant(fixtureAnswer, "stop")
		}
		assistant(text, stop)
		emit(map[string]any{"type": "agent_end", "willRetry": false})
		if scenario == "no-settlement" {
			continue
		}
		emit(map[string]any{"type": "agent_settled", "aborted": scenario == "settlement-aborted"})
		fixtureMark("settled", nil)
		if scenario == "exit-after-success" {
			os.Exit(7)
		}
		if scenario == "descendant" {
			child := exec.Command(os.Args[0], "-test.run=^TestPiFixture$", "--", "child")
			child.Env = append(os.Environ(), "GCM_PI_FIXTURE="+dir)
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := os.WriteFile(filepath.Join(dir, "scenario"), []byte("descendant-child"), 0600); err != nil {
				panic(err)
			}
			if err := child.Start(); err != nil {
				panic(err)
			}
			fixtureMark("spawned", map[string]any{"pid": child.Process.Pid})
			os.Exit(0)
		}
	}
	fixtureMark("eof", nil)
	if scenario == "force" || scenario == "cancel-cleanup" || scenario == "failure-force" || scenario == "stderr-force" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if scenario == "pipe-activity" {
		for range 2000 {
			emit(map[string]any{"type": "ignored", "data": strings.Repeat("x", 1024)})
			fmt.Fprintln(os.Stderr, strings.Repeat("z", 1024))
		}
	}
	fixtureMark("disposed", nil)
	os.Exit(0)
}

func newFixture(t *testing.T, scenario string) (*Generator, string) {
	t.Helper()
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "pi-fixture")
	text := "#!/bin/sh\nexec '" + strings.ReplaceAll(binary, "'", "'\\''") + "' -test.run='^TestPiFixture$' -- \"$@\"\n"
	if err := os.WriteFile(script, []byte(text), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario"), []byte(scenario), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GCM_PI_FIXTURE", dir)
	t.Setenv("HOME", dir)
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	t.Setenv("PATH", dir)
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	t.Setenv("GCM_PI_RUNTIME_MARKER", "retained runtime environment")
	g := NewGenerator(GeneratorConfig{Executable: script, RepoDir: dir})
	g.startupTimeout = time.Second
	g.promptTimeout = time.Second
	g.shutdownTimeout = 100 * time.Millisecond
	return g, dir
}
func fixtureEvents(t *testing.T, dir string) []fixtureRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "events"))
	if err != nil {
		t.Fatal(err)
	}
	var events []fixtureRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e fixtureRecord
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	return events
}
func requestFixture() generation.Request {
	return generation.Request{SubjectMin: 1, SubjectMax: 1, ContextJSON: `{"snapshot":"original staged changes"}`}
}
func assertReaped(t *testing.T, events []fixtureRecord) {
	t.Helper()
	for _, e := range events {
		if e.Kind == "rpc" || e.Kind == "version" {
			var status syscall.WaitStatus
			pid, err := syscall.Wait4(e.PID, &status, syscall.WNOHANG, nil)
			if !errors.Is(err, syscall.ECHILD) {
				t.Errorf("child not already reaped: pid=%d wait=%d err=%v", e.PID, pid, err)
			}
		}
	}
}

func TestGeneratorPromptsAndLifecycle(t *testing.T) {
	for _, tc := range []struct {
		scenario  string
		count     int
		diagnosis string
	}{{"valid", 1, ""}, {"correct-two", 2, ""}, {"correct-three", 3, ""}, {"invalid-three", 3, "attempt3"}, {"native-recovery", 1, ""}, {"queued", 1, ""}, {"unrelated-response", 1, ""}, {"delayed-acceptance", 1, ""}, {"split", 1, ""}, {"pipe-activity", 1, ""}} {
		t.Run(tc.scenario, func(t *testing.T) {
			g, dir := newFixture(t, tc.scenario)
			if tc.scenario == "pipe-activity" {
				g.shutdownTimeout = 2 * time.Second
			}
			var updates []generation.Update
			var _ generation.Generator = g
			if !reflect.DeepEqual(g.Steps(), []string{"Starting Pi...", "Generating commit messages...", "Stopping Pi..."}) {
				t.Fatal(g.Steps())
			}
			result, err := g.Generate(context.Background(), requestFixture(), func(u generation.Update) { updates = append(updates, u) })
			if tc.diagnosis != "" {
				if err == nil || !strings.Contains(err.Error(), tc.diagnosis) || len(result.Messages) != 0 {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			} else if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.scenario == "split" && result.Messages[0].Subject != "fix: literal \u2028\u2029 separators" {
				t.Fatalf("Unicode/content changed: %q", result.Messages[0].Subject)
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			prompts := 0
			rpc := 0
			versions := 0
			ids := map[string]bool{}
			settled := 0
			for _, e := range events {
				switch e.Kind {
				case "version":
					versions++
				case "rpc":
					rpc++
					if e.Command["cwd"] != dir || e.Command["runtime_marker"] != "retained runtime environment" {
						t.Fatal("repository cwd or runtime environment not retained")
					}
					if !reflect.DeepEqual(e.Args, []string{"--mode", "rpc", "--no-session", "--no-extensions", "--no-mcp", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--offline", "--tools", "read,grep,find,ls", "--system-prompt", generation.Instructions, "--append-system-prompt", ""}) {
						t.Fatalf("unexpected invocation: %q", e.Args)
					}
				case "settled":
					settled++
				case "command":
					id, _ := e.Command["id"].(string)
					if id == "" || ids[id] {
						t.Fatalf("nonunique ID %q", id)
					}
					ids[id] = true
					if e.Command["type"] == "prompt" {
						prompts++
						if prompts > 1 && settled < prompts-1 {
							t.Fatal("correction before settlement")
						}
						message := e.Command["message"].(string)
						schema, _ := json.Marshal(generation.ResponseSchema(requestFixture()))
						if !strings.HasSuffix(message, string(schema)) {
							t.Fatal("response schema changed between prompts")
						}
						if prompts > 1 && tc.scenario == "invalid-three" && !strings.Contains(message, fmt.Sprintf("attempt%d", prompts-1)) {
							t.Fatal("feedback omitted actual latest validation diagnosis")
						}
						if prompts > 1 && tc.scenario != "invalid-three" && !strings.Contains(message, "subjects count must be between 1 and 1 inclusive, got 0") {
							t.Fatal("feedback omitted actual count diagnosis")
						}
						if prompts == 1 {
							if !strings.Contains(message, requestFixture().ContextJSON) || !strings.Contains(message, generation.BuildPrompt(requestFixture())) {
								t.Fatal("missing original snapshot/prompt")
							}
						} else if !strings.Contains(message, "subjects") || !strings.Contains(message, "schema") {
							t.Fatalf("missing actual feedback/schema: %s", message)
						}
					}
				}
			}
			if prompts != tc.count || rpc != 1 || versions != 1 {
				t.Fatalf("prompts=%d rpc=%d version=%d", prompts, rpc, versions)
			}
			if len(updates) == 0 || updates[len(updates)-1].Index != 2 || updates[len(updates)-1].Status != generation.StepDone {
				t.Fatalf("updates=%+v", updates)
			}
		})
	}
}

func TestGeneratorRuntimeFailuresNeverCorrect(t *testing.T) {
	for _, scenario := range []string{"reject", "handled", "provider", "latest-error", "length", "aborted", "deferred", "pending", "stop-missing", "missing-response", "premature-exit", "partial-record", "malformed", "no-type", "array-record", "message-missing", "role-missing", "content-malformed", "text-missing", "settlement-missing-field", "settlement-null", "settlement-aborted", "acceptance-malformed", "acceptance-no-data", "acceptance-no-id", "parse-response", "signal-int", "signal-term", "wrong-command", "no-settlement", "acceptance-missing", "progress-forever"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			g.promptTimeout = 80 * time.Millisecond
			result, err := g.Generate(context.Background(), requestFixture(), nil)
			if err == nil || len(result.Messages) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			count := 0
			for _, e := range events {
				if e.Command["type"] == "prompt" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("prompt count=%d", count)
			}
			switch scenario {
			case "no-settlement", "acceptance-missing", "acceptance-no-id", "progress-forever":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected independent prompt deadline: %v", err)
				}
			case "provider", "latest-error":
				if !strings.Contains(err.Error(), "final assistant response stopped with error") {
					t.Fatalf("lost provider failure: %v", err)
				}
			}
		})
	}
}

func TestGeneratorVersionAndReadiness(t *testing.T) {
	for _, scenario := range []string{"old", "unknown-version", "version-error", "version-hang", "readiness-hang", "readiness-rejected", "readiness-wrong-id", "readiness-no-data", "blocked-start"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			g.startupTimeout = 100 * time.Millisecond
			result, err := g.Generate(context.Background(), requestFixture(), nil)
			if err == nil || len(result.Messages) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			for _, e := range events {
				if e.Command["type"] == "prompt" {
					t.Fatal("Git prompt before verified readiness")
				}
				if (scenario == "old" || scenario == "unknown-version" || scenario == "version-error" || scenario == "version-hang") && e.Kind == "rpc" {
					t.Fatal("RPC before version verification")
				}
			}
		})
	}
}

func TestGeneratorCleanupWarningsAndCancellation(t *testing.T) {
	for _, scenario := range []string{"exit-after-success", "force", "descendant", "cancel-cleanup"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result, err := g.Generate(ctx, requestFixture(), func(u generation.Update) {
				if scenario == "cancel-cleanup" && u.Index == 2 && u.Status == generation.StepRunning {
					cancel()
				}
			})
			if scenario == "cancel-cleanup" {
				if !errors.Is(err, context.Canceled) || len(result.Messages) != 0 {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			} else if err != nil || len(result.Messages) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(result.Warnings) == 0 {
				t.Fatal("missing cleanup warning")
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			if scenario == "force" || scenario == "cancel-cleanup" {
				found := false
				for _, e := range events {
					found = found || e.Kind == "eof"
				}
				if !found {
					t.Fatal("forced before stdin EOF grace")
				}
			}
		})
	}
}

func TestGeneratorIndependentPromptBudgets(t *testing.T) {
	g, dir := newFixture(t, "independent-prompts")
	g.promptTimeout = 180 * time.Millisecond
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err != nil || len(result.Messages) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertReaped(t, fixtureEvents(t, dir))
}

func TestGeneratorBlockedInputIsTerminated(t *testing.T) {
	g, dir := newFixture(t, "blocked-stdin")
	g.promptTimeout = 80 * time.Millisecond
	req := requestFixture()
	req.ContextJSON = strings.Repeat("x", 2<<20)
	result, err := g.Generate(context.Background(), req, nil)
	if !errors.Is(err, context.DeadlineExceeded) || len(result.Messages) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	found := false
	for _, e := range events {
		found = found || e.Kind == "not-reading"
	}
	if !found {
		t.Fatal("fixture did not reach blocked-input seam")
	}
}

func TestGeneratorCanceledPromptGetsIndependentCleanup(t *testing.T) {
	g, dir := newFixture(t, "no-settlement")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			data, _ := os.ReadFile(filepath.Join(dir, "events"))
			if strings.Contains(string(data), `"type":"prompt"`) {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	result, err := g.Generate(ctx, requestFixture(), nil)
	<-done
	if !errors.Is(err, context.Canceled) || len(result.Messages) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	disposed := false
	for _, e := range events {
		disposed = disposed || e.Kind == "disposed"
	}
	if !disposed {
		t.Fatal("cancellation skipped independent graceful disposal")
	}
}

func TestGeneratorStartFailure(t *testing.T) {
	g := NewGenerator(GeneratorConfig{Executable: filepath.Join(t.TempDir(), "missing"), RepoDir: t.TempDir()})
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err == nil || len(result.Messages) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestGeneratorDrainsWhileWritingLargePrompt(t *testing.T) {
	g, dir := newFixture(t, "duplex-backpressure")
	req := requestFixture()
	req.ContextJSON = strings.Repeat("snapshot", 300000)
	result, err := g.Generate(context.Background(), req, nil)
	if err != nil || len(result.Messages) != 1 {
		t.Fatalf("duplex result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	drained := false
	prompts := 0
	for _, e := range events {
		if e.Kind == "duplex-drained" {
			drained = true
		}
		if e.Command["type"] == "prompt" {
			prompts++
			if !drained {
				t.Fatal("prompt consumed before output drain handshake")
			}
		}
	}
	if prompts != 1 {
		t.Fatalf("prompts=%d", prompts)
	}
}

func TestGeneratorCleanupDrainsAbandonedQueue(t *testing.T) {
	g, dir := newFixture(t, "queue-after-failure")
	g.shutdownTimeout = 2 * time.Second
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err == nil || !strings.Contains(err.Error(), "protocol") || len(result.Messages) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	drained, disposed := false, false
	for _, e := range events {
		drained = drained || e.Kind == "queue-drained"
		disposed = disposed || e.Kind == "disposed"
	}
	if !drained || !disposed {
		t.Fatalf("drained=%t disposed=%t", drained, disposed)
	}
}

func TestGeneratorPrimaryFailureSurvivesCleanupCancellation(t *testing.T) {
	g, dir := newFixture(t, "failure-force")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := g.Generate(ctx, requestFixture(), func(u generation.Update) {
		if u.Index == 2 && u.Status == generation.StepRunning {
			cancel()
		}
	})
	if err == nil || !strings.Contains(err.Error(), "final assistant response stopped with error") || len(result.Messages) != 0 || len(result.Warnings) == 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertReaped(t, fixtureEvents(t, dir))
}

func TestGeneratorStartupHasOneBudget(t *testing.T) {
	g, dir := newFixture(t, "startup-budget")
	g.startupTimeout = 150 * time.Millisecond
	g.shutdownTimeout = time.Second
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if !errors.Is(err, context.DeadlineExceeded) || len(result.Messages) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	readiness := false
	for _, e := range events {
		readiness = readiness || e.Kind == "readiness-delay"
		if e.Command["type"] == "prompt" {
			t.Fatal("startup budget renewed after version")
		}
	}
	if !readiness {
		t.Fatal("did not reach readiness phase within shared budget")
	}
}

func TestGeneratorCancellationDuringStartupReapsVersion(t *testing.T) {
	g, dir := newFixture(t, "version-hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	guard := time.After(3 * time.Second)
	canceled := make(chan bool, 1)
	go func() {
		for {
			data, _ := os.ReadFile(filepath.Join(dir, "events"))
			if strings.Contains(string(data), `"Kind":"version"`) {
				cancel()
				canceled <- true
				return
			}
			select {
			case <-guard:
				cancel()
				canceled <- false
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	result, err := g.Generate(ctx, requestFixture(), nil)
	if !<-canceled {
		t.Fatal("fixture version handshake not reached")
	}
	if !errors.Is(err, context.Canceled) || len(result.Messages) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	for _, e := range events {
		if e.Kind == "rpc" {
			t.Fatal("started RPC after startup cancellation")
		}
	}
}

func TestGeneratorProductionBudgetsAndSupportedVersion(t *testing.T) {
	g := NewGenerator(GeneratorConfig{})
	if g.startupTimeout != 30*time.Second || g.promptTimeout != 120*time.Second || g.shutdownTimeout != 5*time.Second {
		t.Fatal("production budgets changed")
	}
	for _, version := range []string{"1.4.0", "1.4.99", "1.5.0", "2.0.0", "1.4.0+build.1", " 1.4.0\n"} {
		if err := supportedVersion(version); err != nil {
			t.Errorf("%q rejected: %v", version, err)
		}
	}
	for _, version := range []string{"1.3.999", "0.99.0", "1.4", "pi 1.4.0", "1.4.0\n1.5.0", "1.4.0-beta", "1.04.0", "999999999999999999999.4.0"} {
		if err := supportedVersion(version); err == nil {
			t.Errorf("%q accepted", version)
		}
	}
}

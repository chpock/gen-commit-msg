package pi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

func capturePiLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

func piLogRecords(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(logs.Bytes()))
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func TestProcessStartupStderrDiagnostics(t *testing.T) {
	for _, scenario := range []string{"startup-stderr", "startup-stderr-zero", "startup-stderr-large"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			g.startupTimeout = 3 * time.Second
			g.shutdownTimeout = 2 * time.Second
			logs := capturePiLogs(t)
			normal, err := os.Create(filepath.Join(dir, "normal-output"))
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr := os.Stdout, os.Stderr
			os.Stdout, os.Stderr = normal, normal
			t.Cleanup(func() { os.Stdout, os.Stderr = stdout, stderr; _ = normal.Close() })
			var updates []generation.Update
			result, err := g.Generate(context.Background(), requestFixture(), func(update generation.Update) { updates = append(updates, update) })
			if err == nil || len(result.Messages) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			expected := "fixture RPC initialization: selected model is unavailable"
			if scenario == "startup-stderr-large" {
				expected = "fixture retained stderr tail: model catalog initialization diagnostic"
			}
			if strings.Contains(fmt.Sprint(result, err, updates), expected) {
				t.Fatal("raw stderr leaked into result/progress errors")
			}
			output, readErr := os.ReadFile(normal.Name())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(output) != 0 {
				t.Fatalf("unexpected normal output: %q", output)
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			written := false
			for _, event := range events {
				if event.Kind == "stderr-written" {
					written = true
				}
				if event.Command["type"] == "prompt" {
					t.Fatal("prompt sent before readiness")
				}
			}
			if !written {
				t.Fatal("stderr fixture did not complete its write")
			}
			found := false
			for _, record := range piLogRecords(t, logs) {
				tail, ok := record["stderr_tail"].(string)
				if !ok {
					continue
				}
				if strings.Contains(tail, expected) {
					found = true
					if record["level"] != "ERROR" {
						t.Errorf("startup failure diagnostics must be ERROR: %v", record["level"])
					}
					if len(tail) > 16*1024 {
						t.Fatalf("unbounded stderr evidence: %d bytes", len(tail))
					}
					if scenario == "startup-stderr-large" {
						if record["stderr_truncated"] != true || record["stderr_bytes"].(float64) <= 8<<20 {
							t.Fatal("missing truncation/full-drain evidence")
						}
						if strings.Contains(tail, "fixture discarded stderr prefix") {
							t.Fatal("did not retain bounded recent evidence")
						}
					}
				}
			}
			if !found {
				t.Fatalf("startup explanation missing from structured logs: %s", expected)
			}
		})
	}
}

func TestProcessStderrCapturePreservesSuccessAndForcedCleanup(t *testing.T) {
	for _, scenario := range []string{"stderr-success", "stderr-force"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			g.startupTimeout = 3 * time.Second
			logs := capturePiLogs(t)
			result, err := g.Generate(context.Background(), requestFixture(), nil)
			if err != nil || len(result.Messages) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if (scenario == "stderr-force") != (len(result.Warnings) > 0) {
				t.Fatalf("cleanup warnings changed: %v", result.Warnings)
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			prompts := 0
			written, eof, disposed := false, false, false
			for _, event := range events {
				if event.Command["type"] == "prompt" {
					prompts++
				}
				written = written || event.Kind == "stderr-written"
				eof = eof || event.Kind == "eof"
				disposed = disposed || event.Kind == "disposed"
			}
			if prompts != 1 || !written || !eof || scenario == "stderr-success" && !disposed {
				t.Fatalf("prompts=%d written=%t eof=%t disposed=%t", prompts, written, eof, disposed)
			}
			found := false
			for _, record := range piLogRecords(t, logs) {
				tail, ok := record["stderr_tail"].(string)
				if !ok {
					continue
				}
				found = true
				if len(tail) > 16*1024 || !strings.Contains(tail, "fixture retained stderr tail") || record["stderr_truncated"] != true || record["stderr_bytes"].(float64) <= 8<<20 {
					t.Fatal("stderr was not bounded and continuously drained")
				}
				level := "DEBUG"
				if scenario == "stderr-force" {
					level = "WARN"
				}
				if record["level"] != level {
					t.Fatalf("unexpected diagnostic severity: %v", record["level"])
				}
			}
			if !found {
				t.Fatal("bounded stderr capture missing")
			}
		})
	}
}

func TestProtocolRecoveredFailureDiagnostics(t *testing.T) {
	g, dir := newFixture(t, "native-recovery")
	logs := capturePiLogs(t)
	var updates []generation.Update
	result, err := g.Generate(context.Background(), requestFixture(), func(update generation.Update) { updates = append(updates, update) })
	if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	prompts := 0
	for _, event := range events {
		if event.Command["type"] == "prompt" {
			prompts++
		}
	}
	if prompts != 1 {
		t.Fatalf("native failures caused application corrections: %d prompts", prompts)
	}
	for _, update := range updates {
		if strings.Contains(update.Detail, "Correcting") || update.Err != nil {
			t.Fatalf("native failure changed progress outcome: %+v", update)
		}
	}
	expected := []string{"fixture provider failure", "fixture native grep: rg helper missing", "fixture transient provider overload", "fixture compaction: overflow recovery failed", "fixture summarization connection reset", "fixture retry attempt exhausted"}
	records := piLogRecords(t, logs)
	for _, diagnostic := range expected {
		found := false
		for _, record := range records {
			if record["error"] == diagnostic {
				found = true
				if record["level"] != "WARN" {
					t.Errorf("recovered failure must be WARN, got %v for %s", record["level"], diagnostic)
				}
			}
		}
		if !found {
			t.Errorf("recovered diagnostic missing: %s", diagnostic)
		}
	}
	for _, record := range records {
		if record["error"] == "fixture native grep: rg helper missing" && record["tool_name"] != "grep" {
			t.Error("tool diagnostic lost tool identity")
		}
		if record["level"] == "ERROR" {
			t.Errorf("recovered failure was treated as terminal: %v", record["msg"])
		}
	}
	for _, private := range []string{"fixture-private-payload", requestFixture().ContextJSON, fixtureAnswer, generation.Instructions} {
		if strings.Contains(logs.String(), private) {
			t.Fatal("logged full protocol/input/private payload")
		}
	}
}

func TestProtocolOptionalDiagnosticsRemainTolerable(t *testing.T) {
	g, dir := newFixture(t, "native-diagnostic-optional")
	logs := capturePiLogs(t)
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 0 {
		t.Fatalf("optional diagnostics changed acceptance: result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	prompts := 0
	for _, event := range events {
		if event.Command["type"] == "prompt" {
			prompts++
		}
	}
	if prompts != 1 {
		t.Fatalf("prompts=%d", prompts)
	}
	if strings.Contains(logs.String(), "fixture-private-payload") {
		t.Fatal("dumped malformed optional diagnostic payload")
	}
}

func TestStderrTailRetainsBoundedRecentEvidence(t *testing.T) {
	for _, chunks := range [][]string{
		{""}, {"small diagnostic"}, {strings.Repeat("a", 16*1024)},
		{strings.Repeat("a", 40*1024), "last diagnostic"},
		{strings.Repeat("a", 12000), strings.Repeat("b", 12000), "last diagnostic"},
	} {
		t.Run(fmt.Sprintf("%d-chunks-%d-bytes", len(chunks), len(strings.Join(chunks, ""))), func(t *testing.T) {
			var tail stderrTail
			full := ""
			for _, chunk := range chunks {
				n, err := tail.Write([]byte(chunk))
				if err != nil || n != len(chunk) {
					t.Fatalf("write=%d err=%v", n, err)
				}
				full += chunk
			}
			expected := full
			if len(expected) > 16*1024 {
				expected = expected[len(expected)-16*1024:]
			}
			if tail.total != int64(len(full)) || tail.size > 16*1024 || string(tail.data[:tail.size]) != expected {
				t.Fatal("tail lost recent bytes or exceeded bound")
			}
		})
	}
}

func TestProtocolDiagnosticTextIsBounded(t *testing.T) {
	logs := capturePiLogs(t)
	record := rpcRecord{"isError": json.RawMessage(`true`), "toolName": json.RawMessage(`"grep"`)}
	raw, err := json.Marshal(map[string]any{"content": []any{map[string]any{"type": "text", "text": strings.Repeat("a", 10000)}, map[string]any{"type": "text", "text": strings.Repeat("b", 30000)}}})
	if err != nil {
		t.Fatal(err)
	}
	record["result"] = raw
	logNativeDiagnostic("tool_execution_end", record, "prompt-1")
	records := piLogRecords(t, logs)
	if len(records) != 1 {
		t.Fatalf("diagnostic records=%d", len(records))
	}
	for _, entry := range records {
		detail, _ := entry["error"].(string)
		if len(detail) > 16*1024 || !strings.Contains(detail, strings.Repeat("a", 10000)) || !strings.Contains(detail, "bbb") {
			t.Fatal("selected tool diagnostic not preserved within bound")
		}
	}
}

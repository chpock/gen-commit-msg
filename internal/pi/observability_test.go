package pi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/logging"
)

func emitFixtureActivity(emit func(any)) {
	for _, kind := range []string{"agent_start", "turn_start"} {
		emit(map[string]any{"type": kind})
	}
	emit(map[string]any{"type": "message_start", "message": map[string]any{"role": "assistant"}})
	for _, kind := range []string{"thinking_start", "thinking_delta", "thinking_end", "text_start", "text_delta", "text_end", "toolcall_start", "toolcall_delta", "toolcall_end"} {
		update := map[string]any{"type": kind, "contentIndex": 0, "delta": "stream-private-payload", "content": "block-private-payload"}
		if kind == "toolcall_start" {
			update["id"], update["toolName"] = "fixture-tool", "read"
		}
		emit(map[string]any{"type": "message_update", "assistantMessageEvent": update})
	}
	emit(map[string]any{"type": "tool_execution_start", "toolName": "read", "toolCallId": "fixture-tool", "args": map[string]string{"path": "args-private-payload"}})
	emit(map[string]any{"type": "tool_execution_update", "toolName": "read", "toolCallId": "fixture-tool", "partialResult": map[string]any{"content": []any{map[string]any{"type": "text", "text": "partial-private-payload"}}}})
	emit(map[string]any{"type": "tool_execution_end", "toolName": "read", "toolCallId": "fixture-tool", "isError": false, "durationMs": 17, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "result-private-payload"}}}})
	emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "stopReason": "toolUse", "content": []any{}, "model": "fixture-model", "provider": "fixture-provider", "usage": map[string]any{"input": 120, "output": 30, "cacheRead": 40, "totalTokens": 190, "cost": map[string]any{"total": 0.25}}}})
	emit(map[string]any{"type": "turn_end", "message": map[string]any{"stopReason": "toolUse"}})
	emit(map[string]any{"type": "future_event", "data": "future-private-payload"})
}

func TestPiObservabilityLevels(t *testing.T) {
	for _, level := range []slog.Level{logging.LevelTrace, slog.LevelDebug, slog.LevelInfo} {
		t.Run(level.String(), func(t *testing.T) {
			g, dir := newFixture(t, "observability")
			t.Setenv("PI_TIMING", "operator-setting")
			logs := capturePiLogs(t, level)
			var updates []generation.Update
			result, err := g.Generate(context.Background(), requestFixture(), func(u generation.Update) { updates = append(updates, u) })
			if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			assertReaped(t, fixtureEvents(t, dir))
			records := piLogRecords(t, logs)
			found := map[string]bool{}
			var wire strings.Builder
			for _, r := range records {
				if event, ok := r["event"].(string); ok {
					found[event] = true
				}
				if data, ok := r["data"].(string); ok {
					wire.WriteString(data)
				}
				if r["msg"] == "Pi session state" && r["model"] == "fixture-model" && r["thinking_level"] == "high" {
					found["state"] = true
				}
				if r["msg"] == "Pi session statistics" {
					contextUsage, _ := r["context_usage"].(map[string]any)
					tokens, _ := r["tokens"].(map[string]any)
					found["stats"] = contextUsage["percent"] == float64(19) && contextUsage["tokens"] == float64(190) && tokens["cacheRead"] == float64(40) && r["cost"] == 0.25
				}
				if r["msg"] == "Pi assistant usage" {
					usage, _ := r["usage"].(map[string]any)
					cost, _ := usage["cost"].(map[string]any)
					found["usage"] = usage["input"] == float64(120) && usage["output"] == float64(30) && usage["cacheRead"] == float64(40) && cost["total"] == 0.25
				}
				if r["event"] == "tool_execution_end" && r["duration_ms"] == float64(17) && r["tool_name"] == "read" {
					found["tool_duration"] = true
				}
			}
			if level <= slog.LevelDebug {
				for _, event := range []string{"state", "stats", "usage", "agent_start", "turn_start", "message_start", "thinking_start", "thinking_end", "text_start", "text_end", "toolcall_start", "toolcall_end", "tool_execution_start", "tool_execution_end", "tool_duration", "agent_end"} {
					if !found[event] {
						t.Errorf("missing DEBUG activity: %s", event)
					}
				}
			} else if len(found) != 0 {
				t.Errorf("debug activity at INFO: %v", found)
			}
			payloadText := piLogPayloadText(records)
			for _, payload := range []string{"stream-private-payload", "block-private-payload", "args-private-payload", "partial-private-payload", "result-private-payload", "future-private-payload", "shutdown-private-payload", "unterminated-shutdown", "not output", requestFixture().ContextJSON, generation.Instructions} {
				if got := strings.Contains(payloadText, payload); got != (level == logging.LevelTrace) {
					t.Errorf("payload visibility at %s: %q = %t", level, payload, got)
				}
			}
			if level == logging.LevelTrace && !strings.Contains(wire.String(), "unterminated-shutdown") {
				t.Error("shutdown partial record missing from transport trace")
			}
			stats := 0
			for _, e := range fixtureEvents(t, dir) {
				if e.Kind == "rpc" {
					want := "operator-setting"
					if level == logging.LevelTrace {
						want = "1"
					}
					if e.Command["timing"] != want {
						t.Errorf("native startup timing=%v; want %s", e.Command["timing"], want)
					}
				}
				if e.Command["type"] == "get_session_stats" {
					stats++
				}
			}
			wantStats := 0
			if level <= slog.LevelDebug {
				wantStats = 1
			}
			if stats != wantStats {
				t.Errorf("stats calls=%d at %s; want %d", stats, level, wantStats)
			}
			if strings.Contains(string(mustJSON(t, updates)), "private-payload") {
				t.Fatal("diagnostics leaked into progress")
			}
		})
	}
}

func piLogPayloadText(records []map[string]any) string {
	var text strings.Builder
	var collect func(any)
	collect = func(value any) {
		switch value := value.(type) {
		case string:
			text.WriteString(value)
			var nested any
			if json.Unmarshal([]byte(value), &nested) == nil {
				switch nested.(type) {
				case map[string]any, []any:
					collect(nested)
				}
			}
		case map[string]any:
			for _, v := range value {
				collect(v)
			}
		case []any:
			for _, v := range value {
				collect(v)
			}
		}
	}
	for _, record := range records {
		collect(record)
	}
	return text.String()
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPiStatisticsFailurePreservesCandidates(t *testing.T) {
	for _, scenario := range []string{"stats-hang", "stats-rejected", "stats-malformed"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			g.promptTimeout = 500 * time.Millisecond
			logs := capturePiLogs(t)
			result, err := g.Generate(context.Background(), requestFixture(), nil)
			if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 0 {
				t.Fatalf("diagnostic failure changed outcome: %+v %v", result, err)
			}
			assertReaped(t, fixtureEvents(t, dir))
			if !strings.Contains(logs.String(), "Pi session statistics unavailable") {
				t.Fatal("missing handled statistics failure")
			}
		})
	}
}

func TestPiTraceIncludesLargeAndMalformedRecords(t *testing.T) {
	for _, scenario := range []string{"split", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			g, _ := newFixture(t, scenario)
			logs := capturePiLogs(t, logging.LevelTrace)
			_, err := g.Generate(context.Background(), requestFixture(), nil)
			if (err != nil) != (scenario == "malformed") {
				t.Fatalf("unexpected result: %v", err)
			}
			found := false
			for _, r := range piLogRecords(t, logs) {
				data, _ := r["data"].(string)
				if scenario == "malformed" && data == "not JSON\n" || scenario == "split" && strings.Contains(data, strings.Repeat("x", 150000)+"\u2028\u2029") {
					found = true
				}
			}
			if !found {
				t.Fatal("raw trace lost large/split/malformed record")
			}
		})
	}
}

// The child waits for this acknowledgment before it can respond to readiness.
// A tail-only dump during cleanup cannot satisfy the handshake.
type stderrAckLog struct {
	bytes.Buffer
	ackPath string
}

func (w *stderrAckLog) Write(data []byte) (int, error) {
	var record map[string]any
	if json.Unmarshal(data, &record) == nil && record["stream"] == "stderr" {
		if chunk, ok := record["data"].(string); ok && strings.Contains(chunk, "live-stderr-end") {
			if err := os.WriteFile(w.ackPath, nil, 0600); err != nil {
				return 0, err
			}
		}
	}
	return w.Buffer.Write(data)
}

func TestPiTraceStderrIsCompleteAndLive(t *testing.T) {
	g, dir := newFixture(t, "trace-stderr-live")
	g.startupTimeout = 3 * time.Second
	logs := &stderrAckLog{ackPath: filepath.Join(dir, "trace-ack")}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: logging.LevelTrace})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err != nil || len(result.Messages) != 1 {
		t.Fatalf("live stderr handshake failed: %+v %v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	acknowledged := false
	for _, e := range events {
		acknowledged = acknowledged || e.Kind == "trace-acknowledged"
	}
	if !acknowledged {
		t.Fatal("child continued without live stderr acknowledgment")
	}
	var stderr strings.Builder
	for _, r := range piLogRecords(t, &logs.Buffer) {
		if r["stream"] == "stderr" {
			stderr.WriteString(r["data"].(string))
		}
	}
	if want := "live-stderr-prefix:" + strings.Repeat("s", 200000) + ":live-stderr-end"; stderr.String() != want {
		t.Fatal("stderr trace was truncated or reordered")
	}
}

func TestPiStatisticsHaveIndependentBudget(t *testing.T) {
	g, dir := newFixture(t, "stats-delayed")
	g.promptTimeout = 500 * time.Millisecond
	logs := capturePiLogs(t)
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err != nil || len(result.Messages) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertReaped(t, fixtureEvents(t, dir))
	if !strings.Contains(logs.String(), "Pi session statistics\"") || strings.Contains(logs.String(), "Pi session statistics unavailable") {
		t.Fatal("statistics did not get an independent budget after the prompt context was canceled")
	}
}

func TestPiStatisticsAcrossCorrections(t *testing.T) {
	g, dir := newFixture(t, "correct-two")
	capturePiLogs(t)
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err != nil || len(result.Messages) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	events := fixtureEvents(t, dir)
	assertReaped(t, events)
	var commands []string
	for _, e := range events {
		if kind, ok := e.Command["type"].(string); ok {
			commands = append(commands, kind)
		}
	}
	if strings.Join(commands, ",") != "get_state,prompt,get_session_stats,prompt,get_session_stats" {
		t.Fatalf("unexpected correction/statistics sequence: %v", commands)
	}
}

func TestPiCancellationDuringStatisticsSuppressesOutput(t *testing.T) {
	g, dir := newFixture(t, "stats-hang")
	capturePiLogs(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			data, _ := os.ReadFile(filepath.Join(dir, "events"))
			if strings.Contains(string(data), `"type":"get_session_stats"`) {
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
		t.Fatalf("cancellation during telemetry produced candidates: %+v %v", result, err)
	}
	assertReaped(t, fixtureEvents(t, dir))
}

func TestPiTracePreservesInvalidUTF8Bytes(t *testing.T) {
	logs := capturePiLogs(t, logging.LevelTrace)
	data := []byte{'a', 0xff, 0xe2}
	traceProcessOutput(1, "stderr", data)
	records := piLogRecords(t, logs)
	if len(records) != 1 || records[0]["data_base64"] != base64.StdEncoding.EncodeToString(data) {
		t.Fatal("invalid UTF-8 bytes cannot be recovered from trace")
	}
}

func TestPiToolCallDiagnosticIDs(t *testing.T) {
	g, dir := newFixture(t, "observability")
	logs := capturePiLogs(t)
	result, err := g.Generate(context.Background(), requestFixture(), nil)
	if err != nil || len(result.Messages) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertReaped(t, fixtureEvents(t, dir))
	found := false
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["event"] != "toolcall_start" {
			continue
		}
		found = true
		if strings.Count(line, `"id":`) != 1 || record["id"] != "prompt-1" || record["tool_call_id"] != "fixture-tool" || record["tool_name"] != "read" {
			t.Errorf("ambiguous tool-call correlation: %s", line)
		}
	}
	if !found {
		t.Fatal("missing tool-call start diagnostic")
	}
}

func TestPiLateStatisticsResponseIDs(t *testing.T) {
	for _, scenario := range []string{"stats-late", "stats-late-rejected"} {
		t.Run(scenario, func(t *testing.T) {
			g, dir := newFixture(t, scenario)
			logs := capturePiLogs(t)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			result, err := g.Generate(ctx, requestFixture(), nil)
			if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 0 {
				t.Fatalf("late diagnostic response changed outcome: %+v %v", result, err)
			}
			assertReaped(t, fixtureEvents(t, dir))
			found := false
			for _, record := range piLogRecords(t, logs) {
				if record["event"] == "response" && record["command"] == "get_session_stats" && record["id"] == "stats-1" && record["exchange_id"] == "prompt-2" {
					found = true
					if record["success"] != (scenario == "stats-late") {
						t.Errorf("wrong response status: %v", record)
					}
				}
			}
			if !found {
				t.Fatal("late response did not retain its wire ID separately from the active exchange")
			}
			if strings.Count(logs.String(), "Pi session statistics unavailable") != 1 || !strings.Contains(logs.String(), `"msg":"Pi session statistics"`) {
				t.Fatal("expected one telemetry timeout and successful corrected-response statistics")
			}
		})
	}
}

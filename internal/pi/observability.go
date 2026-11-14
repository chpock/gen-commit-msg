package pi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chpock/gen-commit-msg/internal/logging"
)

func traceProcessOutput(pid int, stream string, data []byte, attrs ...slog.Attr) {
	ctx := context.Background()
	if !slog.Default().Enabled(ctx, logging.LevelTrace) {
		return
	}
	attrs = append(attrs, slog.Int("pid", pid), slog.String("stream", stream), slog.Int("bytes", len(data)), slog.String("data", string(data)))
	if !utf8.Valid(data) {
		attrs = append(attrs, slog.String("data_base64", base64.StdEncoding.EncodeToString(data)))
	}
	slog.LogAttrs(ctx, logging.LevelTrace, "Pi process output", attrs...)
}

type stderrTraceWriter struct{ pid int }

func (w stderrTraceWriter) Write(data []byte) (int, error) {
	traceProcessOutput(w.pid, "stderr", data)
	return len(data), nil
}

// Optional telemetry is best-effort and never adds protocol acceptance requirements.
func diagnosticAttrs(record rpcRecord, stringsFields, numberFields, boolFields []string) []any {
	var attrs []any
	for _, field := range stringsFields {
		if value := diagnosticString(record[field]); value != "" {
			attrs = append(attrs, field, value)
		}
	}
	for _, field := range numberFields {
		var value *float64
		if json.Unmarshal(record[field], &value) == nil && value != nil {
			attrs = append(attrs, field, *value)
		}
	}
	for _, field := range boolFields {
		if value, known := diagnosticBool(record[field]); known {
			attrs = append(attrs, field, value)
		}
	}
	return attrs
}

func diagnosticFields(raw json.RawMessage, numberFields []string) []any {
	var record rpcRecord
	if json.Unmarshal(raw, &record) != nil {
		return nil
	}
	return diagnosticAttrs(record, nil, numberFields, nil)
}

func usageAttrs(raw json.RawMessage) []any {
	var usage rpcRecord
	if json.Unmarshal(raw, &usage) != nil {
		return nil
	}
	attrs := diagnosticAttrs(usage, nil, []string{"input", "output", "cacheRead", "cacheWrite", "totalTokens", "total"}, nil)
	if cost := diagnosticFields(usage["cost"], []string{"input", "output", "cacheRead", "cacheWrite", "total"}); len(cost) > 0 {
		attrs = append(attrs, slog.Group("cost", cost...))
	}
	return attrs
}

func logSessionState(state rpcRecord, pid int) {
	attrs := []any{"pid", pid}
	var model rpcRecord
	if json.Unmarshal(state["model"], &model) == nil {
		attrs = append(attrs, "model", diagnosticString(model["id"]), "provider", diagnosticString(model["provider"]))
		attrs = append(attrs, diagnosticAttrs(model, []string{"api"}, []string{"contextWindow", "maxTokens"}, []string{"reasoning"})...)
	}
	attrs = append(attrs, "thinking_level", diagnosticString(state["thinkingLevel"]))
	attrs = append(attrs, diagnosticAttrs(state, []string{"sessionId", "steeringMode", "followUpMode"}, []string{"messageCount", "pendingMessageCount"}, []string{"isStreaming", "isCompacting", "autoCompactionEnabled"})...)
	slog.Debug("Pi session state", attrs...)
}

func logRPCActivity(record rpcRecord, kind, id string, pid int) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	attrs := []any{"pid", pid, "id", id, "event", kind}
	switch kind {
	case "message_update":
		var update rpcRecord
		if json.Unmarshal(record["assistantMessageEvent"], &update) != nil {
			return
		}
		event := diagnosticString(update["type"])
		if !strings.HasSuffix(event, "_start") && !strings.HasSuffix(event, "_end") {
			return
		}
		attrs = []any{"pid", pid, "id", id, "event", event, "rpc_event", kind}
		for _, field := range []struct{ wire, log string }{{"id", "tool_call_id"}, {"toolName", "tool_name"}} {
			if value := diagnosticString(update[field.wire]); value != "" {
				attrs = append(attrs, field.log, value)
			}
		}
		attrs = append(attrs, diagnosticAttrs(update, nil, []string{"contentIndex"}, nil)...)
	case "message_start", "message_end":
		var message rpcRecord
		if json.Unmarshal(record["message"], &message) != nil {
			return
		}
		attrs = append(attrs, diagnosticAttrs(message, []string{"role", "stopReason", "model", "provider", "api"}, nil, nil)...)
		if kind == "message_end" {
			if usage := usageAttrs(message["usage"]); len(usage) > 0 {
				slog.Debug("Pi assistant usage", append(attrs, slog.Group("usage", usage...))...)
			}
		}
	case "turn_end":
		var message rpcRecord
		if json.Unmarshal(record["message"], &message) == nil {
			attrs = append(attrs, diagnosticAttrs(message, []string{"stopReason"}, nil, nil)...)
		}
	case "agent_end":
		attrs = append(attrs, diagnosticAttrs(record, nil, nil, []string{"willRetry"})...)
	case "thinking_level_changed":
		attrs = append(attrs, diagnosticAttrs(record, []string{"level"}, nil, nil)...)
	case "tool_execution_update":
		attrs = append(attrs, diagnosticAttrs(record, []string{"toolName", "toolCallId"}, nil, nil)...)
	case "tool_execution_start", "tool_execution_end", "auto_retry_start", "auto_retry_end", "summarization_retry_scheduled", "summarization_retry_attempt_start", "summarization_retry_finished", "compaction_start", "compaction_end":
		return // These retain their existing failure severity in logNativeDiagnostic.
	case "response":
		attrs = []any{"pid", pid, "id", diagnosticString(record["id"]), "exchange_id", id, "event", kind}
		attrs = append(attrs, diagnosticAttrs(record, []string{"command"}, nil, []string{"success"})...)
	}
	slog.Debug("Pi RPC activity", attrs...)
}

func (p *process) sessionStats(ctx context.Context, id string) error {
	e := p.command(id, "get_session_stats", "")
	for {
		record, kind, err := e.record(ctx)
		if err != nil {
			return err
		}
		if kind != "response" {
			continue
		}
		match, err := responseMatches(record, id, "get_session_stats")
		if err != nil {
			return err
		}
		if !match {
			continue
		}
		data, err := requiredObject(record["data"], "get_session_stats data")
		if err != nil {
			return err
		}
		attrs := []any{"pid", p.cmd.Process.Pid, "id", id}
		attrs = append(attrs, diagnosticAttrs(data, []string{"sessionId"}, []string{"userMessages", "assistantMessages", "toolCalls", "toolResults", "totalMessages", "cost"}, nil)...)
		if tokens := usageAttrs(data["tokens"]); len(tokens) > 0 {
			attrs = append(attrs, slog.Group("tokens", tokens...))
		}
		if usage := diagnosticFields(data["contextUsage"], []string{"tokens", "contextWindow", "percent"}); len(usage) > 0 {
			attrs = append(attrs, slog.Group("context_usage", usage...))
		}
		slog.Debug("Pi session statistics", attrs...)
		return e.finishWrite(ctx)
	}
}

func logPhase(name string, budget time.Duration) func() {
	start := time.Now()
	slog.Debug("Pi phase starting", "phase", name, "budget", budget)
	return func() { slog.Debug("Pi phase finished", "phase", name, "budget", budget, "elapsed", time.Since(start)) }
}

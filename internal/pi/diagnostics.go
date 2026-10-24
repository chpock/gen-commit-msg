package pi

import (
	"context"
	"encoding/json"
	"log/slog"
	"unicode/utf8"
)

const diagnosticLimit = 16 * 1024

// Only the stderr reader writes this tail; cleanup reads it after all drains finish.
type stderrTail struct {
	data  [diagnosticLimit]byte
	size  int
	total int64
}

func (s *stderrTail) Write(data []byte) (int, error) {
	n := len(data)
	s.total += int64(n)
	if n >= diagnosticLimit {
		copy(s.data[:], data[n-diagnosticLimit:])
		s.size = diagnosticLimit
	} else {
		if overflow := s.size + n - diagnosticLimit; overflow > 0 {
			copy(s.data[:], s.data[overflow:s.size])
			s.size -= overflow
		}
		copy(s.data[s.size:], data)
		s.size += n
	}
	return n, nil
}

func boundedDiagnostic(text string) string {
	if len(text) <= diagnosticLimit {
		return text
	}
	end := diagnosticLimit
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}

// Diagnostic fields are best-effort metadata, never additional protocol requirements.
func diagnosticString(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return boundedDiagnostic(value)
}
func diagnosticBool(raw json.RawMessage) (bool, bool) {
	var value *bool
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return false, false
	}
	return *value, true
}

func toolDiagnostic(raw json.RawMessage) string {
	var result struct {
		Content []struct {
			Type string
			Text string
		}
	}
	if json.Unmarshal(raw, &result) != nil {
		return ""
	}
	text := ""
	for _, block := range result.Content {
		if block.Type == "text" {
			remaining := diagnosticLimit - len(text)
			if remaining == 0 {
				break
			}
			part := boundedDiagnostic(block.Text)
			if len(part) > remaining {
				part = part[:remaining]
			}
			text += part
		}
	}
	return boundedDiagnostic(text)
}

func logNativeDiagnostic(kind string, record rpcRecord, promptID string) {
	level := slog.LevelDebug
	detail := ""
	switch kind {
	case "tool_execution_end":
		failed, _ := diagnosticBool(record["isError"])
		if !failed {
			return
		}
		level = slog.LevelWarn
		detail = toolDiagnostic(record["result"])
	case "auto_retry_start", "summarization_retry_scheduled":
		level = slog.LevelWarn
		detail = diagnosticString(record["errorMessage"])
	case "auto_retry_end":
		detail = diagnosticString(record["finalError"])
		success, known := diagnosticBool(record["success"])
		if known && !success || detail != "" {
			level = slog.LevelWarn
		}
	case "compaction_end":
		detail = diagnosticString(record["errorMessage"])
		aborted, _ := diagnosticBool(record["aborted"])
		if aborted || detail != "" {
			level = slog.LevelWarn
		}
	}
	attrs := []any{"event", kind, "prompt_id", promptID}
	if detail != "" {
		attrs = append(attrs, "error", detail)
	}
	for _, field := range []struct{ wire, log string }{{"toolName", "tool_name"}, {"toolCallId", "tool_call_id"}, {"reason", "reason"}, {"source", "source"}} {
		if value := diagnosticString(record[field.wire]); value != "" {
			attrs = append(attrs, field.log, value)
		}
	}
	for _, field := range []struct{ wire, log string }{{"attempt", "attempt"}, {"maxAttempts", "max_attempts"}, {"delayMs", "delay_ms"}} {
		var value *int64
		if json.Unmarshal(record[field.wire], &value) == nil && value != nil {
			attrs = append(attrs, field.log, *value)
		}
	}
	for _, field := range []string{"success", "aborted", "willRetry"} {
		if value, known := diagnosticBool(record[field]); known {
			attrs = append(attrs, field, value)
		}
	}
	slog.Log(context.Background(), level, "Pi native activity", attrs...)
}

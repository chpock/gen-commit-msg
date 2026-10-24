package pi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type rpcRecord map[string]json.RawMessage

func requiredString(record rpcRecord, key string) (string, error) {
	var value *string
	if err := json.Unmarshal(record[key], &value); err != nil || value == nil || *value == "" {
		return "", fmt.Errorf("pi protocol: %s must be a nonempty string", key)
	}
	return *value, nil
}
func requiredBool(record rpcRecord, key string) (bool, error) {
	var value *bool
	if err := json.Unmarshal(record[key], &value); err != nil || value == nil {
		return false, fmt.Errorf("pi protocol: %s must be a boolean", key)
	}
	return *value, nil
}
func requiredObject(data json.RawMessage, name string) (rpcRecord, error) {
	var value rpcRecord
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return nil, fmt.Errorf("pi protocol: %s must be an object", name)
	}
	return value, nil
}

func (e *exchange) record(ctx context.Context) (rpcRecord, string, error) {
	line, err := e.next(ctx)
	if err != nil {
		return nil, "", err
	}
	if !line.terminated {
		return nil, "", fmt.Errorf("pi protocol: unterminated JSONL record")
	}
	record, err := requiredObject(line.data, "record")
	if err != nil {
		return nil, "", err
	}
	kind, err := requiredString(record, "type")
	return record, kind, err
}

func (p *process) command(id, kind, message string) *exchange {
	command := map[string]string{"id": id, "type": kind}
	if kind == "prompt" {
		command["message"] = message
	}
	data, _ := json.Marshal(command)
	return &exchange{process: p, written: p.send(append(data, '\n'))}
}

func responseMatches(record rpcRecord, id, command string) (bool, error) {
	kind, err := requiredString(record, "command")
	if err != nil {
		return false, err
	}
	success, err := requiredBool(record, "success")
	if err != nil {
		return false, err
	}
	responseID := ""
	if raw, ok := record["id"]; ok {
		if responseID, err = requiredString(rpcRecord{"id": raw}, "id"); err != nil {
			return false, err
		}
	}
	if kind == "parse" {
		return false, fmt.Errorf("pi rejected RPC command framing")
	}
	if responseID != id {
		return false, nil
	}
	if kind != command {
		return false, fmt.Errorf("pi protocol: correlated response command must be %s", command)
	}
	if !success {
		detail, err := requiredString(record, "error")
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("pi %s rejected: %s", command, detail)
	}
	return true, nil
}

func (p *process) ready(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			p.operationFailed = true
		}
	}()
	e := p.command("ready-1", "get_state", "")
	for {
		record, kind, err := e.record(ctx)
		if err != nil {
			return err
		}
		if kind != "response" {
			continue
		}
		match, err := responseMatches(record, "ready-1", "get_state")
		if err != nil {
			return err
		}
		if match {
			if _, err = requiredObject(record["data"], "get_state data"); err != nil {
				return err
			}
			return e.finishWrite(ctx)
		}
	}
}

type assistantMessage struct {
	text   string
	stop   string
	detail string
}

func completedAssistant(record rpcRecord) (*assistantMessage, error) {
	message, err := requiredObject(record["message"], "message_end.message")
	if err != nil {
		return nil, err
	}
	role, err := requiredString(message, "role")
	if err != nil {
		return nil, err
	}
	if role != "assistant" {
		return nil, nil
	}
	stop, err := requiredString(message, "stopReason")
	if err != nil {
		return nil, err
	}
	switch stop {
	case "stop", "length", "toolUse", "error", "aborted", "deferred":
	default:
		return nil, fmt.Errorf("pi protocol: invalid completed assistant stopReason %q", stop)
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(message["content"], &blocks); err != nil || blocks == nil {
		return nil, fmt.Errorf("pi protocol: assistant content must be an array")
	}
	var text strings.Builder
	for _, raw := range blocks {
		block, err := requiredObject(raw, "assistant content block")
		if err != nil {
			return nil, err
		}
		kind, err := requiredString(block, "type")
		if err != nil {
			return nil, err
		}
		if kind == "text" {
			var value *string
			if err := json.Unmarshal(block["text"], &value); err != nil || value == nil {
				return nil, fmt.Errorf("pi protocol: text block text must be a string")
			}
			text.WriteString(*value)
		}
	}
	detail := ""
	if raw, ok := message["errorMessage"]; ok {
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, fmt.Errorf("pi protocol: errorMessage must be a string")
		}
		detail = *value
	}
	return &assistantMessage{text: text.String(), stop: stop, detail: detail}, nil
}

func (p *process) prompt(ctx context.Context, id, message string) (text string, err error) {
	defer func() {
		if err != nil {
			p.operationFailed = true
		}
	}()
	e := p.command(id, "prompt", message)
	accepted, settled := false, false
	var final *assistantMessage
	for {
		record, kind, err := e.record(ctx)
		if err != nil {
			return "", err
		}
		switch kind {
		case "response":
			match, err := responseMatches(record, id, "prompt")
			if err != nil {
				return "", err
			}
			if match {
				if accepted {
					return "", fmt.Errorf("pi protocol: duplicate prompt acceptance")
				}
				data, err := requiredObject(record["data"], "prompt data")
				if err != nil {
					return "", err
				}
				disposition, err := requiredString(data, "disposition")
				if err != nil {
					return "", err
				}
				switch disposition {
				case "started", "queued":
					accepted = true
				case "handled":
					return "", fmt.Errorf("pi prompt was handled without starting generation")
				default:
					return "", fmt.Errorf("pi protocol: invalid prompt disposition")
				}
				slog.Debug("Pi prompt accepted", "id", id, "disposition", disposition)
			}
		case "message_end":
			assistant, err := completedAssistant(record)
			if err != nil {
				return "", err
			}
			if assistant != nil {
				final = assistant
				slog.Debug("Pi completed assistant message", "id", id, "stop_reason", assistant.stop)
				if assistant.stop != "stop" && assistant.stop != "toolUse" {
					slog.Warn("Pi assistant response failed", "prompt_id", id, "stop_reason", assistant.stop, "error", boundedDiagnostic(assistant.detail))
				}
			}
		case "agent_settled":
			aborted, err := requiredBool(record, "aborted")
			if err != nil {
				return "", err
			}
			if aborted {
				return "", fmt.Errorf("pi generation settled after abort")
			}
			settled = true
			slog.Debug("Pi agent settled", "id", id)
		case "tool_execution_end", "auto_retry_start", "auto_retry_end", "summarization_retry_scheduled", "summarization_retry_attempt_start", "summarization_retry_finished", "compaction_start", "compaction_end":
			logNativeDiagnostic(kind, record, id)
		}
		if accepted && settled {
			if final == nil {
				return "", fmt.Errorf("pi generation settled without a completed assistant response")
			}
			if final.stop != "stop" {
				return "", fmt.Errorf("pi final assistant response stopped with %s: %s", final.stop, final.detail)
			}
			if err := e.finishWrite(ctx); err != nil {
				return "", err
			}
			return final.text, nil
		}
	}
}

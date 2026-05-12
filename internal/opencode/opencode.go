package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	opencode "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
)

type GenerateParams struct {
	SubjectCount int
	Body         bool
}

type CommitMessage struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type Client struct {
	sdkClient *opencode.Client
	baseURL   string
	repoDir   string
	agent     string
}

func NewClient(baseURL, repoDir, agent string) *Client {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	oc := opencode.NewClient(option.WithBaseURL(baseURL), option.WithHTTPClient(httpClient))
	return &Client{sdkClient: oc, baseURL: baseURL, repoDir: repoDir, agent: agent}
}

func (c *Client) CreateSession(ctx context.Context, agentName string) (string, error) {
	slog.Debug("creating session", "agent", agentName, "dir", c.repoDir)
	session, err := c.sdkClient.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.F(c.repoDir),
		Title:     opencode.String(agentName),
	})
	if err != nil {
		slog.Error("failed to create session", "agent", agentName, "error", err)
		return "", fmt.Errorf("create session: %w", err)
	}
	slog.Debug("session created", "id", session.ID, "agent", agentName)
	return session.ID, nil
}

func (c *Client) GenerateMessages(ctx context.Context, sessionID string, params GenerateParams) ([]CommitMessage, error) {
	slog.Info("sending generation prompt", "session_id", sessionID,
		"subject_count", params.SubjectCount, "body", params.Body, "dir", c.repoDir)

	prompt := fmt.Sprintf(
		"Analyze the current repository changes and generate %d Git commit message candidates."+
			" Include message body: %v.",
		params.SubjectCount, params.Body,
	)

	format := map[string]any{
		"type": "json_schema",
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"subjects": map[string]any{
					"type":        "array",
					"minItems":    1,
					"description": "Candidate subjects for a Git commit message.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"body": map[string]any{
					"type":        "string",
					"description": "Detailed commit message body. Empty string if not needed.",
				},
			},
			"required": []string{"subjects", "body"},
		},
		"retryCount": 2,
	}

	res, err := c.sdkClient.Session.Prompt(
		ctx,
		sessionID,
		opencode.SessionPromptParams{
			Directory: opencode.F(c.repoDir),
			Agent:     opencode.F(c.agent),
			Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
				opencode.TextPartInputParam{
					Type: opencode.F(opencode.TextPartInputTypeText),
					Text: opencode.F(prompt),
				},
			}),
		},
		option.WithJSONSet("format", format),
	)
	if err != nil {
		slog.Error("prompt failed", "session_id", sessionID, "error", err)
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	raw, err := getStructuredJSON(res)
	if err != nil {
		slog.Error("failed to extract structured output", "session_id", sessionID, "error", err)
		return nil, fmt.Errorf("extract structured output: %w", err)
	}

	var result struct {
		Subjects []string `json:"subjects"`
		Body     string   `json:"body"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		slog.Error("failed to decode structured output", "session_id", sessionID, "error", err, "raw", string(raw))
		return nil, fmt.Errorf("decode structured output: %w", err)
	}

	messages := make([]CommitMessage, len(result.Subjects))
	for i, subject := range result.Subjects {
		messages[i] = CommitMessage{Subject: subject, Body: result.Body}
	}

	slog.Info("messages generated", "session_id", sessionID, "count", len(messages))
	return messages, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	slog.Debug("deleting session", "session_id", sessionID)
	_, err := c.sdkClient.Session.Delete(ctx, sessionID, opencode.SessionDeleteParams{})
	if err != nil {
		slog.Warn("failed to delete session", "session_id", sessionID, "error", err)
	}
	return err
}

func getStructuredJSON(res *opencode.SessionPromptResponse) ([]byte, error) {
	if res == nil {
		return nil, errors.New("nil opencode response")
	}

	for _, key := range []string{"structured", "structured_output"} {
		field, ok := res.Info.JSON.ExtraFields[key]
		if !ok {
			continue
		}

		raw := field.Raw()
		if raw == "" || raw == "null" {
			continue
		}

		var s string
		if err := json.Unmarshal([]byte(raw), &s); err == nil {
			if s != "" {
				return []byte(s), nil
			}
			continue
		}
		return []byte(raw), nil
	}

	return nil, fmt.Errorf(
		"structured output was not found in response: %s",
		res.Info.JSON.RawJSON(),
	)
}

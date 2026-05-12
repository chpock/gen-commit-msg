package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
}

func NewClient(baseURL string) *Client {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	oc := opencode.NewClient(option.WithBaseURL(baseURL), option.WithHTTPClient(httpClient))
	return &Client{sdkClient: oc, baseURL: baseURL}
}

func (c *Client) CreateSession(ctx context.Context, agentName string) (string, error) {
	slog.Debug("creating session", "agent", agentName)
	pwd, err := os.Getwd()
	if err != nil {
		slog.Warn("failed to get current directory for session", "error", err)
	}
	session, err := c.sdkClient.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.F(pwd),
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
	prompt := fmt.Sprintf(
		"Generate %d commit message variants. Include message body: %v. "+
			"Output the result as a JSON array of objects, each with 'subject' and 'body' fields. "+
			"Example: [{\"subject\":\"feat: add feature\",\"body\":\"details...\"}]",
		params.SubjectCount, params.Body,
	)

	slog.Info("sending generation prompt", "session_id", sessionID,
		"subject_count", params.SubjectCount, "body", params.Body)
	slog.Debug("generation prompt", "prompt", prompt)

	result, err := c.sdkClient.Session.Prompt(ctx, sessionID, opencode.SessionPromptParams{
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			opencode.TextPartInputParam{
				Text: opencode.String(prompt),
				Type: opencode.F(opencode.TextPartInputTypeText),
			},
		}),
	})
	if err != nil {
		slog.Error("prompt failed", "session_id", sessionID, "error", err)
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	var responseText string
	for _, part := range result.Parts {
		responseText += part.Text
	}
	slog.Debug("prompt response received", "session_id", sessionID, "response_length", len(responseText))

	var messages []CommitMessage
	if err := json.Unmarshal([]byte(responseText), &messages); err != nil {
		slog.Warn("failed to parse response as JSON, using raw text", "session_id", sessionID, "error", err)
		messages = []CommitMessage{{Subject: responseText}}
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

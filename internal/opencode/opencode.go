package opencode

import (
	"context"
	"encoding/json"
	"fmt"
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
}

func NewClient(baseURL string) *Client {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	oc := opencode.NewClient(option.WithBaseURL(baseURL), option.WithHTTPClient(httpClient))
	return &Client{sdkClient: oc, baseURL: baseURL}
}

func (c *Client) CreateSession(ctx context.Context, agentName string) (string, error) {
	session, err := c.sdkClient.Session.New(ctx, opencode.SessionNewParams{
		Title: opencode.String(agentName),
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return session.ID, nil
}

func (c *Client) GenerateMessages(ctx context.Context, sessionID string, params GenerateParams) ([]CommitMessage, error) {
	prompt := fmt.Sprintf(
		"Generate %d commit message variants. Include message body: %v. "+
			"Output the result as a JSON array of objects, each with 'subject' and 'body' fields. "+
			"Example: [{\"subject\":\"feat: add feature\",\"body\":\"details...\"}]",
		params.SubjectCount, params.Body,
	)

	result, err := c.sdkClient.Session.Prompt(ctx, sessionID, opencode.SessionPromptParams{
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			opencode.TextPartInputParam{
				Text: opencode.String(prompt),
				Type: opencode.F(opencode.TextPartInputTypeText),
			},
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	var responseText string
	for _, part := range result.Parts {
		responseText += part.Text
	}

	var messages []CommitMessage
	if err := json.Unmarshal([]byte(responseText), &messages); err != nil {
		messages = []CommitMessage{{Subject: responseText}}
	}
	return messages, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := c.sdkClient.Session.Delete(ctx, sessionID, opencode.SessionDeleteParams{})
	return err
}

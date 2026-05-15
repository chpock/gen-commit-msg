package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"time"

	opencode "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"github.com/sst/opencode-sdk-go/shared"

	"github.com/chpock/gen-commit-msg/internal/logging"
)

type GenerateParams struct {
	SubjectMin int
	SubjectMax int
	Body       bool
}

type CommitMessage struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type sessionClient interface {
	New(ctx context.Context, params opencode.SessionNewParams) (*opencode.Session, error)
	Prompt(ctx context.Context, sessionID string, params opencode.SessionPromptParams, opts ...option.RequestOption) (*opencode.SessionPromptResponse, error)
	Delete(ctx context.Context, sessionID string, params opencode.SessionDeleteParams, opts ...option.RequestOption) (*bool, error)
}

type realSessionClient struct {
	session *opencode.SessionService
}

func (r *realSessionClient) New(ctx context.Context, params opencode.SessionNewParams) (*opencode.Session, error) {
	return r.session.New(ctx, params)
}

func (r *realSessionClient) Prompt(ctx context.Context, sessionID string, params opencode.SessionPromptParams, opts ...option.RequestOption) (*opencode.SessionPromptResponse, error) {
	return r.session.Prompt(ctx, sessionID, params, opts...)
}

func (r *realSessionClient) Delete(ctx context.Context, sessionID string, params opencode.SessionDeleteParams, opts ...option.RequestOption) (*bool, error) {
	return r.session.Delete(ctx, sessionID, params, opts...)
}

type Client struct {
	session sessionClient
	baseURL string
	repoDir string
	agent   string
}

func NewClient(baseURL, repoDir, agent string) *Client {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	oc := opencode.NewClient(option.WithBaseURL(baseURL), option.WithHTTPClient(httpClient))
	return &Client{session: &realSessionClient{session: oc.Session}, baseURL: baseURL, repoDir: repoDir, agent: agent}
}

func newClientWithSession(s sessionClient, repoDir, agent string) *Client {
	return &Client{session: s, repoDir: repoDir, agent: agent}
}

func (c *Client) CreateSession(ctx context.Context, agentName string) (string, error) {
	slog.Info("creating session", "agent", agentName, "dir", c.repoDir)

	params := opencode.SessionNewParams{
		Directory: opencode.F(c.repoDir),
		Title:     opencode.String(agentName),
	}

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		paramsJSON, _ := json.Marshal(params)
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode request: session new",
			slog.String("params", string(paramsJSON)))
	}

	session, err := c.session.New(ctx, params)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Debug("session creation cancelled", "agent", agentName)
		} else {
			slog.Error("failed to create session", "agent", agentName, "error", err)
		}
		return "", &AppError{
			Op:      "create_session",
			Message: "failed to create OpenCode session",
			OC:      buildHTTPOCError(err, "create_session", "", agentName),
			Err:     err,
		}
	}

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		sessionJSON, _ := json.Marshal(session)
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode response: session new",
			slog.String("response", string(sessionJSON)))
	}

	slog.Info("session created", "id", session.ID, "agent", agentName)
	return session.ID, nil
}

func (c *Client) GenerateMessages(ctx context.Context, sessionID string, params GenerateParams) ([]CommitMessage, error) {
	slog.Info("sending generation prompt", "session_id", sessionID,
		"subject_min", params.SubjectMin, "subject_max", params.SubjectMax, "body", params.Body, "dir", c.repoDir)

	prompt := fmt.Sprintf(
		"Analyze staged repository changes and generate Git commit message candidates.\n\n"+
			"Subject count:\n"+
			"- Minimum subjects: %[1]d\n"+
			"- Maximum subjects: %[2]d\n"+
			"- Choose the optimal number of subjects within this inclusive range.\n"+
			"- Use the minimum when the change is small or has one clear interpretation.\n"+
			"- Use more subjects only when the staged changes support genuinely useful alternatives.\n"+
			"- Use the maximum only when the change is substantial or can be accurately described from several useful angles.\n"+
			"- Do not pad the subjects array with weak or repetitive candidates.\n"+
			"- Sort subjects by preference, best first.\n"+
			"- The first subject must be the single best commit message choice.\n\n"+
			"Body:\n"+
			"- Include body: %[3]t\n"+
			"- If body is not requested, return an empty string for body.\n"+
			"- If body is requested, return an empty string when the best subject fully describes the change.\n\n"+
			"Scope:\n"+
			"- Use staged changes only.\n"+
			"- Follow repository-specific commit message instructions when present.\n"+
			"- If no repository instructions exist, follow the style of the last 5 commits.\n"+
			"- Otherwise use the default Conventional Commits rules from the active agent.\n\n"+
			"Return only JSON matching the supplied schema.",
		params.SubjectMin,
		params.SubjectMax,
		params.Body,
	)

	format := map[string]any{
		"type": "json_schema",
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"subjects": map[string]any{
					"type":        "array",
					"minItems":    params.SubjectMin,
					"maxItems":    params.SubjectMax,
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

	promptParams := opencode.SessionPromptParams{
		Directory: opencode.F(c.repoDir),
		Agent:     opencode.F(c.agent),
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			opencode.TextPartInputParam{
				Type: opencode.F(opencode.TextPartInputTypeText),
				Text: opencode.F(prompt),
			},
		}),
	}

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		paramsJSON, _ := json.Marshal(promptParams)
		formatJSON, _ := json.Marshal(format)
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode request: prompt",
			slog.String("session_id", sessionID),
			slog.String("params", string(paramsJSON)),
			slog.String("format", string(formatJSON)))
	}

	res, err := c.session.Prompt(
		ctx,
		sessionID,
		promptParams,
		option.WithJSONSet("format", format),
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Debug("prompt cancelled", "session_id", sessionID)
		} else {
			slog.Error("prompt failed", "session_id", sessionID, "error", err)
		}
		return nil, &AppError{
			Op:      "generate_messages",
			Message: "OpenCode prompt request failed",
			OC:      buildHTTPOCError(err, "prompt", sessionID, c.agent),
			Err:     err,
		}
	}

	if ocErr := buildAPIOCError(&res.Info, "prompt", sessionID, c.agent); ocErr != nil {
		slog.Error("prompt returned API error", "session_id", sessionID,
			"error_name", ocErr.Code, "error_message", ocErr.Message)
		return nil, &AppError{
			Op:      "generate_messages",
			Message: "OpenCode returned an error for the prompt request",
			OC:      ocErr,
		}
	}

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		respJSON, _ := json.Marshal(res)
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode response: prompt",
			slog.String("session_id", sessionID),
			slog.String("response", string(respJSON)))
	}

	rawJSON, err := getStructuredJSON(res)
	if err != nil {
		slog.Error("failed to extract structured output", "session_id", sessionID, "error", err)
		var noStrErr *noStructuredOutputError
		if errors.As(err, &noStrErr) {
			return nil, &AppError{
				Op:      "generate_messages",
				Message: "OpenCode prompt returned no structured output",
				OC: &OCError{
					Kind:        OCErrNoStructuredOutput,
					RequestType: "prompt",
					SessionID:   sessionID,
					Agent:       c.agent,
					Code:        "no_structured_output",
					Message:     "structured output was not found in the response",
					RawJSON:     noStrErr.RawJSON,
				},
			}
		}
		return nil, &AppError{
			Op:      "generate_messages",
			Message: "failed to extract structured output",
			Err:     err,
		}
	}

	var result struct {
		Subjects []string `json:"subjects"`
		Body     string   `json:"body"`
	}
	if err := json.Unmarshal(rawJSON, &result); err != nil {
		slog.Error("failed to decode structured output", "session_id", sessionID, "error", err, "raw", string(rawJSON))
		return nil, &AppError{
			Op:      "generate_messages",
			Message: "failed to decode structured output",
			Err:     fmt.Errorf("decode structured output: %w", err),
		}
	}

	messages := make([]CommitMessage, len(result.Subjects))
	for i, subject := range result.Subjects {
		messages[i] = CommitMessage{Subject: subject, Body: result.Body}
	}

	if len(messages) == 0 {
		slog.Error("no subjects in structured output", "session_id", sessionID)
		return nil, &AppError{
			Op:      "generate_messages",
			Message: "OpenCode returned no commit message subjects",
			OC: &OCError{
				Kind:        OCErrNoSubjects,
				RequestType: "prompt",
				SessionID:   sessionID,
				Agent:       c.agent,
				Code:        "empty_subjects",
				Message:     "the structured output contained an empty subjects array",
			},
		}
	}

	slog.Info("messages generated", "session_id", sessionID, "count", len(messages), "has_body", result.Body != "")
	slog.Debug("subjects", "session_id", sessionID, "subjects", result.Subjects)
	slog.Debug("body", "session_id", sessionID, "body", result.Body)
	return messages, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	slog.Info("deleting session", "session_id", sessionID)

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode request: session delete",
			slog.String("session_id", sessionID))
	}

	_, err := c.session.Delete(ctx, sessionID, opencode.SessionDeleteParams{})
	if err != nil {
		slog.Warn("failed to delete session", "session_id", sessionID, "error", err)
	}

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		deleted := false
		if err == nil {
			deleted = true
		}
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode response: session delete",
			slog.String("session_id", sessionID),
			slog.Bool("deleted", deleted),
			slog.Any("error", err))
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

	rawJSON := res.Info.JSON.RawJSON()
	slog.Debug("structured output not found in response", "raw", rawJSON)
	return nil, &noStructuredOutputError{RawJSON: rawJSON}
}

type noStructuredOutputError struct {
	RawJSON string
}

func (e *noStructuredOutputError) Error() string {
	return "structured output was not found in response"
}

func buildHTTPOCError(err error, requestType, sessionID, agent string) *OCError {
	code, method, url, body := extractHTTPFields(err)
	if code == 0 {
		return nil
	}
	return &OCError{
		Kind:        OCErrHTTP,
		RequestType: requestType,
		SessionID:   sessionID,
		Agent:       agent,
		Code:        strconv.Itoa(code) + " " + method,
		Message:     url,
		Status:      code,
		RawJSON:     body,
	}
}

func extractHTTPFields(err error) (code int, method, url, body string) {
	v := reflect.Indirect(reflect.ValueOf(err))
	if v.Kind() != reflect.Struct {
		return
	}

	if f := v.FieldByName("StatusCode"); f.IsValid() {
		switch f.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			code = int(f.Int())
		}
	}

	if f := v.FieldByName("Request"); f.IsValid() && f.Kind() == reflect.Ptr && !f.IsNil() {
		req := f.Elem()
		if m := req.FieldByName("Method"); m.IsValid() && m.Kind() == reflect.String {
			method = m.String()
		}
		if u := req.FieldByName("URL"); u.IsValid() && !u.IsNil() {
			if s := u.MethodByName("String"); s.IsValid() {
				url = s.Call(nil)[0].String()
			}
		}
	}

	if f := v.FieldByName("JSON"); f.IsValid() {
		if raw := f.MethodByName("RawJSON"); raw.IsValid() {
			body = raw.Call(nil)[0].String()
		}
	}

	return
}

func buildAPIOCError(msg *opencode.AssistantMessage, requestType, sessionID, agent string) *OCError {
	if msg == nil || msg.Error.Name == "" {
		return nil
	}

	rawJSON := msg.JSON.RawJSON()

	oc := &OCError{
		Kind:        OCErrAPI,
		RequestType: requestType,
		SessionID:   sessionID,
		Agent:       agent,
		Code:        string(msg.Error.Name),
		RawJSON:     rawJSON,
	}

	switch u := msg.Error.AsUnion().(type) {
	case opencode.AssistantMessageErrorAPIError:
		oc.Message = u.Data.Message
		oc.Status = int(u.Data.StatusCode)
	case shared.ProviderAuthError:
		oc.Message = u.Data.Message
	case shared.UnknownError:
		oc.Message = u.Data.Message
	case shared.MessageAbortedError:
		oc.Message = u.Data.Message
	case opencode.AssistantMessageErrorMessageOutputLengthError:
		if m, ok := u.Data.(string); ok {
			oc.Message = m
		}
	}

	if oc.Message == "" {
		oc.Message = "(no message in error data)"
	}

	return oc
}

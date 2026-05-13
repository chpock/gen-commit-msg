package opencode

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	opencode "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
)

type mockSessionClient struct {
	sessionFixture string
	promptFixture  string
	deleteFixture  string

	newReturn    *opencode.Session
	promptReturn *opencode.SessionPromptResponse
	deleteReturn bool

	deleteReturnSet bool

	newErr    error
	promptErr error
	deleteErr error

	deleteCalled bool
}

var (
	errMockNewSession    = errors.New("mock: session new failed")
	errMockPrompt        = errors.New("mock: prompt failed")
	errMockDeleteSession = errors.New("mock: session delete failed")
)

func newMockSession() *mockSessionClient {
	return &mockSessionClient{
		sessionFixture: "session_create_success.json",
		promptFixture:  "prompt_success_basic.json",
		deleteFixture:  "session_delete_success.json",
	}
}

func (m *mockSessionClient) WithSessionFixture(name string) *mockSessionClient {
	m.sessionFixture = name
	return m
}

func (m *mockSessionClient) WithPromptFixture(name string) *mockSessionClient {
	m.promptFixture = name
	return m
}

func (m *mockSessionClient) WithDeleteFixture(name string) *mockSessionClient {
	m.deleteFixture = name
	return m
}

func (m *mockSessionClient) WithNewError() *mockSessionClient {
	m.newErr = errMockNewSession
	return m
}

func (m *mockSessionClient) WithPromptError() *mockSessionClient {
	m.promptErr = errMockPrompt
	return m
}

func (m *mockSessionClient) WithDeleteError() *mockSessionClient {
	m.deleteErr = errMockDeleteSession
	return m
}

func (m *mockSessionClient) New(ctx context.Context, params opencode.SessionNewParams) (*opencode.Session, error) {
	if m.newErr != nil {
		return nil, m.newErr
	}
	if m.newReturn != nil {
		return m.newReturn, nil
	}
	s, err := loadSessionFixture(m.sessionFixture)
	if err != nil {
		slog.Debug("mock session new: failed to load fixture", "fixture", m.sessionFixture, "error", err)
		return nil, fmt.Errorf("mock session new: %w", err)
	}
	return s, nil
}

func (m *mockSessionClient) Prompt(ctx context.Context, sessionID string, params opencode.SessionPromptParams, opts ...option.RequestOption) (*opencode.SessionPromptResponse, error) {
	if m.promptErr != nil {
		return nil, m.promptErr
	}
	if m.promptReturn != nil {
		return m.promptReturn, nil
	}
	r, err := loadPromptFixture(m.promptFixture)
	if err != nil {
		slog.Debug("mock prompt: failed to load fixture", "fixture", m.promptFixture, "error", err)
		return nil, fmt.Errorf("mock prompt: %w", err)
	}
	return r, nil
}

func (m *mockSessionClient) Delete(ctx context.Context, sessionID string, params opencode.SessionDeleteParams, opts ...option.RequestOption) (*bool, error) {
	m.deleteCalled = true
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	if m.deleteReturnSet {
		return &m.deleteReturn, nil
	}
	v, err := loadDeleteFixture(m.deleteFixture)
	if err != nil {
		slog.Debug("mock delete: failed to load fixture", "fixture", m.deleteFixture, "error", err)
		return nil, fmt.Errorf("mock delete: %w", err)
	}
	return &v, nil
}

func (m *mockSessionClient) DeleteCalled() bool {
	return m.deleteCalled
}

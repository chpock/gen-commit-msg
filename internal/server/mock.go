package server

import "context"

type MockServer struct {
	BaseURL    string
	StartError error
	StopError  error
}

func (m *MockServer) Start(ctx context.Context) (string, error) {
	if m.StartError != nil {
		return "", m.StartError
	}
	return m.BaseURL, nil
}

func (m *MockServer) Stop() error {
	return m.StopError
}

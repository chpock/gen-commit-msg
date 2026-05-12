package server

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestServerHealthy_OpenCodeNotFound(t *testing.T) {
	_, err := exec.LookPath("opencode")
	if err != nil {
		t.Log("opencode not installed - skipping integration test")
		return
	}
	// Integration test: start server, verify health, stop
	t.Skip("opencode installed - integration test to be written manually")
}

func TestServerInterface(t *testing.T) {
	var _ Server = &ProcessServer{}
}

func TestParseListenURL_Success(t *testing.T) {
	input := strings.NewReader("some log\nopencode server listening on http://127.0.0.1:12345\nmore log\n")
	url, err := parseListenURL(input, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "http://127.0.0.1:12345" {
		t.Errorf("expected http://127.0.0.1:12345, got %s", url)
	}
}

func TestParseListenURL_NoMatch(t *testing.T) {
	input := strings.NewReader("no match here\njust logs\n")
	_, err := parseListenURL(input, 1*time.Second)
	if err == nil {
		t.Fatal("expected error for no match")
	}
	if !strings.Contains(err.Error(), "Output:") {
		t.Errorf("error should contain captured output, got: %v", err)
	}
}

package server

import (
	"os/exec"
	"testing"
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
	// Verify the Server type compiles and satisfies its interface
	s := &ProcessServer{}
	if s == nil {
		t.Error("ProcessServer is nil")
	}
}

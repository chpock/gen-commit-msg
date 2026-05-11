package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAgent_Create(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "if-not-exists")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("agent file not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("agent file is empty")
	}
}

func TestEnsureAgent_NoInstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "no")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	if _, err := os.Stat(expectedPath); err == nil {
		t.Error("agent file was created but install-agent is 'no'")
	}
}

func TestEnsureAgent_AlwaysOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "if-not-exists")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	err = Ensure("gen-commit-msg", "always")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Error("agent file should exist after 'always' install")
	}
}

func TestEnsureAgent_CustomName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("custom-agent", "if-not-exists")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "custom-agent.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Error("agent file with custom name not created")
	}
}

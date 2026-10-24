package agent

import (
	"crypto/sha256"
	"fmt"
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
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("agent file not found after 'always' install: %v", err)
	}
	if string(data) != DefaultPrompt {
		t.Errorf("file content = %q, want DefaultPrompt", string(data))
	}
}

func TestEnsureAgent_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "if-not-exists")
	if err != nil {
		t.Fatalf("first Ensure: %v", err)
	}

	err = Ensure("gen-commit-msg", "if-not-exists")
	if err != nil {
		t.Fatalf("second Ensure should be nil, got: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Error("agent file should exist after both calls")
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

func TestEnsureDefaultPromptCompatibility(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := Ensure("default", "always"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "opencode", "agents", "default.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the installed OpenCode instructions and frontmatter across extraction.
	const want = "a2f4b02a61d482e799e0baf8833a8090087903b46e5e98b7244896cdd12587a4"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
		t.Fatalf("installed default prompt changed: sha256 = %s, want %s", got, want)
	}
}

func TestEnsurePreservesCustomAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	agents := filepath.Join(dir, "opencode", "agents")
	if err := os.MkdirAll(agents, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(agents, "custom.md")
	const custom = "---\nmodel: local/custom\n---\nCustom commit instructions.\n"
	if err := os.WriteFile(path, []byte(custom), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Ensure("custom", "if-not-exists"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom || info.Mode().Perm() != 0600 {
		t.Fatalf("custom agent modified: content = %q, mode = %v", data, info.Mode())
	}
}

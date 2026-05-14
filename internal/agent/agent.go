package agent

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

//go:embed gen-commit-msg.md
var DefaultPrompt string

func agentsDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, "opencode", "agents")
}

func Ensure(name, installMode string) error {
	if installMode == "no" {
		slog.Debug("agent install skipped", "mode", installMode)
		return nil
	}

	dir := agentsDir()
	if dir == "" {
		return fmt.Errorf("cannot determine agents directory")
	}

	filePath := filepath.Join(dir, name+".md")

	if installMode == "if-not-exists" {
		if _, err := os.Stat(filePath); err == nil {
			slog.Debug("agent file already exists", "path", filePath)
			return nil
		}
	}

	slog.Info("installing agent file", "agent", name, "path", filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create agents directory", "dir", dir, "error", err)
		return fmt.Errorf("create agents directory: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(DefaultPrompt), 0644); err != nil {
		slog.Error("failed to write agent file", "path", filePath, "error", err)
		return fmt.Errorf("write agent file: %w", err)
	}
	return nil
}

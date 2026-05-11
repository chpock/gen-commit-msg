package agent

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed prompt.md
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
		return nil
	}

	dir := agentsDir()
	if dir == "" {
		return fmt.Errorf("cannot determine agents directory")
	}

	filePath := filepath.Join(dir, name+".md")

	if installMode == "if-not-exists" {
		if _, err := os.Stat(filePath); err == nil {
			return nil // already exists
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agents directory: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(DefaultPrompt), 0644); err != nil {
		return fmt.Errorf("write agent file: %w", err)
	}
	return nil
}

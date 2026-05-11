package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultPrompt = `You are a git commit message generator. Your task is to generate commit messages for the current git repository.

Rules:
- Output commit messages (both subject line and body) based on the git diff
- First line: subject (50-72 chars, imperative mood, lowercase, no period)
- Include a body if the diff warrants explanation
- Follow the conventional commits style if the diff clearly matches a type
  (feat, fix, refactor, docs, test, chore, style, perf, ci, build)
- Otherwise, use a plain descriptive subject
- Do not include any additional explanations, markdown formatting, code blocks,
  or backticks in the output
`

func agentsDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			base = os.Getenv("HOME")
		} else {
			base = home
		}
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

	return os.WriteFile(filePath, []byte(DefaultPrompt), 0644)
}

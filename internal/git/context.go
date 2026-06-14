package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/chpock/gen-commit-msg/internal/logging"
)

const (
	MaxContextOutputBytes    = 200000
	RecentCommitMessageCount = 15
	truncationStrategy       = "head_tail"
)

type Truncation struct {
	MaxBytes      int    `json:"max_bytes"`
	OriginalBytes int    `json:"original_bytes"`
	KeptBytes     int    `json:"kept_bytes"`
	Strategy      string `json:"strategy"`
}

type CommandOutput struct {
	ID          string      `json:"id"`
	Command     []string    `json:"command"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Output      string      `json:"output"`
	Stderr      string      `json:"stderr"`
	ExitCode    int         `json:"exit_code"`
	Truncated   bool        `json:"truncated"`
	Truncation  *Truncation `json:"truncation,omitempty"`
}

type ContextSection struct {
	Outputs []CommandOutput `json:"outputs"`
}

type CommitMessageContext struct {
	FormatVersion string         `json:"format_version"`
	StagedChanges ContextSection `json:"staged_changes"`
	StyleContext  ContextSection `json:"style_context"`
}

type commandSpec struct {
	ID          string
	Command     []string
	Description string
	Required    bool
	Section     string
}

var commitContextCommandSpecs = []commandSpec{
	{
		ID:          "staged_name_status",
		Command:     []string{"git", "diff", "--cached", "--name-status", "--find-renames", "--find-copies"},
		Description: "Staged file status summary. Shows added, modified, deleted, renamed, and copied files using git diff --name-status format.",
		Required:    true,
		Section:     "staged_changes",
	},
	{
		ID:          "staged_stat",
		Command:     []string{"git", "diff", "--cached", "--stat", "--find-renames", "--find-copies", "--compact-summary"},
		Description: "Human-readable staged diff summary by file. Useful for quickly understanding change size and affected files.",
		Required:    true,
		Section:     "staged_changes",
	},
	{
		ID:          "staged_numstat",
		Command:     []string{"git", "diff", "--cached", "--numstat", "--find-renames", "--find-copies"},
		Description: "Machine-readable staged additions and deletions by file. Useful for estimating the scale of each changed file.",
		Required:    true,
		Section:     "staged_changes",
	},
	{
		ID:          "staged_summary",
		Command:     []string{"git", "diff", "--cached", "--summary", "--find-renames", "--find-copies"},
		Description: "Staged structural summary. Useful for detecting renames, creates, deletes, mode changes, symlinks, and similar metadata changes.",
		Required:    true,
		Section:     "staged_changes",
	},
	{
		ID:          "staged_dirstat",
		Command:     []string{"git", "diff", "--cached", "--dirstat=files,0", "--find-renames", "--find-copies"},
		Description: "Directory-level distribution of staged file changes. Useful for choosing a Conventional Commits scope.",
		Required:    false,
		Section:     "staged_changes",
	},
	{
		ID:          "staged_diff",
		Command:     []string{"git", "diff", "--cached", "--no-ext-diff", "--no-color", "--find-renames", "--find-copies", "--submodule=short"},
		Description: "Full staged patch. This is the main source of truth for understanding what changed and why.",
		Required:    true,
		Section:     "staged_changes",
	},
	{
		ID:          "recent_commits",
		Command:     []string{"git", "log", fmt.Sprintf("-%d", RecentCommitMessageCount), "--format=%H%n%s%n%n%b%n%x1e"},
		Description: fmt.Sprintf("Last %d commit messages. Use only as style examples when no explicit repository commit-message instructions are available.", RecentCommitMessageCount),
		Required:    false,
		Section:     "style_context",
	},
	{
		ID:          "branch",
		Command:     []string{"git", "branch", "--show-current"},
		Description: "Current branch name. This is weak metadata and must not override staged changes.",
		Required:    false,
		Section:     "style_context",
	},
}

func CollectCommitMessageContext(ctx context.Context) (CommitMessageContext, error) {
	result := CommitMessageContext{
		FormatVersion: "1.0",
		StagedChanges: ContextSection{Outputs: make([]CommandOutput, 0)},
		StyleContext:  ContextSection{Outputs: make([]CommandOutput, 0)},
	}

	for _, spec := range commitContextCommandSpecs {
		output, err := runCommandSpec(ctx, spec)
		switch spec.Section {
		case "staged_changes":
			result.StagedChanges.Outputs = append(result.StagedChanges.Outputs, output)
		case "style_context":
			result.StyleContext.Outputs = append(result.StyleContext.Outputs, output)
		default:
			return CommitMessageContext{}, fmt.Errorf("unknown command section %q for command id %q", spec.Section, spec.ID)
		}

		if err != nil && spec.Required {
			return CommitMessageContext{}, err
		}
	}

	return result, nil
}

func (c CommitMessageContext) Marshal() (string, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal commit context: %w", err)
	}
	return string(b), nil
}

func runCommandSpec(ctx context.Context, spec commandSpec) (CommandOutput, error) {
	execArgs, err := buildExecutionArgs(spec.Command)
	if err != nil {
		return CommandOutput{}, err
	}

	slog.Info("executing git command", "id", spec.ID, "required", spec.Required)

	cmd := exec.CommandContext(ctx, spec.Command[0], execArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	outputValue, truncated, truncation := maybeTruncate(stdout.String(), MaxContextOutputBytes)

	out := CommandOutput{
		ID:          spec.ID,
		Command:     spec.Command,
		Description: spec.Description,
		Required:    spec.Required,
		Output:      outputValue,
		Stderr:      stderr.String(),
		ExitCode:    exitCode,
		Truncated:   truncated,
		Truncation:  truncation,
	}

	slog.Debug("git command finished",
		"id", spec.ID,
		"required", spec.Required,
		"exit_code", exitCode,
		"truncated", truncated,
		"output_bytes", len(out.Output),
		"stderr_bytes", len(out.Stderr),
	)

	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		fullCommand := make([]string, 0, 1+len(execArgs))
		fullCommand = append(fullCommand, spec.Command[0])
		fullCommand = append(fullCommand, execArgs...)
		slog.LogAttrs(ctx, logging.LevelTrace, "git command full result",
			slog.String("id", spec.ID),
			slog.Any("command", fullCommand),
			slog.String("output", out.Output),
			slog.String("stderr", out.Stderr),
			slog.Int("exit_code", out.ExitCode),
			slog.Bool("truncated", out.Truncated),
			slog.Any("truncation", out.Truncation),
		)
	}

	if runErr != nil {
		return out, fmt.Errorf("git command %q failed (exit_code=%d): %w", spec.ID, exitCode, runErr)
	}

	return out, nil
}

func buildExecutionArgs(command []string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	if command[0] != "git" {
		return nil, fmt.Errorf("unsupported command binary %q", command[0])
	}

	args := []string{"--no-pager", "-c", "color.ui=false", "-c", "core.quotepath=false"}
	if len(command) > 1 {
		args = append(args, command[1:]...)
	}
	return args, nil
}

func maybeTruncate(input string, maxBytes int) (string, bool, *Truncation) {
	if maxBytes <= 0 {
		return input, false, nil
	}

	data := []byte(input)
	original := len(data)
	if original <= maxBytes {
		return input, false, nil
	}

	head := maxBytes / 2
	tail := maxBytes - head
	kept := append([]byte{}, data[:head]...)
	kept = append(kept, data[original-tail:]...)

	return string(kept), true, &Truncation{
		MaxBytes:      maxBytes,
		OriginalBytes: original,
		KeptBytes:     len(kept),
		Strategy:      truncationStrategy,
	}
}

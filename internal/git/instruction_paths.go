package git

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/chpock/gen-commit-msg/internal/logging"
)

// InstructionPaths returns the worktree root and root-relative staged paths.
// Disabling rename detection includes both the removed and added path.
func InstructionPaths(ctx context.Context, dir string) (string, []string, error) {
	root, err := instructionQuery(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, err
	}
	data, err := instructionQuery(ctx, dir, "diff", "--cached", "--name-only", "--no-renames", "--no-relative", "-z")
	if err != nil {
		return "", nil, err
	}
	var paths []string
	if len(data) > 0 {
		if data[len(data)-1] != 0 {
			return "", nil, fmt.Errorf("git staged-path output is not NUL-terminated")
		}
		for _, path := range bytes.Split(data[:len(data)-1], []byte{0}) {
			paths = append(paths, string(path))
		}
	}
	return strings.TrimSuffix(string(root), "\n"), paths, nil
}

func instructionQuery(ctx context.Context, dir string, args ...string) ([]byte, error) {
	execArgs, err := buildExecutionArgs(append([]string{"git"}, args...))
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", execArgs...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	slog.Debug("repository instruction git query finished", "args", execArgs, "output_bytes", len(data), "error", err)
	slog.Log(ctx, logging.LevelTrace, "repository instruction git query result", "args", execArgs, "output", string(data), "stderr", stderr.String())
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		slog.Error("repository instruction git query failed", "args", execArgs, "error", err, "stderr", stderr.String())
		return nil, fmt.Errorf("collect repository instruction paths: git %s: %w", strings.Join(args, " "), err)
	}
	return data, nil
}

package git

import (
	"log/slog"
	"os/exec"
)

func IsRepo() bool {
	args, err := buildExecutionArgs([]string{"git", "rev-parse", "--git-dir"})
	if err != nil {
		slog.Error("failed to build git repo check args", "error", err)
		return false
	}
	isRepo := exec.Command("git", args...).Run() == nil
	slog.Debug("git repo check", "is_repo", isRepo)
	return isRepo
}

func HasStagedFiles() (bool, error) {
	args, err := buildExecutionArgs([]string{"git", "diff", "--staged", "--quiet"})
	if err != nil {
		slog.Error("failed to build git staged check args", "error", err)
		return false, err
	}
	cmd := exec.Command("git", args...)
	err = cmd.Run()
	if err == nil {
		slog.Debug("git staged check", "has_staged", false)
		return false, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			slog.Debug("git staged check", "has_staged", true)
			return true, nil
		}
		slog.Error("git diff --staged failed", "error", err, "exit_code", exitErr.ExitCode())
		return false, err
	}
	slog.Error("git diff --staged failed", "error", err)
	return false, err
}

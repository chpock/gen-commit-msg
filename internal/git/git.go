package git

import (
	"log/slog"
	"os/exec"
)

func IsRepo() bool {
	isRepo := exec.Command("git", "rev-parse", "--git-dir").Run() == nil
	slog.Debug("git repo check", "is_repo", isRepo)
	return isRepo
}

func HasStagedFiles() (bool, error) {
	cmd := exec.Command("git", "diff", "--staged", "--quiet")
	err := cmd.Run()
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

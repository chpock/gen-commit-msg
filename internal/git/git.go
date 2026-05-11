package git

import (
	"os/exec"
)

func IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func HasStagedFiles() (bool, error) {
	cmd := exec.Command("git", "diff", "--staged", "--quiet")
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	return false, err
}

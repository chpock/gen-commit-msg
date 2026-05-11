package git

import (
	"testing"
)

func TestIsRepo(t *testing.T) {
	if !IsRepo() {
		t.Error("IsRepo() returned false, but we are inside a git repo")
	}
}

func TestIsRepoOutside(t *testing.T) {
	t.Skip("requires non-git directory")
}

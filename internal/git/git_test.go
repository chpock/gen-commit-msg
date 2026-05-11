package git

import (
	"os"
	"testing"
)

func TestIsRepo(t *testing.T) {
	if !IsRepo() {
		t.Error("IsRepo() returned false, but we are inside a git repo")
	}
}

func TestIsRepoOutside(t *testing.T) {
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldCWD); err != nil {
			t.Fatalf("failed to restore CWD: %v", err)
		}
	}()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir to temp dir failed: %v", err)
	}

	if IsRepo() {
		t.Error("IsRepo() returned true, but we are outside a git repo")
	}
}

func TestHasStagedFiles(t *testing.T) {
	has, err := HasStagedFiles()
	if err != nil {
		t.Fatalf("HasStagedFiles() returned error: %v", err)
	}
	t.Logf("HasStagedFiles() = %v", has)
}

package repositorycontext

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func put(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}
func repository(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "init", "-q")
	return root
}
func paths(files []File) []string {
	var result []string
	for _, file := range files {
		result = append(result, file.Path)
	}
	return result
}

func collect(ctx context.Context, cwd string, profile Profile) ([]File, error) {
	result, err := Collect(ctx, cwd, profile)
	return result.Files, err
}

func TestCollectStagedAncestryAndScopedPiFiles(t *testing.T) {
	root := repository(t)
	put(t, filepath.Dir(root), "AGENTS.md", "OUTSIDE_REPO")
	for name, content := range map[string]string{
		"AGENTS.override.md": "\ufeffROOT_RULES", "AGENTS.md": "SHADOWED_ROOT",
		".pi/SYSTEM.md": "ROOT_SYSTEM", ".pi/APPEND_SYSTEM.md": "ROOT_APPEND",
		"a/AGENTS.MD": "A_RULES", "a/.pi/SYSTEM.md": "A_SYSTEM", "a/deep/CLAUDE.md": "DEEP_RULES",
		"a/deep/CLAUDE.MD": "SHADOWED_DEEP", "b/CLAUDE.MD": "B_RULES", "unrelated/AGENTS.md": "UNRELATED",
		"a/deep/line\n✓.txt": "staged", "b/change.txt": "staged",
	} {
		put(t, root, name, content)
	}
	gitRun(t, root, "add", "a/deep/line\n✓.txt", "b/change.txt")
	result, err := Collect(context.Background(), filepath.Join(root, "unrelated"), PiProfile())
	files := result.Files
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"AGENTS.override.md", ".pi/SYSTEM.md", ".pi/APPEND_SYSTEM.md", "a/AGENTS.MD", "a/.pi/SYSTEM.md", "b/CLAUDE.MD", "a/deep/CLAUDE.md"}
	if !reflect.DeepEqual(paths(files), want) {
		t.Fatalf("files=%v; want %v", paths(files), want)
	}
	if files[0].Content != "ROOT_RULES" || files[6].Directory != "a/deep" {
		t.Fatalf("BOM/scope lost: %+v", files)
	}
	prompt := Render(result)
	for _, text := range []string{"ROOT_RULES", "ROOT_SYSTEM", "ROOT_APPEND", "A_RULES", "DEEP_RULES", "B_RULES", "a/deep"} {
		if !strings.Contains(prompt, text) {
			t.Errorf("prompt missing %q", text)
		}
	}
	for _, text := range []string{"OUTSIDE_REPO", "UNRELATED", "SHADOWED"} {
		if strings.Contains(prompt, text) {
			t.Errorf("unexpected instructions: %s", text)
		}
	}
}

func TestPiCandidatePrecedenceAndNonregularFallback(t *testing.T) {
	names := []string{"AGENTS.override.md", "AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD"}
	for first := range names {
		t.Run(names[first], func(t *testing.T) {
			root := repository(t)
			for _, name := range names[first:] {
				put(t, root, name, name)
			}
			files, err := collect(context.Background(), root, PiProfile())
			if err != nil || len(files) != 1 || files[0].Path != names[first] {
				t.Fatalf("files=%+v err=%v", files, err)
			}
		})
	}
	root := repository(t)
	if err := os.Mkdir(filepath.Join(root, "AGENTS.override.md"), 0755); err != nil {
		t.Fatal(err)
	}
	put(t, root, "AGENTS.md", "")
	put(t, root, "CLAUDE.md", "NOT_SELECTED")
	files, err := collect(context.Background(), root, PiProfile())
	if err != nil || len(files) != 1 || files[0].Path != "AGENTS.md" || files[0].Content != "" {
		t.Fatalf("files=%+v err=%v", files, err)
	}
}

func TestProfileSelectionAndNativeSymlinkFollowing(t *testing.T) {
	root := repository(t)
	put(t, root, "RULES.md", "ALTERNATE_RULES")
	put(t, root, "AGENTS.md", "PI_RULES")
	files, err := collect(context.Background(), root, Profile{ContextNames: []string{"RULES.md"}})
	if err != nil || !reflect.DeepEqual(paths(files), []string{"RULES.md"}) {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Dir(root), "shared.md", "SHARED_RULES")
	if err := os.Symlink("../shared.md", filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	files, err = collect(context.Background(), root, PiProfile())
	if err != nil || len(files) != 1 || files[0].Content != "SHARED_RULES" {
		t.Fatalf("files=%+v err=%v", files, err)
	}
}

func TestRenamedAndDeletedPathsAndNestedWorktree(t *testing.T) {
	root := repository(t)
	put(t, root, "old/file.txt", "base")
	put(t, root, "deleted/file.txt", "base")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
	put(t, root, "old/AGENTS.md", "OLD")
	put(t, root, "new/AGENTS.md", "NEW")
	put(t, root, "deleted/AGENTS.md", "DELETED")
	gitRun(t, root, "mv", "old/file.txt", "new/file.txt")
	gitRun(t, root, "rm", "deleted/file.txt")
	files, err := collect(context.Background(), root, PiProfile())
	if err != nil || !reflect.DeepEqual(paths(files), []string{"deleted/AGENTS.md", "new/AGENTS.md", "old/AGENTS.md"}) {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	worktree := filepath.Join(root, "worktrees", "nested")
	gitRun(t, root, "worktree", "add", "-qb", "nested-context", worktree, "HEAD")
	put(t, root, "AGENTS.md", "MAIN_WORKTREE_ONLY")
	put(t, worktree, "AGENTS.md", "NESTED_WORKTREE")
	put(t, worktree, "deep/AGENTS.md", "NESTED_DEEP")
	put(t, worktree, "deep/change.txt", "staged nested change")
	gitRun(t, worktree, "add", "deep/change.txt")
	files, err = collect(context.Background(), filepath.Join(worktree, "deep"), PiProfile())
	if err != nil || len(files) != 2 || files[0].Content != "NESTED_WORKTREE" || files[1].Content != "NESTED_DEEP" {
		t.Fatalf("files=%+v err=%v", files, err)
	}
}

func TestCollectCancellationAndNonRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Collect(ctx, repository(t), PiProfile()); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	outside := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))
	if _, err := Collect(context.Background(), outside, PiProfile()); err == nil {
		t.Fatal("non-repository accepted")
	}
}

func TestReadFailureLogsWarningAndUsesNextCandidate(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file read permissions")
	}
	root := repository(t)
	put(t, root, "AGENTS.override.md", "PRIVATE_UNREADABLE_INSTRUCTIONS")
	put(t, root, "AGENTS.md", "FALLBACK_RULES")
	if err := os.Chmod(filepath.Join(root, "AGENTS.override.md"), 0); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	files, err := collect(context.Background(), root, PiProfile())
	if err != nil || len(files) != 1 || files[0].Content != "FALLBACK_RULES" {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "AGENTS.override.md") {
		t.Fatalf("read error not logged: %s", logs.String())
	}
	if strings.Contains(logs.String(), "FALLBACK_RULES") {
		t.Fatal("instruction contents leaked into DEBUG logs")
	}
}

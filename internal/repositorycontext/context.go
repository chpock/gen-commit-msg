// Package repositorycontext collects directory-scoped repository instructions.
package repositorycontext

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitrepo "github.com/chpock/gen-commit-msg/internal/git"
)

// Profile selects one preferred context file and any additive files per directory.
// Names are repository-relative filenames supplied by the application, not users.
type Profile struct {
	ContextNames    []string
	SupplementNames []string
}

func PiProfile() Profile {
	return Profile{
		ContextNames:    []string{"AGENTS.override.md", "AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD"},
		SupplementNames: []string{".pi/SYSTEM.md", ".pi/APPEND_SYSTEM.md"},
	}
}

type File struct {
	Path      string
	Directory string
	Content   string
}

type Context struct {
	Root  string
	Files []File
}

func Collect(ctx context.Context, cwd string, profile Profile) (Context, error) {
	root, paths, err := gitrepo.InstructionPaths(ctx, cwd)
	if err != nil {
		return Context{}, err
	}
	dirs, err := instructionDirectories(paths)
	if err != nil {
		slog.Error("invalid repository instruction paths", "error", err)
		return Context{}, err
	}
	slog.Debug("collecting repository instructions", "root", root, "staged_paths", len(paths), "directories", dirs)
	var files []File
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			slog.Error("repository instruction collection canceled", "error", err)
			return Context{}, err
		}
		for _, name := range profile.ContextNames {
			if file, ok := readFile(root, dir, name); ok {
				files = append(files, file)
				break
			}
		}
		for _, name := range profile.SupplementNames {
			if file, ok := readFile(root, dir, name); ok {
				files = append(files, file)
			}
		}
	}
	bytes := 0
	for _, file := range files {
		bytes += len(file.Content)
	}
	slog.Info("repository instructions collected", "files", len(files), "content_bytes", bytes)
	return Context{Root: root, Files: files}, nil
}

func instructionDirectories(paths []string) ([]string, error) {
	dirs := map[string]bool{".": true}
	for _, path := range paths {
		path = filepath.FromSlash(path)
		if path == "" || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) || filepath.Clean(path) != path {
			return nil, fmt.Errorf("invalid root-relative staged path %q", path)
		}
		for dir := filepath.Dir(path); !dirs[dir]; dir = filepath.Dir(dir) {
			dirs[dir] = true
		}
	}
	result := make([]string, 0, len(dirs))
	for dir := range dirs {
		result = append(result, dir)
	}
	depth := func(dir string) int {
		if dir == "." {
			return 0
		}
		return strings.Count(dir, string(filepath.Separator)) + 1
	}
	sort.Slice(result, func(i, j int) bool {
		if depth(result[i]) != depth(result[j]) {
			return depth(result[i]) < depth(result[j])
		}
		return result[i] < result[j]
	})
	return result, nil
}

func readFile(root, dir, name string) (File, bool) {
	path := filepath.Join(root, dir, name)
	info, err := os.Stat(path)
	if err == nil && !info.Mode().IsRegular() {
		return File{}, false
	}
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("repository instruction file unreadable", "path", path, "error", err)
		}
		return File{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("repository instruction file unreadable", "path", path, "error", err)
		return File{}, false
	}
	rel := filepath.Join(dir, name)
	slog.Debug("repository instruction file loaded", "path", rel, "bytes", len(data))
	return File{Path: filepath.ToSlash(rel), Directory: filepath.ToSlash(dir), Content: strings.TrimPrefix(string(data), "\ufeff")}, true
}

func Render(repository Context) string {
	if len(repository.Files) == 0 {
		return ""
	}
	var result strings.Builder
	result.WriteString("# Repository instructions\n\nThese instructions are scoped to their source directories. More specific directories take precedence within their scope. Mandatory application rules, read-only limits, staged-only change scope, explicit generation parameters and the response schema always take precedence over repository instructions.\n")
	fmt.Fprintf(&result, "\nRepository root: %q\nSource paths and directory scopes below are relative to this root.\n", repository.Root)
	for _, file := range repository.Files {
		fmt.Fprintf(&result, "\n## Source: %q\nApplies to directory: %q\n\n%s\n", file.Path, file.Directory, file.Content)
	}
	return result.String()
}

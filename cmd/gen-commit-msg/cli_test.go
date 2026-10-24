package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCLIProcess(t *testing.T) {
	if os.Getenv("GCM_TEST_CLI_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"gen-commit-msg"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	t.Fatal("missing CLI argument separator")
}

func newCLICommand(t *testing.T, dir string, args ...string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(cliTestEnvironment(), "GCM_TEST_CLI_PROCESS=1", "XDG_CONFIG_HOME="+t.TempDir())
	return cmd
}

func cliTestEnvironment() []string {
	var env []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GCM_") && !strings.HasPrefix(value, "GIT_") && !strings.HasPrefix(value, "OPENCODE_") {
			env = append(env, value)
		}
	}
	return env
}

func newCLIRepository(t *testing.T, staged bool) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.com"}} {
		cmd := exec.Command("git", args...)
		cmd.Env = cliTestEnvironment()
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	if staged {
		if err := os.WriteFile(filepath.Join(dir, "change.txt"), []byte("staged change\n"), 0644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "add", "change.txt")
		cmd.Env = cliTestEnvironment()
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add: %v\n%s", err, output)
		}
	}
	return dir
}

func TestNonInteractiveSuccessCleansUp(t *testing.T) {
	for _, destination := range []string{"stdout", "file"} {
		t.Run(destination, func(t *testing.T) {
			var mu sync.Mutex
			var requests []string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request := r.Method + " " + r.URL.Path
				mu.Lock()
				requests = append(requests, request)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				var fixture string
				switch request {
				case "GET /health":
					_, _ = fmt.Fprint(w, `{}`)
					return
				case "POST /session":
					fixture = "session_create_success.json"
				case "POST /session/mock_sess_001/message":
					fixture = "prompt_success_single.json"
				case "DELETE /session/mock_sess_001":
					fixture = "session_delete_success.json"
				default:
					t.Errorf("unexpected request: %s", request)
					http.NotFound(w, r)
					return
				}
				data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", fixture))
				if err != nil {
					t.Error(err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				_, _ = w.Write(data)
			}))
			defer api.Close()

			binDir := t.TempDir()
			script := "#!/bin/sh\nprintf 'opencode server listening on " + api.URL + "\\n'\nexec sleep 60\n"
			if err := os.WriteFile(filepath.Join(binDir, "opencode"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			dir := newCLIRepository(t, true)
			logPath := filepath.Join(t.TempDir(), "cli.log")
			args := []string{"--harness", "opencode", "--subject-max", "1", "--pause", "off", "--install-agent", "no", "--log-level", "info", "--log-file", logPath}
			outputPath := filepath.Join(t.TempDir(), "message.txt")
			if destination == "file" {
				args = append(args, "--output", outputPath)
			}
			cmd := newCLICommand(t, dir, args...)
			cmd.Env = append(cmd.Env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("CLI failed: %v\nstdout: %s\nstderr: %s", err, &stdout, &stderr)
			}
			const message = "refactor(git): simplify diff parsing\n\nSimplified the git diff parsing logic by removing redundant checks.\n"
			if destination == "file" {
				data, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != message || stdout.Len() != 0 {
					t.Errorf("file = %q, stdout = %q; want message only in file", data, &stdout)
				}
			} else if stdout.String() != message {
				t.Errorf("stdout = %q, want %q", &stdout, message)
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %s", &stderr)
			}
			mu.Lock()
			gotRequests := strings.Join(requests, "\n")
			mu.Unlock()
			wantRequests := "GET /health\nPOST /session\nPOST /session/mock_sess_001/message\nDELETE /session/mock_sess_001"
			if gotRequests != wantRequests {
				t.Errorf("requests:\n%s\nwant:\n%s", gotRequests, wantRequests)
			}
			logData, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(logData), `msg="opencode process exited cleanly"`) != 1 {
				t.Errorf("expected exactly one completed server shutdown, log:\n%s", logData)
			}
		})
	}
}

func TestCLIRepositoryIgnoresGitEnvironment(t *testing.T) {
	external := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(external, "repo.git"))
	t.Setenv("GIT_WORK_TREE", external)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(external, "index"))
	dir := newCLIRepository(t, true)
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); err != nil {
		t.Fatalf("fixture repository has no staged index: %v", err)
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("fixture Git commands modified external paths: %v", entries)
	}
}

func TestCLIPreservesOutputWithoutMessage(t *testing.T) {
	for _, reason := range []string{"no staged changes", "not a repository", "non-interactive range"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%t", reason, existing), func(t *testing.T) {
				dir := t.TempDir()
				wantExit := 1
				var wantError string
				switch reason {
				case "no staged changes":
					dir = newCLIRepository(t, false)
					wantExit = 0
				case "not a repository":
					wantError = "not a git repository"
				case "non-interactive range":
					dir = newCLIRepository(t, true)
					wantError = "needs an interactive terminal"
				}
				path := filepath.Join(t.TempDir(), "message.txt")
				const original = "previous message\n"
				if existing {
					if err := os.WriteFile(path, []byte(original), 0644); err != nil {
						t.Fatal(err)
					}
				}
				cmd := newCLICommand(t, dir, "--output", path, "--pause", "off")
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				_ = cmd.Run()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != wantExit {
					t.Fatalf("CLI state = %v, want exit %d; stderr: %s", cmd.ProcessState, wantExit, &stderr)
				}
				if stdout.Len() != 0 || !strings.Contains(stderr.String(), wantError) || (wantExit == 0 && stderr.Len() != 0) {
					t.Fatalf("unexpected output: stdout=%q stderr=%q", &stdout, &stderr)
				}
				data, err := os.ReadFile(path)
				if existing {
					if err != nil || string(data) != original {
						t.Errorf("existing output = %q, error = %v; want %q", data, err, original)
					}
				} else if !os.IsNotExist(err) {
					t.Errorf("output created without a message: content=%q error=%v", data, err)
				}
			})
		}
	}
}

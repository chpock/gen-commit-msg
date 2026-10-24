package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type workflowScenario struct {
	subjects            []string
	generationError     bool
	cleanupError        bool
	beforeDelete        func()
	promptStarted       chan struct{}
	waitForCancellation bool
}

type workflowFixture struct {
	binDir   string
	mu       sync.Mutex
	requests []string
}

const workflowBody = "Keep confirmed candidates available after cleanup."

func newWorkflowFixture(t *testing.T, scenario workflowScenario) *workflowFixture {
	t.Helper()
	fixture := &workflowFixture{binDir: t.TempDir()}
	promptData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "prompt_success_single.json"))
	if err != nil {
		t.Fatal(err)
	}
	var prompt map[string]any
	if err := json.Unmarshal(promptData, &prompt); err != nil {
		t.Fatal(err)
	}
	structured, err := json.Marshal(map[string]any{"subjects": scenario.subjects, "body": workflowBody})
	if err != nil {
		t.Fatal(err)
	}
	prompt["info"].(map[string]any)["structured"] = string(structured)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := r.Method + " " + r.URL.Path
		fixture.mu.Lock()
		fixture.requests = append(fixture.requests, request)
		fixture.mu.Unlock()
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			if r.Context().Err() == nil {
				t.Error(err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request {
		case "GET /health":
			_, _ = fmt.Fprint(w, `{}`)
		case "POST /session":
			data, readErr := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "session_create_success.json"))
			if readErr != nil {
				t.Error(readErr)
				http.Error(w, readErr.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(data)
		case "POST /session/mock_sess_001/message":
			if scenario.promptStarted != nil {
				close(scenario.promptStarted)
			}
			if scenario.waitForCancellation {
				<-r.Context().Done()
				return
			}
			if scenario.generationError {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprint(w, `{"message":"fixture generation failure"}`)
				return
			}
			if err := json.NewEncoder(w).Encode(prompt); err != nil {
				t.Error(err)
			}
		case "DELETE /session/mock_sess_001":
			if scenario.beforeDelete != nil {
				scenario.beforeDelete()
			}
			if scenario.cleanupError {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprint(w, `{"message":"fixture cleanup failure"}`)
				return
			}
			_, _ = fmt.Fprint(w, `true`)
		default:
			t.Errorf("unexpected workflow request: %s", request)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	sleepPath, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nprintf 'opencode server listening on %s\\n'\nexec %q 60\n", api.URL, sleepPath)
	if err := os.WriteFile(filepath.Join(fixture.binDir, "opencode"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *workflowFixture) command(t *testing.T, dir string, args ...string) *exec.Cmd {
	t.Helper()
	base := []string{"--harness", "opencode", "--pause", "off", "--install-agent", "no", "--log-level", "none"}
	cmd := newCLICommand(t, dir, append(base, args...)...)
	cmd.Env = append(cmd.Env, "PATH="+f.binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return cmd
}

func (f *workflowFixture) assertCleanup(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	const want = "GET /health\nPOST /session\nPOST /session/mock_sess_001/message\nDELETE /session/mock_sess_001"
	if got := strings.Join(f.requests, "\n"); got != want {
		t.Fatalf("workflow requests=%q, want %q", got, want)
	}
}

func TestCLIWorkflowCleansBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "message.txt")
	const previous = "previous message\n"
	if err := os.WriteFile(path, []byte(previous), 0644); err != nil {
		t.Fatal(err)
	}
	fixture := newWorkflowFixture(t, workflowScenario{subjects: []string{"fix: finish cleanup before output"}, beforeDelete: func() {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != previous {
			t.Errorf("output changed before cleanup: %q, %v", data, err)
		}
	}})
	cmd := fixture.command(t, newCLIRepository(t, true), "--subject-max", "1", "--output", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "fix: finish cleanup before output\n\n"+workflowBody+"\n" {
		t.Fatalf("final output=%q, %v", data, err)
	}
	fixture.assertCleanup(t)
}

func TestCLIWorkflowCleanupWarningsPreserveCandidates(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		for _, toFile := range []bool{false, true} {
			t.Run(fmt.Sprintf("quiet=%t/file=%t", quiet, toFile), func(t *testing.T) {
				fixture := newWorkflowFixture(t, workflowScenario{subjects: []string{"fix: preserve confirmed candidates"}, cleanupError: true})
				args := []string{"--subject-max", "1"}
				if quiet {
					args = append(args, "--quiet")
				}
				path := filepath.Join(t.TempDir(), "message.txt")
				if toFile {
					args = append(args, "--output", path)
				}
				cmd := fixture.command(t, newCLIRepository(t, true), args...)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("cleanup warning became failure: %v\n%s", err, &stderr)
				}
				const want = "fix: preserve confirmed candidates\n\n" + workflowBody + "\n"
				if toFile {
					data, err := os.ReadFile(path)
					if err != nil || string(data) != want || stdout.Len() != 0 {
						t.Errorf("file=%q, error=%v, stdout=%q", data, err, &stdout)
					}
				} else if stdout.String() != want {
					t.Errorf("stdout=%q, want %q", &stdout, want)
				}
				if strings.Count(stderr.String(), "Warning:") != 1 || !strings.Contains(stderr.String(), "fixture cleanup failure") {
					t.Errorf("cleanup diagnostic missing/duplicated: %s", &stderr)
				}
				fixture.assertCleanup(t)
			})
		}
	}
}

func TestCLIWorkflowKeepsPrimaryFailureAndOutput(t *testing.T) {
	fixture := newWorkflowFixture(t, workflowScenario{generationError: true, cleanupError: true})
	path := filepath.Join(t.TempDir(), "message.txt")
	const previous = "keep this message\n"
	if err := os.WriteFile(path, []byte(previous), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := fixture.command(t, newCLIRepository(t, true), "--subject-max", "1", "--output", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || stdout.Len() != 0 {
		t.Fatalf("state=%v stdout=%q stderr=%q", cmd.ProcessState, &stdout, &stderr)
	}
	if !strings.Contains(stderr.String(), "Error: generate_messages:") || !strings.Contains(stderr.String(), "Warning:") || !strings.Contains(stderr.String(), "fixture cleanup failure") {
		t.Errorf("primary failure or cleanup diagnosis lost: %s", &stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != previous {
		t.Errorf("failed generation changed output: %q, %v", data, err)
	}
	fixture.assertCleanup(t)
}

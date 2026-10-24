package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type piCLIEvent struct {
	Kind    string
	PID     int
	Message string
}

func TestPiCLIProcess(t *testing.T) {
	root := os.Getenv("GCM_TEST_PI_CLI")
	if root == "" {
		return
	}
	mark := func(kind, message string) {
		file, err := os.OpenFile(filepath.Join(root, "events"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(file).Encode(piCLIEvent{kind, os.Getpid(), message}); err != nil {
			panic(err)
		}
		_ = file.Close()
	}
	scenario := os.Getenv("GCM_TEST_PI_SCENARIO")
	if os.Args[len(os.Args)-1] == "--version" {
		mark("version", "")
		switch scenario {
		case "old":
			fmt.Println("1.3.9")
		case "unknown":
			fmt.Println("not a Pi version")
		default:
			fmt.Println("1.4.0")
		}
		os.Exit(0)
	}
	mark("rpc", "")
	emit := func(record any) {
		if err := json.NewEncoder(os.Stdout).Encode(record); err != nil {
			panic(err)
		}
	}
	decoder := json.NewDecoder(os.Stdin)
	attempt := 0
	for {
		var command struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if err := decoder.Decode(&command); err != nil {
			if err != io.EOF {
				panic(err)
			}
			break
		}
		mark(command.Type, command.Message)
		emit(map[string]any{"type": "response", "id": command.ID, "command": command.Type, "success": true, "data": map[string]any{"disposition": "started"}})
		if command.Type != "prompt" || scenario == "blocked" {
			continue
		}
		attempt++
		subjects := []string{"fix(pi): preserve staged change"}
		if strings.HasPrefix(scenario, "multiple") {
			subjects = append(subjects, "refactor(pi): clarify staged change")
		}
		if scenario == "invalid" || scenario == "corrected" && attempt == 1 {
			subjects = nil
		}
		text, _ := json.Marshal(map[string]any{"subjects": subjects, "body": ""})
		stop, detail := "stop", ""
		if scenario == "provider" {
			stop, detail = "error", "fixture authentication failure"
		}
		emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "stopReason": stop, "errorMessage": detail, "content": []any{map[string]any{"type": "text", "text": string(text)}}}})
		emit(map[string]any{"type": "agent_settled", "aborted": false})
	}
	mark("eof", "")
	if scenario == "hold-cleanup" {
		for {
			if _, err := os.Stat(filepath.Join(root, "release")); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	mark("disposed", "")
	if strings.HasSuffix(scenario, "warning") {
		os.Exit(7)
	}
	os.Exit(0)
}

type piCLIFixture struct {
	root string
	bin  string
	repo string
}

func newPiCLIFixture(t *testing.T, staged bool) *piCLIFixture {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nexec %q -test.run=^TestPiCLIProcess$ -- \"$@\"\n", self)
	if err := os.WriteFile(filepath.Join(bin, "pi"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	trap := fmt.Sprintf("#!/bin/sh\nprintf invoked > %q\nexit 61\n", filepath.Join(root, "opencode-invoked"))
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte(trap), 0755); err != nil {
		t.Fatal(err)
	}
	return &piCLIFixture{root: root, bin: bin, repo: newCLIRepository(t, staged)}
}

func (fixture *piCLIFixture) command(t *testing.T, scenario string, args ...string) *exec.Cmd {
	t.Helper()
	args = append([]string{"--pause", "off", "--subject-max", "1"}, args...)
	cmd := newCLICommand(t, fixture.repo, args...)
	cmd.Env = append(cmd.Env, "PATH="+fixture.bin, "HOME="+fixture.root, "XDG_CONFIG_HOME="+filepath.Join(fixture.root, "config"), "PI_CODING_AGENT_DIR="+filepath.Join(fixture.root, "pi-config"), "GCM_TEST_PI_CLI="+fixture.root, "GCM_TEST_PI_SCENARIO="+scenario, "GORACE=atexit_sleep_ms=0")
	return cmd
}

func (fixture *piCLIFixture) events(t *testing.T) []piCLIEvent {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixture.root, "events"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var events []piCLIEvent
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var event piCLIEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue // A running helper may still be appending its final record.
		}
		events = append(events, event)
	}
	return events
}

func (fixture *piCLIFixture) await(t *testing.T, kind string) {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		for _, event := range fixture.events(t) {
			if event.Kind == kind {
				return
			}
		}
		select {
		case <-deadline.C:
			t.Fatalf("Pi helper never reached %s; events=%v", kind, fixture.events(t))
		case <-tick.C:
		}
	}
}

func (fixture *piCLIFixture) assertLifecycle(t *testing.T, prompts int, rpc bool) {
	t.Helper()
	counts := map[string]int{}
	for _, event := range fixture.events(t) {
		counts[event.Kind]++
		if event.Kind == "version" || event.Kind == "rpc" {
			if err := syscall.Kill(event.PID, 0); err != syscall.ESRCH {
				t.Errorf("owned %s process %d remains: %v", event.Kind, event.PID, err)
			}
		}
	}
	wantRPC := 0
	if rpc {
		wantRPC = 1
	}
	if counts["version"] != 1 || counts["rpc"] != wantRPC || counts["prompt"] != prompts || counts["disposed"] != wantRPC || counts["eof"] != wantRPC {
		t.Errorf("unexpected lifecycle counters: %v; want one version, %d RPC/disposal, %d prompts", counts, wantRPC, prompts)
	}
	if _, err := os.Stat(filepath.Join(fixture.root, "opencode-invoked")); !os.IsNotExist(err) {
		t.Errorf("OpenCode was invoked after selecting Pi: %v", err)
	}
}

func TestHarnessCLIPiSelectionAndOutput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scenario string
		file     bool
		args     []string
		prompts  int
	}{
		{name: "auto prefers pi", prompts: 1},
		{name: "explicit pi file", file: true, args: []string{"--harness", "pi"}, prompts: 1},
		{name: "environment pi", prompts: 1},
		{name: "flag overrides environment", args: []string{"--harness", "pi"}, prompts: 1},
		{name: "corrected", scenario: "corrected", prompts: 2},
		{name: "cleanup warning", scenario: "warning", prompts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newPiCLIFixture(t, true)
			path := filepath.Join(fixture.root, "message")
			args := append([]string{"--agent", "custom-agent", "--install-agent", "always"}, tc.args...)
			if tc.file {
				args = append(args, "--output", path)
			}
			cmd := fixture.command(t, tc.scenario, args...)
			switch tc.name {
			case "environment pi":
				cmd.Env = append(cmd.Env, "GCM_HARNESS=pi")
			case "flag overrides environment":
				cmd.Env = append(cmd.Env, "GCM_HARNESS=opencode")
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("CLI failed: %v\nstdout=%s\nstderr=%s", err, &stdout, &stderr)
			}
			const message = "fix(pi): preserve staged change\n"
			if tc.file {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != message || stdout.Len() != 0 {
					t.Fatalf("file=%q err=%v stdout=%q", data, err, &stdout)
				}
			} else if stdout.String() != message {
				t.Fatalf("stdout=%q; want %q", &stdout, message)
			}
			if tc.scenario == "warning" {
				if !strings.Contains(stderr.String(), "Warning:") || !strings.Contains(stderr.String(), "exit status 7") {
					t.Errorf("missing cleanup warning: %s", &stderr)
				}
			} else if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %s", &stderr)
			}
			if _, err := os.Stat(filepath.Join(fixture.root, "config", "opencode")); !os.IsNotExist(err) {
				t.Errorf("Pi created OpenCode agent configuration: %v", err)
			}
			fixture.assertLifecycle(t, tc.prompts, true)
		})
	}
}

func TestHarnessCLIPiFailuresNeverFallbackOrOverwrite(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		prompts  int
		rpc      bool
		error    string
	}{
		{scenario: "old", error: "unsupported Pi version"},
		{scenario: "unknown", error: "unrecognizable Pi version"},
		{scenario: "provider", prompts: 1, rpc: true, error: "fixture authentication failure"},
		{scenario: "invalid", prompts: 3, rpc: true, error: "invalid after 3 attempts"},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			fixture := newPiCLIFixture(t, true)
			path := filepath.Join(fixture.root, "message")
			const original = "previous message\n"
			if err := os.WriteFile(path, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			cmd := fixture.command(t, tc.scenario, "--output", path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.error) {
				t.Fatalf("error=%v state=%v stdout=%q stderr=%q", err, cmd.ProcessState, &stdout, &stderr)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != original {
				t.Errorf("output changed on failure: %q, %v", data, err)
			}
			fixture.assertLifecycle(t, tc.prompts, tc.rpc)
		})
	}
}

func TestHarnessCLIPrechecksDoNotStartRuntimes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		staged bool
		args   []string
		exit   int
	}{
		{name: "help", staged: true, args: []string{"--help", "--harness", "invalid"}},
		{name: "version", staged: true, args: []string{"--version", "--harness", "invalid"}},
		{name: "no staged changes"},
		{name: "non-TTY range", staged: true, args: []string{"--subject-max", "5"}, exit: 1},
		{name: "invalid mode", staged: true, args: []string{"--harness", "invalid"}, exit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newPiCLIFixture(t, tc.staged)
			cmd := fixture.command(t, "", tc.args...)
			output, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.exit {
				t.Fatalf("state=%v error=%v output=%s", cmd.ProcessState, err, output)
			}
			if events := fixture.events(t); len(events) != 0 {
				t.Errorf("precheck started Pi: %v", events)
			}
			if _, err := os.Stat(filepath.Join(fixture.root, "opencode-invoked")); !os.IsNotExist(err) {
				t.Errorf("precheck started OpenCode: %v", err)
			}
		})
	}
}

func TestHarnessCLIOpenCodeSelection(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%t", explicit), func(t *testing.T) {
			fixture := newWorkflowFixture(t, workflowScenario{subjects: []string{"fix: select opencode correctly"}})
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(git, filepath.Join(fixture.binDir, "git")); err != nil {
				t.Fatal(err)
			}
			mode := "auto"
			marker := filepath.Join(t.TempDir(), "pi-invoked")
			if explicit {
				mode = "opencode"
				trap := fmt.Sprintf("#!/bin/sh\nprintf invoked > %q\nexit 62\n", marker)
				if err := os.WriteFile(filepath.Join(fixture.binDir, "pi"), []byte(trap), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := fixture.command(t, newCLIRepository(t, true), "--harness", mode, "--subject-max", "1")
			cmd.Env = append(cmd.Env, "PATH="+fixture.binDir)
			output, err := cmd.CombinedOutput()
			if err != nil || string(output) != "fix: select opencode correctly\n\n"+workflowBody+"\n" {
				t.Fatalf("error=%v output=%s", err, output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("explicit OpenCode probed Pi: %v", err)
			}
			fixture.assertCleanup(t)
		})
	}
}

func TestHarnessCLIPiTTY(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scenario string
		keys     string
		selected string
		existing bool
		quiet    bool
	}{
		{name: "second candidate", scenario: "multiple", keys: "\x1b[B\r", selected: "refactor(pi): clarify staged change"},
		{name: "cancel existing", scenario: "multiple", keys: "\x1b", existing: true},
		{name: "cancel missing", scenario: "multiple", keys: "\x1b"},
		{name: "visible warning", scenario: "multiple-warning", keys: "\r", selected: "fix(pi): preserve staged change"},
		{name: "quiet warning", scenario: "multiple-warning", keys: "\r", selected: "fix(pi): preserve staged change", quiet: true},
		{name: "single auto-select", selected: "fix(pi): preserve staged change"},
		{name: "single warning", scenario: "warning", selected: "fix(pi): preserve staged change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newPiCLIFixture(t, true)
			path := filepath.Join(fixture.root, "message")
			const original = "previous message\n"
			if tc.existing {
				if err := os.WriteFile(path, []byte(original), 0644); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--output", path}
			if tc.keys != "" {
				args = append(args, "--subject-max", "2")
			}
			if tc.quiet {
				args = append(args, "--quiet")
			}
			terminal := startTestTerminal(t, fixture.command(t, tc.scenario, args...))
			if tc.keys != "" {
				terminal.await(t, "refactor(pi): clarify staged change")
				terminal.send(t, tc.keys)
			}
			terminal.wait(t)
			data, err := os.ReadFile(path)
			switch {
			case tc.selected != "":
				if err != nil || string(data) != tc.selected+"\n" {
					t.Errorf("selected output=%q, error=%v", data, err)
				}
			case tc.existing:
				if err != nil || string(data) != original {
					t.Errorf("cancellation changed output=%q, error=%v", data, err)
				}
			default:
				if !os.IsNotExist(err) {
					t.Errorf("cancellation created output=%q, error=%v", data, err)
				}
			}
			if strings.HasSuffix(tc.scenario, "warning") {
				if !strings.Contains(strings.Join(strings.Fields(terminal.text()), " "), "exit status 7") {
					t.Errorf("TTY lost cleanup warning:\n%s", terminal.text())
				}
			}
			if strings.Contains(terminal.text(), "Starting OpenCode") || strings.Contains(terminal.text(), "Creating session") {
				t.Errorf("Pi TTY displayed OpenCode steps:\n%s", terminal.text())
			}
			if tc.quiet && strings.Contains(terminal.text(), "Starting Pi") {
				t.Errorf("quiet TTY displayed progress:\n%s", terminal.text())
			}
			fixture.assertLifecycle(t, 1, true)
		})
	}
}

func TestHarnessCLIPiSignalsCancelWithoutOutput(t *testing.T) {
	for _, received := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(received.String(), func(t *testing.T) {
			fixture := newPiCLIFixture(t, true)
			path := filepath.Join(fixture.root, "message")
			const original = "preserve existing output\n"
			if err := os.WriteFile(path, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			cmd := fixture.command(t, "blocked", "--output", path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			fixture.await(t, "prompt")
			if err := cmd.Process.Signal(received); err != nil {
				t.Fatal(err)
			}
			err := cmd.Wait()
			if err == nil || cmd.ProcessState.ExitCode() != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "context canceled") {
				t.Fatalf("error=%v state=%v stdout=%q stderr=%q", err, cmd.ProcessState, &stdout, &stderr)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != original {
				t.Errorf("signal changed output=%q, error=%v", data, err)
			}
			fixture.assertLifecycle(t, 1, true)
		})
	}
}

func TestHarnessCLIPiCleanupPrecedesOutput(t *testing.T) {
	fixture := newPiCLIFixture(t, true)
	path := filepath.Join(fixture.root, "message")
	const original = "preserve until cleanup\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := fixture.command(t, "hold-cleanup", "--output", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	fixture.await(t, "eof")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Errorf("output changed while Pi cleanup was blocked: %q, %v", data, err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("CLI failed: %v; stderr=%s", err, &stderr)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "fix(pi): preserve staged change\n" || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("output=%q error=%v stdout=%q stderr=%q", data, err, &stdout, &stderr)
	}
	fixture.assertLifecycle(t, 1, true)
}

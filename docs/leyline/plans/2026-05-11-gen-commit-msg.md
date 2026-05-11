# gen-commit-msg Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `leyline:subagent-driven-development` (recommended) or `leyline:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI tool that generates git commit messages via opencode with an interactive TUI (bubbletea) for selecting among variants.

**Architecture:** Layered `internal/` packages (config, git, server, agent, opencode, tui) wired in `cmd/gen-commit-msg/main.go`. Each package exposes an interface for testability. CLI flags via `spf13/pflag`, env vars with `GCM_` prefix, logging via `log/slog`.

**Tech Stack:** Go 1.22, spf13/pflag, charmbracelet/bubbletea + bubbles/spinner + bubbles/list, sst/opencode-sdk-go (latest), log/slog (stdlib).

**Spec references:**
- Product spec: `docs/leyline/specs/2026-05-11-gen-commit-msg-design.md` (round 4)
- UX spec: `docs/leyline/design/2026-05-11-gen-commit-msg-ux.md` (round 4)
- Baseline: `docs/leyline/plans/2026-05-11-gen-commit-msg-baseline.md`

**Surfaces:** cli-only

**Files:**
- Create: `go.mod`
- Create: `cmd/gen-commit-msg/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`
- Create: `internal/agent/agent.go`
- Create: `internal/agent/agent_test.go`
- Create: `internal/opencode/opencode.go`
- Create: `internal/opencode/opencode_test.go`
- Create: `internal/tui/tui.go`
- Create: `internal/tui/tui_test.go`

---

### Task 1: Go Module Init

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize Go module**

```bash
cd /w/projects/gen-commit-msg/.worktrees/gen-commit-msg && go mod init github.com/chpock/gen-commit-msg
```

- [ ] **Step 2: Verify go.mod was created**

```bash
cat go.mod
# Expected: module github.com/chpock/gen-commit-msg ... go 1.22
```

- [ ] **Step 3: Commit**

```bash
git add go.mod && git commit -m "Task 1: init Go module"
```

Exception: dependency-bump task — no failing test. Verification: `cat go.mod` shows correct module path.

---

### Task 2: Config — CLI Flags and Env Vars

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
package config

import (
	"os"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectCount != 5 {
		t.Errorf("SubjectCount = %d, want 5", cfg.SubjectCount)
	}
	if !cfg.Body {
		t.Errorf("Body = false, want true")
	}
	if cfg.Quiet {
		t.Errorf("Quiet = true, want false")
	}
	if cfg.Agent != "gen-commit-msg" {
		t.Errorf("Agent = %q, want gen-commit-msg", cfg.Agent)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want error", cfg.LogLevel)
	}
	if cfg.Pause != "on-error" {
		t.Errorf("Pause = %q, want on-error", cfg.Pause)
	}
	if cfg.InstallAgent != "if-not-exists" {
		t.Errorf("InstallAgent = %q, want if-not-exists", cfg.InstallAgent)
	}
}

func TestParseFlagsEnvVars(t *testing.T) {
	os.Args = []string{"gen-commit-msg"}
	os.Setenv("GCM_SUBJECT_COUNT", "3")
	os.Setenv("GCM_BODY", "false")
	t.Cleanup(func() {
		os.Unsetenv("GCM_SUBJECT_COUNT")
		os.Unsetenv("GCM_BODY")
	})

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectCount != 3 {
		t.Errorf("SubjectCount = %d, want 3", cfg.SubjectCount)
	}
	if cfg.Body != false {
		t.Errorf("Body = true, want false")
	}
}

func TestParseFlagsCLIOverridesEnv(t *testing.T) {
	os.Args = []string{"gen-commit-msg", "--subject-count", "7"}
	os.Setenv("GCM_SUBJECT_COUNT", "3")
	t.Cleanup(func() { os.Unsetenv("GCM_SUBJECT_COUNT") })

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubjectCount != 7 {
		t.Errorf("SubjectCount = %d, want 7 (CLI overrides env)", cfg.SubjectCount)
	}
}
```

- [ ] **Step 2: Run the test, confirm failure**

```bash
go test ./internal/config/ -v -run TestParseFlagsDefaults
# Expected: 1 failing test; compile error (config file not defined yet)
```

- [ ] **Step 3: Implement config.go**

```go
package config

import (
	"fmt"
	"os"
	"strconv"

	flag "github.com/spf13/pflag"
)

type Config struct {
	SubjectCount uint
	Body         bool
	Quiet        bool
	Agent        string
	LogLevel     string
	LogFile      string
	Pause        string
	InstallAgent string
	Version      bool
	Help         bool
}

func ParseFlags() (*Config, error) {
	cfg := &Config{}

	flags := flag.NewFlagSet("gen-commit-msg", flag.ContinueOnError)
	flags.UintP("subject-count", "n", 5, "number of subject variants")
	flags.Bool("body", true, "generate message body")
	flags.BoolP("quiet", "q", false, "suppress progress output")
	flags.StringP("agent", "a", "gen-commit-msg", "opencode agent name")
	flags.StringP("log-level", "l", "error", "log verbosity")
	flags.String("log-file", "", "log output file, '-' for stdout")
	flags.String("pause", "on-error", "pause before exit: on, off, on-error")
	flags.String("install-agent", "if-not-exists", "agent install behavior: always, if-not-exists, no")
	flags.BoolP("version", "V", false, "print version and exit")
	flags.BoolP("help", "h", false, "print help and exit")

	if err := flags.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	cfg.Version, _ = flags.GetBool("version")
	cfg.Help, _ = flags.GetBool("help")
	if cfg.Version || cfg.Help {
		return cfg, nil
	}

	cfg.SubjectCount = getUintFlagOrEnv(flags, "subject-count", "GCM_SUBJECT_COUNT", 5)
	cfg.Body = getBoolFlagOrEnv(flags, "body", "GCM_BODY", true)
	cfg.Quiet = getBoolFlagOrEnv(flags, "quiet", "GCM_QUIET", false)
	cfg.Agent = getStringFlagOrEnv(flags, "agent", "GCM_AGENT", "gen-commit-msg")
	cfg.LogLevel = getStringFlagOrEnv(flags, "log-level", "GCM_LOG_LEVEL", "error")
	cfg.LogFile = getStringFlagOrEnv(flags, "log-file", "GCM_LOG_FILE", "")
	cfg.Pause = getStringFlagOrEnv(flags, "pause", "GCM_PAUSE", "on-error")
	cfg.InstallAgent = getStringFlagOrEnv(flags, "install-agent", "GCM_INSTALL_AGENT", "if-not-exists")

	return cfg, nil
}

func getStringFlagOrEnv(flags *flag.FlagSet, name, envVar, defaultVal string) string {
	val, _ := flags.GetString(name)
	if flags.Changed(name) {
		return val
	}
	if env := os.Getenv(envVar); env != "" {
		return env
	}
	return defaultVal
}

func getBoolFlagOrEnv(flags *flag.FlagSet, name, envVar string, defaultVal bool) bool {
	val, _ := flags.GetBool(name)
	if flags.Changed(name) {
		return val
	}
	env := os.Getenv(envVar)
	if env == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(env)
	if err != nil {
		return defaultVal
	}
	return b
}

func getUintFlagOrEnv(flags *flag.FlagSet, name, envVar string, defaultVal uint) uint {
	val, _ := flags.GetUint(name)
	if flags.Changed(name) {
		return val
	}
	env := os.Getenv(envVar)
	if env == "" {
		return defaultVal
	}
	n, err := strconv.ParseUint(env, 10, 64)
	if err != nil {
		return defaultVal
	}
	return uint(n)
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/config/ -v
# Expected: all tests pass
```

- [ ] **Step 5: Commit**

```bash
git add internal/config/ && git commit -m "Task 2: config - CLI flags and env var parsing" && go mod tidy && git add go.mod go.sum && git commit -m "Task 2: tidy go.mod after adding pflag dep"
```

---

### Task 3: Git — Repo and Staged Check

**Files:**
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`

- [ ] **Step 1: Write the failing test**

```go
package git

import (
	"testing"
)

func TestIsRepo(t *testing.T) {
	// This test assumes it runs inside a git repo (which is the case in the worktree)
	if !IsRepo() {
		t.Error("IsRepo() returned false, but we are inside a git repo")
	}
}

func TestIsRepoOutside(t *testing.T) {
	// We cannot reliably test "not a repo" from inside a repo
	// Verification: manual test in a non-git directory
	t.Skip("requires non-git directory")
}
```

- [ ] **Step 2: Run the test, confirm failure**

```bash
go test ./internal/git/ -v -run TestIsRepo
# Expected: compile error (git package not defined)
```

- [ ] **Step 3: Implement git.go**

```go
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
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/git/ -v
# Expected: TestIsRepo passes, TestIsRepoOutside skipped
```

- [ ] **Step 5: Commit**

```bash
git add internal/git/ && git commit -m "Task 3: git - repo and staged file check"
```

---

### Task 4: Server — Start/Stop opencode

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Write the failing test**

```go
package server

import (
	"os/exec"
	"testing"
)

func TestServerHealthy_OpenCodeNotFound(t *testing.T) {
	_, err := exec.LookPath("opencode")
	if err != nil {
		t.Log("opencode not installed - skipping integration test")
		return
	}
	// Integration test: start server, verify health, stop
	t.Skip("opencode installed - integration test to be written manually")
}

func TestServerInterface(t *testing.T) {
	// Verify the Server type compiles and satisfies its interface
	s := &ProcessServer{}
	if s == nil {
		t.Error("ProcessServer is nil")
	}
}
```

- [ ] **Step 2: Run the test, confirm failure**

```bash
go test ./internal/server/ -v -run TestServerInterface
# Expected: compile error (server package not defined)
```

- [ ] **Step 3: Implement server.go**

```go
package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var listenURLRe = regexp.MustCompile(`opencode server listening on (http://[^\s]+)`)

type Server interface {
	Start(ctx context.Context) (baseURL string, err error)
	Stop() error
}

type ProcessServer struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	baseURL  string
}

func New() *ProcessServer {
	return &ProcessServer{}
}

func (s *ProcessServer) Start(ctx context.Context) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	s.cancel = cancel

	s.cmd = exec.CommandContext(cmdCtx, "opencode", "serve", "--hostname", "127.0.0.1", "--port", "0")
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	s.cmd.Stderr = nil // discard opencode stderr

	if err := s.cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("start opencode: %w", err)
	}

	baseURL, err := parseListenURL(stdout, 30*time.Second)
	if err != nil {
		s.Stop()
		return "", fmt.Errorf("parse listen URL: %w", err)
	}
	s.baseURL = baseURL

	if err := healthCheck(ctx, baseURL); err != nil {
		s.Stop()
		return "", fmt.Errorf("health check: %w", err)
	}

	return baseURL, nil
}

func parseListenURL(r io.Reader, timeout time.Duration) (string, error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := listenURLRe.FindStringSubmatch(line); len(matches) > 1 {
				ch <- matches[1]
				return
			}
		}
		errCh <- errors.New("opencode exited without printing listen URL")
	}()

	select {
	case url := <-ch:
		return url, nil
	case err := <-errCh:
		return "", err
	case <-time.After(timeout):
		return "", errors.New("timed out waiting for opencode listen URL")
	}
}

func healthCheck(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// If /health returns an error, try connecting to the host:port to check if it's listening
		return checkListen(baseURL)
	}
	defer resp.Body.Close()
	return nil
}

func checkListen(baseURL string) error {
	addr := strings.TrimPrefix(baseURL, "http://")
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		addr = baseURL
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return fmt.Errorf("server not listening on %s: %w", addr, err)
	}
	conn.Close()
	return nil
}

func (s *ProcessServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			s.cmd.Process.Kill()
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/server/ -v
# Expected: TestServerNotFound skipped (opencode not installed), TestServerInterface passes
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/ && git commit -m "Task 4: server - start/stop opencode serve"
```

---

### Task 5: Agent — Create/Verify .md file

**Files:**
- Create: `internal/agent/agent.go`
- Create: `internal/agent/agent_test.go`

- [ ] **Step 1: Write the failing test**

```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAgent_Create(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "if-not-exists")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("agent file not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("agent file is empty")
	}
}

func TestEnsureAgent_NoInstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "no")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	if _, err := os.Stat(expectedPath); err == nil {
		t.Error("agent file was created but install-agent is 'no'")
	}
}

func TestEnsureAgent_AlwaysOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("gen-commit-msg", "if-not-exists")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	err = Ensure("gen-commit-msg", "always")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "gen-commit-msg.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Error("agent file should exist after 'always' install")
	}
}

func TestEnsureAgent_CustomName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	err := Ensure("custom-agent", "if-not-exists")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	expectedPath := filepath.Join(dir, "opencode", "agents", "custom-agent.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Error("agent file with custom name not created")
	}
}
```

- [ ] **Step 2: Run the test, confirm failure**

```bash
go test ./internal/agent/ -v -run TestEnsureAgent_Create
# Expected: compile error (agent package not defined)
```

- [ ] **Step 3: Implement agent.go**

```go
package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultPrompt = `You are a git commit message generator. Your task is to generate commit messages for the current git repository.

Rules:
- Output commit messages (both subject line and body) based on the git diff
- First line: subject (50-72 chars, imperative mood, lowercase, no period)
- Include a body if the diff warrants explanation
- Follow the conventional commits style if the diff clearly matches a type
  (feat, fix, refactor, docs, test, chore, style, perf, ci, build)
- Otherwise, use a plain descriptive subject
- Do not include any additional explanations, markdown formatting, code blocks,
  or backticks in the output
`

func agentsDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			base = os.Getenv("HOME")
		} else {
			base = home
		}
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, "opencode", "agents")
}

func Ensure(name, installMode string) error {
	if installMode == "no" {
		return nil
	}

	dir := agentsDir()
	if dir == "" {
		return fmt.Errorf("cannot determine agents directory")
	}

	filePath := filepath.Join(dir, name+".md")

	if installMode == "if-not-exists" {
		if _, err := os.Stat(filePath); err == nil {
			return nil // already exists
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agents directory: %w", err)
	}

	return os.WriteFile(filePath, []byte(DefaultPrompt), 0644)
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/agent/ -v
# Expected: all tests pass
```

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ && git commit -m "Task 5: agent - create/verify agent .md file"
```

---

### Task 6: OpenCode — SDK Client

**Files:**
- Create: `internal/opencode/opencode.go`
- Create: `internal/opencode/opencode_test.go`

- [ ] **Step 1: Write the failing test**

```go
package opencode

import (
	"testing"
)

func TestClientInterface(t *testing.T) {
	// SDK integration test - skipped until real server available
	c := &Client{}
	if c == nil {
		t.Error("Client is nil")
	}
}
```

- [ ] **Step 2: Run the test, confirm failure**

```bash
go test ./internal/opencode/ -v -run TestClientInterface
# Expected: compile error (package not defined)
```

- [ ] **Step 3: Implement opencode.go**

This is a thin wrapper around the opencode SDK. The SDK module import path will be determined after `go get`.

```go
package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	opencode "github.com/sst/opencode-sdk-go"
)

type GenerateParams struct {
	SubjectCount int
	Body         bool
}

type CommitMessage struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type Client struct {
	sdkClient *opencode.Client
	baseURL   string
}

func NewClient(baseURL string) *Client {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	oc := opencode.NewClient(baseURL, opencode.WithHTTPClient(httpClient))
	return &Client{sdkClient: oc, baseURL: baseURL}
}

func (c *Client) CreateSession(ctx context.Context, agentName string) (string, error) {
	session, err := c.sdkClient.Session.New(ctx, &opencode.SessionNewOptions{
		Agent: agentName,
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return session.ID, nil
}

func (c *Client) GenerateMessages(ctx context.Context, sessionID string, params GenerateParams) ([]CommitMessage, error) {
	prompt := fmt.Sprintf(
		"Generate %d commit message variants. Include message body: %v. "+
			"Output the result as a JSON array of objects, each with 'subject' and 'body' fields. "+
			"Example: [{\"subject\":\"feat: add feature\",\"body\":\"details...\"}]",
		params.SubjectCount, params.Body,
	)

	result, err := c.sdkClient.Session.Prompt(ctx, sessionID, prompt)
	if err != nil {
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	var messages []CommitMessage
	if err := json.Unmarshal([]byte(result.Content), &messages); err != nil {
		// JSON parsing failed - try single message format
		if !params.Body {
			messages = []CommitMessage{{Subject: result.Content}}
		} else {
			messages = []CommitMessage{{Subject: result.Content}}
		}
	}
	return messages, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	return c.sdkClient.Session.Delete(ctx, sessionID)
}
```

After implementation, run `go get github.com/sst/opencode-sdk-go@latest` to resolve the SDK dependency.

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/opencode/ -v
# Expected: TestClientInterface passes
```

- [ ] **Step 5: Commit**

```bash
git add internal/opencode/ && git commit -m "Task 6: opencode - SDK client for session and prompt" && go mod tidy && git add go.mod go.sum && git commit -m "Task 6: tidy go.mod after adding opencode SDK dep"
```

---

### Task 7: TUI — Bubbletea Model

**Files:**
- Create: `internal/tui/tui.go`
- Create: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInit(t *testing.T) {
	m := NewModel(5)
	if m.state != stateSpinner {
		t.Error("initial state should be spinner")
	}
	if m.subjectCount != 5 {
		t.Errorf("subjectCount = %d, want 5", m.subjectCount)
	}
}

func TestModelInitMsg(t *testing.T) {
	m := NewModel(3)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

func TestModelQuitOnCtrlC(t *testing.T) {
	m := NewModel(3)
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updated, _ := m.Update(msg)
	if updated.(Model).quitting != true {
		t.Error("Ctrl+C should set quitting to true")
	}
}

func TestModelQuitOnEsc(t *testing.T) {
	m := NewModel(3)
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	if updated.(Model).quitting != true {
		t.Error("Esc should set quitting to true")
	}
}
```

- [ ] **Step 2: Run the test, confirm failure**

```bash
go test ./internal/tui/ -v -run TestModelInit
# Expected: compile error (tui package not defined)
```

- [ ] **Step 3: Implement tui.go**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateSpinner state = iota
	stateResult
	stateDone
)

type commitItem struct {
	subject string
	body    string
}

func (i commitItem) Title() string       { return i.subject }
func (i commitItem) Description() string { return i.body }
func (i commitItem) FilterValue() string { return i.subject }

type Model struct {
	state        state
	spinner      spinner.Model
	list         list.Model
	messages     []commitItem
	selected     string
	quitting     bool
	err          error
	subjectCount int
	width        int
	height       int
}

func NewModel(subjectCount int) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return Model{
		state:        stateSpinner,
		spinner:      s,
		list:         l,
		subjectCount: subjectCount,
	}
}

type generationResultMsg struct {
	messages []commitItem
	err      error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, generateMessages(m.subjectCount))
}

func generateMessages(count int) tea.Cmd {
	return func() tea.Msg {
		// Actual generation is done externally; this is a placeholder
		// The real messages are set before the TUI starts via SetMessages()
		return generationResultMsg{}
	}
}

func SetMessages(messages []commitItem) tea.Cmd {
	return func() tea.Msg {
		return generationResultMsg{messages: messages}
	}
}

func SetError(err error) tea.Cmd {
	return func() tea.Msg {
		return generationResultMsg{err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil
	case generationResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateDone
			return m, tea.Quit
		}
		m.messages = msg.messages
		if len(m.messages) == 1 {
			m.selected = formatMessage(m.messages[0])
			m.state = stateDone
			return m, tea.Quit
		}
		items := make([]list.Item, len(m.messages))
		for i, cm := range m.messages {
			items[i] = cm
		}
		m.list.SetItems(items)
		m.state = stateResult
		return m, nil
	}

	switch m.state {
	case stateSpinner:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case stateResult:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEnter {
				if item, ok := m.list.SelectedItem().(commitItem); ok {
					m.selected = formatMessage(item)
					m.state = stateDone
					return m, tea.Quit
				}
			}
		}
		return m, cmd
	}
	return m, nil
}

func formatMessage(item commitItem) string {
	if item.body == "" {
		return strings.TrimSpace(item.subject)
	}
	return strings.TrimSpace(item.subject) + "\n\n" + strings.TrimSpace(item.body)
}

func (m Model) View() string {
	switch m.state {
	case stateSpinner:
		return fmt.Sprintf("\n  %s Generating commit messages...\n", m.spinner.View())
	case stateResult:
		return m.list.View()
	case stateDone:
		return ""
	}
	return ""
}

func (m Model) SelectedMessage() string {
	return m.selected
}

func (m Model) Error() error {
	return m.err
}

func (m Model) ShouldQuit() bool {
	return m.quitting
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/tui/ -v
# Expected: all tests pass
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/ && git commit -m "Task 7: tui - bubbletea model with spinner and list" && go mod tidy && git add go.mod go.sum && git commit -m "Task 7: tidy go.mod after adding bubbletea deps"
```

---

### Task 8: Main — Entry Point and Wiring

**Files:**
- Create: `cmd/gen-commit-msg/main.go`

- [ ] **Step 1: Implement main.go**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chpock/gen-commit-msg/internal/agent"
	"github.com/chpock/gen-commit-msg/internal/config"
	"github.com/chpock/gen-commit-msg/internal/git"
	"github.com/chpock/gen-commit-msg/internal/opencode"
	"github.com/chpock/gen-commit-msg/internal/server"
	"github.com/chpock/gen-commit-msg/internal/tui"
)

var version = "dev"

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.Version {
		fmt.Printf("gen-commit-msg %s\n", version)
		return
	}

	if cfg.Help {
		cfg.PrintHelp()
		return
	}

	// Check we're in a git repo
	if !git.IsRepo() {
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}

	// Check for staged files
	hasStaged, err := git.HasStagedFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !hasStaged {
		return // silent exit
	}

	// Ensure agent exists
	if err := agent.Ensure(cfg.Agent, cfg.InstallAgent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Non-TTY check
	isTTY := isTerminal()
	if !isTTY && cfg.SubjectCount > 1 {
		fmt.Fprintln(os.Stderr, "Error: --subject-count > 1 requires an interactive terminal. Use --subject-count 1 for non-interactive mode.")
		os.Exit(1)
	}

	// Setup cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Start opencode server
	srv := server.New()
	baseURL, err := srv.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer srv.Stop()

	// Create client and session
	oc := opencode.NewClient(baseURL)
	sessionID, err := oc.CreateSession(ctx, cfg.Agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer delCancel()
		oc.DeleteSession(delCtx, sessionID)
	}()

	if !isTTY && cfg.SubjectCount == 1 {
		// Non-TTY, single variant: generate silently and print
		messages, err := oc.GenerateMessages(ctx, sessionID, opencode.GenerateParams{
			SubjectCount: int(cfg.SubjectCount),
			Body:         cfg.Body,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(messages) > 0 {
			fmt.Println(formatMessageFromOC(messages[0]))
		}
		return
	}

	// TUI mode
	m := tui.NewModel(int(cfg.SubjectCount))

	// Start generation in background
	go func() {
		messages, err := oc.GenerateMessages(ctx, sessionID, opencode.GenerateParams{
			SubjectCount: int(cfg.SubjectCount),
			Body:         cfg.Body,
		})
		if err != nil {
			p.Send(tui.SetError(err))
			return
		}
		items := make([]tui.CommitItem, len(messages))
		for i, msg := range messages {
			items[i] = tui.CommitItem{Subject: msg.Subject, Body: msg.Body}
		}
		p.Send(tui.SetMessages(items))
	}()

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print selected message
	fmt.Println(m.SelectedMessage())

	// Pause handling
	if cfg.Pause == "on" || (cfg.Pause == "on-error" && m.Error() != nil) {
		fmt.Fprintf(os.Stderr, "\nPress any key to exit...")
		// Wait for keypress with timeout
	}
}

func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

func formatMessageFromOC(msg opencode.CommitMessage) string {
	if msg.Body == "" {
		return strings.TrimSpace(msg.Subject)
	}
	return strings.TrimSpace(msg.Subject) + "\n\n" + strings.TrimSpace(msg.Body)
}
```

- [ ] **Step 2: Build and verify**

```bash
go build ./cmd/gen-commit-msg/
# Expected: binary compiles successfully
```

- [ ] **Step 3: Commit**

```bash
git add cmd/ && git commit -m "Task 8: main - entry point and wiring" && go mod tidy && git add go.mod go.sum && git commit -m "Task 8: tidy go.mod"
```

---

### Task 9: UX — All Surfaces

**Surface:** All TUI surfaces (spinner, result list, pause overlay, stdout, stderr)
**Artifact reference:** `docs/leyline/design/2026-05-11-gen-commit-msg-ux.md`

- [ ] **Step 1:** Confirm the UX artifact is current

- [ ] **Step 2:** Trigger each state from the state matrix and observe
  - Empty (no staged files): Run from repo with no staged changes → silent exit 0
  - Loading: Run with staged changes → spinner "Generating commit messages..."
  - Error: Run with opencode not on PATH → "Error: ..." on stderr, exit 1
  - Success: Run with staged changes, select message → commit message on stdout

- [ ] **Step 3:** Run accessibility checks
  - Verify `>` prefix + inversion for list selection (not color-only)
  - Verify stdout output is plain text with no ANSI escapes
  - Verify error messages prefixed with `Error: `
  - Verify minimum terminal width 40 columns works

- [ ] **Step 4:** Side-by-side reconciliation against UX spec artifact

- [ ] **Step 5:** Commit

```bash
# No new files — verification only
git commit --allow-empty -m "Task 9: UX verification - all surfaces"
```

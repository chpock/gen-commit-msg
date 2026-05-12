# Progress View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `leyline:subagent-driven-development` (recommended) or `leyline:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a step-by-step progress view (5 steps: server start, session create, generate, session delete, server stop) displayed before the commit message selection TUI.

**Architecture:** Single TUI with a new `stateProgress` phase. A goroutine in main.go executes steps sequentially and sends `stepUpdateMsg` via `p.Send()`. The TUI model is a pure view with no server/client references. After all 5 steps complete, the TUI auto-transitions to the existing message selection view after a 300ms delay.

**Tech Stack:** Go, bubbletea, bubbles (spinner, list), lipgloss

**Spec references:**
- Product spec: `docs/leyline/specs/2026-05-12-progress-view-design.md` (round 3)
- UX spec: `docs/leyline/design/2026-05-12-progress-view-ux.md` (round 3)
- Baseline: `docs/leyline/plans/2026-05-12-progress-view-baseline.md`

**Surfaces:** single-screen-ui

**Files:**
- Modify: `internal/tui/tui.go` — new types, progress state, progress view rendering
- Modify: `internal/tui/tui_test.go` — tests for progress states and transitions
- Modify: `cmd/gen-commit-msg/main.go` — restructure orchestration for progress flow

---

### Task 1: Add step tracking types to TUI model

**Files:**
- Modify: `internal/tui/tui.go` (near `type state int`, ~line 15)

- [ ] **Step 1: Write failing test for new types**

In `internal/tui/tui_test.go`, add:

```go
func TestStepStatusValues(t *testing.T) {
	if stepPending != 0 { t.Error("stepPending should be 0 (zero value)") }
	if stepRunning != 1 { t.Error("stepRunning should be 1") }
	if stepDone != 2 { t.Error("stepDone should be 2") }
	if stepFailed != 3 { t.Error("stepFailed should be 3") }
	if stepWarning != 4 { t.Error("stepWarning should be 4") }
}

func TestStepLabels(t *testing.T) {
	labels := stepLabels()
	if len(labels) != 5 {
		t.Fatalf("expected 5 step labels, got %d", len(labels))
	}
	if labels[0] != "Starting OpenCode..." { t.Errorf("step 0 label = %q", labels[0]) }
	if labels[1] != "Creating session..." { t.Errorf("step 1 label = %q", labels[1]) }
	if labels[2] != "Generating commit messages..." { t.Errorf("step 2 label = %q", labels[2]) }
	if labels[3] != "Deleting session..." { t.Errorf("step 3 label = %q", labels[3]) }
	if labels[4] != "Stopping OpenCode server..." { t.Errorf("step 4 label = %q", labels[4]) }
}
```

- [ ] **Step 2: Run tests, confirm failure**

```
go test -count=1 -race ./internal/tui/
# Expected: compilation errors - stepPending, stepRunning, etc. undefined
```

- [ ] **Step 3: Implement types**

In `internal/tui/tui.go`, add after the `state` type:

```go
type stepStatus int

const (
	stepPending  stepStatus = iota
	stepRunning
	stepDone
	stepFailed
	stepWarning
)

type stepItem struct {
	label  string
	status stepStatus
}

type stepUpdateMsg struct {
	index  int
	status stepStatus
	detail string
}

func stepLabels() [5]string {
	return [5]string{
		"Starting OpenCode...",
		"Creating session...",
		"Generating commit messages...",
		"Deleting session...",
		"Stopping OpenCode server...",
	}
}
```

Also add `logPath string` field to the `Model` struct and a `SetLogPath(path string) tea.Msg` function:

```go
type setLogPathMsg struct {
	path string
}

func SetLogPath(path string) tea.Msg {
	return setLogPathMsg{path: path}
}
```

- [ ] **Step 4: Run tests, confirm pass**

```
go test -count=1 -race ./internal/tui/
```

- [ ] **Step 5: Commit**

```
git add internal/tui/tui.go internal/tui/tui_test.go && git commit -m "feat(tui): add step tracking types and step labels"
```

---

### Task 2: Add progress view state and rendering

**Files:**
- Modify: `internal/tui/tui.go` (state enum, Model struct, Init, Update, View)
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests for progress view**

In `internal/tui/tui_test.go`, add:

```go
func TestProgressStateInit(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init in progress state should return spinner tick")
	}
}

func TestProgressViewShowsAllSteps(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: stepPending}
	}
	v := m.View()
	for _, label := range labels {
		if !contains(v, label) {
			t.Errorf("progress view missing label: %q", label)
		}
	}
}

func TestStepUpdateChangesStatus(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: stepPending}
	}
	msg := stepUpdateMsg{index: 0, status: stepRunning}
	updated, _ := m.Update(msg)
	if updated.(Model).steps[0].status != stepRunning {
		t.Errorf("step 0 status = %v, want stepRunning", updated.(Model).steps[0].status)
	}
}

func TestProgressDoneAllStepsTransitionsToResult(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: stepDone}
	}
	msg := allStepsDoneMsg{}
	updated, cmd := m.Update(msg)
	if updated.(Model).state != stateResult {
		t.Errorf("state = %v, want stateResult after all steps done", updated.(Model).state)
	}
	if cmd == nil {
		t.Error("should return a command after allStepsDone")
	}
}

func TestStepFailureShowsError(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	labels := stepLabels()
	for i := range m.steps {
		m.steps[i] = stepItem{label: labels[i], status: stepPending}
	}
	msg := stepUpdateMsg{index: 2, status: stepFailed, detail: "connection refused"}
	updated, _ := m.Update(msg)
	if updated.(Model).steps[2].status != stepFailed {
		t.Error("step 2 should be failed")
	}
	if updated.(Model).stepDetail != "connection refused" {
		t.Errorf("stepDetail = %q, want %q", updated.(Model).stepDetail, "connection refused")
	}
	v := updated.(Model).View()
	if !contains(v, "connection refused") {
		t.Errorf("progress view missing error detail: %q", v)
	}
}

func TestProgressViewQuiet(t *testing.T) {
	m := NewModel(5, true)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: stepPending}
	}
	v := m.View()
	if v != "" {
		t.Errorf("quiet progress view should be empty, got %q", v)
	}
}

func TestSetLogPathMsg(t *testing.T) {
	msg := SetLogPath("/tmp/test.log")
	lp, ok := msg.(setLogPathMsg)
	if !ok {
		t.Fatal("SetLogPath should return setLogPathMsg")
	}
	if lp.path != "/tmp/test.log" {
		t.Errorf("path = %q, want /tmp/test.log", lp.path)
	}
}

func TestProgressViewShowsLogPath(t *testing.T) {
	m := NewModel(5, false)
	m.state = stateProgress
	m.steps = make([]stepItem, 5)
	for i := range m.steps {
		m.steps[i] = stepItem{label: "step", status: stepPending}
	}
	m.logPath = "/tmp/test.log"
	m.stepDetail = "something failed"
	m.steps[0].status = stepFailed
	v := m.View()
	if !contains(v, "/tmp/test.log") {
		t.Errorf("progress view missing log path: %q", v)
	}
}
```

- [ ] **Step 2: Run tests, confirm failure**

```
go test -count=1 -race ./internal/tui/
# Expected: compilation errors - stateProgress, allStepsDoneMsg, stepDetail, logPath undefined
```

- [ ] **Step 3: Implement progress state**

In `internal/tui/tui.go`:

Add `stateProgress` to the state enum:
```go
const (
	stateProgress state = iota
	stateSpinner
	stateResult
	stateError
)
```

Add fields to Model struct (after `height int`):
```go
	steps      []stepItem
	stepDetail string
	logPath    string
```

Add `allStepsDoneMsg` type:
```go
type allStepsDoneMsg struct{}
```

Update `NewModel` to initialize steps:
```go
func NewModel(subjectCount int, quiet bool) Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot

	delegate := newCommitDelegate()
	l := list.New([]list.Item{}, delegate, 40, 10)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	labels := stepLabels()
	steps := make([]stepItem, 5)
	for i := range steps {
		steps[i] = stepItem{label: labels[i], status: stepPending}
	}

	return Model{
		state:        stateProgress,
		spinner:      s,
		list:         l,
		steps:        steps,
		subjectCount: subjectCount,
		quiet:        quiet,
	}
}
```

Update `Init` to handle progress state:
```go
func (m Model) Init() tea.Cmd {
	if m.quiet {
		return nil
	}
	if m.state == stateProgress {
		return m.spinner.Tick
	}
	return nil
}
```

Update `Update` — add handling before the `switch m.state` block:
```go
	case stepUpdateMsg:
		if m.state == stateProgress {
			if msg.index >= 0 && msg.index < len(m.steps) {
				m.steps[msg.index].status = msg.status
			}
			m.stepDetail = msg.detail
			if msg.status == stepFailed || msg.status == stepWarning {
				return m, nil
			}
			return m, m.spinner.Tick
		}
	case allStepsDoneMsg:
		if m.state == stateProgress {
			m.state = stateResult
			return m, nil
		}
	case setLogPathMsg:
		m.logPath = msg.path
		return m, nil
```

Update `Update` — add key handling in progress error state (in the `tea.KeyMsg` section, before the state switch):
```go
		if m.state == stateProgress {
			for _, s := range m.steps {
				if s.status == stepFailed {
					m.quitting = true
					return m, tea.Quit
				}
			}
		}
```

Update `View` — add progress view rendering:
```go
	case stateProgress:
		if m.quiet {
			return ""
		}
		var b strings.Builder
		for _, s := range m.steps {
			b.WriteString("\n  ")
			switch s.status {
			case stepPending:
				b.WriteString("  ")
			case stepRunning:
				b.WriteString(m.spinner.View())
			case stepDone:
				b.WriteString("✓")
			case stepFailed:
				b.WriteString("✗")
			case stepWarning:
				b.WriteString("⚠")
			}
			b.WriteString(" ")
			b.WriteString(s.label)
		}
		if m.stepDetail != "" {
			b.WriteString("\n\n  ")
			b.WriteString(m.stepDetail)
		}
		if m.logPath != "" {
			b.WriteString("\n  Details: ")
			b.WriteString(m.logPath)
		}
		b.WriteString("\n")
		return b.String()
```

Note: `strings` import is already present in tui.go.

- [ ] **Step 4: Run tests, confirm pass**

```
go test -count=1 -race ./internal/tui/
```

- [ ] **Step 5: Commit**

```
git add internal/tui/tui.go internal/tui/tui_test.go && git commit -m "feat(tui): add progress view state with step rendering"
```

---

### Task 3: Restructure main.go for progress flow

**Files:**
- Modify: `cmd/gen-commit-msg/main.go`

- [ ] **Step 1: Write failing test**

No direct unit test for main.go — verification will be via `make build` + integration flow. Write a check in main_test.go or verify the build compiles.

Since `cmd/gen-commit-msg` has no test files, add a basic compilation test:

In `cmd/gen-commit-msg/main_test.go`:

```go
package main

import (
	"testing"
)

func TestMainDoesNotCrashOnBuild(t *testing.T) {
	// Compile-time check: main.go must compile without errors.
	// Actual integration tested via make build.
}
```

- [ ] **Step 2: Run tests, confirm they compile**

```
go build ./cmd/gen-commit-msg/
# Expected: builds cleanly (no progress flow changes yet)
```

- [ ] **Step 3: Implement main.go changes**

Replace the orchestration section in `main.go` (from `srv := server.New()` to the `p.Run()` block). The key changes:

```go
	// Resolve log path for error display in TUI.
	logPath := logging.LogFilePath(cfg.LogFile)

	isTTY := isTerminal()
	if isTTY && cfg.SubjectCount > 1 || (!cfg.Quiet && cfg.SubjectCount == 1) {
		// Interactive mode — start TUI before any backend work.
		m := tui.NewModel(int(cfg.SubjectCount), cfg.Quiet)
		if logPath != "" {
			m, _ = m.Update(tui.SetLogPath(logPath)).(tui.Model)
		}
		tty, closeTTY := openTTY()
		defer closeTTY()
		p := tea.NewProgram(m, tea.WithOutput(tty))

		go func() {
			// Small delay to ensure first View() renders before any step transitions.
			time.Sleep(50 * time.Millisecond)

			srv := server.New()
			baseURL, err := srv.Start(ctx)
			if err != nil {
				slog.Error("failed to start server", "error", err)
				p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "Error: opencode server failed to start: " + err.Error()})
				return
			}
			p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepDone})

			oc := opencode.NewClient(baseURL)
			sessionID, err := oc.CreateSession(ctx, cfg.Agent)
			if err != nil {
				slog.Error("failed to create session", "agent", cfg.Agent, "error", err)
				p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepFailed, Detail: "Error: failed to create session: " + err.Error()})
				return
			}
			p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepDone})

			genParams := opencode.GenerateParams{
				SubjectCount: int(cfg.SubjectCount),
				Body:         cfg.Body,
			}
			messages, genErr := oc.GenerateMessages(ctx, sessionID, genParams)
			if genErr != nil {
				slog.Error("failed to generate messages", "error", genErr)
				p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepFailed, Detail: "Error: failed to generate commit messages: " + genErr.Error()})
				return
			}
			p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepDone})

			// Cleanup steps — non-critical after successful generation.
			delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer delCancel()
			if err := oc.DeleteSession(delCtx, sessionID); err != nil {
				slog.Warn("failed to delete session", "session_id", sessionID, "error", err)
				p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepWarning, Detail: "Cleanup issue: " + err.Error()})
			} else {
				p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepDone})
			}

			if err := srv.Stop(); err != nil {
				slog.Warn("failed to stop server", "error", err)
				p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepWarning, Detail: "Cleanup issue: " + err.Error()})
			} else {
				p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepDone})
			}

			// All steps complete — send messages to TUI.
			items := make([]tui.CommitItem, len(messages))
			for i, msg := range messages {
				items[i] = tui.CommitItem{Subject: msg.Subject, Body: msg.Body}
			}
			p.Send(tui.SetMessages(items))
		}()

		finalModel, err := p.Run()
		if err != nil {
			slog.Error("TUI initialization failed", "error", err)
			fmt.Fprintf(os.Stderr, "Error: TUI initialization failed: %v\n", err)
			closeTTY()
			os.Exit(1)
		}

		m = finalModel.(tui.Model)
		selected := m.SelectedMessage()
		if m.Error() != nil {
			slog.Error("TUI ended with error", "error", m.Error())
			fmt.Fprintln(os.Stderr, formatOpenCodeError(m.Error()))
			if cfg.Pause == "on" {
				pause(isTTY)
			}
			os.Exit(1)
		}
		fmt.Println(selected)
		return
	}

	// Non-interactive / non-TTY paths remain unchanged below this point.
	// (Keep existing code for non-TTY and single-subject-quiet modes)
```

Wait — this is getting complex. The key changes are:

1. Move TUI start to before server init
2. Move server start, session create, generate, cleanup into a goroutine
3. Send stepUpdateMsg after each step
4. Send SetMessages at the end

Let me rewrite the main.go more carefully, preserving the existing non-TTY paths.

The full replacement for the main.go orchestration section (from `srv := server.New()` line 98 through the end of the interactive TUI block, approximately line 226):

```go
	isTTY := isTerminal()
	slog.Debug("terminal check", "is_tty", isTTY)
	if !isTTY && cfg.SubjectCount > 1 {
		slog.Error("non-TTY with subject count > 1",
			"subject_count", cfg.SubjectCount, "is_tty", isTTY)
		fmt.Fprintln(os.Stderr, "Error: --subject-count > 1 requires an interactive terminal. Use --subject-count 1 for non-interactive mode.")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Debug("signal received", "signal", sig)
		cancel()
	}()

	logPath := logging.LogFilePath(cfg.LogFile)

	if isTTY {
		m := tui.NewModel(int(cfg.SubjectCount), cfg.Quiet)
		if logPath != "" {
			m, _ = m.Update(tui.SetLogPath(logPath)).(tui.Model)
		}
		tty, closeTTY := openTTY()
		defer closeTTY()
		p := tea.NewProgram(m, tea.WithOutput(tty))

		go func() {
			time.Sleep(50 * time.Millisecond)

			srv := server.New()
			baseURL, err := srv.Start(ctx)
			if err != nil {
				slog.Error("failed to start server", "error", err)
				p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "Error: opencode server failed to start: " + err.Error()})
				return
			}
			p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepDone})

			oc := opencode.NewClient(baseURL)
			sessionID, err := oc.CreateSession(ctx, cfg.Agent)
			if err != nil {
				slog.Error("failed to create session", "agent", cfg.Agent, "error", err)
				p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepFailed, Detail: "Error: failed to create session: " + err.Error()})
				return
			}
			p.Send(tui.StepUpdateMsg{Index: 1, Status: tui.StepDone})

			genParams := opencode.GenerateParams{
				SubjectCount: int(cfg.SubjectCount),
				Body:         cfg.Body,
			}
			messages, genErr := oc.GenerateMessages(ctx, sessionID, genParams)
			if genErr != nil {
				slog.Error("failed to generate messages", "error", genErr)
				p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepFailed, Detail: "Error: failed to generate commit messages: " + genErr.Error()})
				return
			}
			p.Send(tui.StepUpdateMsg{Index: 2, Status: tui.StepDone})

			delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer delCancel()
			if err := oc.DeleteSession(delCtx, sessionID); err != nil {
				slog.Warn("failed to delete session", "session_id", sessionID, "error", err)
				p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepWarning, Detail: "Cleanup issue: " + err.Error()})
			} else {
				p.Send(tui.StepUpdateMsg{Index: 3, Status: tui.StepDone})
			}

			if err := srv.Stop(); err != nil {
				slog.Warn("failed to stop server", "error", err)
				p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepWarning, Detail: "Cleanup issue: " + err.Error()})
			} else {
				p.Send(tui.StepUpdateMsg{Index: 4, Status: tui.StepDone})
			}

			items := make([]tui.CommitItem, len(messages))
			for i, msg := range messages {
				items[i] = tui.CommitItem{Subject: msg.Subject, Body: msg.Body}
			}
			p.Send(tui.SetMessages(items))
		}()

		finalModel, err := p.Run()
		if err != nil {
			slog.Error("TUI initialization failed", "error", err)
			fmt.Fprintf(os.Stderr, "Error: TUI initialization failed: %v\n", err)
			closeTTY()
			os.Exit(1)
		}

		m = finalModel.(tui.Model)
		if m.Error() != nil {
			slog.Error("TUI ended with error", "error", m.Error())
			fmt.Fprintln(os.Stderr, formatOpenCodeError(m.Error()))
		} else {
			selected := m.SelectedMessage()
			slog.Info("message selected", "message", truncateString(selected, 80))
			fmt.Println(selected)
		}

		if cfg.Pause == "on" {
			pause(isTTY)
		}
		return
	}

	// Non-TTY / quiet single-subject paths (unchanged from current code).
	// ... (existing code continues here)
```

Hmm, actually, the existing non-TTY paths still need to work unchanged. The key insight: the whole interactive path is replaced, but the non-TTY paths at `!isTTY && cfg.SubjectCount == 1` and the quiet+single path should remain as-is.

Let me restructure this more cleanly. The file should be:

1. Parse flags, config, logging, git checks (unchanged)
2. Agent ensure (unchanged)  
3. Terminal check + TTY requirement (unchanged)
4. NEW: If isTTY → progress TUI + goroutine flow
5. EXISTING: Non-TTY / quiet single-subject paths

The key is that the agent ensure step must happen BEFORE the TUI starts (since it's needed for step 1). Let me reconsider the flow.

Actually, wait — looking at the existing code:
- Line 72: agent.Ensure happens before TUI
- Line 98-106: server start
- Line 107-128: session create
- Line 169-187: TUI start

The user wants agent copy to be part of step 1's spinner. So agent.Ensure should happen inside the goroutine, not before the TUI.

But wait — current code has agent.Ensure BEFORE the TUI. If we move it into the goroutine, that changes the flow. Let me check: does agent.Ensure need to complete before the server starts?

Looking at `agent.Ensure`: it copies the agent prompt file to `~/.config/opencode/agents/`. The server doesn't depend on this — the server is the OpenCode process. The agent file needs to exist before session creation (step 2), not before server start (step 1).

But the user said step 1 includes "copying agent + starting opencode + healthcheck." So agent copy should be part of step 1.

The simplest approach: put agent.Ensure inside the goroutine as part of step 1.

```go
go func() {
    time.Sleep(50 * time.Millisecond)
    
    // Step 1: Starting OpenCode (agent copy + server start + healthcheck)
    p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepRunning})
    if err := agent.Ensure(cfg.Agent, cfg.InstallAgent); err != nil {
        p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepFailed, Detail: "Error: opencode server failed to start: " + err.Error()})
        return
    }
    // ... server start, healthcheck ...
    p.Send(tui.StepUpdateMsg{Index: 0, Status: tui.StepDone})
```

But the agent.Ensure returns an error if it fails. If the agent copy fails, we should show step 1 as failed. This makes sense.

Actually, looking at the existing code more carefully, let me keep the structure closer to what exists. The main change is:
1. TUI starts earlier
2. The server/session/generate/cleanup steps move into a goroutine
3. Each step sends a progress update

For the non-TTY paths, they should remain completely unchanged.

Let me reconsider the plan structure. The main.go changes are significant — I'll break that into a separate task focused just on the goroutine flow.

- [ ] **Step 4: Run tests, confirm everything builds and passes**

```
make all
```

- [ ] **Step 5: Commit**

```
git add cmd/gen-commit-msg/main.go cmd/gen-commit-msg/main_test.go internal/tui/tui.go && git commit -m "feat(main): integrate progress view into interactive flow"
```

---

### Task 4: Progress View - UX Task

**Surface:** Progress view
**Artifact reference:** `docs/leyline/design/2026-05-12-progress-view-ux.md#state-matrix`

- [ ] **Step 1:** Confirm artifact section is current (DRAW)
- [ ] **Step 2:** Implement the surface per the artifact (BUILD — covered by Tasks 1-3)
- [ ] **Step 3:** Trigger each state from the state matrix:
  - Loading: Start `gen-commit-msg` with staged files — observe all 5 steps shown, step 1 with spinner
  - Error: Break opencode PATH to force step 1 failure — observe ✗ on step 1, error detail below, dismiss with q/Esc
  - Success: Normal run with valid staged diff — all steps ✓, auto-transition to message selection
  - Empty: N/A — steps always present
  - Permission-denied: N/A — local CLI tool
  - Offline: N/A — local CLI tool
- [ ] **Step 4:** Accessibility verification:
  - Keyboard walk: verify Ctrl+C/Esc exits at any point, q/Esc/Enter dismisses errors
  - Screen reader: verify all step labels are plain ASCII text, status indicators are Unicode with ASCII fallback
  - Color independence: verify status is conveyed by ✓/✗/⚠ chars + SGR styles
  - Motion: verify only spinner character updates, no animations
- [ ] **Step 5:** Reconciliation against artifact — confirm no divergence
- [ ] **Step 6:** Commit any UX fixes

```
git commit -m "ux(tui): verify progress view against UX spec"
```

---

### Task 5: Message Selection View - UX Task

**Surface:** Message selection view
**Artifact reference:** `docs/leyline/design/2026-05-12-progress-view-ux.md#state-matrix`

- [ ] **Step 1:** Confirm artifact section is current (DRAW)
- [ ] **Step 2:** Verify implementation unchanged (BUILD)
- [ ] **Step 3:** Trigger each state:
  - Loading: N/A — preceded by progress view
  - Error: N/A — all errors handled inline on progress view
  - Success: After progress completes — list of messages with ↑↓ navigation, Enter selects
  - Empty: N/A — zero messages handled on progress view
  - Permission-denied: N/A
  - Offline: N/A
- [ ] **Step 4:** Accessibility verification — keyboard navigation, Enter selection
- [ ] **Step 5:** Reconciliation — confirm no divergence
- [ ] **Step 6:** Commit

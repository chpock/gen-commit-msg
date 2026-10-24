package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sdk "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"

	"github.com/chpock/gen-commit-msg/internal/agent"
	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/server"
)

type generatorServer struct {
	start func(context.Context) (string, error)
	stop  func() error
}

func (s *generatorServer) Start(ctx context.Context) (string, error) { return s.start(ctx) }
func (s *generatorServer) Stop() error                               { return s.stop() }

type generatorSession struct {
	*mockSessionClient
	newHook    func(context.Context)
	promptHook func(context.Context)
	deleteHook func(context.Context)
	calls      *[]string
}

func (s *generatorSession) New(ctx context.Context, p sdk.SessionNewParams) (*sdk.Session, error) {
	*s.calls = append(*s.calls, "create")
	if p.Directory.Value != "/repo" || p.Title.Value != "custom-agent" {
		return nil, errors.New("incorrect session routing")
	}
	if s.newHook != nil {
		s.newHook(ctx)
	}
	return s.mockSessionClient.New(ctx, p)
}

func (s *generatorSession) Prompt(ctx context.Context, id string, p sdk.SessionPromptParams, opts ...option.RequestOption) (*sdk.SessionPromptResponse, error) {
	*s.calls = append(*s.calls, "generate")
	if id != "mock_sess_001" {
		return nil, errors.New("incorrect generation session")
	}
	if s.promptHook != nil {
		s.promptHook(ctx)
	}
	return s.mockSessionClient.Prompt(ctx, id, p, opts...)
}

func (s *generatorSession) Delete(ctx context.Context, id string, p sdk.SessionDeleteParams, opts ...option.RequestOption) (*bool, error) {
	*s.calls = append(*s.calls, "delete")
	if id != "mock_sess_001" {
		return nil, errors.New("incorrect deletion session")
	}
	if s.deleteHook != nil {
		s.deleteHook(ctx)
	}
	return s.mockSessionClient.Delete(ctx, id, p, opts...)
}

type generatorFixture struct {
	generator *Generator
	session   *generatorSession
	server    *generatorServer
	calls     []string
	updates   []generation.Update
}

func newGeneratorFixture(t *testing.T, budget time.Duration) *generatorFixture {
	t.Helper()
	f := &generatorFixture{}
	f.generator = NewGenerator(GeneratorConfig{
		Executable: "/selected/opencode", RepoDir: "/repo", Agent: "custom-agent",
		InstallAgent: "if-not-exists", SessionDeleteTimeout: budget,
	})
	f.session = &generatorSession{mockSessionClient: newMockSession(), calls: &f.calls,
		deleteHook: func(ctx context.Context) {
			if ctx.Err() != nil {
				t.Errorf("cleanup inherited canceled generation: %v", ctx.Err())
			}
		},
	}
	f.server = &generatorServer{
		start: func(context.Context) (string, error) {
			f.calls = append(f.calls, "start")
			return "http://fixture", nil
		},
		stop: func() error { f.calls = append(f.calls, "stop"); return nil },
	}
	f.generator.ensureAgent = func(name, mode string) error {
		f.calls = append(f.calls, "ensure")
		if name != "custom-agent" || mode != "if-not-exists" {
			t.Fatalf("agent policy = %q, %q", name, mode)
		}
		return nil
	}
	f.generator.newServer = func(executable string) server.Server {
		f.calls = append(f.calls, "server")
		if executable != "/selected/opencode" {
			t.Fatalf("selected executable = %q", executable)
		}
		return f.server
	}
	f.generator.newClient = func(url, dir, name string) *Client {
		f.calls = append(f.calls, "client")
		if url != "http://fixture" || dir != "/repo" || name != "custom-agent" {
			t.Fatalf("client routing = %q, %q, %q", url, dir, name)
		}
		return newClientWithSession(f.session, dir, name)
	}
	return f
}

func (f *generatorFixture) generate(ctx context.Context) (generation.Result, error) {
	return f.generator.Generate(ctx, generation.Request{SubjectMin: 2, SubjectMax: 3, Body: false, ContextJSON: `{"snapshot":"original"}`}, func(u generation.Update) { f.updates = append(f.updates, u) })
}

func (f *generatorFixture) assertCalls(t *testing.T, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}

func (f *generatorFixture) assertFinalStatuses(t *testing.T, want ...generation.StepStatus) {
	t.Helper()
	got := make([]generation.StepStatus, len(want))
	for _, u := range f.updates {
		if u.Index < 0 || u.Index >= len(got) {
			t.Fatalf("invalid backend index: %+v", u)
		}
		got[u.Index] = u.Status
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statuses = %v, want %v; updates: %+v", got, want, f.updates)
	}
}

func TestGeneratorSuccess(t *testing.T) {
	f := newGeneratorFixture(t, 10*time.Second)
	var _ generation.Generator = f.generator
	wantSteps := []string{"Starting OpenCode...", "Creating session...", "Generating commit messages...", "Cleaning up OpenCode resources..."}
	if !reflect.DeepEqual(f.generator.Steps(), wantSteps) {
		t.Fatalf("steps = %v", f.generator.Steps())
	}
	result, err := f.generate(context.Background())
	if err != nil || len(result.Warnings) != 0 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	// Preserve OpenCode's acceptance even when a response does not match the requested count/body.
	if len(result.Messages) != 1 || result.Messages[0].Subject != "feat(config): add --subject-count flag" || result.Messages[0].Body == "" {
		t.Fatalf("messages = %+v", result.Messages)
	}
	f.assertCalls(t, "ensure", "server", "start", "client", "create", "generate", "delete", "stop")
	f.assertFinalStatuses(t, generation.StepDone, generation.StepDone, generation.StepDone, generation.StepDone)
	var transitions []string
	for _, u := range f.updates {
		transitions = append(transitions, string(u.Status))
	}
	if !reflect.DeepEqual(transitions, []string{"running", "done", "running", "done", "running", "done", "running", "done"}) {
		t.Fatalf("progress order = %v", f.updates)
	}
	p := f.session.LastPrompt()
	if p.Agent.Value != "custom-agent" || p.Directory.Value != "/repo" {
		t.Fatalf("prompt routing = %+v", p)
	}
	text := p.Parts.Value[0].(sdk.TextPartInputParam).Text.Value
	for _, part := range []string{"Minimum subjects: 2", "Maximum subjects: 3", "Include body: false", `{"snapshot":"original"}`} {
		if !strings.Contains(text, part) {
			t.Errorf("prompt missing %q: %s", part, text)
		}
	}
}

func TestGeneratorFailureSeams(t *testing.T) {
	primary := errors.New("primary failure")
	for _, seam := range []string{"ensure", "start", "create", "prompt", "decode"} {
		t.Run(seam, func(t *testing.T) {
			f := newGeneratorFixture(t, 10*time.Second)
			wantCalls := []string{"ensure"}
			wantStatuses := []generation.StepStatus{generation.StepFailed, generation.StepSkipped, generation.StepSkipped, generation.StepSkipped}
			switch seam {
			case "ensure":
				f.generator.ensureAgent = func(string, string) error { f.calls = append(f.calls, "ensure"); return primary }
			case "start":
				f.server.start = func(context.Context) (string, error) { f.calls = append(f.calls, "start"); return "", primary }
				wantCalls = []string{"ensure", "server", "start", "stop"}
				wantStatuses[3] = generation.StepDone
			case "create":
				f.session.newErr = primary
				wantCalls = []string{"ensure", "server", "start", "client", "create", "stop"}
				wantStatuses = []generation.StepStatus{generation.StepDone, generation.StepFailed, generation.StepSkipped, generation.StepDone}
			default:
				if seam == "prompt" {
					f.session.promptErr = primary
				} else {
					f.session.WithPromptFixture("prompt_error_no_structured.json")
				}
				wantCalls = []string{"ensure", "server", "start", "client", "create", "generate", "delete", "stop"}
				wantStatuses = []generation.StepStatus{generation.StepDone, generation.StepDone, generation.StepFailed, generation.StepDone}
			}
			result, err := f.generate(context.Background())
			if err == nil || len(result.Messages) != 0 || len(result.Warnings) != 0 {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			var appErr *AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("lost rich error: %T", err)
			}
			if seam == "decode" {
				if appErr.OC == nil || appErr.OC.Kind != OCErrNoStructuredOutput {
					t.Fatalf("lost decoder diagnostic: %+v", appErr)
				}
			} else if !errors.Is(err, primary) {
				t.Fatalf("lost primary error: %v", err)
			}
			f.assertCalls(t, wantCalls...)
			f.assertFinalStatuses(t, wantStatuses...)
			for _, u := range f.updates {
				if u.Status == generation.StepFailed && u.Err != err {
					t.Errorf("progress lost primary error: %+v", u)
				}
			}
		})
	}
}

func TestGeneratorCleanupWarnings(t *testing.T) {
	deleteErr, stopErr := errors.New("delete failed"), errors.New("stop failed")
	for _, primaryFailure := range []bool{false, true} {
		for _, seam := range []string{"delete", "stop", "both"} {
			t.Run(seam+map[bool]string{false: "/success", true: "/failure"}[primaryFailure], func(t *testing.T) {
				f := newGeneratorFixture(t, 5*time.Second)
				if primaryFailure {
					f.session.WithPromptError()
				}
				var want []error
				if seam != "stop" {
					f.session.deleteErr = deleteErr
					want = append(want, deleteErr)
				}
				if seam != "delete" {
					f.server.stop = func() error { f.calls = append(f.calls, "stop"); return stopErr }
					want = append(want, stopErr)
				}
				result, err := f.generate(context.Background())
				if primaryFailure {
					if !errors.Is(err, errMockPrompt) || len(result.Messages) != 0 {
						t.Fatalf("primary error/messages lost: %+v, %v", result, err)
					}
				} else if err != nil || len(result.Messages) != 1 {
					t.Fatalf("cleanup discarded success: %+v, %v", result, err)
				}
				if len(result.Warnings) != len(want) {
					t.Fatalf("warnings = %v, want %v", result.Warnings, want)
				}
				for i, warning := range want {
					if !errors.Is(result.Warnings[i], warning) {
						t.Errorf("warning %d = %v", i, result.Warnings[i])
					}
					found := false
					for _, u := range f.updates {
						if u.Index == 3 && u.Status == generation.StepWarning && errors.Is(u.Err, warning) {
							found = true
						}
					}
					if !found {
						t.Errorf("warning missing from progress: %v", warning)
					}
				}
				f.assertCalls(t, "ensure", "server", "start", "client", "create", "generate", "delete", "stop")
			})
		}
	}
}

func TestGeneratorCancellation(t *testing.T) {
	for _, phase := range []string{"before", "ensure", "start", "create", "prompt", "delete", "stop"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := newGeneratorFixture(t, 5*time.Second)
			want := []generation.StepStatus{generation.StepDone, generation.StepDone, generation.StepDone, generation.StepDone}
			switch phase {
			case "before":
				cancel()
				want = []generation.StepStatus{generation.StepInterrupted, generation.StepSkipped, generation.StepSkipped, generation.StepSkipped}
			case "ensure":
				f.generator.ensureAgent = func(string, string) error { cancel(); return nil }
				want = []generation.StepStatus{generation.StepInterrupted, generation.StepSkipped, generation.StepSkipped, generation.StepSkipped}
			case "start":
				f.server.start = func(context.Context) (string, error) { cancel(); return "http://fixture", nil }
				want = []generation.StepStatus{generation.StepInterrupted, generation.StepSkipped, generation.StepSkipped, generation.StepDone}
			case "create":
				f.session.newHook = func(context.Context) { cancel() }
				want = []generation.StepStatus{generation.StepDone, generation.StepInterrupted, generation.StepSkipped, generation.StepDone}
			case "prompt":
				f.session.promptHook = func(context.Context) { cancel() }
				want[2] = generation.StepInterrupted
			case "delete":
				f.session.deleteHook = func(cleanup context.Context) {
					cancel()
					if cleanup.Err() != nil {
						t.Error("deletion inherited cancellation")
					}
				}
			case "stop":
				f.server.stop = func() error { f.calls = append(f.calls, "stop"); cancel(); return nil }
			}
			result, err := f.generate(ctx)
			if !errors.Is(err, context.Canceled) || len(result.Messages) != 0 {
				t.Fatalf("canceled result = %+v, err = %v", result, err)
			}
			f.assertFinalStatuses(t, want...)
			if phase == "before" || phase == "ensure" {
				return
			}
			if f.calls[len(f.calls)-1] != "stop" {
				t.Fatalf("cleanup unfinished: %v", f.calls)
			}
			if phase != "start" && !f.session.DeleteCalled() {
				t.Fatal("acquired session was not deleted")
			}
		})
	}
}

func TestGeneratorDeleteBudgets(t *testing.T) {
	for _, budget := range []time.Duration{5 * time.Second, 10 * time.Second} {
		t.Run(budget.String(), func(t *testing.T) {
			f := newGeneratorFixture(t, budget)
			f.session.deleteHook = func(ctx context.Context) {
				deadline, ok := ctx.Deadline()
				remaining := time.Until(deadline)
				if !ok || remaining > budget || remaining < budget-time.Second {
					t.Errorf("delete deadline = %v, want independent %v", remaining, budget)
				}
			}
			if _, err := f.generate(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGeneratorFreshResourcesAndNilReporter(t *testing.T) {
	f := newGeneratorFixture(t, 5*time.Second)
	var servers []*generatorServer
	f.generator.newServer = func(string) server.Server {
		s := &generatorServer{start: f.server.start, stop: f.server.stop}
		servers = append(servers, s)
		return s
	}
	for range 2 {
		if _, err := f.generator.Generate(context.Background(), generation.Request{SubjectMin: 1, SubjectMax: 1}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(servers) != 2 || servers[0] == servers[1] {
		t.Fatal("server resources reused")
	}
	for _, call := range []string{"client", "create", "delete", "stop"} {
		count := 0
		for _, got := range f.calls {
			if got == call {
				count++
			}
		}
		if count != 2 {
			t.Errorf("%s calls = %d", call, count)
		}
	}
}

func TestGeneratorAgentInstallPolicy(t *testing.T) {
	for _, mode := range []string{"no", "always", "if-not-exists"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			path := filepath.Join(dir, "opencode", "agents", "custom-agent.md")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("custom instructions"), 0644); err != nil {
				t.Fatal(err)
			}
			f := newGeneratorFixture(t, 5*time.Second)
			f.generator.config.InstallAgent = mode
			f.generator.ensureAgent = agent.Ensure
			if _, err := f.generate(context.Background()); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "custom instructions"
			if mode == "always" {
				want = agent.DefaultPrompt
			}
			if string(got) != want {
				t.Fatalf("install policy %q changed agent content", mode)
			}
		})
	}
}

func TestGeneratorEarlyFailureCleanupWarning(t *testing.T) {
	primary, cleanupErr := errors.New("startup/session failure"), errors.New("cleanup failure")
	for _, phase := range []string{"start", "create"} {
		t.Run(phase, func(t *testing.T) {
			f := newGeneratorFixture(t, 5*time.Second)
			if phase == "start" {
				f.server.start = func(context.Context) (string, error) { return "", primary }
			} else {
				f.session.newErr = primary
			}
			f.server.stop = func() error { f.calls = append(f.calls, "stop"); return cleanupErr }
			result, err := f.generate(context.Background())
			if !errors.Is(err, primary) || len(result.Warnings) != 1 || !errors.Is(result.Warnings[0], cleanupErr) || len(result.Messages) != 0 {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			if f.session.DeleteCalled() {
				t.Fatal("deleted a session that was never acquired")
			}
			last := f.updates[len(f.updates)-1]
			if last.Status != generation.StepWarning || !errors.Is(last.Err, cleanupErr) {
				t.Fatalf("lost cleanup progress: %+v", last)
			}
		})
	}
}

func TestGeneratorCanceledCleanupRetainsWarningsAndPrimary(t *testing.T) {
	for _, primaryFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[primaryFailure], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := newGeneratorFixture(t, 5*time.Second)
			if primaryFailure {
				f.session.WithPromptError()
			}
			f.session.deleteHook = func(context.Context) { cancel() }
			f.session.WithDeleteError()
			result, err := f.generate(ctx)
			wantErr := error(context.Canceled)
			if primaryFailure {
				wantErr = errMockPrompt
			}
			if !errors.Is(err, wantErr) || len(result.Messages) != 0 || len(result.Warnings) != 1 || !errors.Is(result.Warnings[0], errMockDeleteSession) {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			f.assertCalls(t, "ensure", "server", "start", "client", "create", "generate", "delete", "stop")
		})
	}
}

func TestGeneratorDeleteDeadlineWarning(t *testing.T) {
	f := newGeneratorFixture(t, time.Millisecond)
	f.session.deleteHook = func(ctx context.Context) { <-ctx.Done(); f.session.deleteErr = ctx.Err() }
	result, err := f.generate(context.Background())
	if err != nil || len(result.Messages) != 1 || len(result.Warnings) != 1 || !errors.Is(result.Warnings[0], context.DeadlineExceeded) {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	f.assertCalls(t, "ensure", "server", "start", "client", "create", "generate", "delete", "stop")
}

func TestGeneratorDefaultDependenciesMissingExecutable(t *testing.T) {
	g := NewGenerator(GeneratorConfig{Executable: filepath.Join(t.TempDir(), "missing-opencode"), InstallAgent: "no", SessionDeleteTimeout: 5 * time.Second})
	result, err := g.Generate(context.Background(), generation.Request{}, nil)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Op != "server_start" || len(result.Messages) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestGeneratorSDKRequestCompatibility(t *testing.T) {
	response, err := os.ReadFile("../../testdata/fixtures/prompt_success_basic.json")
	if err != nil {
		t.Fatal(err)
	}
	calls := make(chan string, 3)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /session":
			calls <- "create"
			if r.URL.Query().Get("directory") != "/repo" {
				t.Error("session repository not forwarded")
			}
			_, _ = w.Write([]byte(`{"id":"session-generator"}`))
		case "POST /session/session-generator/message":
			calls <- "prompt"
			var got struct {
				Agent string `json:"agent"`
				Parts []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"parts"`
				Format struct {
					Type       string          `json:"type"`
					RetryCount int             `json:"retryCount"`
					Schema     json.RawMessage `json:"schema"`
				} `json:"format"`
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			if got.Agent != "custom-agent" || r.URL.Query().Get("directory") != "/repo" || got.Format.Type != "json_schema" || got.Format.RetryCount != 2 {
				t.Errorf("native request changed: %+v", got)
			}
			if len(got.Parts) != 1 || got.Parts[0].Type != "text" || !strings.Contains(got.Parts[0].Text, `{"snapshot":"original"}`) {
				t.Errorf("prompt changed: %+v", got.Parts)
			}
			var schema map[string]any
			if err := json.Unmarshal(got.Format.Schema, &schema); err != nil {
				t.Error(err)
			}
			wantSchema := generation.ResponseSchema(generation.Request{SubjectMin: 2, SubjectMax: 3})
			wantJSON, err := json.Marshal(wantSchema)
			if err != nil {
				t.Error(err)
			}
			var want map[string]any
			if err := json.Unmarshal(wantJSON, &want); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(schema, want) {
				t.Errorf("schema = %s, want %s", got.Format.Schema, wantJSON)
			}
			_, _ = w.Write(response)
		case "DELETE /session/session-generator":
			calls <- "delete"
			_, _ = w.Write([]byte("true"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	f := newGeneratorFixture(t, 5*time.Second)
	f.generator.newClient = NewClient
	f.server.start = func(context.Context) (string, error) { return api.URL, nil }
	result, err := f.generate(context.Background())
	if err != nil || len(result.Messages) != 1 || result.Messages[0].Body == "" {
		t.Fatalf("native acceptance changed: %+v, %v", result, err)
	}
	var gotCalls []string
	for range 3 {
		select {
		case call := <-calls:
			gotCalls = append(gotCalls, call)
		default:
			t.Fatal("missing SDK operation")
		}
	}
	if !reflect.DeepEqual(gotCalls, []string{"create", "prompt", "delete"}) {
		t.Fatalf("SDK operations = %v", gotCalls)
	}
}

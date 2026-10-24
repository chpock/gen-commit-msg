package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/chpock/gen-commit-msg/internal/generation"
	"github.com/chpock/gen-commit-msg/internal/git"
)

type fixtureGenerator struct {
	run func(context.Context, generation.Request, generation.Reporter) (generation.Result, error)
}

func (fixtureGenerator) Steps() []string {
	return []string{"Generating fixture...", "Cleaning fixture..."}
}

func (g fixtureGenerator) Generate(ctx context.Context, request generation.Request, report generation.Reporter) (generation.Result, error) {
	return g.run(ctx, request, report)
}

func clearGitEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSharedGenerationCollectsContextAndOffsetsProgress(t *testing.T) {
	clearGitEnvironment(t)
	t.Chdir(newCLIRepository(t, true))
	request := generation.Request{SubjectMin: 2, SubjectMax: 3, Body: true}
	messages := []generation.Message{{Subject: "fix: preserve the staged snapshot", Body: "Keep the supplied context."}}
	var updates []generation.Update
	calls := 0
	backend := fixtureGenerator{run: func(_ context.Context, got generation.Request, report generation.Reporter) (generation.Result, error) {
		calls++
		if got.SubjectMin != request.SubjectMin || got.SubjectMax != request.SubjectMax || got.Body != request.Body {
			t.Errorf("generation parameters changed: %+v", got)
		}
		var snapshot git.CommitMessageContext
		if err := json.Unmarshal([]byte(got.ContextJSON), &snapshot); err != nil {
			t.Fatalf("invalid Git context: %v", err)
		}
		found := false
		for _, output := range snapshot.StagedChanges.Outputs {
			if output.ID == "staged_diff" && strings.Contains(output.Output, "+staged change") {
				found = true
			}
		}
		if !found || snapshot.FormatVersion != "1.0" {
			t.Fatalf("missing actual staged snapshot: %s", got.ContextJSON)
		}
		for index := range 2 {
			report.Emit(generation.Update{Index: index, Status: generation.StepRunning})
			report.Emit(generation.Update{Index: index, Status: generation.StepDone})
		}
		return generation.Result{Messages: messages}, nil
	}}
	result, err := generateMessages(context.Background(), backend, request, func(update generation.Update) { updates = append(updates, update) })
	if err != nil || calls != 1 || len(result.Messages) != 1 || result.Messages[0] != messages[0] {
		t.Fatalf("result=%+v error=%v calls=%d", result, err, calls)
	}
	if len(updates) != 6 {
		t.Fatalf("updates=%+v", updates)
	}
	for index, update := range updates {
		wantStatus := generation.StepRunning
		if index%2 == 1 {
			wantStatus = generation.StepDone
		}
		if update.Index != index/2 || update.Status != wantStatus {
			t.Errorf("update %d=%+v, want step %d/%s", index, update, index/2, wantStatus)
		}
	}
}

func TestSharedGenerationSkipsBackendOnCollectionFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "Git failure", true: "cancellation"}[canceled], func(t *testing.T) {
			clearGitEnvironment(t)
			t.Chdir(t.TempDir())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			var updates []generation.Update
			backend := fixtureGenerator{run: func(context.Context, generation.Request, generation.Reporter) (generation.Result, error) {
				t.Fatal("backend ran without collected context")
				return generation.Result{}, nil
			}}
			result, err := generateMessages(ctx, backend, generation.Request{}, func(update generation.Update) { updates = append(updates, update) })
			if err == nil || len(result.Messages) != 0 || (canceled && !errors.Is(err, context.Canceled)) {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			statuses := make(map[int]generation.StepStatus)
			for _, update := range updates {
				statuses[update.Index] = update.Status
			}
			wantFirst := generation.StepFailed
			if canceled {
				wantFirst = generation.StepInterrupted
			}
			if statuses[0] != wantFirst || statuses[1] != generation.StepSkipped || statuses[2] != generation.StepSkipped {
				t.Fatalf("unexpected final statuses: %+v", statuses)
			}
		})
	}
}

func TestSharedGenerationPreservesResultAndCancellationPolicy(t *testing.T) {
	for _, scenario := range []string{"success", "primary failure", "cancellation", "primary failure and cancellation", "empty result"} {
		t.Run(scenario, func(t *testing.T) {
			clearGitEnvironment(t)
			t.Chdir(newCLIRepository(t, true))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			primary := errors.New("fixture generation failure")
			warning := errors.New("fixture cleanup warning")
			backend := fixtureGenerator{run: func(context.Context, generation.Request, generation.Reporter) (generation.Result, error) {
				result := generation.Result{Messages: []generation.Message{{Subject: "fix: retain valid candidates"}}, Warnings: []error{warning}}
				if strings.Contains(scenario, "cancellation") {
					cancel()
				}
				if strings.Contains(scenario, "primary failure") {
					return result, primary
				}
				if scenario == "empty result" {
					result.Messages = nil
				}
				return result, nil
			}}
			result, err := generateMessages(ctx, backend, generation.Request{SubjectMin: 1, SubjectMax: 1}, nil)
			if len(result.Warnings) != 1 || !errors.Is(result.Warnings[0], warning) {
				t.Fatalf("cleanup warning lost: %+v", result)
			}
			switch {
			case strings.Contains(scenario, "primary failure"):
				if !errors.Is(err, primary) {
					t.Errorf("primary error replaced: %v", err)
				}
			case scenario == "cancellation":
				if !errors.Is(err, context.Canceled) {
					t.Errorf("cancellation lost: %v", err)
				}
			case scenario == "empty result":
				if err == nil || !strings.Contains(err.Error(), "no commit messages") {
					t.Errorf("empty result accepted: %v", err)
				}
			default:
				if err != nil || len(result.Messages) != 1 {
					t.Fatalf("valid result lost: %+v, %v", result, err)
				}
			}
			if err != nil && len(result.Messages) != 0 {
				t.Errorf("failed/canceled operation leaked candidates: %+v", result)
			}
		})
	}
}

func TestWriteWarningsKeepsDiagnosticsSeparate(t *testing.T) {
	var destination bytes.Buffer
	writeWarnings(&destination, []error{errors.New("session deletion failed"), errors.New("server stop failed")})
	if got, want := destination.String(), "Warning: session deletion failed\nWarning: server stop failed\n"; got != want {
		t.Fatalf("warnings=%q, want %q", got, want)
	}
}

func TestSharedGenerationRecoversPanicAndRetainsCleanupWarning(t *testing.T) {
	clearGitEnvironment(t)
	t.Chdir(newCLIRepository(t, true))
	warning := errors.New("cleanup failed before panic return")
	backend := fixtureGenerator{run: func(_ context.Context, _ generation.Request, report generation.Reporter) (generation.Result, error) {
		report.Emit(generation.Update{Index: 1, Status: generation.StepWarning, Err: warning})
		panic("fixture backend panic")
	}}
	result, err := generateMessages(context.Background(), backend, generation.Request{SubjectMin: 1, SubjectMax: 1}, nil)
	if err == nil || !strings.Contains(err.Error(), "fixture backend panic") || len(result.Messages) != 0 {
		t.Fatalf("panic outcome=%+v, %v", result, err)
	}
	if len(result.Warnings) != 1 || !errors.Is(result.Warnings[0], warning) {
		t.Fatalf("cleanup diagnostic lost during panic: %+v", result)
	}
}

package git

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestBuildExecutionArgs(t *testing.T) {
	args, err := buildExecutionArgs([]string{"git", "diff", "--cached"})
	if err != nil {
		t.Fatalf("buildExecutionArgs returned error: %v", err)
	}
	expectedPrefix := []string{"--no-pager", "-c", "color.ui=false", "-c", "core.quotepath=false", "diff", "--cached"}
	if len(args) != len(expectedPrefix) {
		t.Fatalf("unexpected args len: got %d want %d", len(args), len(expectedPrefix))
	}
	for i, expected := range expectedPrefix {
		if args[i] != expected {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], expected)
		}
	}
}

func TestBuildExecutionArgsInvalid(t *testing.T) {
	if _, err := buildExecutionArgs(nil); err == nil {
		t.Fatal("expected error for empty command")
	}
	if _, err := buildExecutionArgs([]string{"sh", "-c", "echo"}); err == nil {
		t.Fatal("expected error for non-git command")
	}
}

func TestMaybeTruncate(t *testing.T) {
	input := strings.Repeat("x", 25)
	out, truncated, meta := maybeTruncate(input, 10)
	if !truncated {
		t.Fatal("expected output to be truncated")
	}
	if meta == nil {
		t.Fatal("expected truncation metadata")
	}
	if meta.MaxBytes != 10 {
		t.Fatalf("meta.MaxBytes = %d, want 10", meta.MaxBytes)
	}
	if meta.OriginalBytes != 25 {
		t.Fatalf("meta.OriginalBytes = %d, want 25", meta.OriginalBytes)
	}
	if meta.KeptBytes != 10 {
		t.Fatalf("meta.KeptBytes = %d, want 10", meta.KeptBytes)
	}
	if meta.Strategy != "head_tail" {
		t.Fatalf("meta.Strategy = %q, want %q", meta.Strategy, "head_tail")
	}
	if len(out) != 10 {
		t.Fatalf("truncated output len = %d, want 10", len(out))
	}
}

func TestCollectCommitMessageContext(t *testing.T) {
	ctx := context.Background()
	result, err := CollectCommitMessageContext(ctx)
	if err != nil {
		t.Fatalf("CollectCommitMessageContext returned error: %v", err)
	}
	if result.FormatVersion != "1.0" {
		t.Fatalf("FormatVersion = %q, want %q", result.FormatVersion, "1.0")
	}
	if len(result.StagedChanges.Outputs) != 6 {
		t.Fatalf("staged output count = %d, want 6", len(result.StagedChanges.Outputs))
	}
	if len(result.StyleContext.Outputs) != 2 {
		t.Fatalf("style output count = %d, want 2", len(result.StyleContext.Outputs))
	}
	jsonValue, err := result.Marshal()
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if !strings.Contains(jsonValue, "\"format_version\": \"1.0\"") {
		t.Fatalf("marshaled JSON missing format version: %s", jsonValue)
	}
}

func TestRecentCommitsCommandSpecUsesConfigurableCount(t *testing.T) {
	wantArg := fmt.Sprintf("-%d", RecentCommitMessageCount)
	wantDescription := fmt.Sprintf("Last %d commit messages. Use only as style examples when no explicit repository commit-message instructions are available.", RecentCommitMessageCount)

	for _, spec := range commitContextCommandSpecs {
		if spec.ID != "recent_commits" {
			continue
		}

		if len(spec.Command) < 3 {
			t.Fatalf("recent_commits command too short: %v", spec.Command)
		}
		if spec.Command[2] != wantArg {
			t.Fatalf("recent_commits command count arg = %q, want %q", spec.Command[2], wantArg)
		}
		if spec.Description != wantDescription {
			t.Fatalf("recent_commits description = %q, want %q", spec.Description, wantDescription)
		}
		return
	}

	t.Fatal("recent_commits command spec not found")
}

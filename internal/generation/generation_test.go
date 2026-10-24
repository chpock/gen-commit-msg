package generation

import (
	"strings"
	"testing"
)

func TestMessageFormat(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message Message
		want    string
	}{
		{"subject only", Message{Subject: " fix: retain routing "}, "fix: retain routing"},
		{"common body", Message{Subject: "feat: add feature", Body: " explanation\n "}, "feat: add feature\n\nexplanation"},
		{"existing whitespace-body behavior", Message{Subject: "fix: keep formatting", Body: " \n "}, "fix: keep formatting\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.message.Format(); got != tc.want {
				t.Fatalf("formatted message = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPromptPreservesGenerationParametersAndSnapshot(t *testing.T) {
	const snapshot = "{\"format_version\":\"1.0\",\"marker\":\"staged snapshot ✓\"}"
	request := Request{SubjectMin: 2, SubjectMax: 4, Body: false, ContextJSON: snapshot}
	prompt := BuildPrompt(request)
	for _, part := range []string{
		"- Minimum subjects: 2\n", "- Maximum subjects: 4\n", "- Include body: false\n",
		"<commit_message_context_json>\n" + snapshot + "\n</commit_message_context_json>",
	} {
		if strings.Count(prompt, part) != 1 {
			t.Errorf("prompt does not preserve %q exactly once: %s", part, prompt)
		}
	}
}

func TestReporterOptionalAndPreservesUpdate(t *testing.T) {
	update := Update{Index: 2, Status: StepWarning, Detail: "Cleanup issue: fixture"}
	Reporter(nil).Emit(update)
	var got []Update
	report := Reporter(func(update Update) { got = append(got, update) })
	report.Emit(update)
	if len(got) != 1 || got[0] != update {
		t.Fatalf("reported updates = %+v, want exactly %+v", got, update)
	}
}

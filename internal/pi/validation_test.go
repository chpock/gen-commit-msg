package pi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

func TestValidateResponseAcceptsContractValues(t *testing.T) {
	longSubject := strings.Repeat("An ordinary subject without a conventional prefix ", 5)
	for _, tc := range []struct {
		name    string
		text    string
		request generation.Request
		want    []generation.Message
	}{
		{
			name:    "inclusive minimum and empty enabled body",
			text:    `{"subjects":["first","second"],"body":""}`,
			request: generation.Request{SubjectMin: 2, SubjectMax: 3, Body: true},
			want:    []generation.Message{{Subject: "first"}, {Subject: "second"}},
		},
		{
			name:    "inclusive maximum preserves order content and common body",
			text:    `{"body":"  explanation\n\nmore detail\r\n ","subjects":[" third ","first\tchoice","second"]}`,
			request: generation.Request{SubjectMin: 2, SubjectMax: 3, Body: true},
			want: []generation.Message{
				{Subject: " third ", Body: "  explanation\n\nmore detail\r\n "},
				{Subject: "first\tchoice", Body: "  explanation\n\nmore detail\r\n "},
				{Subject: "second", Body: "  explanation\n\nmore detail\r\n "},
			},
		},
		{
			name:    "exact single count and disabled body",
			text:    `{"subjects":["one"],"body":""}`,
			request: generation.Request{SubjectMin: 1, SubjectMax: 1},
			want:    []generation.Message{{Subject: "one"}},
		},
		{
			name:    "surrounding JSON whitespace",
			text:    " \t\r\n {\"subjects\":[\"one\"],\"body\":\"\"} \n\r\t ",
			request: generation.Request{SubjectMin: 1, SubjectMax: 1, Body: true},
			want:    []generation.Message{{Subject: "one"}},
		},
		{
			name:    "long non conventional subject is not a hard constraint",
			text:    `{"subjects":["` + longSubject + `"],"body":""}`,
			request: generation.Request{SubjectMin: 1, SubjectMax: 1, Body: true},
			want:    []generation.Message{{Subject: longSubject}},
		},
		{
			name:    "short unicode and duplicate candidates are preserved",
			text:    `{"subjects":["✓","✓","普通の変更"],"body":" \t\n "}`,
			request: generation.Request{SubjectMin: 1, SubjectMax: 3, Body: true},
			want: []generation.Message{
				{Subject: "✓", Body: " \t\n "},
				{Subject: "✓", Body: " \t\n "},
				{Subject: "普通の変更", Body: " \t\n "},
			},
		},
		{
			name:    "unicode separators are not CR or LF",
			text:    `{"subjects":["one\u2028two\u2029three"],"body":""}`,
			request: generation.Request{SubjectMin: 1, SubjectMax: 1, Body: true},
			want:    []generation.Message{{Subject: "one\u2028two\u2029three"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateResponse(tc.text, tc.request)
			if err != nil {
				t.Fatalf("valid response rejected: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("messages = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestValidateResponseRejectsContractViolations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		text    string
		noBody  bool
		wantErr string
	}{
		{"empty response", "", false, "JSON"},
		{"whitespace response", " \n\t", false, "JSON"},
		{"malformed JSON", `{"subjects":["one","two"],"body":`, false, "JSON"},
		{"null root", `null`, false, "object"},
		{"array root", `[]`, false, "object"},
		{"string root", `"response"`, false, "object"},
		{"number root", `42`, false, "object"},
		{"boolean root", `true`, false, "object"},
		{"empty object", `{}`, false, "subjects"},
		{"missing subjects", `{"body":""}`, false, "subjects"},
		{"null subjects", `{"subjects":null,"body":""}`, false, "subjects"},
		{"string subjects", `{"subjects":"one","body":""}`, false, "subjects"},
		{"object subjects", `{"subjects":{},"body":""}`, false, "subjects"},
		{"number subjects", `{"subjects":2,"body":""}`, false, "subjects"},
		{"boolean subjects", `{"subjects":true,"body":""}`, false, "subjects"},
		{"missing body", `{"subjects":["one","two"]}`, false, "body"},
		{"null body", `{"subjects":["one","two"],"body":null}`, false, "body"},
		{"number body", `{"subjects":["one","two"],"body":2}`, false, "body"},
		{"boolean body", `{"subjects":["one","two"],"body":false}`, false, "body"},
		{"array body", `{"subjects":["one","two"],"body":[]}`, false, "body"},
		{"object body", `{"subjects":["one","two"],"body":{}}`, false, "body"},
		{"unknown key", `{"subjects":["one","two"],"body":"","extra":true}`, false, "extra"},
		{"case variant subjects", `{"Subjects":["one","two"],"body":""}`, false, "Subjects"},
		{"case variant body", `{"subjects":["one","two"],"Body":""}`, false, "Body"},
		{"case variant alongside valid key", `{"subjects":["one","two"],"body":"","BODY":""}`, false, "BODY"},
		{"empty array", `{"subjects":[],"body":""}`, false, "count"},
		{"below inclusive minimum", `{"subjects":["one"],"body":""}`, false, "count"},
		{"above inclusive maximum", `{"subjects":["one","two","three","four"],"body":""}`, false, "count"},
		{"null element", `{"subjects":["one",null],"body":""}`, false, "subjects[1]"},
		{"number element", `{"subjects":["one",2],"body":""}`, false, "subjects[1]"},
		{"boolean element", `{"subjects":["one",true],"body":""}`, false, "subjects[1]"},
		{"array element", `{"subjects":["one",[]],"body":""}`, false, "subjects[1]"},
		{"object element", `{"subjects":["one",{}],"body":""}`, false, "subjects[1]"},
		{"empty subject", `{"subjects":["one",""],"body":""}`, false, "subjects[1]"},
		{"spaces subject", `{"subjects":["one","   "],"body":""}`, false, "subjects[1]"},
		{"tabs subject", `{"subjects":["one","\t\t"],"body":""}`, false, "subjects[1]"},
		{"unicode whitespace subject", `{"subjects":["one","\u00a0\u2003"],"body":""}`, false, "subjects[1]"},
		{"embedded LF", `{"subjects":["one","two\nthree"],"body":""}`, false, "subjects[1]"},
		{"embedded CR", `{"subjects":["one","two\rthree"],"body":""}`, false, "subjects[1]"},
		{"embedded CRLF", `{"subjects":["one","two\r\nthree"],"body":""}`, false, "subjects[1]"},
		{"leading LF", `{"subjects":["\none","two"],"body":""}`, false, "subjects[0]"},
		{"trailing CR", `{"subjects":["one","two\r"],"body":""}`, false, "subjects[1]"},
		{"disabled nonempty body", `{"subjects":["one","two"],"body":"explanation"}`, true, "body"},
		{"disabled whitespace body", `{"subjects":["one","two"],"body":" \n "}`, true, "body"},
		{"disabled missing body", `{"subjects":["one","two"]}`, true, "body"},
		{"disabled null body", `{"subjects":["one","two"],"body":null}`, true, "body"},
		{"Markdown fence", "```json\n{\"subjects\":[\"one\",\"two\"],\"body\":\"\"}\n```", false, "JSON"},
		{"leading prose", `Here: {"subjects":["one","two"],"body":""}`, false, "JSON"},
		{"trailing JSON object", `{"subjects":["one","two"],"body":""} {}`, false, "JSON"},
		{"trailing JSON null", `{"subjects":["one","two"],"body":""} null`, false, "JSON"},
		{"trailing prose", `{"subjects":["one","two"],"body":""} done`, false, "JSON"},
		{"trailing fence", "{\"subjects\":[\"one\",\"two\"],\"body\":\"\"}\n```", false, "JSON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := generation.Request{SubjectMin: 2, SubjectMax: 3, Body: !tc.noBody}
			got, err := validateResponse(tc.text, request)
			if err == nil {
				t.Fatalf("invalid response accepted: %#v", got)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want diagnosis containing %q", err, tc.wantErr)
			}
			if len(got) != 0 {
				t.Errorf("invalid response returned partial candidates: %#v", got)
			}
		})
	}
}

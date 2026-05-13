package color

import (
	"strings"
	"testing"
)

func TestColorizeJSON_keys_and_values(t *testing.T) {
	input := `{"key": "value"}`
	out := ColorizeJSON(input)

	if !strings.Contains(out, "key") {
		t.Error("output should contain 'key'")
	}
	if !strings.Contains(out, "value") {
		t.Error("output should contain 'value'")
	}
	if !strings.Contains(out, green) {
		t.Error("output should contain green for string values")
	}
	if !strings.Contains(out, blue) {
		t.Error("output should contain blue for keys")
	}
	if !strings.Contains(out, gray) {
		t.Error("output should contain gray for structural chars")
	}
}

func TestColorizeJSON_numbers(t *testing.T) {
	input := `{"count": 42, "pi": 3.14}`
	out := ColorizeJSON(input)

	if !strings.Contains(out, yellow) {
		t.Error("output should contain yellow for numbers")
	}
}

func TestColorizeJSON_literals(t *testing.T) {
	input := `{"ok": true, "nil": null, "no": false}`
	out := ColorizeJSON(input)

	if !strings.Contains(out, magenta) {
		t.Error("output should contain magenta for true/false/null")
	}
}

func TestColorizeJSON_empty(t *testing.T) {
	if out := ColorizeJSON(""); out != "" {
		t.Errorf("expected empty string, got %q", out)
	}
}

func TestColorizeJSON_plain(t *testing.T) {
	input := `just text`
	out := ColorizeJSON(input)
	if out != input {
		t.Errorf("expected unchanged plain text, got %q", out)
	}
}

func TestColorizeJSON_reset(t *testing.T) {
	input := `{"a": 1}`
	out := ColorizeJSON(input)

	opens := strings.Count(out, reset)
	if opens < 3 {
		t.Errorf("expected at least 3 reset sequences, got %d in: %q", opens, out)
	}
}

func TestRedText(t *testing.T) {
	out := RedText("error")
	if !strings.HasPrefix(out, red) {
		t.Error("RedText should start with red ANSI code")
	}
	if !strings.HasSuffix(out, reset) {
		t.Error("RedText should end with reset ANSI code")
	}
	if !strings.Contains(out, "error") {
		t.Error("RedText should contain the message")
	}
}

func TestIndent(t *testing.T) {
	out := Indent("line1\nline2", 2)
	if out != "  line1\n  line2" {
		t.Errorf("expected indented lines, got %q", out)
	}
}

func TestIndent_single(t *testing.T) {
	out := Indent("line", 4)
	if out != "    line" {
		t.Errorf("expected 4-space indented line, got %q", out)
	}
}

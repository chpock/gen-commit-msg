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

func TestColorizeKeyValueBlock_key_value(t *testing.T) {
	input := "  Session:    ses_abc\n  Agent:      my-agent\n"
	out := ColorizeKeyValueBlock(input)
	if !strings.Contains(out, cyan) {
		t.Error("output should contain cyan for keys")
	}
	if !strings.Contains(out, green) {
		t.Error("output should contain green for values")
	}
	if !strings.Contains(out, gray) {
		t.Error("output should contain gray for separator")
	}
	if !strings.Contains(out, "Session") {
		t.Error("output should contain key text")
	}
	if !strings.Contains(out, "ses_abc") {
		t.Error("output should contain value text")
	}
	if !strings.Contains(out, "my-agent") {
		t.Error("output should contain second value text")
	}
}

func TestColorizeKeyValueBlock_header_line(t *testing.T) {
	input := "  Response:\n"
	out := ColorizeKeyValueBlock(input)
	if !strings.Contains(out, cyan) {
		t.Error("header line key should be cyan")
	}
	if !strings.Contains(out, gray) {
		t.Error("header line colon should be gray")
	}
	if !strings.Contains(out, "Response") {
		t.Error("output should contain header line text")
	}
}

func TestColorizeKeyValueBlock_mixed(t *testing.T) {
	input := "  Error:      APIError\n  Message:    something\n  Response:\n"
	out := ColorizeKeyValueBlock(input)
	c1 := strings.Count(out, cyan)
	if c1 < 3 {
		t.Errorf("expected at least 3 cyan sections, got %d in: %q", c1, out)
	}
	c2 := strings.Count(out, green)
	if c2 < 2 {
		t.Errorf("expected at least 2 green sections, got %d in: %q", c2, out)
	}
}

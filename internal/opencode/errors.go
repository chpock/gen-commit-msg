package opencode

import (
	"strconv"
	"strings"

	col "github.com/chpock/gen-commit-msg/internal/color"
)

type AppError struct {
	Op      string
	Message string
	OC      *OCError
	Err     error
}

func (e *AppError) Error() string {
	if e.OC != nil {
		return e.Op + ": " + e.OC.ErrorText()
	}
	if e.Err != nil {
		return e.Op + ": " + e.Err.Error()
	}
	if e.Message != "" {
		return e.Op + ": " + e.Message
	}
	return e.Op
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) Render() string {
	var b strings.Builder
	b.WriteString(col.Red)
	b.WriteString("Error: ")
	b.WriteString(col.Reset)
	b.WriteString(e.Op)
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}

	if e.OC != nil {
		b.WriteString("\n")
		b.WriteString(e.OC.RenderDetails())
	} else if e.Err != nil {
		b.WriteString("\n")
		b.WriteString(e.Err.Error())
	}

	return b.String()
}

type OCErrorKind string

const (
	OCErrAPI                OCErrorKind = "api_error"
	OCErrHTTP               OCErrorKind = "http_error"
	OCErrNoStructuredOutput OCErrorKind = "no_structured_output"
)

type OCError struct {
	Kind        OCErrorKind
	RequestType string
	SessionID   string
	Agent       string
	Code        string
	Message     string
	Status      int
	RawJSON     string
}

func (e *OCError) ErrorText() string {
	var b strings.Builder
	b.WriteString(string(e.Kind))
	if e.Code != "" {
		b.WriteString(": ")
		b.WriteString(e.Code)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	return b.String()
}

type detailField struct {
	key        string
	value      string
	valueColor string
}

func (e *OCError) RenderDetails() string {
	var fields []detailField

	add := func(key, value, color string) {
		fields = append(fields, detailField{key: key, value: value, valueColor: color})
	}

	add("Request", e.RequestType, col.Green)
	if e.SessionID != "" {
		add("Session", e.SessionID, col.Green)
	}
	if e.Agent != "" {
		add("Agent", e.Agent, col.Green)
	}
	add("Error", e.Code, col.Green)
	if e.Message != "" {
		add("Message", e.Message, col.Red)
	}
	if e.Status != 0 {
		add("StatusCode", strconv.Itoa(e.Status), col.Yellow)
	}
	if e.RawJSON != "" {
		add("Response", col.ColorizeJSON(e.RawJSON), "")
	}

	return renderFields(fields)
}

func renderFields(fields []detailField) string {
	maxKey := 0
	for _, f := range fields {
		if len(f.key) > maxKey {
			maxKey = len(f.key)
		}
	}
	// Total header width: indent(2) + key + colon(1) + padding.
	headerWidth := 2 + maxKey + 1 + 1

	var b strings.Builder
	for _, f := range fields {
		b.WriteString(col.Cyan)
		b.WriteString("  ")
		b.WriteString(f.key)
		b.WriteString(col.Reset)
		b.WriteString(col.Gray)
		b.WriteString(":")
		b.WriteString(col.Reset)
		pad := headerWidth - (2 + len(f.key) + 1)
		for pad > 0 {
			b.WriteByte(' ')
			pad--
		}
		if f.valueColor != "" {
			b.WriteString(f.valueColor)
		}
		b.WriteString(f.value)
		if f.valueColor != "" {
			b.WriteString(col.Reset)
		}
		b.WriteString("\n")
	}
	return b.String()
}

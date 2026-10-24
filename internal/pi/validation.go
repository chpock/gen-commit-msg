package pi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

func validateResponse(text string, request generation.Request) ([]generation.Message, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &fields); err != nil {
		return nil, fmt.Errorf("response must be exactly one JSON object: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("response must be a JSON object, not null")
	}
	for key := range fields {
		if key != "subjects" && key != "body" {
			return nil, fmt.Errorf("unexpected field %q; only subjects and body are allowed", key)
		}
	}

	rawSubjects, ok := fields["subjects"]
	if !ok {
		return nil, fmt.Errorf("subjects is required")
	}
	var subjects []json.RawMessage
	if err := json.Unmarshal(rawSubjects, &subjects); err != nil {
		return nil, fmt.Errorf("subjects must be an array of strings: %w", err)
	}
	if subjects == nil {
		return nil, fmt.Errorf("subjects must be an array of strings, not null")
	}

	rawBody, ok := fields["body"]
	if !ok {
		return nil, fmt.Errorf("body is required")
	}
	var body *string
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, fmt.Errorf("body must be a string: %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("body must be a string, not null")
	}
	if !request.Body && *body != "" {
		return nil, fmt.Errorf("body must be empty when body generation is disabled")
	}
	if len(subjects) < request.SubjectMin || len(subjects) > request.SubjectMax {
		return nil, fmt.Errorf("subjects count must be between %d and %d inclusive, got %d",
			request.SubjectMin, request.SubjectMax, len(subjects))
	}

	messages := make([]generation.Message, len(subjects))
	for i, rawSubject := range subjects {
		var subject *string
		if err := json.Unmarshal(rawSubject, &subject); err != nil {
			return nil, fmt.Errorf("subjects[%d] must be a string: %w", i, err)
		}
		if subject == nil {
			return nil, fmt.Errorf("subjects[%d] must be a string, not null", i)
		}
		if strings.ContainsAny(*subject, "\r\n") {
			return nil, fmt.Errorf("subjects[%d] must be single-line without CR or LF", i)
		}
		if strings.TrimSpace(*subject) == "" {
			return nil, fmt.Errorf("subjects[%d] must not be blank", i)
		}
		messages[i] = generation.Message{Subject: *subject, Body: *body}
	}
	return messages, nil
}

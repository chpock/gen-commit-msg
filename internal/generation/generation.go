package generation

import (
	"context"
	_ "embed"
	"strings"
)

//go:embed instructions.md
var Instructions string

const CollectContextStep = "Collecting information about current changes..."

type Request struct {
	SubjectMin  int
	SubjectMax  int
	Body        bool
	ContextJSON string
}

type Message struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (m Message) Format() string {
	if m.Body == "" {
		return strings.TrimSpace(m.Subject)
	}
	return strings.TrimSpace(m.Subject) + "\n\n" + strings.TrimSpace(m.Body)
}

type Result struct {
	Messages []Message
	Warnings []error
}

type StepStatus string

const (
	StepPending     StepStatus = "pending"
	StepRunning     StepStatus = "running"
	StepDone        StepStatus = "done"
	StepFailed      StepStatus = "failed"
	StepWarning     StepStatus = "warning"
	StepSkipped     StepStatus = "skipped"
	StepInterrupted StepStatus = "interrupted"
)

type Update struct {
	Index  int
	Status StepStatus
	Detail string
	Err    error
}

type Reporter func(Update)

func (r Reporter) Emit(update Update) {
	if r != nil {
		r(update)
	}
}

type Generator interface {
	Steps() []string
	Generate(context.Context, Request, Reporter) (Result, error)
}

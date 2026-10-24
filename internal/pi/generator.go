package pi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chpock/gen-commit-msg/internal/generation"
)

type GeneratorConfig struct {
	Executable string
	RepoDir    string
}

type Generator struct {
	config          GeneratorConfig
	startupTimeout  time.Duration
	promptTimeout   time.Duration
	shutdownTimeout time.Duration
}

func NewGenerator(config GeneratorConfig) *Generator {
	return &Generator{config: config, startupTimeout: 30 * time.Second, promptTimeout: 120 * time.Second, shutdownTimeout: 5 * time.Second}
}

func (g *Generator) Steps() []string {
	return []string{"Starting Pi...", "Generating commit messages...", "Stopping Pi..."}
}

func (g *Generator) Generate(ctx context.Context, request generation.Request, report generation.Reporter) (result generation.Result, err error) {
	slog.Info("Pi generation starting", "executable", g.config.Executable, "repo", g.config.RepoDir)
	var rpc *process
	step := 0
	defer func() {
		if step == 0 {
			report.Emit(generation.Update{Index: 1, Status: generation.StepSkipped})
		}
		if rpc != nil {
			report.Emit(generation.Update{Index: 2, Status: generation.StepRunning})
			if cleanupErr := rpc.stop(g.shutdownTimeout); cleanupErr != nil {
				result.Warnings = append(result.Warnings, cleanupErr)
			}
		}
		if len(result.Warnings) > 0 {
			warning := errors.Join(result.Warnings...)
			slog.Warn("Pi cleanup produced warnings", "error", warning)
			report.Emit(generation.Update{Index: 2, Status: generation.StepWarning, Err: warning})
		} else if rpc != nil {
			report.Emit(generation.Update{Index: 2, Status: generation.StepDone})
		} else {
			report.Emit(generation.Update{Index: 2, Status: generation.StepSkipped})
		}
		if ctx.Err() != nil && err == nil {
			err = ctx.Err()
		}
		if err != nil {
			result.Messages = nil
			slog.Error("Pi generation failed", "error", err)
		} else {
			slog.Info("Pi generation completed", "message_count", len(result.Messages), "warning_count", len(result.Warnings))
		}
	}()
	report.Emit(generation.Update{Index: 0, Status: generation.StepRunning})
	startup, cancel := context.WithTimeout(ctx, g.startupTimeout)
	var warnings []error
	warnings, err = g.checkVersion(startup)
	result.Warnings = append(result.Warnings, warnings...)
	if err == nil {
		err = startup.Err()
	}
	if err == nil {
		rpc, err = startProcess(g.config.Executable, g.config.RepoDir,
			"--mode", "rpc", "--no-session", "--no-extensions", "--no-mcp", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--offline", "--tools", "read,grep,find,ls", "--system-prompt", generation.Instructions, "--append-system-prompt", "")
	}
	if err == nil {
		err = rpc.ready(startup)
	}
	if err == nil {
		err = startup.Err()
	}
	cancel()
	if err = finishPiStep(report, 0, err); err != nil {
		return result, fmt.Errorf("start Pi: %w", err)
	}
	step = 1
	report.Emit(generation.Update{Index: 1, Status: generation.StepRunning})
	schemaBytes, _ := json.Marshal(generation.ResponseSchema(request))
	schema := string(schemaBytes)
	prompt := generation.BuildPrompt(request) + "\n\nRequired response schema (return only the JSON object):\n" + schema
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			report.Emit(generation.Update{Index: 1, Status: generation.StepRunning, Detail: fmt.Sprintf("Correcting response format (attempt %d of 3)", attempt)})
		}
		slog.Info("Pi application prompt starting", "attempt", attempt, "correction", attempt > 1)
		operation, done := context.WithTimeout(ctx, g.promptTimeout)
		text, promptErr := rpc.prompt(operation, fmt.Sprintf("prompt-%d", attempt), prompt)
		if promptErr == nil {
			promptErr = operation.Err()
		}
		done()
		if promptErr != nil {
			err = fmt.Errorf("pi prompt %d: %w", attempt, promptErr)
			break
		}
		result.Messages, err = validateResponse(text, request)
		if err == nil {
			slog.Info("Pi response validated", "attempt", attempt)
			break
		}
		slog.Warn("Pi response validation failed", "attempt", attempt, "error", err)
		if attempt == 3 {
			err = fmt.Errorf("pi response invalid after 3 attempts: %w", err)
			break
		}
		prompt = "The previous response failed validation: " + err.Error() + ".\nReturn a corrected response using the same original staged-change snapshot and generation parameters. Do not collect a new snapshot. Return only JSON matching the unchanged required response schema:\n" + schema
	}
	err = finishPiStep(report, 1, err)
	return result, err
}

func finishPiStep(report generation.Reporter, index int, err error) error {
	status := generation.StepDone
	if err != nil {
		status = generation.StepFailed
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = generation.StepInterrupted
		}
		slog.Error("Pi step failed", "step", index, "error", err)
	}
	report.Emit(generation.Update{Index: index, Status: status, Err: err})
	return err
}

var piVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\+[0-9A-Za-z.-]+)?$`)

func supportedVersion(text string) error {
	parts := piVersionPattern.FindStringSubmatch(strings.TrimSpace(text))
	if parts == nil {
		return fmt.Errorf("unrecognizable Pi version; require Pi 1.4.0 or newer")
	}
	major, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("unrecognizable Pi version")
	}
	minor, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return fmt.Errorf("unrecognizable Pi version")
	}
	if _, err := strconv.ParseUint(parts[3], 10, 64); err != nil {
		return fmt.Errorf("unrecognizable Pi version")
	}
	if major < 1 || major == 1 && minor < 4 {
		return fmt.Errorf("unsupported Pi version %s; require Pi 1.4.0 or newer", strings.TrimSpace(text))
	}
	return nil
}

func (g *Generator) checkVersion(ctx context.Context) (warnings []error, err error) {
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	p, err := startProcess(g.config.Executable, g.config.RepoDir, "--version")
	if err != nil {
		return nil, fmt.Errorf("pi version preflight: %w", err)
	}
	defer func() {
		if cleanupErr := p.stop(g.shutdownTimeout); cleanupErr != nil {
			warnings = append(warnings, cleanupErr)
		}
	}()
	var output strings.Builder
	for {
		select {
		case line, ok := <-p.lines:
			if !ok {
				select {
				case <-p.done:
					if p.waitErr != nil {
						return nil, fmt.Errorf("pi version preflight: %w", p.waitErr)
					}
					err = supportedVersion(output.String())
					if err == nil {
						slog.Info("Pi version verified", "version", strings.TrimSpace(output.String()))
					}
					return nil, err
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			if line.err != nil {
				return nil, line.err
			}
			if output.Len()+len(line.data) > 4096 {
				return nil, fmt.Errorf("unrecognizable Pi version: output too long")
			}
			output.Write(line.data)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

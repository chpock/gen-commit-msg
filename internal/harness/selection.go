package harness

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
)

type Kind string

const (
	Pi       Kind = "pi"
	OpenCode Kind = "opencode"
)

type Selection struct {
	Kind       Kind
	Executable string
}

// Select discovers executables without invoking them. Callers must execute the
// retained path and must not fall back after a selected runtime fails.
func Select(requested string) (Selection, error) {
	slog.Debug("resolving harness", "requested", requested)
	var candidates []Kind
	switch requested {
	case "auto":
		candidates = []Kind{Pi, OpenCode}
	case string(Pi), string(OpenCode):
		candidates = []Kind{Kind(requested)}
	default:
		err := fmt.Errorf("harness must be one of auto, pi, opencode, got %q", requested)
		slog.Error("invalid harness selection", "requested", requested, "error", err)
		return Selection{}, err
	}

	var missing []error
	for _, kind := range candidates {
		path, err := exec.LookPath(string(kind))
		if err == nil {
			// A relative PATH entry can yield a bare name when execerrdot is
			// disabled. Retain an absolute path to avoid a second PATH lookup.
			path, err = filepath.Abs(path)
			if err != nil {
				err = fmt.Errorf("retain %s executable path: %w", kind, err)
				slog.Error("harness executable resolution failed", "requested", requested, "harness", kind, "error", err)
				return Selection{}, err
			}
			slog.Info("harness selected", "requested", requested, "harness", kind, "executable", path)
			return Selection{Kind: kind, Executable: path}, nil
		}
		if !errors.Is(err, exec.ErrNotFound) {
			err = fmt.Errorf("look up %s executable in PATH: %w", kind, err)
			slog.Error("harness executable lookup failed", "requested", requested, "harness", kind, "error", err)
			return Selection{}, err
		}
		slog.Debug("harness executable not found", "harness", kind)
		missing = append(missing, err)
	}
	err := fmt.Errorf("no harness executable found in PATH: %w", errors.Join(missing...))
	slog.Error("harness selection failed", "requested", requested, "error", err)
	return Selection{}, err
}

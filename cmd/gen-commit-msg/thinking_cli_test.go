package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInvalidThinkingLevelPreservesOutput(t *testing.T) {
	for _, harness := range []string{"pi", "opencode"} {
		for _, level := range []string{"", "none", "LOW", "low ", "ultra"} {
			t.Run(harness+"/"+level, func(t *testing.T) {
				fixture := newPiCLIFixture(t, true)
				output := filepath.Join(fixture.root, "message")
				if err := os.WriteFile(output, []byte("previous message\n"), 0600); err != nil {
					t.Fatal(err)
				}
				cmd := fixture.command(t, "valid", "--harness", harness, "--thinking-level="+level, "--output", output)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "thinking-level") {
					t.Fatalf("error=%v stdout=%q stderr=%q; want configuration exit 2", err, &stdout, &stderr)
				}
				if data, err := os.ReadFile(output); err != nil || string(data) != "previous message\n" {
					t.Fatalf("invalid level changed output: %q %v", data, err)
				}
				if events := fixture.events(t); len(events) != 0 {
					t.Fatalf("invalid level started Pi: %+v", events)
				}
				if _, err := os.Stat(filepath.Join(fixture.root, "opencode-invoked")); !os.IsNotExist(err) {
					t.Fatalf("invalid level started OpenCode: %v", err)
				}
			})
		}
	}
}

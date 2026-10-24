package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestWriteSelectedMessageWritesMessageAndNewline(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "message.txt")
			if existing {
				if err := os.WriteFile(path, []byte("old message that must be completely overwritten\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			wrote, err := writeSelectedMessage(path, "feat: add tests")
			if err != nil || !wrote {
				t.Fatalf("writeSelectedMessage = %t, %v; want true, nil", wrote, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "feat: add tests\n" {
				t.Fatalf("output = %q, error = %v", data, err)
			}
		})
	}
}

func TestWriteSelectedMessageSkipsEmptyOutput(t *testing.T) {
	for _, selected := range []string{"", " \n\t"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("selected=%q/existing=%t", selected, existing), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "message.txt")
				const original = "keep this message\n"
				if existing {
					if err := os.WriteFile(path, []byte(original), 0644); err != nil {
						t.Fatal(err)
					}
				}
				wrote, err := writeSelectedMessage(path, selected)
				if err != nil || wrote {
					t.Fatalf("empty selection = %t, %v; want false, nil", wrote, err)
				}
				data, err := os.ReadFile(path)
				if existing {
					if err != nil || string(data) != original {
						t.Fatalf("existing output = %q, error = %v", data, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("empty selection created output: %q, error = %v", data, err)
				}
			})
		}
	}
}

func TestResolveOutputWriterStdout(t *testing.T) {
	w, closer := resolveOutputWriter("")
	if w != os.Stdout {
		t.Error("expected os.Stdout when output path is empty")
	}
	if err := closer(); err != nil {
		t.Errorf("closer returned unexpected error: %v", err)
	}
}

func TestResolveOutputWriterFile(t *testing.T) {
	path := t.TempDir() + "/out.txt"
	w, closer := resolveOutputWriter(path)
	if w == os.Stdout {
		t.Error("expected file writer, got os.Stdout")
	}
	if w == nil {
		t.Fatal("writer is nil — resolveOutputWriter failed")
	}
	_, err := fmt.Fprintln(w, "hello")
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := closer(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("file content = %q, want %q", string(data), "hello\n")
	}
}

func TestWriteSelectedMessageReportsOpenError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "message.txt")
	wrote, err := writeSelectedMessage(path, "feat: add tests")
	if wrote || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "failed to open output file") {
		t.Fatalf("writeSelectedMessage = %t, %v; want an output-open error", wrote, err)
	}
}

func TestWriteSelectedMessageReportsWriteError(t *testing.T) {
	if _, err := os.Stat("/dev/full"); os.IsNotExist(err) {
		t.Skip("/dev/full is unavailable")
	}
	wrote, err := writeSelectedMessage("/dev/full", "feat: add tests")
	if wrote || !errors.Is(err, syscall.ENOSPC) || !strings.Contains(err.Error(), "failed to write output file") {
		t.Fatalf("writeSelectedMessage = %t, %v; want an output-write error", wrote, err)
	}
}

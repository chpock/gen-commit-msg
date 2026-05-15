package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestWriteSelectedMessageWritesMessageAndNewline(t *testing.T) {
	var buf bytes.Buffer
	wrote, err := writeSelectedMessage(&buf, "feat: add tests")
	if err != nil {
		t.Fatalf("writeSelectedMessage returned error: %v", err)
	}
	if !wrote {
		t.Fatal("writeSelectedMessage should report wrote=true")
	}
	if got, want := buf.String(), "feat: add tests\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestWriteSelectedMessageSkipsEmptyOutput(t *testing.T) {
	var buf bytes.Buffer
	wrote, err := writeSelectedMessage(&buf, "")
	if err != nil {
		t.Fatalf("writeSelectedMessage returned error: %v", err)
	}
	if wrote {
		t.Fatal("writeSelectedMessage should report wrote=false for empty selection")
	}
	if got := buf.Len(); got != 0 {
		t.Fatalf("stdout bytes = %d, want 0", got)
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

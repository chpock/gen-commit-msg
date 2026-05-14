package main

import (
	"bytes"
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

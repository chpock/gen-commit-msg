package harness

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, dir, name, script string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSelectPATHMatrix(t *testing.T) {
	for _, available := range []struct {
		name string
		pi   bool
		oc   bool
	}{
		{name: "both", pi: true, oc: true},
		{name: "pi only", pi: true},
		{name: "opencode only", oc: true},
		{name: "neither"},
	} {
		for _, mode := range []string{"auto", "pi", "opencode"} {
			t.Run(available.name+"/"+mode, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("PATH", dir)
				marker := filepath.Join(t.TempDir(), "invoked")
				t.Setenv("GCM_TEST_SELECTION_MARKER", marker)
				const script = "printf invoked >> \"$GCM_TEST_SELECTION_MARKER\"\nexit 23\n"
				if available.pi {
					writeExecutable(t, dir, "pi", script)
				}
				if available.oc {
					writeExecutable(t, dir, "opencode", script)
				}
				want := mode
				if mode == "auto" {
					want = "opencode"
					if available.pi {
						want = "pi"
					}
				}
				found := (want == "pi" && available.pi) || (want == "opencode" && available.oc)
				got, err := Select(mode)
				if found {
					if err != nil || string(got.Kind) != want || got.Executable != filepath.Join(dir, want) {
						t.Fatalf("Select(%q)=%+v, %v; want %s at fixture path", mode, got, err, want)
					}
				} else {
					if err == nil || got != (Selection{}) || !errors.Is(err, exec.ErrNotFound) {
						t.Fatalf("Select(%q)=%+v, %v; want missing-executable error", mode, got, err)
					}
					if !strings.Contains(err.Error(), want) || (mode == "auto" && !strings.Contains(err.Error(), "pi")) {
						t.Errorf("missing alternatives not identified: %v", err)
					}
				}
				if data, err := os.ReadFile(marker); !os.IsNotExist(err) {
					t.Fatalf("selection invoked a runtime: marker=%q, error=%v", data, err)
				}
			})
		}
	}
}

func TestSelectRetainsExecutable(t *testing.T) {
	for _, relative := range []bool{false, true} {
		name := "absolute PATH"
		if relative {
			name = "explicitly permitted current directory"
		}
		t.Run(name, func(t *testing.T) {
			first, second := t.TempDir(), t.TempDir()
			firstPath := writeExecutable(t, first, "pi", "printf first\n")
			writeExecutable(t, second, "pi", "printf second\n")
			t.Setenv("PATH", first)
			if relative {
				t.Chdir(first)
				t.Setenv("PATH", ".")
				t.Setenv("GODEBUG", "execerrdot=0")
			}
			selected, err := Select("auto")
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", second)
			output, err := exec.Command(selected.Executable).Output()
			if err != nil || string(output) != "first" || selected.Executable != firstPath {
				t.Fatalf("retained executable=%q, output=%q, error=%v", selected.Executable, output, err)
			}
		})
	}
}

func TestSelectLookupFailureIsNotAbsence(t *testing.T) {
	for _, mode := range []string{"auto", "pi", "opencode"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("GODEBUG", "execerrdot=1")
			t.Setenv("PATH", ".")
			writeExecutable(t, dir, "pi", "exit 0\n")
			writeExecutable(t, dir, "opencode", "exit 0\n")
			got, err := Select(mode)
			if !errors.Is(err, exec.ErrDot) || errors.Is(err, exec.ErrNotFound) || got != (Selection{}) {
				t.Fatalf("Select(%q)=%+v, %v; want retained lookup failure, not fallback/absence", mode, got, err)
			}
		})
	}
}

func TestSelectExplicitDoesNotInspectOtherExecutable(t *testing.T) {
	for _, mode := range []string{"pi", "opencode"} {
		t.Run(mode, func(t *testing.T) {
			cwd, bin := t.TempDir(), t.TempDir()
			t.Chdir(cwd)
			t.Setenv("GODEBUG", "execerrdot=1")
			t.Setenv("PATH", "."+string(os.PathListSeparator)+bin)
			other := "pi"
			if mode == "pi" {
				other = "opencode"
			}
			writeExecutable(t, cwd, other, "exit 0\n")
			want := writeExecutable(t, bin, mode, "exit 0\n")
			got, err := Select(mode)
			if err != nil || string(got.Kind) != mode || got.Executable != want {
				t.Fatalf("Select(%q)=%+v, %v; unrequested executable must not affect selection", mode, got, err)
			}
		})
	}
}

func TestSelectSkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "pi"), []byte("not executable"), 0644); err != nil {
		t.Fatal(err)
	}
	want := writeExecutable(t, dir, "opencode", "exit 0\n")
	got, err := Select("auto")
	if err != nil || got.Kind != OpenCode || got.Executable != want {
		t.Fatalf("Select(auto)=%+v, %v; want executable OpenCode", got, err)
	}
}

func TestSelectInvalidMode(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, mode := range []string{"", "unknown", "Pi", " pi "} {
		got, err := Select(mode)
		if err == nil || got != (Selection{}) || !strings.Contains(err.Error(), "harness") {
			t.Errorf("Select(%q)=%+v, %v; want invalid-mode error", mode, got, err)
		}
	}
}

package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"none", LevelNone},
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"NONE", LevelNone},
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseLevelInvalid(t *testing.T) {
	if got := ParseLevel("invalid"); got != slog.LevelError {
		t.Errorf("ParseLevel(\"invalid\") = %v, want %v (default)", got, slog.LevelError)
	}
	if got := ParseLevel(""); got != slog.LevelError {
		t.Errorf("ParseLevel(\"\") = %v, want %v (default)", got, slog.LevelError)
	}
}

func TestSetupFromConfigNone(t *testing.T) {
	closeLog, err := SetupFromConfig("none", "")
	defer func() { _ = closeLog() }()
	if err != nil {
		t.Fatalf("SetupFromConfig(\"none\", \"\"): %v", err)
	}
	slog.Info("should not appear anywhere")
}

func TestSetupHandlerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	handler := newHandler(&buf, slog.LevelWarn)
	logger := slog.New(handler)

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	output := buf.String()
	if strings.Contains(output, "debug msg") {
		t.Error("debug message should be filtered at warn level")
	}
	if strings.Contains(output, "info msg") {
		t.Error("info message should be filtered at warn level")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("warn message should be present at warn level")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("error message should be present at warn level")
	}
}

func TestSetupHandlerDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := newHandler(&buf, slog.LevelDebug)
	logger := slog.New(handler)

	logger.Debug("debug msg")
	logger.Info("info msg")

	output := buf.String()
	if !strings.Contains(output, "debug msg") {
		t.Error("debug message should be present at debug level")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("info message should be present at debug level")
	}
}

func TestSetupSetsDefaultLogger(t *testing.T) {
	var buf bytes.Buffer
	Setup(&buf, slog.LevelInfo)

	if slog.Default() == nil {
		t.Fatal("slog.Default() should not be nil after Setup")
	}

	slog.Info("test info")
	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Error("output should contain logged message")
	}
}

func TestSetupFileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create log file: %v", err)
	}

	handler := newHandler(f, slog.LevelInfo)
	logger := slog.New(handler)
	logger.Info("file log msg")
	_ = f.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "file log msg") {
		t.Error("log file should contain the message")
	}
}

func TestSetupFromConfigFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	closeLog, err := SetupFromConfig("info", logPath)
	if err != nil {
		t.Fatalf("SetupFromConfig: %v", err)
	}
	t.Cleanup(func() {
		_ = closeLog()
		_ = os.Remove(logPath)
	})

	slog.Info("config file msg")
	// The handler writes asynchronously, so we need to check the file
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "config file msg") {
		t.Error("log file should contain message from configured logger")
	}
}

func TestSetupFromConfigStdout(t *testing.T) {
	saved := slog.Default()

	closeLog, err := SetupFromConfig("warn", "-")
	defer func() { _ = closeLog() }()
	if err != nil {
		t.Fatalf("SetupFromConfig: %v", err)
	}
	// Restore after test
	defer slog.SetDefault(saved)

	if slog.Default() == saved {
		t.Error("Default logger should have been replaced")
	}
}

func TestSetupFromConfigDefault(t *testing.T) {
	saved := slog.Default()

	closeLog, err := SetupFromConfig("error", "")
	defer func() { _ = closeLog() }()
	if err != nil {
		t.Fatalf("SetupFromConfig: %v", err)
	}
	defer slog.SetDefault(saved)

	if slog.Default() == saved {
		t.Error("Default logger should have been replaced even with empty log file")
	}
}

package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		slog.Warn("invalid log level, using default", "level", level, "default", slog.LevelError)
		return slog.LevelError
	}
}

func newHandler(w io.Writer, level slog.Level) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	})
}

func Setup(w io.Writer, level slog.Level) {
	handler := newHandler(w, level)
	slog.SetDefault(slog.New(handler))
}

func SetupFromConfig(logLevel, logFile string) error {
	level := ParseLevel(logLevel)

	var w io.Writer
	switch logFile {
	case "", "-":
		w = os.Stderr
	default:
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		w = f
	}

	handler := newHandler(w, level)
	slog.SetDefault(slog.New(handler))
	return nil
}

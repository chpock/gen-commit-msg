package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	LevelTrace = slog.LevelDebug - 10
	LevelNone  = slog.LevelError + 10
)

func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "none":
		return LevelNone
	default:
		slog.Warn("invalid log level, using default", "level", level, "default", slog.LevelError)
		return slog.LevelError
	}
}

func newHandler(w io.Writer, level slog.Level) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				val := a.Value.Any().(slog.Level)
				if val == LevelTrace {
					a.Value = slog.StringValue("TRACE")
				}
			}
			return a
		},
	})
}

func Setup(w io.Writer, level slog.Level) {
	handler := newHandler(w, level)
	slog.SetDefault(slog.New(handler))
}

func SetupFromConfig(logLevel, logFile string) (func() error, error) {
	level := ParseLevel(logLevel)

	var w io.Writer
	var closer func() error

	if strings.EqualFold(logLevel, "none") {
		w = io.Discard
		closer = func() error { return nil }
	} else {
		switch logFile {
		case "", "-":
			w = os.Stderr
			closer = func() error { return nil }
		default:
			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return nil, err
			}
			w = f
			closer = f.Close
		}
	}

	handler := newHandler(w, level)
	slog.SetDefault(slog.New(handler))
	return closer, nil
}

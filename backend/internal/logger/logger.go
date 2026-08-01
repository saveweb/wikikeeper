package logger

import (
	"log/slog"
	"os"
)

var (
	// Default logger instance
	Log *slog.Logger
)

func init() {
	// Initialize logger with INFO level by default (for tests)
	Init("DEBUG")
}

// Init initializes the global logger
func Init(level string) {
	// Parse log level
	var slogLevel slog.Level
	switch level {
	case "DEBUG":
		slogLevel = slog.LevelDebug
	case "INFO":
		slogLevel = slog.LevelInfo
	case "WARN":
		slogLevel = slog.LevelWarn
	case "ERROR":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	// Create logger with JSON handler for production
	opts := &slog.HandlerOptions{
		Level:       slogLevel,
		ReplaceAttr: utcTimeAttr,
	}

	Log = slog.New(slog.NewTextHandler(os.Stdout, opts))

	// Set default logger
	slog.SetDefault(Log)
}

func utcTimeAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.Time(slog.TimeKey, attr.Value.Time().UTC())
	}
	return attr
}

// With returns a new logger with default attributes
func With(attrs ...any) *slog.Logger {
	return Log.With(attrs...)
}

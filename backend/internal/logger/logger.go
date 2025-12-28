package logger

import (
	"log/slog"
	"os"
)

var (
	// Default logger instance
	Log *slog.Logger
)

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
		Level: slogLevel,
	}

	// Use JSON format in production, text in development
	env := os.Getenv("DEBUG")
	if env == "true" || env == "1" {
		// Development: human-readable text format
		Log = slog.New(slog.NewTextHandler(os.Stdout, opts))
	} else {
		// Production: JSON format
		Log = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}

	// Set default logger
	slog.SetDefault(Log)
}

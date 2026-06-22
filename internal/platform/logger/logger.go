// This file creates structured loggers for Saturn runtime components.
package logger

import (
	"io"
	"log/slog"
	"strings"
)

func New(out io.Writer, level string) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(out, handlerOptions))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Package logger builds the structured JSON logger used by every service.
package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON logger that tags every line with the service name.
// level is one of: debug, info, warn, error.
func New(service, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler).With("service", service)
}

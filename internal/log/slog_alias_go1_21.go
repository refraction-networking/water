//go:build go1.21

package log

import (
	"log/slog"
)

// Logger is an alias for slog.Logger. It is used here to avoid
// importing slog in the rest of the code, since slog needs to be
// imported conditionally based on the Go version.
type Logger = slog.Logger
type Handler = slog.Handler

var defaultLogger *Logger = slog.Default()

// SetLogger specifies the logger to be used by the package.
// By default, slog.Default() is used.
//
// It overrides the logger created by SetDefaultHandler.
func SetDefaultLogger(logger *slog.Logger) {
	defaultLogger = logger
}

// SetDefaultHandler specifies the handler to be used by the package.
//
// It overrides the logger specified by SetDefaultLogger.
func SetDefaultHandler(handler slog.Handler) {
	defaultLogger = slog.New(handler)
}

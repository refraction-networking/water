//go:build !go1.21

package log

import (
	"golang.org/x/exp/slog"
)

// Logger is an alias for slog.Logger. It is used here to avoid
// importing slog in the rest of the code, since slog needs to be
// imported conditionally based on the Go version.
type Logger = slog.Logger
type Handler = LogHandler // Deprecated: use LogHandler. The naming of this type is ambiguous.
type LogHandler = slog.Handler

var defaultLogger *Logger = slog.Default()

// SetLogger specifies the logger to be used by the package.
// By default, slog.Default() is used.
//
// It overrides the logger created by SetDefaultHandler.
func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

// SetDefaultHandler specifies the handler to be used by the package.
//
// It overrides the logger specified by SetDefaultLogger.
func SetDefaultHandler(handler LogHandler) {
	defaultLogger = slog.New(handler)
}

// DefaultLogger returns the current default logger.
//
// It might not be equivalent to slog.Default() if SetDefaultLogger
// or SetDefaultHandler was called before.
func DefaultLogger() *Logger {
	return defaultLogger
}

//go:build !go1.21

package log

import (
	"fmt"

	"golang.org/x/exp/slog"
)

var slogger *slog.Logger = slog.Default()

func ReplaceLogger(logger *slog.Logger) {
	slogger = logger
}

func Debugf(format string, args ...any) {
	slogger.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	slogger.Info(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	slogger.Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	slogger.Error(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	slogger.Error(fmt.Sprintf(format, args...))
	panic("fatal error occurred")
}

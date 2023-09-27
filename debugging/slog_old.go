//go:build !go1.21

package debugging

import (
	"fmt"

	"golang.org/x/exp/slog"
)

func Debugf(format string, args ...any) {
	slog.Default().Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	slog.Default().Info(fmt.Sprintf(format, args...))
}

func Warningf(format string, args ...any) {
	slog.Default().Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	slog.Default().Error(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	slog.Default().Error(fmt.Sprintf(format, args...))
	panic("fatal error occurred")
}

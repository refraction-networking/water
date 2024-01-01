package water

import (
	"github.com/gaukas/water/internal/log"
)

// SetDefaultLogger sets the logger to be used by the package
// if no logger is specifically configured for each component.
//
// By default, slog.Default() is used.
func SetDefaultLogger(logger *log.Logger) {
	log.SetDefaultLogger(logger)
}

// SetDefaultHandler sets the handler to be used by the package
// if no logger is specifically configured for each component.
//
// It overrides the logger specified by SetDefaultLogger.
func SetDefaultHandler(handler log.Handler) {
	log.SetDefaultHandler(handler)
}

package water

import (
	"github.com/refraction-networking/water/internal/log"
)

// SetDefaultLogger sets the logger to be used by the package
// if no logger is specifically configured for each component.
//
// By default, slog.Default() is used.
//
// Deprecated: specify OverrideLogger in the Config instead.
func SetDefaultLogger(logger *log.Logger) {
	log.SetDefaultLogger(logger)
}

// SetDefaultLogHandler sets the handler to be used by the package
// if no logger is specifically configured for each component.
// Renamed from SetDefaultHandler.
//
// It overrides the logger specified by SetDefaultLogger.
//
// Deprecated: specify OverrideLogger in the Config instead.
func SetDefaultLogHandler(handler log.LogHandler) {
	log.SetDefaultHandler(handler)
}

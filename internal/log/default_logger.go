package log

// GetLogger returns a pointer to the logger used by the package.
func GetDefaultLogger() *Logger {
	return defaultLogger
}

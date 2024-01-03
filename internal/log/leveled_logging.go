package log

import "fmt"

func Debugf(format string, args ...any) {
	defaultLogger.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	defaultLogger.Info(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	defaultLogger.Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	defaultLogger.Error(fmt.Sprintf(format, args...))
}

func LDebugf(logger *Logger, format string, args ...any) {
	logger.Debug(fmt.Sprintf(format, args...))
}

func LInfof(logger *Logger, format string, args ...any) {
	logger.Info(fmt.Sprintf(format, args...))
}

func LWarnf(logger *Logger, format string, args ...any) {
	logger.Warn(fmt.Sprintf(format, args...))
}

func LErrorf(logger *Logger, format string, args ...any) {
	logger.Error(fmt.Sprintf(format, args...))
}

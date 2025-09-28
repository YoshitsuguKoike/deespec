package fs

import (
	"fmt"
	"os"
)

// Logger interface for fs layer
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// defaultLogger writes directly to stderr
type defaultLogger struct{}

func (l *defaultLogger) Debug(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
}

func (l *defaultLogger) Info(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "INFO: "+format+"\n", args...)
}

func (l *defaultLogger) Warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "WARN: "+format+"\n", args...)
}

func (l *defaultLogger) Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
}

// globalLogger is the logger instance
var globalLogger Logger = &defaultLogger{}

// SetLogger sets the global logger
func SetLogger(logger Logger) {
	if logger != nil {
		globalLogger = logger
	}
}

// GetLogger returns the current logger
func GetLogger() Logger {
	return globalLogger
}

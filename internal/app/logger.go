package app

import (
	"fmt"
	"io"
	"os"
)

// Logger interface for app layer
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// defaultLogger writes directly to stderr without level control
type defaultLogger struct {
	output io.Writer
}

func (l *defaultLogger) Debug(format string, args ...interface{}) {
	fmt.Fprintf(l.output, "DEBUG: "+format+"\n", args...)
}

func (l *defaultLogger) Info(format string, args ...interface{}) {
	fmt.Fprintf(l.output, "INFO: "+format+"\n", args...)
}

func (l *defaultLogger) Warn(format string, args ...interface{}) {
	fmt.Fprintf(l.output, "WARN: "+format+"\n", args...)
}

func (l *defaultLogger) Error(format string, args ...interface{}) {
	fmt.Fprintf(l.output, "ERROR: "+format+"\n", args...)
}

// globalLogger is the logger instance used by app layer
var globalLogger Logger = &defaultLogger{output: os.Stderr}

// SetLogger sets the global logger for app layer
func SetLogger(logger Logger) {
	if logger != nil {
		globalLogger = logger
	}
}

// GetLogger returns the current logger
func GetLogger() Logger {
	return globalLogger
}
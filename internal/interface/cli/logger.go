package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Logger provides centralized logging with level control
type Logger struct {
	mu       sync.RWMutex
	minLevel LogLevel
	output   io.Writer
}

// NewLogger creates a new logger with the specified minimum level
func NewLogger(minLevel LogLevel, output io.Writer) *Logger {
	return &Logger{
		minLevel: minLevel,
		output:   output,
	}
}

// SetLevel changes the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// GetLevel returns the current minimum log level
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.minLevel
}

// SetOutput changes the output writer
func (l *Logger) SetOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = output
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LogLevelDebug, "DEBUG", format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LogLevelInfo, "INFO", format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LogLevelWarn, "WARN", format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LogLevelError, "ERROR", format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	// Fatal messages are always logged regardless of level
	l.mu.RLock()
	output := l.output
	l.mu.RUnlock()

	fmt.Fprintf(output, "FATAL: %s\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}

// log writes a log message if it meets the minimum level
func (l *Logger) log(level LogLevel, prefix string, format string, args ...interface{}) {
	l.mu.RLock()
	minLevel := l.minLevel
	output := l.output
	l.mu.RUnlock()

	if level >= minLevel {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(output, "%s: %s\n", prefix, msg)
	}
}

// LogLevelFromString converts a string to LogLevel with better defaults
func LogLevelFromString(level string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "fatal":
		return LogLevelError // Fatal is treated as Error level for filtering
	default:
		// Default to WARN level if not specified or invalid
		return LogLevelWarn
	}
}

// Global logger instance
var globalLogger *Logger

// InitGlobalLogger initializes the global logger
func InitGlobalLogger(level string) {
	if level == "" {
		// Default to WARN if not specified
		level = "warn"
	}
	globalLogger = NewLogger(LogLevelFromString(level), os.Stderr)
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// Initialize with default WARN level if not initialized
		InitGlobalLogger("warn")
	}
	return globalLogger
}

// Convenience functions for global logger

// Debug logs a debug message using the global logger
func Debug(format string, args ...interface{}) {
	GetLogger().Debug(format, args...)
}

// Info logs an info message using the global logger
func Info(format string, args ...interface{}) {
	GetLogger().Info(format, args...)
}

// Warn logs a warning message using the global logger
func Warn(format string, args ...interface{}) {
	GetLogger().Warn(format, args...)
}

// Error logs an error message using the global logger
func Error(format string, args ...interface{}) {
	GetLogger().Error(format, args...)
}

// Fatal logs a fatal message using the global logger and exits
func Fatal(format string, args ...interface{}) {
	GetLogger().Fatal(format, args...)
}

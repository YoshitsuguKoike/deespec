package common

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// ParseLogLevel converts a string to LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// LogEntry represents a buffered log entry
type LogEntry struct {
	Level   LogLevel
	Message string
}

// LogBuffer buffers log messages until they can be flushed
// This is used to buffer logs before policy configuration is loaded
type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	flushed bool
}

// NewLogBuffer creates a new log buffer
func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0),
	}
}

// Debug adds a debug message to the buffer
func (b *LogBuffer) Debug(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.flushed {
		// If already flushed, write directly to stderr
		fmt.Fprintf(defaultStderr, "DEBUG: "+format+"\n", args...)
		return
	}

	b.entries = append(b.entries, LogEntry{
		Level:   LogLevelDebug,
		Message: fmt.Sprintf(format, args...),
	})
}

// Info adds an info message to the buffer
func (b *LogBuffer) Info(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.flushed {
		// If already flushed, write directly to stderr
		fmt.Fprintf(defaultStderr, "INFO: "+format+"\n", args...)
		return
	}

	b.entries = append(b.entries, LogEntry{
		Level:   LogLevelInfo,
		Message: fmt.Sprintf(format, args...),
	})
}

// Warn adds a warning message to the buffer
func (b *LogBuffer) Warn(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.flushed {
		// If already flushed, write directly to stderr
		fmt.Fprintf(defaultStderr, "WARN: "+format+"\n", args...)
		return
	}

	b.entries = append(b.entries, LogEntry{
		Level:   LogLevelWarn,
		Message: fmt.Sprintf(format, args...),
	})
}

// Error adds an error message to the buffer
// Error messages are always output immediately
func (b *LogBuffer) Error(format string, args ...interface{}) {
	// Error messages are always output immediately
	fmt.Fprintf(defaultStderr, "ERROR: "+format+"\n", args...)
}

// Flush outputs buffered messages based on the minimum log level
func (b *LogBuffer) Flush(minLevel LogLevel, output io.Writer) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.flushed {
		return
	}

	for _, entry := range b.entries {
		if entry.Level >= minLevel {
			prefix := ""
			switch entry.Level {
			case LogLevelDebug:
				prefix = "DEBUG: "
			case LogLevelInfo:
				prefix = "INFO: "
			case LogLevelWarn:
				prefix = "WARN: "
			case LogLevelError:
				prefix = "ERROR: "
			}
			fmt.Fprintf(output, "%s%s\n", prefix, entry.Message)
		}
	}

	b.flushed = true
	b.entries = nil
}

// Clear discards all buffered messages without outputting them
func (b *LogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = nil
	b.flushed = true
}

// Size returns the number of buffered entries
func (b *LogBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.entries)
}

// Global log buffer for use during initialization
var globalLogBuffer = NewLogBuffer()

// defaultStderr is the default output for immediate error messages
var defaultStderr io.Writer = io.Discard

// SetDefaultStderr sets the default stderr writer
func SetDefaultStderr(w io.Writer) {
	defaultStderr = w
}

package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
)

// loggerBridge adapts CLI logger to app.Logger interface
type loggerBridge struct {
	cliLogger *Logger
}

func (b *loggerBridge) Debug(format string, args ...interface{}) {
	b.cliLogger.Debug(format, args...)
}

func (b *loggerBridge) Info(format string, args ...interface{}) {
	b.cliLogger.Info(format, args...)
}

func (b *loggerBridge) Warn(format string, args ...interface{}) {
	b.cliLogger.Warn(format, args...)
}

func (b *loggerBridge) Error(format string, args ...interface{}) {
	b.cliLogger.Error(format, args...)
}

// InitializeLoggers sets up loggers for all layers
func InitializeLoggers(logger *Logger) {
	// Set app layer logger
	appLogger := &loggerBridge{cliLogger: logger}
	app.SetLogger(appLogger)

	// Set infra layer loggers
	fs.SetLogger(appLogger)
	txn.SetLogger(appLogger)
}

package common

import (
	"github.com/YoshitsuguKoike/deespec/internal/app/config"
)

// globalConfig holds the loaded configuration for all commands
var globalConfig config.Config

// SetGlobalConfig sets the global configuration
func SetGlobalConfig(cfg config.Config) {
	globalConfig = cfg
}

// GetGlobalConfig returns the global configuration
func GetGlobalConfig() config.Config {
	return globalConfig
}

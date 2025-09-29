// Deprecated: This file is no longer used. Use settings.go instead.
// Kept for reference but will be removed in a future version.

package config

import (
	"strconv"
	"time"
)

type Config struct {
	AgentBin string        // ex: "claude"
	Timeout  time.Duration // default 60s
}

// Deprecated: Use LoadSettings instead
func Load() Config {
	// Always return defaults - environment variables are not used
	get := func(k, def string) string {
		return def
	}
	toDur := func(s string, def time.Duration) time.Duration {
		if s == "" {
			return def
		}
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second
		}
		return def
	}
	return Config{
		AgentBin: get("DEE_AGENT_BIN", "claude"),
		Timeout:  toDur(get("DEE_TIMEOUT_SEC", "60"), 60*time.Second),
	}
}

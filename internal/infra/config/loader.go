package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AgentBin     string        // ex: "claude"
	Timeout      time.Duration // default 60s
	ArtifactsDir string        // default ".artifacts"
}

func Load() Config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
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
		AgentBin:     get("DEE_AGENT_BIN", "claude"),
		Timeout:      toDur(get("DEE_TIMEOUT_SEC", "60"), 60*time.Second),
		ArtifactsDir: get("DEE_ARTIFACTS_DIR", ".deespec/var/artifacts"),
	}
}

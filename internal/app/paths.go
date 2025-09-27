package app

import (
	"os"
	"path/filepath"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
)

// Paths holds all resolved paths for deespec v1 structure
type Paths struct {
	Home      string // .deespec directory
	Etc       string // .deespec/etc
	Prompts   string // .deespec/prompts
	Policies  string // .deespec/etc/policies
	SpecsSBI  string // .deespec/specs/sbi
	SpecsPBI  string // .deespec/specs/pbi
	Var       string // .deespec/var
	Artifacts string // .deespec/var/artifacts

	// Key files
	Workflow  string // .deespec/etc/workflow.yaml
	State     string // .deespec/var/state.json
	Journal   string // .deespec/var/journal.ndjson
	Health    string // .deespec/var/health.json
	StateLock string // .deespec/var/state.lock
}

// ResolvePaths returns all paths based on DEE_HOME environment variable
// All paths are resolved to absolute paths with symlinks evaluated
func ResolvePaths() Paths {
	// Get base home directory
	// Still check ENV for backward compatibility
	home := os.Getenv("DEE_HOME")
	if home == "" {
		home = ".deespec"
	}
	return ResolvePathsWithHome(home)
}

// ResolvePathsWithConfig returns all paths based on Config
func ResolvePathsWithConfig(cfg config.Config) Paths {
	return ResolvePathsWithHome(cfg.Home())
}

// ResolvePathsWithHome returns all paths based on the given home directory
func ResolvePathsWithHome(home string) Paths {

	// Resolve home to absolute path with symlinks
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		homeAbs = home // Fallback to original if error
	}
	homeAbs, err = filepath.EvalSymlinks(homeAbs)
	if err != nil {
		// If symlink evaluation fails (e.g., path doesn't exist yet),
		// just use the absolute path
		homeAbs, _ = filepath.Abs(home)
	}

	// Build all paths (now all absolute)
	p := Paths{
		Home:    homeAbs,
		Etc:     filepath.Join(homeAbs, "etc"),
		Prompts: filepath.Join(homeAbs, "prompts"),
		Var:     filepath.Join(homeAbs, "var"),
	}

	// Derived paths
	p.Policies = filepath.Join(p.Etc, "policies")
	p.SpecsSBI = filepath.Join(homeAbs, "specs", "sbi")
	p.SpecsPBI = filepath.Join(homeAbs, "specs", "pbi")
	p.Artifacts = filepath.Join(p.Var, "artifacts")

	// Key files
	p.Workflow = filepath.Join(p.Etc, "workflow.yaml")
	p.State = filepath.Join(p.Var, "state.json")
	p.Journal = filepath.Join(p.Var, "journal.ndjson")
	p.Health = filepath.Join(p.Var, "health.json")
	p.StateLock = filepath.Join(p.Var, "state.lock")

	return p
}

// GetPaths is a convenience function that returns singleton paths
var cachedPaths *Paths
var pathsConfig config.Config

func GetPaths() Paths {
	// Skip cache if in test mode
	if os.Getenv("DEESPEC_TEST_MODE") == "true" {
		return ResolvePaths()
	}

	if cachedPaths == nil {
		paths := ResolvePaths()
		cachedPaths = &paths
	}
	return *cachedPaths
}

// GetPathsWithConfig returns paths based on the provided config
func GetPathsWithConfig(cfg config.Config) Paths {
	// Skip cache if in test mode or if config changed
	if cfg != nil && cfg.TestMode() {
		return ResolvePathsWithConfig(cfg)
	}

	// Check if config changed
	if cfg != pathsConfig {
		pathsConfig = cfg
		paths := ResolvePathsWithConfig(cfg)
		cachedPaths = &paths
	}
	return *cachedPaths
}

// ClearPathsCache clears the cached paths (useful for testing)
func ClearPathsCache() {
	cachedPaths = nil
}

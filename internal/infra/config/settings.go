package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
)

// RawSettings represents the structure of setting.json file.
// JSON tags are used for marshaling/unmarshaling.
type RawSettings struct {
	// Core settings
	Home       *string `json:"home"`
	AgentBin   *string `json:"agent_bin"`
	TimeoutSec *int    `json:"timeout_sec"`

	// Workflow variables
	ProjectName *string `json:"project_name"`
	Language    *string `json:"language"`
	Turn        *string `json:"turn"`
	TaskID      *string `json:"task_id"`

	// Feature flags
	Validate    *bool `json:"validate"`
	AutoFB      *bool `json:"auto_fb"`
	StrictFsync *bool `json:"strict_fsync"`

	// Transaction settings
	TxDestRoot      *string `json:"tx_dest_root"`
	DisableRecovery *bool   `json:"disable_recovery"`

	// Metrics and audit
	DisableMetricsRotation *bool `json:"disable_metrics_rotation"`
	FsyncAudit             *bool `json:"fsync_audit"`

	// Test and debug
	TestMode  *bool `json:"test_mode"`
	TestQuiet *bool `json:"test_quiet"`

	// Paths and logging
	Workflow    *string `json:"workflow"`
	PolicyPath  *string `json:"policy_path"`
	StderrLevel *string `json:"stderr_level"`
}

// LoadSettings loads configuration from setting.json only.
// Priority: setting.json > defaults
func LoadSettings(baseDir string) (*config.AppConfig, error) {
	// Start with empty settings
	settings := &RawSettings{}
	configSource := "default"
	settingPath := ""

	// Try to load setting.json
	jsonPath := filepath.Join(baseDir, "setting.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		if err := json.Unmarshal(data, settings); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", jsonPath, err)
		}
		configSource = "json"
		settingPath = jsonPath
	}

	// No environment variable overrides - setting.json only

	// Apply defaults
	applyDefaults(settings)

	// Warn about deprecated settings
	checkDeprecated(settings)

	// Build AppConfig
	return buildAppConfig(settings, configSource, settingPath), nil
}

// Removed: Environment variable overrides are no longer supported.
// Configuration must be set in setting.json only.

// applyDefaults fills in default values for any nil fields
func applyDefaults(settings *RawSettings) {
	// Core defaults
	if settings.Home == nil {
		v := ".deespec"
		settings.Home = &v
	}
	if settings.AgentBin == nil {
		v := "claude"
		settings.AgentBin = &v
	}
	if settings.TimeoutSec == nil {
		v := 900 // 15 minutes for complex tasks
		settings.TimeoutSec = &v
	}

	// Workflow variables (default to empty)
	if settings.ProjectName == nil {
		v := ""
		settings.ProjectName = &v
	}
	if settings.Language == nil {
		v := ""
		settings.Language = &v
	}
	if settings.Turn == nil {
		v := ""
		settings.Turn = &v
	}
	if settings.TaskID == nil {
		v := ""
		settings.TaskID = &v
	}

	// Feature flags (default to false)
	if settings.Validate == nil {
		v := false
		settings.Validate = &v
	}
	if settings.AutoFB == nil {
		v := false
		settings.AutoFB = &v
	}
	if settings.StrictFsync == nil {
		v := false
		settings.StrictFsync = &v
	}

	// Transaction settings
	if settings.TxDestRoot == nil {
		v := ""
		settings.TxDestRoot = &v
	}
	if settings.DisableRecovery == nil {
		v := false
		settings.DisableRecovery = &v
	}

	// Metrics and audit
	if settings.DisableMetricsRotation == nil {
		v := false
		settings.DisableMetricsRotation = &v
	}
	if settings.FsyncAudit == nil {
		v := false
		settings.FsyncAudit = &v
	}

	// Test and debug
	if settings.TestMode == nil {
		v := false
		settings.TestMode = &v
	}
	if settings.TestQuiet == nil {
		v := false
		settings.TestQuiet = &v
	}

	// Paths and logging
	if settings.Workflow == nil {
		v := ""
		settings.Workflow = &v
	}
	if settings.PolicyPath == nil {
		v := ""
		settings.PolicyPath = &v
	}
	if settings.StderrLevel == nil {
		v := "warn" // Default to WARN level
		settings.StderrLevel = &v
	}
}

// checkDeprecated warns about deprecated settings
func checkDeprecated(settings *RawSettings) {
	// Currently no deprecated settings
}

// buildAppConfig converts RawSettings to AppConfig
func buildAppConfig(settings *RawSettings, configSource, settingPath string) *config.AppConfig {
	return config.NewAppConfig(
		*settings.Home,
		*settings.AgentBin,
		*settings.TimeoutSec,
		*settings.ProjectName,
		*settings.Language,
		*settings.Turn,
		*settings.TaskID,
		*settings.Validate,
		*settings.AutoFB,
		*settings.StrictFsync,
		*settings.TxDestRoot,
		*settings.DisableRecovery,
		*settings.DisableMetricsRotation,
		*settings.FsyncAudit,
		*settings.TestMode,
		*settings.TestQuiet,
		*settings.Workflow,
		*settings.PolicyPath,
		*settings.StderrLevel,
		configSource,
		settingPath,
	)
}

// toBool converts various string representations to boolean
func toBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}

// CreateDefaultSettings creates a default setting.json content
func CreateDefaultSettings() []byte {
	settings := &RawSettings{}
	applyDefaults(settings)

	data, _ := json.MarshalIndent(settings, "", "  ")
	return data
}

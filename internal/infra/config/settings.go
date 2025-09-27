package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
)

// RawSettings represents the structure of setting.json file.
// JSON tags are used for marshaling/unmarshaling.
type RawSettings struct {
	// Core settings
	Home         *string `json:"home"`
	AgentBin     *string `json:"agent_bin"`
	TimeoutSec   *int    `json:"timeout_sec"`
	ArtifactsDir *string `json:"artifacts_dir"`

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
	DisableStateTx  *bool   `json:"disable_state_tx"`
	UseTx           *bool   `json:"use_tx"`

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

// LoadSettings loads configuration from multiple sources with the following priority:
// 1. setting.json (if exists)
// 2. Environment variables (override JSON)
// 3. Default values (fill missing)
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

	// Override with environment variables
	overrideFromEnv(settings, &configSource)

	// Apply defaults
	applyDefaults(settings)

	// Warn about deprecated settings
	checkDeprecated(settings)

	// Build AppConfig
	return buildAppConfig(settings, configSource, settingPath), nil
}

// overrideFromEnv overrides settings with environment variables if they are set
func overrideFromEnv(settings *RawSettings, configSource *string) {
	// Core settings
	if v := os.Getenv("DEE_HOME"); v != "" {
		settings.Home = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_AGENT_BIN"); v != "" {
		settings.AgentBin = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			settings.TimeoutSec = &n
			if *configSource == "default" {
				*configSource = "env"
			}
		}
	}
	if v := os.Getenv("DEE_ARTIFACTS_DIR"); v != "" {
		settings.ArtifactsDir = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}

	// Workflow variables
	if v := os.Getenv("DEE_PROJECT_NAME"); v != "" {
		settings.ProjectName = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_LANGUAGE"); v != "" {
		settings.Language = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_TURN"); v != "" {
		settings.Turn = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_TASK_ID"); v != "" {
		settings.TaskID = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}

	// Feature flags
	if v := os.Getenv("DEE_VALIDATE"); v != "" {
		b := toBool(v)
		settings.Validate = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_AUTO_FB"); v != "" {
		b := toBool(v)
		settings.AutoFB = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEE_STRICT_FSYNC"); v != "" {
		b := toBool(v)
		settings.StrictFsync = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}

	// Transaction settings
	if v := os.Getenv("DEESPEC_TX_DEST_ROOT"); v != "" {
		settings.TxDestRoot = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_DISABLE_RECOVERY"); v != "" {
		b := toBool(v)
		settings.DisableRecovery = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_DISABLE_STATE_TX"); v != "" {
		b := toBool(v)
		settings.DisableStateTx = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_USE_TX"); v != "" {
		b := toBool(v)
		settings.UseTx = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}

	// Metrics and audit
	if v := os.Getenv("DEESPEC_DISABLE_METRICS_ROTATION"); v != "" {
		b := toBool(v)
		settings.DisableMetricsRotation = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_FSYNC_AUDIT"); v != "" {
		b := toBool(v)
		settings.FsyncAudit = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}

	// Test and debug
	if v := os.Getenv("DEESPEC_TEST_MODE"); v != "" {
		b := toBool(v)
		settings.TestMode = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_TEST_QUIET"); v != "" {
		b := toBool(v)
		settings.TestQuiet = &b
		if *configSource == "default" {
			*configSource = "env"
		}
	}

	// Paths and logging
	if v := os.Getenv("DEESPEC_WORKFLOW"); v != "" {
		settings.Workflow = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_POLICY_PATH"); v != "" {
		settings.PolicyPath = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
	if v := os.Getenv("DEESPEC_STDERR_LEVEL"); v != "" {
		settings.StderrLevel = &v
		if *configSource == "default" {
			*configSource = "env"
		}
	}
}

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
		v := 60
		settings.TimeoutSec = &v
	}
	if settings.ArtifactsDir == nil {
		v := ".deespec/var/artifacts"
		settings.ArtifactsDir = &v
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
	if settings.DisableStateTx == nil {
		v := false
		settings.DisableStateTx = &v
	}
	if settings.UseTx == nil {
		v := false
		settings.UseTx = &v
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
		v := ""
		settings.StderrLevel = &v
	}
}

// checkDeprecated warns about deprecated settings
func checkDeprecated(settings *RawSettings) {
	if settings.DisableStateTx != nil && *settings.DisableStateTx {
		fmt.Fprintf(os.Stderr, "WARN: DEESPEC_DISABLE_STATE_TX/disable_state_tx is deprecated and will be removed in v0.2.0\n")
	}
}

// buildAppConfig converts RawSettings to AppConfig
func buildAppConfig(settings *RawSettings, configSource, settingPath string) *config.AppConfig {
	return config.NewAppConfig(
		*settings.Home,
		*settings.AgentBin,
		*settings.TimeoutSec,
		*settings.ArtifactsDir,
		*settings.ProjectName,
		*settings.Language,
		*settings.Turn,
		*settings.TaskID,
		*settings.Validate,
		*settings.AutoFB,
		*settings.StrictFsync,
		*settings.TxDestRoot,
		*settings.DisableRecovery,
		*settings.DisableStateTx,
		*settings.UseTx,
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

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
	Editor     *string `json:"editor"`

	// Workflow variables
	ProjectName *string `json:"project_name"`
	Language    *string `json:"language"`
	Turn        *string `json:"turn"`
	TaskID      *string `json:"task_id"`

	// Feature flags
	Validate    *bool `json:"validate"`
	AutoFB      *bool `json:"auto_fb"`
	StrictFsync *bool `json:"strict_fsync"`

	// Execution limits
	MaxAttempts *int `json:"max_attempts"`
	MaxTurns    *int `json:"max_turns"`

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

	// Label system configuration
	LabelConfig *RawLabelConfig `json:"label_config"`

	// Agent pool configuration
	AgentPoolConfig *RawAgentPoolConfig `json:"agent_pool_config"`
}

// RawLabelImportConfig represents import settings for labels
type RawLabelImportConfig struct {
	AutoPrefixFromDir *bool     `json:"auto_prefix_from_dir"`
	MaxLineCount      *int      `json:"max_line_count"`
	ExcludePatterns   *[]string `json:"exclude_patterns"`
}

// RawLabelValidationConfig represents validation settings for labels
type RawLabelValidationConfig struct {
	AutoSyncOnMismatch *bool `json:"auto_sync_on_mismatch"`
	WarnOnLargeFiles   *bool `json:"warn_on_large_files"`
}

// RawLabelConfig represents label system configuration in setting.json
type RawLabelConfig struct {
	TemplateDirs *[]string                 `json:"template_dirs"`
	Import       *RawLabelImportConfig     `json:"import"`
	Validation   *RawLabelValidationConfig `json:"validation"`
}

// RawAgentPoolConfig represents agent pool configuration in setting.json
type RawAgentPoolConfig struct {
	MaxConcurrent *map[string]int `json:"max_concurrent"`
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
	if settings.Editor == nil {
		// Priority: setting.json > $EDITOR > $VISUAL > vim
		v := os.Getenv("EDITOR")
		if v == "" {
			v = os.Getenv("VISUAL")
		}
		if v == "" {
			v = "vim"
		}
		settings.Editor = &v
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

	// Execution limits (defaults)
	if settings.MaxAttempts == nil {
		v := 3 // Maximum 3 attempts before force termination
		settings.MaxAttempts = &v
	}
	if settings.MaxTurns == nil {
		v := 8 // Maximum 8 turns total
		settings.MaxTurns = &v
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

	// Label system configuration
	if settings.LabelConfig == nil {
		settings.LabelConfig = &RawLabelConfig{}
	}
	if settings.LabelConfig.TemplateDirs == nil {
		v := []string{".claude", ".deespec/prompts/labels"}
		settings.LabelConfig.TemplateDirs = &v
	}
	if settings.LabelConfig.Import == nil {
		settings.LabelConfig.Import = &RawLabelImportConfig{}
	}
	if settings.LabelConfig.Import.AutoPrefixFromDir == nil {
		v := true
		settings.LabelConfig.Import.AutoPrefixFromDir = &v
	}
	if settings.LabelConfig.Import.MaxLineCount == nil {
		v := 1000
		settings.LabelConfig.Import.MaxLineCount = &v
	}
	if settings.LabelConfig.Import.ExcludePatterns == nil {
		v := []string{"*.secret.md", "settings.*.json", "tmp/**"}
		settings.LabelConfig.Import.ExcludePatterns = &v
	}
	if settings.LabelConfig.Validation == nil {
		settings.LabelConfig.Validation = &RawLabelValidationConfig{}
	}
	if settings.LabelConfig.Validation.AutoSyncOnMismatch == nil {
		v := false
		settings.LabelConfig.Validation.AutoSyncOnMismatch = &v
	}
	if settings.LabelConfig.Validation.WarnOnLargeFiles == nil {
		v := true
		settings.LabelConfig.Validation.WarnOnLargeFiles = &v
	}

	// Agent pool configuration
	if settings.AgentPoolConfig == nil {
		settings.AgentPoolConfig = &RawAgentPoolConfig{}
	}
	if settings.AgentPoolConfig.MaxConcurrent == nil {
		v := map[string]int{
			"claude-code": 2,
			"gemini-cli":  1,
			"codex":       1,
		}
		settings.AgentPoolConfig.MaxConcurrent = &v
	}
}

// checkDeprecated warns about deprecated settings
func checkDeprecated(settings *RawSettings) {
	// Currently no deprecated settings
}

// buildAppConfig converts RawSettings to AppConfig
func buildAppConfig(settings *RawSettings, configSource, settingPath string) *config.AppConfig {
	// Convert RawLabelConfig to config.LabelConfig
	labelConfig := config.LabelConfig{
		TemplateDirs: *settings.LabelConfig.TemplateDirs,
		Import: config.LabelImportConfig{
			AutoPrefixFromDir: *settings.LabelConfig.Import.AutoPrefixFromDir,
			MaxLineCount:      *settings.LabelConfig.Import.MaxLineCount,
			ExcludePatterns:   *settings.LabelConfig.Import.ExcludePatterns,
		},
		Validation: config.LabelValidationConfig{
			AutoSyncOnMismatch: *settings.LabelConfig.Validation.AutoSyncOnMismatch,
			WarnOnLargeFiles:   *settings.LabelConfig.Validation.WarnOnLargeFiles,
		},
	}

	// Convert RawAgentPoolConfig to config.AgentPoolConfig
	agentPoolConfig := config.AgentPoolConfig{
		MaxConcurrent: *settings.AgentPoolConfig.MaxConcurrent,
	}

	return config.NewAppConfig(
		*settings.Home,
		*settings.AgentBin,
		*settings.TimeoutSec,
		*settings.Editor,
		*settings.ProjectName,
		*settings.Language,
		*settings.Turn,
		*settings.TaskID,
		*settings.Validate,
		*settings.AutoFB,
		*settings.StrictFsync,
		*settings.MaxAttempts,
		*settings.MaxTurns,
		*settings.TxDestRoot,
		*settings.DisableRecovery,
		*settings.DisableMetricsRotation,
		*settings.FsyncAudit,
		*settings.TestMode,
		*settings.TestQuiet,
		*settings.Workflow,
		*settings.PolicyPath,
		*settings.StderrLevel,
		labelConfig,
		agentPoolConfig,
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

package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/app/config"
	infraConfig "github.com/YoshitsuguKoike/deespec/internal/infra/config"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/clear"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/doctor"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/health"
	initcmd "github.com/YoshitsuguKoike/deespec/internal/interface/cli/init"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/journal"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/label"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/lock_cmd"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/run"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/status"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/version"
	"github.com/spf13/cobra"
)

// globalLogLevel is the CLI flag override for log level
var globalLogLevel string

func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deespec",
		Short: "DeeSpec CLI - Parallel workflow orchestration for specification processing",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration before any command runs
			// Priority: CLI flag > setting.json > defaults
			// Always use .deespec as the base directory
			baseDir := ".deespec"

			cfg, err := infraConfig.LoadSettings(baseDir)
			if err != nil {
				// Continue with defaults if loading fails
				defaultLabelConfig := config.LabelConfig{
					TemplateDirs: []string{".claude", ".deespec/prompts/labels"},
					Import: config.LabelImportConfig{
						AutoPrefixFromDir: true,
						MaxLineCount:      1000,
						ExcludePatterns:   []string{"*.secret.md", "settings.*.json", "tmp/**"},
					},
					Validation: config.LabelValidationConfig{
						AutoSyncOnMismatch: false,
						WarnOnLargeFiles:   true,
					},
				}
				defaultAgentPoolConfig := config.AgentPoolConfig{
					MaxConcurrent: map[string]int{
						"claude-code": 2,
						"gemini-cli":  1,
						"codex":       1,
					},
				}
				cfg = config.NewAppConfig(
					".deespec", "claude", 60, "vim", // Add default editor
					"", "", "", "",
					false, false, false,
					3, 8, // max_attempts=3, max_turns=8
					"", false,
					false, false,
					false, false,
					"", "", "warn", // Default log level
					defaultLabelConfig,
					defaultAgentPoolConfig,
					"default", "",
				)
			}
			common.SetGlobalConfig(cfg)

			// Determine log level: CLI flag takes precedence
			logLevel := cfg.StderrLevel()
			if globalLogLevel != "" {
				logLevel = globalLogLevel
			}

			// Initialize global logger with determined level
			common.InitGlobalLogger(logLevel)

			// Initialize loggers for all layers
			common.InitializeLoggers(common.GetLogger())

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(initcmd.NewCommand())
	cmd.AddCommand(status.NewCommand())
	cmd.AddCommand(run.NewCommand())
	cmd.AddCommand(doctor.NewCommand())
	cmd.AddCommand(journal.NewCommand())
	cmd.AddCommand(health.NewCommand())
	cmd.AddCommand(pbi.NewPBICommand()) // PBI management
	cmd.AddCommand(sbi.NewSBICommand())
	cmd.AddCommand(clear.NewCommand())
	cmd.AddCommand(lock_cmd.NewCommand()) // SQLite-based lock management
	cmd.AddCommand(label.NewCommand())
	cmd.AddCommand(version.NewCommand())

	// Add global log level flag
	cmd.PersistentFlags().StringVar(&globalLogLevel, "log-level", "",
		"Set log level (debug, info, warn, error). Overrides setting.json")

	return cmd
}

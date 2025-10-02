package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/app/config"
	infraConfig "github.com/YoshitsuguKoike/deespec/internal/infra/config"
	"github.com/spf13/cobra"
)

// globalConfig holds the loaded configuration for all commands
var globalConfig config.Config

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
				cfg = config.NewAppConfig(
					".deespec", "claude", 60,
					"", "", "", "",
					false, false, false,
					3, 8, // max_attempts=3, max_turns=8
					"", false,
					false, false,
					false, false,
					"", "", "warn", // Default log level
					"default", "",
				)
			}
			globalConfig = cfg

			// Determine log level: CLI flag takes precedence
			logLevel := cfg.StderrLevel()
			if globalLogLevel != "" {
				logLevel = globalLogLevel
			}

			// Initialize global logger with determined level
			InitGlobalLogger(logLevel)

			// Initialize loggers for all layers
			InitializeLoggers(GetLogger())

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newDoctorIntegratedCmd())
	cmd.AddCommand(newJournalCmd())
	cmd.AddCommand(newStateCmd())
	cmd.AddCommand(newHealthCmd())
	cmd.AddCommand(workflowCmd)
	cmd.AddCommand(NewSBICommand())
	cmd.AddCommand(newClearCmd())
	cmd.AddCommand(newCleanupLocksCmd())
	cmd.AddCommand(newLabelCmd())

	// Add global log level flag
	cmd.PersistentFlags().StringVar(&globalLogLevel, "log-level", "",
		"Set log level (debug, info, warn, error). Overrides setting.json")

	return cmd
}

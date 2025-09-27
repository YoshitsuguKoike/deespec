package cli

import (
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
	infraConfig "github.com/YoshitsuguKoike/deespec/internal/infra/config"
	"github.com/spf13/cobra"
)

// globalConfig holds the loaded configuration for all commands
var globalConfig config.Config

func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deespec",
		Short: "DeeSpec CLI",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration before any command runs
			// Priority: setting.json > ENV > defaults
			baseDir := ".deespec"
			if home := os.Getenv("DEE_HOME"); home != "" {
				baseDir = home
			}

			cfg, err := infraConfig.LoadSettings(baseDir)
			if err != nil {
				// Continue with defaults if loading fails
				cfg = config.NewAppConfig(
					".deespec", "claude", 60, ".deespec/var/artifacts",
					"", "", "", "",
					false, false, false,
					"", false, false, false,
					false, false,
					false, false,
					"", "", "",
					"default", "",
				)
			}
			globalConfig = cfg
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
	return cmd
}

package sbi

import (
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/run"
	"github.com/spf13/cobra"
)

// NewSBIRunCommand creates the sbi run command
func NewSBIRunCommand() *cobra.Command {
	var once bool
	var autoFB bool
	var intervalStr string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run SBI workflow continuously",
		Long: `Execute SBI (Spec Backlog Item) workflow steps continuously.
This command processes SBI specifications through implementation and review cycles.

By default, runs continuously until stopped with Ctrl+C.
Use --once for single execution (legacy mode).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check config for auto-fb (config takes precedence over flag)
			if common.GetGlobalConfig() != nil && common.GetGlobalConfig().AutoFB() {
				autoFB = true
			}

			// Legacy mode: single execution
			if once {
				common.Warn("--once flag is deprecated and will be removed in v0.2.0\n")
				return run.RunTurn(autoFB)
			}

			// Parse interval
			interval, err := run.ParseInterval(intervalStr)
			if err != nil {
				return fmt.Errorf("invalid interval: %v", err)
			}

			// Setup signal handling for graceful shutdown
			ctx, cancel := run.SetupSignalHandler()
			defer cancel()

			// Run continuously
			config := run.RunConfig{
				AutoFB:   autoFB,
				Interval: interval,
			}
			return run.RunContinuous(ctx, config)
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "Execute once and exit (DEPRECATED)")
	cmd.Flags().BoolVar(&autoFB, "auto-fb", false, "Automatically register FB-SBI drafts")
	cmd.Flags().StringVar(&intervalStr, "interval", "", "Execution interval (default: 5s, min: 1s, max: 10m)")

	return cmd
}

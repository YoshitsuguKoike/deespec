package clear

import (
	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// NewCommand creates the clear command
func NewCommand() *cobra.Command {
	var prune bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear past instructions by archiving current state",
		Long: `Clear archives all current specifications and journal entries to a
timestamped directory, then resets the system state for new work.

The archive is stored in .deespec/archives/ with a timestamp and ULID.
This operation is blocked if there's a work-in-progress (WIP) task.

Examples:
  # Archive current work and reset state
  deespec clear

  # Archive and also delete all previous archives
  deespec clear --prune`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := app.GetPathsWithConfig(common.GetGlobalConfig())

			opts := ClearOptions{
				Prune: prune,
			}

			return Clear(paths, opts)
		},
	}

	cmd.Flags().BoolVar(&prune, "prune", false, "Delete all archives after confirmation")

	return cmd
}

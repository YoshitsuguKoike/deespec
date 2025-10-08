package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/spf13/cobra"
)

// newCleanupLocksCmd creates the cleanup-locks command
// Deprecated: Use `deespec lock cleanup` instead.
// This command uses the old file-based lock system.
// The new SQLite-based lock system is available via `deespec lock` commands.
func newCleanupLocksCmd() *cobra.Command {
	var showOnly bool

	cmd := &cobra.Command{
		Use:        "cleanup-locks",
		Short:      "[DEPRECATED] Clean up expired lock files (use 'deespec lock cleanup' instead)",
		Deprecated: "Use 'deespec lock cleanup' instead for the new SQLite-based lock system",
		Long: `[DEPRECATED] Clean up expired lock files that may prevent deespec from running.

⚠️  This command uses the old file-based lock system and will be removed in a future version.
    Please use 'deespec lock cleanup' instead for the new SQLite-based lock system.

Clean up expired lock files that may prevent deespec from running.

This command checks for three types of locks:
1. runlock - Process lock file with PID and expiration
2. state.lock - File system lock for state.json access
3. lease - Time-based lease in state.json for WIP tasks

The command will only remove locks that have expired or belong to
dead processes. Active locks are preserved.

Examples:
  # Show current lock status without cleaning
  deespec cleanup-locks --show

  # Clean up expired locks
  deespec cleanup-locks`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := app.GetPathsWithConfig(globalConfig)

			if showOnly {
				return ShowLocks(paths)
			}

			return CleanupLocks(paths)
		},
	}

	cmd.Flags().BoolVar(&showOnly, "show", false, "Show lock status without cleaning")

	return cmd
}

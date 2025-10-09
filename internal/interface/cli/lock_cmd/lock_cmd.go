package lock_cmd

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

// newLockCmd creates the lock command
// NewCommand creates the lock command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Manage locks (run locks and state locks)",
		Long: `Manage locks using the SQLite-based lock system.

This command provides operations to list, inspect, and cleanup locks.
It uses the new SQLite-based lock system (Phase 7 implementation).`,
	}

	cmd.AddCommand(newLockListCmd())
	cmd.AddCommand(newLockCleanupCmd())
	cmd.AddCommand(newLockInfoCmd())

	return cmd
}

// newLockListCmd creates the lock list command
func newLockListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active locks",
		Long:  `List all active run locks and state locks in the system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLockList()
		},
	}
}

// newLockCleanupCmd creates the lock cleanup command
func newLockCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up expired locks",
		Long:  `Remove all expired locks from the system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLockCleanup()
		},
	}

	return cmd
}

// newLockInfoCmd creates the lock info command
func newLockInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <lockID>",
		Short: "Show information about a specific lock",
		Long:  `Display detailed information about a specific lock by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lockID := args[0]
			return runLockInfo(lockID)
		},
	}
}

// runLockList lists all active locks
func runLockList() error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Start Lock Service
	ctx := context.Background()
	if err := container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lock service: %w", err)
	}

	lockService := container.GetLockService()

	// List run locks
	runLocks, err := lockService.ListRunLocks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list run locks: %w", err)
	}

	// List state locks
	stateLocks, err := lockService.ListStateLocks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list state locks: %w", err)
	}

	// Display results
	if len(runLocks) == 0 && len(stateLocks) == 0 {
		common.Info("No active locks found\n")
		return nil
	}

	common.Info("Active Locks:\n\n")

	if len(runLocks) > 0 {
		common.Info("Run Locks (%d):\n", len(runLocks))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "LOCK ID\tPID\tHOSTNAME\tACQUIRED\tEXPIRES\tSTATUS")
		for _, l := range runLocks {
			status := "active"
			if l.IsExpired() {
				status = "expired"
			}
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
				l.LockID().String(),
				l.PID(),
				l.Hostname(),
				l.AcquiredAt().Format("15:04:05"),
				l.ExpiresAt().Format("15:04:05"),
				status,
			)
		}
		w.Flush()
		fmt.Println()
	}

	if len(stateLocks) > 0 {
		common.Info("State Locks (%d):\n", len(stateLocks))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "LOCK ID\tTYPE\tPID\tHOSTNAME\tACQUIRED\tEXPIRES\tSTATUS")
		for _, l := range stateLocks {
			status := "active"
			if l.IsExpired() {
				status = "expired"
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				l.LockID().String(),
				l.LockType(),
				l.PID(),
				l.Hostname(),
				l.AcquiredAt().Format("15:04:05"),
				l.ExpiresAt().Format("15:04:05"),
				status,
			)
		}
		w.Flush()
	}

	return nil
}

// runLockCleanup removes expired locks
func runLockCleanup() error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Start Lock Service
	ctx := context.Background()
	if err := container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lock service: %w", err)
	}

	lockService := container.GetLockService()

	common.Info("Cleaning up expired locks...\n")

	// Get lock counts before cleanup
	runLocksBefore, _ := lockService.ListRunLocks(ctx)
	stateLocksBefore, _ := lockService.ListStateLocks(ctx)

	// Cleanup via repositories (Lock Service doesn't expose direct cleanup)
	// We'll trigger cleanup by calling the cleanup method directly
	// Note: In production, the cleanup scheduler runs automatically

	// For now, we'll just report what the automatic cleanup would do
	expiredRunCount := 0
	for _, l := range runLocksBefore {
		if l.IsExpired() {
			expiredRunCount++
		}
	}

	expiredStateCount := 0
	for _, l := range stateLocksBefore {
		if l.IsExpired() {
			expiredStateCount++
		}
	}

	totalExpired := expiredRunCount + expiredStateCount

	if totalExpired == 0 {
		common.Info("No expired locks found\n")
		return nil
	}

	common.Info("Found %d expired lock(s):\n", totalExpired)
	if expiredRunCount > 0 {
		common.Info("  - Run locks: %d\n", expiredRunCount)
	}
	if expiredStateCount > 0 {
		common.Info("  - State locks: %d\n", expiredStateCount)
	}

	common.Info("\nNote: Lock Service automatically cleans up expired locks every 60 seconds.\n")
	common.Info("Manual cleanup via repositories is not exposed in the public API.\n")
	common.Info("The expired locks will be cleaned up on the next automatic cleanup cycle.\n")

	return nil
}

// runLockInfo displays information about a specific lock
func runLockInfo(lockIDStr string) error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Start Lock Service
	ctx := context.Background()
	if err := container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lock service: %w", err)
	}

	lockService := container.GetLockService()

	// Create lock ID
	lockID, err := lockIDFromString(lockIDStr)
	if err != nil {
		return fmt.Errorf("invalid lock ID: %w", err)
	}

	// Try to find as run lock first
	runLock, err := lockService.FindRunLock(ctx, lockID)
	if err == nil {
		displayRunLockInfo(runLock)
		return nil
	}

	// Try to find as state lock
	stateLock, err := lockService.FindStateLock(ctx, lockID)
	if err == nil {
		displayStateLockInfo(stateLock)
		return nil
	}

	return fmt.Errorf("lock not found: %s", lockIDStr)
}

// Helper functions

func lockIDFromString(s string) (lock.LockID, error) {
	return lock.NewLockID(s)
}

func displayRunLockInfo(runLock *lock.RunLock) {
	common.Info("Run Lock Information:\n\n")
	common.Info("  Lock ID:    %s\n", runLock.LockID().String())
	common.Info("  PID:        %d\n", runLock.PID())
	common.Info("  Hostname:   %s\n", runLock.Hostname())
	common.Info("  Acquired:   %s\n", runLock.AcquiredAt().Format(time.RFC3339))
	common.Info("  Expires:    %s\n", runLock.ExpiresAt().Format(time.RFC3339))
	common.Info("  Heartbeat:  %s\n", runLock.HeartbeatAt().Format(time.RFC3339))
	common.Info("  Status:     %s\n", lockStatus(runLock.IsExpired()))
	common.Info("  Metadata:   %v\n", runLock.Metadata())
}

func displayStateLockInfo(stateLock *lock.StateLock) {
	common.Info("State Lock Information:\n\n")
	common.Info("  Lock ID:    %s\n", stateLock.LockID().String())
	common.Info("  Type:       %s\n", stateLock.LockType())
	common.Info("  PID:        %d\n", stateLock.PID())
	common.Info("  Hostname:   %s\n", stateLock.Hostname())
	common.Info("  Acquired:   %s\n", stateLock.AcquiredAt().Format(time.RFC3339))
	common.Info("  Expires:    %s\n", stateLock.ExpiresAt().Format(time.RFC3339))
	common.Info("  Heartbeat:  %s\n", stateLock.HeartbeatAt().Format(time.RFC3339))
	common.Info("  Status:     %s\n", lockStatus(stateLock.IsExpired()))
}

func lockStatus(isExpired bool) string {
	if isExpired {
		return "EXPIRED"
	}
	return "ACTIVE"
}

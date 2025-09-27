package app

import (
	"context"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
)

// RunStartupRecovery performs transaction recovery at application startup
// This should be called before acquiring any locks (per ARCHITECTURE.md Section 3.7)
func RunStartupRecovery() error {
	// Check if recovery is enabled
	if os.Getenv("DEESPEC_DISABLE_RECOVERY") == "1" {
		fmt.Fprintf(os.Stderr, "INFO: Transaction recovery disabled by environment variable\n")
		return nil
	}

	// Initialize transaction manager
	txnBaseDir := ".deespec/var/txn"
	manager := txn.NewManager(txnBaseDir)

	// Create recovery handler
	recovery := txn.NewRecovery(manager)

	// Perform recovery
	ctx := context.Background()
	result, err := recovery.RecoverAll(ctx)
	if err != nil {
		return fmt.Errorf("startup recovery failed: %w", err)
	}

	// Log results
	if result.RecoveredCount > 0 || result.CleanedCount > 0 {
		fmt.Fprintf(os.Stderr, "INFO: Startup recovery completed: %d recovered, %d cleaned up\n",
			result.RecoveredCount, result.CleanedCount)
	}

	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "WARN: Recovery completed with %d errors\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "  - %v\n", err)
		}
	}

	return nil
}

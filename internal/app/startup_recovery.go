package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
)

// RunStartupRecovery performs transaction recovery at application startup
// This should be called before acquiring any locks (per ARCHITECTURE.md Section 3.7)
// Deprecated: Use RunStartupRecoveryWithConfig instead
func RunStartupRecovery() error {
	return RunStartupRecoveryWithConfig(nil)
}

// RunStartupRecoveryWithConfig performs transaction recovery with configuration
func RunStartupRecoveryWithConfig(cfg config.Config) error {
	// Check if recovery is enabled
	disableRecovery := false
	destRoot := ".deespec"
	txnBaseDir := ".deespec/var/txn"

	if cfg != nil {
		disableRecovery = cfg.DisableRecovery()
		destRoot = cfg.Home()
		txnBaseDir = filepath.Join(cfg.Home(), "var", "txn")
	}

	if disableRecovery {
		fmt.Fprintf(os.Stderr, "INFO: Transaction recovery disabled\n")
		return nil
	}

	// Initialize transaction manager
	manager := txn.NewManager(txnBaseDir)

	// Create recovery handler
	recovery := txn.NewRecovery(manager, destRoot)

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

package app

import (
	"context"
	"fmt"
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
		GetLogger().Info("Transaction recovery disabled")
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
		GetLogger().Info("Startup recovery completed: %d recovered, %d cleaned up",
			result.RecoveredCount, result.CleanedCount)
	}

	if len(result.Errors) > 0 {
		GetLogger().Warn("Recovery completed with %d errors", len(result.Errors))
		for _, err := range result.Errors {
			GetLogger().Warn("  - %v", err)
		}
	}

	return nil
}

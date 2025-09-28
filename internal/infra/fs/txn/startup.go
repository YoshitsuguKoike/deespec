package txn

import (
	"context"
	"fmt"
)

// RunStartupRecovery provides a convenience entrypoint to perform transaction
// recovery for a given transactions base directory. This is intended for use
// by higher layers (e.g., CLI) that want to run forward recovery prior to
// acquiring locks.
//
// The destRoot parameter specifies the destination root directory for recovery.
// The disableRecovery parameter can be used to skip recovery.
func RunStartupRecovery(ctx context.Context, txnBaseDir string, destRoot string, disableRecovery bool) error {
	if disableRecovery {
		GetLogger().Info("Transaction recovery disabled")
		return nil
	}

	manager := NewManager(txnBaseDir)
	recovery := NewRecovery(manager, destRoot)

	if _, err := recovery.RecoverAll(ctx); err != nil {
		return fmt.Errorf("startup recovery failed: %w", err)
	}

	return nil
}

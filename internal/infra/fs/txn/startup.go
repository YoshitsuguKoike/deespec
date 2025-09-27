package txn

import (
	"context"
	"fmt"
	"os"
)

// RunStartupRecovery provides a convenience entrypoint to perform transaction
// recovery for a given transactions base directory. This is intended for use
// by higher layers (e.g., CLI) that want to run forward recovery prior to
// acquiring locks.
//
// If the environment variable DEESPEC_DISABLE_RECOVERY is set to "1",
// recovery is skipped and this returns nil.
func RunStartupRecovery(ctx context.Context, txnBaseDir string) error {
	if os.Getenv("DEESPEC_DISABLE_RECOVERY") == "1" {
		fmt.Fprintf(os.Stderr, "INFO: Transaction recovery disabled by environment variable\n")
		return nil
	}

	manager := NewManager(txnBaseDir)
	recovery := NewRecovery(manager)

	if _, err := recovery.RecoverAll(ctx); err != nil {
		return fmt.Errorf("startup recovery failed: %w", err)
	}

	return nil
}

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
)

// InitializeContainer creates and returns a DI container with default configuration
func InitializeContainer() (*di.Container, error) {
	var dbPath string

	// Check if we're in a local .deespec directory (e.g., in tests)
	// This allows tests to run in temporary directories
	localDeespecDir := filepath.Join(".", ".deespec")
	if stat, err := os.Stat(localDeespecDir); err == nil && stat.IsDir() {
		// Use local .deespec directory if it exists
		dbPath = filepath.Join(localDeespecDir, "deespec.db")
	} else {
		// Otherwise use global home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, ".deespec", "deespec.db")
	}

	// Create container config
	config := di.Config{
		DBPath:                dbPath,
		StorageType:           "local",
		LockHeartbeatInterval: 30 * time.Second,
		LockCleanupInterval:   60 * time.Second,
	}

	return di.NewContainer(config)
}

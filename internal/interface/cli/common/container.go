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
	// Get database path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(homeDir, ".deespec", "deespec.db")

	// Create container config
	config := di.Config{
		DBPath:                dbPath,
		StorageType:           "local",
		LockHeartbeatInterval: 30 * time.Second,
		LockCleanupInterval:   60 * time.Second,
	}

	return di.NewContainer(config)
}

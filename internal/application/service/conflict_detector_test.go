package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// createTestSBIForConflict creates a test SBI with specified file paths
func createTestSBIForConflict(id string, filePaths []string) *sbi.SBI {
	title := fmt.Sprintf("Test SBI %s", id)
	description := fmt.Sprintf("Description for %s", id)

	metadata := sbi.SBIMetadata{
		EstimatedHours: 1.0,
		Priority:       0,
		Sequence:       1,
		RegisteredAt:   time.Now(),
		Labels:         []string{},
		AssignedAgent:  "claude-code",
		FilePaths:      filePaths,
	}

	s, _ := sbi.NewSBI(title, description, nil, metadata)

	// Set specific ID using reconstruction
	taskID, _ := model.NewTaskIDFromString(id)
	s = sbi.ReconstructSBI(
		taskID,
		title,
		description,
		model.StatusPending,
		model.StepPick,
		nil,
		metadata,
		s.ExecutionState(),
		time.Now(),
		time.Now(),
	)

	return s
}

func TestConflictDetector_NewConflictDetector(t *testing.T) {
	detector := NewConflictDetector()
	require.NotNil(t, detector)
	assert.Equal(t, 0, detector.GetActiveFileCount())
}

func TestConflictDetector_RegisterAndUnregister(t *testing.T) {
	detector := NewConflictDetector()

	// Create SBI with file paths
	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
		"internal/domain/model/user_test.go",
	})

	// Register SBI
	detector.Register(sbi1)

	// Verify files are registered
	assert.Equal(t, 2, detector.GetActiveFileCount())
	activeFiles := detector.GetActiveFiles()
	assert.Equal(t, "SBI-001", activeFiles["internal/domain/model/user.go"])
	assert.Equal(t, "SBI-001", activeFiles["internal/domain/model/user_test.go"])

	// Unregister SBI
	detector.Unregister(sbi1)

	// Verify files are unregistered
	assert.Equal(t, 0, detector.GetActiveFileCount())
}

func TestConflictDetector_HasConflict_NoConflict(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
	})

	sbi2 := createTestSBIForConflict("SBI-002", []string{
		"internal/domain/model/product.go",
	})

	// Register first SBI
	detector.Register(sbi1)

	// Check second SBI - should have no conflict
	hasConflict := detector.HasConflict(sbi2)
	assert.False(t, hasConflict, "SBI-002 should not conflict with SBI-001")
}

func TestConflictDetector_HasConflict_WithConflict(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
		"internal/domain/model/user_test.go",
	})

	sbi2 := createTestSBIForConflict("SBI-002", []string{
		"internal/domain/model/user.go", // Same file as SBI-001
		"internal/domain/model/product.go",
	})

	// Register first SBI
	detector.Register(sbi1)

	// Check second SBI - should have conflict
	hasConflict := detector.HasConflict(sbi2)
	assert.True(t, hasConflict, "SBI-002 should conflict with SBI-001 on user.go")
}

func TestConflictDetector_HasConflict_SameSBI(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
	})

	// Register SBI
	detector.Register(sbi1)

	// Check same SBI - should have no conflict with itself
	hasConflict := detector.HasConflict(sbi1)
	assert.False(t, hasConflict, "SBI should not conflict with itself")
}

func TestConflictDetector_GetConflictingSBIID(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
	})

	// Initially no conflict
	conflictingID := detector.GetConflictingSBIID("internal/domain/model/user.go")
	assert.Equal(t, "", conflictingID)

	// Register SBI
	detector.Register(sbi1)

	// Now should return SBI-001
	conflictingID = detector.GetConflictingSBIID("internal/domain/model/user.go")
	assert.Equal(t, "SBI-001", conflictingID)

	// Unregister
	detector.Unregister(sbi1)

	// Should return empty again
	conflictingID = detector.GetConflictingSBIID("internal/domain/model/user.go")
	assert.Equal(t, "", conflictingID)
}

func TestConflictDetector_Clear(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
		"internal/domain/model/product.go",
	})

	sbi2 := createTestSBIForConflict("SBI-002", []string{
		"internal/domain/model/order.go",
	})

	// Register multiple SBIs
	detector.Register(sbi1)
	detector.Register(sbi2)

	assert.Equal(t, 3, detector.GetActiveFileCount())

	// Clear all
	detector.Clear()

	assert.Equal(t, 0, detector.GetActiveFileCount())
}

func TestConflictDetector_ConcurrentAccess(t *testing.T) {
	detector := NewConflictDetector()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrently register and unregister SBIs
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			sbiID := fmt.Sprintf("SBI-%03d", index)
			filePath := fmt.Sprintf("file_%d.go", index)

			testSBI := createTestSBIForConflict(sbiID, []string{filePath})

			// Register
			detector.Register(testSBI)

			// Check conflict (should be false for different files)
			_ = detector.HasConflict(testSBI)

			// Get active files
			_ = detector.GetActiveFiles()

			// Unregister
			detector.Unregister(testSBI)
		}(i)
	}

	wg.Wait()

	// After all goroutines complete, should have 0 active files
	assert.Equal(t, 0, detector.GetActiveFileCount())
}

func TestConflictDetector_MultipleFilesConflict(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"internal/domain/model/user.go",
		"internal/domain/model/product.go",
		"internal/domain/model/order.go",
	})

	sbi2 := createTestSBIForConflict("SBI-002", []string{
		"internal/domain/model/payment.go",
		"internal/domain/model/invoice.go",
	})

	sbi3 := createTestSBIForConflict("SBI-003", []string{
		"internal/domain/model/shipping.go",
		"internal/domain/model/order.go", // Conflicts with SBI-001
	})

	// Register SBI-001
	detector.Register(sbi1)
	assert.False(t, detector.HasConflict(sbi2), "SBI-002 should not conflict")
	assert.True(t, detector.HasConflict(sbi3), "SBI-003 should conflict on order.go")

	// Register SBI-002
	detector.Register(sbi2)
	assert.False(t, detector.HasConflict(sbi2), "SBI-002 should not conflict with itself")

	// Verify active files count
	assert.Equal(t, 5, detector.GetActiveFileCount(), "Should have 5 active files")
}

func TestConflictDetector_UnregisterPartial(t *testing.T) {
	detector := NewConflictDetector()

	sbi1 := createTestSBIForConflict("SBI-001", []string{
		"file1.go",
		"file2.go",
	})

	sbi2 := createTestSBIForConflict("SBI-002", []string{
		"file2.go", // Register same file with different SBI
		"file3.go",
	})

	// Register SBI-001
	detector.Register(sbi1)
	assert.Equal(t, 2, detector.GetActiveFileCount())

	// Try to register SBI-002 (would conflict on file2.go)
	// In real scenario, this should be prevented by HasConflict check
	// But for testing, let's see what happens
	detector.Register(sbi2)

	// file2.go should be overwritten to SBI-002
	// This demonstrates why HasConflict should be checked before Register
	assert.Equal(t, "SBI-002", detector.GetConflictingSBIID("file2.go"))

	// Unregister SBI-001
	detector.Unregister(sbi1)

	// file1.go should be removed, but file2.go is owned by SBI-002 so it stays
	assert.Equal(t, "", detector.GetConflictingSBIID("file1.go"))
	assert.Equal(t, "SBI-002", detector.GetConflictingSBIID("file2.go"))
	assert.Equal(t, "SBI-002", detector.GetConflictingSBIID("file3.go"))
}

func TestConflictDetector_EmptyFilePaths(t *testing.T) {
	detector := NewConflictDetector()

	// Create SBI with no file paths
	sbiEmpty := createTestSBIForConflict("SBI-EMPTY", []string{})

	// Should not have conflicts
	assert.False(t, detector.HasConflict(sbiEmpty))

	// Register should succeed without adding anything
	detector.Register(sbiEmpty)
	assert.Equal(t, 0, detector.GetActiveFileCount())

	// Unregister should succeed without doing anything
	detector.Unregister(sbiEmpty)
	assert.Equal(t, 0, detector.GetActiveFileCount())
}

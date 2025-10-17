package clear

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/oklog/ulid/v2"
)

// ClearOptions represents options for the clear command
type ClearOptions struct {
	Prune bool // If true, delete all archives after confirmation
}

// Clear clears past instructions by archiving current state
func Clear(paths app.Paths, opts ClearOptions) error {
	// 1. Check for WIP (Work In Progress)
	if err := checkNoWIP(); err != nil {
		return err
	}

	// 2. Create archive directory with timestamp + ULID
	archiveDir, err := createArchiveDirectory()
	if err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	common.Info("Creating archive at: %s\n", archiveDir)

	// 3. Archive journal.ndjson (copy, not move)
	if err := archiveJournal(paths.Journal, archiveDir); err != nil {
		return fmt.Errorf("failed to archive journal: %w", err)
	}

	// 4. Move all files from .deespec/specs to archive
	if err := archiveSpecs(archiveDir); err != nil {
		return fmt.Errorf("failed to archive specs: %w", err)
	}

	// 5. Clear database (physical deletion)
	if err := clearDatabase(); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	// 6. Reset state files
	if err := resetStateFiles(paths); err != nil {
		return fmt.Errorf("failed to reset state files: %w", err)
	}

	// 7. Handle --prune option if specified
	if opts.Prune {
		if err := pruneArchives(); err != nil {
			return fmt.Errorf("failed to prune archives: %w", err)
		}
	}

	common.Info("Clear completed successfully. Archive created at: %s\n", archiveDir)
	return nil
}

// checkNoWIP ensures there's no work in progress
func checkNoWIP() error {
	// Initialize container to access DB and LockService
	container, err := common.InitializeContainer()
	if err != nil {
		// If we can't initialize, allow clear with warning
		common.Warn("Failed to initialize container, allowing clear to proceed: %v\n", err)
		return nil
	}
	defer container.Close()

	ctx := context.Background()

	// Check for active SBIs in the database
	sbiRepo := container.GetSBIRepository()
	filter := repository.SBIFilter{
		Statuses: []model.Status{
			model.StatusPicked,
			model.StatusImplementing,
			model.StatusReviewing,
		},
		Limit:  10,
		Offset: 0,
	}
	sbis, err := sbiRepo.List(ctx, filter)
	if err != nil {
		// If we can't query, allow clear with warning
		common.Warn("Failed to query SBI database, allowing clear to proceed: %v\n", err)
	} else if len(sbis) > 0 {
		// Found active SBIs
		sbiIDs := make([]string, len(sbis))
		for i, sbi := range sbis {
			sbiIDs[i] = sbi.ID().String()
		}
		return fmt.Errorf("cannot clear: %d active SBI(s) in progress: %v", len(sbis), sbiIDs)
	}

	// Check for active locks in LockService
	lockService := container.GetLockService()

	// Check for system runlock
	runlockID, _ := lock.NewLockID("system-runlock")
	if runLock, err := lockService.FindRunLock(ctx, runlockID); err == nil && runLock != nil {
		if !runLock.IsExpired() {
			return fmt.Errorf("cannot clear: active runlock exists (PID %d, expires: %s)",
				runLock.PID(), runLock.ExpiresAt().Format("15:04:05"))
		}
	}

	common.Info("No active work in progress, safe to proceed with clear\n")
	return nil
}

// createArchiveDirectory creates archive directory with timestamp + ULID
func createArchiveDirectory() (string, error) {
	// Generate ULID
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)

	// Format: archives/2006-01-02T15-04-05_ULID
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	dirName := fmt.Sprintf("%s_%s", timestamp, id.String())

	archiveDir := filepath.Join(".deespec", "archives", dirName)

	// Create directory
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}

	return archiveDir, nil
}

// archiveJournal copies journal.ndjson to archive
func archiveJournal(journalPath, archiveDir string) error {
	// Check if journal exists
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		common.Info("No journal.ndjson to archive\n")
		return nil
	}

	// Open source file
	src, err := os.Open(journalPath)
	if err != nil {
		return fmt.Errorf("failed to open journal: %w", err)
	}
	defer src.Close()

	// Create destination file
	dstPath := filepath.Join(archiveDir, "journal.ndjson")
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create archive journal: %w", err)
	}
	defer dst.Close()

	// Copy content
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy journal: %w", err)
	}

	common.Info("Archived journal.ndjson\n")

	// Clear original journal (truncate to 0 bytes)
	if err := os.Truncate(journalPath, 0); err != nil {
		return fmt.Errorf("failed to clear journal: %w", err)
	}

	return nil
}

// archiveSpecs moves all files from .deespec/specs to archive
func archiveSpecs(archiveDir string) error {
	specsDir := filepath.Join(".deespec", "specs")

	// Check if specs directory exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		common.Info("No specs directory to archive\n")
		return nil
	}

	// Create specs directory in archive
	archiveSpecsDir := filepath.Join(archiveDir, "specs")
	if err := os.MkdirAll(archiveSpecsDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive specs dir: %w", err)
	}

	// Move all contents
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return fmt.Errorf("failed to read specs directory: %w", err)
	}

	movedCount := 0
	for _, entry := range entries {
		srcPath := filepath.Join(specsDir, entry.Name())
		dstPath := filepath.Join(archiveSpecsDir, entry.Name())

		// Move file or directory
		if err := os.Rename(srcPath, dstPath); err != nil {
			// If cross-device, fall back to copy and delete
			if err := copyAndDelete(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to move %s: %w", entry.Name(), err)
			}
		}
		movedCount++
	}

	common.Info("Moved %d items from specs to archive\n", movedCount)
	return nil
}

// copyAndDelete copies a file/directory and then deletes the original
func copyAndDelete(src, dst string) error {
	// Get file info
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Recursive directory copy
		if err := copyDir(src, dst); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}

	// File copy
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// clearDatabase clears all task data from the database (physical deletion)
func clearDatabase() error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Get database connection
	db := container.GetDB()
	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete task-related data (in correct order due to foreign keys)
	tables := []string{
		"task_labels", // Task-label associations
		"epic_pbis",   // Epic-PBI associations
		"sbis",        // SBI tasks
		"pbis",        // PBI tasks
		"epics",       // EPIC tasks
		"run_locks",   // Run locks
		"state_locks", // State locks
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s", table)
		if _, err := tx.ExecContext(ctx, query); err != nil {
			// If table doesn't exist, skip it
			if strings.Contains(err.Error(), "no such table") {
				common.Warn("Table %s does not exist, skipping\n", table)
				continue
			}
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
		common.Info("Cleared table: %s\n", table)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	common.Info("Database cleared successfully\n")
	return nil
}

// resetStateFiles resets state.json and health.json
func resetStateFiles(paths app.Paths) error {
	// Delete state.json if it exists (DB-based state management is now preferred)
	if err := os.Remove(paths.State); err != nil && !os.IsNotExist(err) {
		common.Warn("Failed to remove state.json: %v\n", err)
	}

	// Reset health.json
	healthData := []byte(`{"status":"ok","message":"cleared","updated_at":"` +
		time.Now().UTC().Format(time.RFC3339) + `"}`)
	if err := os.WriteFile(paths.Health, healthData, 0644); err != nil {
		// Non-critical, just warn
		common.Warn("Failed to reset health.json: %v\n", err)
	}

	// Clear any notes files
	notesPattern := filepath.Join(paths.Var, "*_notes.md")
	if matches, _ := filepath.Glob(notesPattern); len(matches) > 0 {
		for _, match := range matches {
			os.Remove(match)
		}
	}

	common.Info("Reset state files\n")
	return nil
}

// pruneArchives deletes all archives after user confirmation
func pruneArchives() error {
	archivesDir := filepath.Join(".deespec", "archives")

	// Check if archives directory exists
	if _, err := os.Stat(archivesDir); os.IsNotExist(err) {
		common.Info("No archives to prune\n")
		return nil
	}

	// List archives
	entries, err := os.ReadDir(archivesDir)
	if err != nil {
		return fmt.Errorf("failed to read archives directory: %w", err)
	}

	if len(entries) == 0 {
		common.Info("No archives to prune\n")
		return nil
	}

	// Show what will be deleted
	fmt.Println("\n⚠️  WARNING: The following archives will be permanently deleted:")
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("  - %s\n", entry.Name())
		}
	}

	// Ask for confirmation
	fmt.Print("\nAre you sure you want to delete all archives? Type 'Yes' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.TrimSpace(response)
	if response != "Yes" {
		common.Info("Prune cancelled by user\n")
		return nil
	}

	// Delete all archives
	deletedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			archivePath := filepath.Join(archivesDir, entry.Name())
			if err := os.RemoveAll(archivePath); err != nil {
				common.Warn("Failed to delete archive %s: %v\n", entry.Name(), err)
			} else {
				deletedCount++
			}
		}
	}

	common.Info("Deleted %d archives\n", deletedCount)
	return nil
}

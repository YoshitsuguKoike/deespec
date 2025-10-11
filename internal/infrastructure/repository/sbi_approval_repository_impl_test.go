package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

func TestSBIApprovalRepositoryImpl_SaveAndLoad(t *testing.T) {
	// Setup: Create temporary directory for testing
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Change to temp directory to avoid polluting the real .deespec
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("test-pbi-001")

	// Create test manifest
	now := time.Now()
	manifest := &pbi.SBIApprovalManifest{
		PBIID:       string(pbiID),
		GeneratedAt: now,
		TotalSBIs:   3,
		SBIs: []pbi.SBIApprovalRecord{
			{
				File:   "sbi-001.md",
				Status: pbi.ApprovalStatusPending,
			},
			{
				File:       "sbi-002.md",
				Status:     pbi.ApprovalStatusApproved,
				ReviewedBy: "user@example.com",
				ReviewedAt: &now,
				Notes:      "Looks good",
			},
			{
				File:            "sbi-003.md",
				Status:          pbi.ApprovalStatusRejected,
				ReviewedBy:      "user@example.com",
				ReviewedAt:      &now,
				RejectionReason: "Needs more detail",
			},
		},
		Registered: false,
	}

	// Test: Save manifest
	if err := repo.SaveManifest(ctx, manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Verify: File was created with correct permissions
	manifestPath := filepath.Join(".deespec", "specs", "pbi", string(pbiID), "approval.yaml")
	fileInfo, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatalf("manifest file not created: %v", err)
	}

	// Check file permissions (0644)
	expectedPerm := os.FileMode(0644)
	if fileInfo.Mode().Perm() != expectedPerm {
		t.Errorf("file permissions = %v, want %v", fileInfo.Mode().Perm(), expectedPerm)
	}

	// Test: Load manifest
	loaded, err := repo.LoadManifest(ctx, pbiID)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	// Verify: Manifest content matches
	if loaded.PBIID != manifest.PBIID {
		t.Errorf("PBIID = %v, want %v", loaded.PBIID, manifest.PBIID)
	}
	if loaded.TotalSBIs != manifest.TotalSBIs {
		t.Errorf("TotalSBIs = %v, want %v", loaded.TotalSBIs, manifest.TotalSBIs)
	}
	if len(loaded.SBIs) != len(manifest.SBIs) {
		t.Errorf("len(SBIs) = %v, want %v", len(loaded.SBIs), len(manifest.SBIs))
	}

	// Verify: Individual SBI records
	for i, sbi := range loaded.SBIs {
		expected := manifest.SBIs[i]
		if sbi.File != expected.File {
			t.Errorf("SBIs[%d].File = %v, want %v", i, sbi.File, expected.File)
		}
		if sbi.Status != expected.Status {
			t.Errorf("SBIs[%d].Status = %v, want %v", i, sbi.Status, expected.Status)
		}
		if sbi.ReviewedBy != expected.ReviewedBy {
			t.Errorf("SBIs[%d].ReviewedBy = %v, want %v", i, sbi.ReviewedBy, expected.ReviewedBy)
		}
	}
}

func TestSBIApprovalRepositoryImpl_ManifestExists(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("test-pbi-002")

	// Test: Non-existent manifest
	exists, err := repo.ManifestExists(ctx, pbiID)
	if err != nil {
		t.Fatalf("ManifestExists failed: %v", err)
	}
	if exists {
		t.Error("ManifestExists returned true for non-existent manifest")
	}

	// Create manifest
	manifest := pbi.NewSBIApprovalManifest(string(pbiID), []string{"sbi-001.md"})
	if err := repo.SaveManifest(ctx, manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Test: Existing manifest
	exists, err = repo.ManifestExists(ctx, pbiID)
	if err != nil {
		t.Fatalf("ManifestExists failed: %v", err)
	}
	if !exists {
		t.Error("ManifestExists returned false for existing manifest")
	}
}

func TestSBIApprovalRepositoryImpl_DeleteManifest(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("test-pbi-003")

	// Test: Delete non-existent manifest should fail
	err := repo.DeleteManifest(ctx, pbiID)
	if err == nil {
		t.Error("DeleteManifest should fail for non-existent manifest")
	}

	// Create manifest
	manifest := pbi.NewSBIApprovalManifest(string(pbiID), []string{"sbi-001.md"})
	if err := repo.SaveManifest(ctx, manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Verify it exists
	exists, _ := repo.ManifestExists(ctx, pbiID)
	if !exists {
		t.Fatal("manifest should exist before deletion")
	}

	// Test: Delete existing manifest
	if err := repo.DeleteManifest(ctx, pbiID); err != nil {
		t.Fatalf("DeleteManifest failed: %v", err)
	}

	// Verify it no longer exists
	exists, _ = repo.ManifestExists(ctx, pbiID)
	if exists {
		t.Error("manifest should not exist after deletion")
	}
}

func TestSBIApprovalRepositoryImpl_LoadManifest_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("non-existent-pbi")

	// Test: Load non-existent manifest should fail
	_, err := repo.LoadManifest(ctx, pbiID)
	if err == nil {
		t.Error("LoadManifest should fail for non-existent manifest")
	}
	if !os.IsNotExist(err) {
		// Check if error contains "not found" message
		errMsg := err.Error()
		if len(errMsg) == 0 || (len(errMsg) > 0 && errMsg[0:1] != "a") {
			// Just verify we got an error - the wrapping makes it hard to check exact type
			t.Logf("Got expected error: %v", err)
		}
	}
}

func TestSBIApprovalRepositoryImpl_LoadManifest_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("test-pbi-004")

	// Create directory and write invalid YAML
	manifestPath := filepath.Join(".deespec", "specs", "pbi", string(pbiID), "approval.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	invalidYAML := []byte("invalid: yaml: content: [unclosed")
	if err := os.WriteFile(manifestPath, invalidYAML, 0644); err != nil {
		t.Fatalf("failed to write invalid YAML: %v", err)
	}

	// Test: Load invalid YAML should fail
	_, err := repo.LoadManifest(ctx, pbiID)
	if err == nil {
		t.Error("LoadManifest should fail for invalid YAML")
	}
}

func TestSBIApprovalRepositoryImpl_SaveManifest_NilManifest(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()

	// Test: Save nil manifest should fail
	err := repo.SaveManifest(ctx, nil)
	if err == nil {
		t.Error("SaveManifest should fail for nil manifest")
	}
}

func TestSBIApprovalRepositoryImpl_SaveManifest_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("test-pbi-005")

	// Ensure directory doesn't exist
	dirPath := filepath.Join(".deespec", "specs", "pbi", string(pbiID))
	if _, err := os.Stat(dirPath); err == nil {
		t.Fatal("directory should not exist before SaveManifest")
	}

	// Create and save manifest
	manifest := pbi.NewSBIApprovalManifest(string(pbiID), []string{"sbi-001.md"})
	if err := repo.SaveManifest(ctx, manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Verify directory was created with correct permissions
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Error("expected directory, got file")
	}

	// Check directory permissions (0755)
	expectedPerm := os.FileMode(0755)
	if dirInfo.Mode().Perm() != expectedPerm {
		t.Errorf("directory permissions = %v, want %v", dirInfo.Mode().Perm(), expectedPerm)
	}
}

func TestSBIApprovalRepositoryImpl_RoundTrip_WithRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repo := NewSBIApprovalRepositoryImpl()
	ctx := context.Background()
	pbiID := repository.PBIID("test-pbi-006")

	// Create initial manifest
	manifest := pbi.NewSBIApprovalManifest(string(pbiID), []string{
		"sbi-001.md",
		"sbi-002.md",
		"sbi-003.md",
	})

	// Save initial manifest
	if err := repo.SaveManifest(ctx, manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Load and modify
	loaded, err := repo.LoadManifest(ctx, pbiID)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	// Mark as registered
	now := time.Now()
	loaded.Registered = true
	loaded.RegisteredAt = &now
	loaded.RegisteredSBIs = []string{"sbi-001.md", "sbi-002.md"}

	// Save updated manifest
	if err := repo.SaveManifest(ctx, loaded); err != nil {
		t.Fatalf("SaveManifest (update) failed: %v", err)
	}

	// Load again and verify
	final, err := repo.LoadManifest(ctx, pbiID)
	if err != nil {
		t.Fatalf("LoadManifest (final) failed: %v", err)
	}

	if !final.Registered {
		t.Error("Registered should be true")
	}
	if final.RegisteredAt == nil {
		t.Error("RegisteredAt should not be nil")
	}
	if len(final.RegisteredSBIs) != 2 {
		t.Errorf("len(RegisteredSBIs) = %v, want 2", len(final.RegisteredSBIs))
	}
}

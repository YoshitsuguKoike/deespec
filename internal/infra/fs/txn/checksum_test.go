package txn

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestChecksumCalculation(t *testing.T) {
	testData := []byte("hello world checksum test")

	// Test SHA256 checksum calculation
	checksum, err := CalculateDataChecksum(testData, ChecksumSHA256)
	if err != nil {
		t.Fatalf("CalculateDataChecksum failed: %v", err)
	}

	// Verify checksum properties
	if checksum.Algorithm != ChecksumSHA256 {
		t.Errorf("Expected algorithm %s, got %s", ChecksumSHA256, checksum.Algorithm)
	}

	if checksum.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), checksum.Size)
	}

	// SHA256 produces 64 hex characters
	if len(checksum.Value) != 64 {
		t.Errorf("Expected 64 character hex string, got %d characters: %s", len(checksum.Value), checksum.Value)
	}
}

func TestChecksumValidation(t *testing.T) {
	testData := []byte("validation test data")

	// Calculate original checksum
	original, err := CalculateDataChecksum(testData, ChecksumSHA256)
	if err != nil {
		t.Fatalf("CalculateDataChecksum failed: %v", err)
	}

	// Test valid data validation
	err = ValidateDataChecksum(testData, original)
	if err != nil {
		t.Errorf("Valid data should pass validation: %v", err)
	}

	// Test invalid data validation
	modifiedData := []byte("modified validation test data")
	err = ValidateDataChecksum(modifiedData, original)
	if err == nil {
		t.Error("Modified data should fail validation")
	}

	// Test size mismatch
	shorterData := []byte("short")
	err = ValidateDataChecksum(shorterData, original)
	if err == nil {
		t.Error("Data with different size should fail validation")
	}
}

func TestFileChecksumValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "checksum_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("file checksum test content")

	// Write test file
	err = os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Calculate file checksum
	checksum, err := CalculateFileChecksum(testFile, ChecksumSHA256)
	if err != nil {
		t.Fatalf("CalculateFileChecksum failed: %v", err)
	}

	// Validate file checksum
	err = ValidateFileChecksum(testFile, checksum)
	if err != nil {
		t.Errorf("File should pass checksum validation: %v", err)
	}

	// Modify file and test validation failure
	modifiedData := []byte("modified file content")
	err = os.WriteFile(testFile, modifiedData, 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	err = ValidateFileChecksum(testFile, checksum)
	if err == nil {
		t.Error("Modified file should fail checksum validation")
	}
}

func TestTransactionChecksumIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_checksum_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	destRoot := filepath.Join(tempDir, ".deespec")

	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage file with checksum calculation
	testContent := []byte("transaction checksum test content")
	err = manager.StageFile(tx, "checksum_test.txt", testContent)
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Verify checksum info was stored
	if len(tx.Manifest.Files) != 1 {
		t.Fatalf("Expected 1 file operation, got %d", len(tx.Manifest.Files))
	}

	op := tx.Manifest.Files[0]
	if op.ChecksumInfo == nil {
		t.Error("ChecksumInfo should be populated")
	}

	if op.Checksum == "" {
		t.Error("Legacy checksum field should be populated")
	}

	if op.ChecksumInfo.Algorithm != ChecksumSHA256 {
		t.Errorf("Expected SHA256 algorithm, got %s", op.ChecksumInfo.Algorithm)
	}

	if op.ChecksumInfo.Size != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %d", len(testContent), op.ChecksumInfo.Size)
	}

	// Mark intent
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Commit with checksum validation
	err = manager.Commit(tx, destRoot, nil)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify final file exists and has correct content
	finalPath := filepath.Join(destRoot, "checksum_test.txt")
	finalContent, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("Failed to read final file: %v", err)
	}

	if string(finalContent) != string(testContent) {
		t.Errorf("Final file content mismatch")
	}

	// Verify final file checksum
	finalChecksum, err := CalculateFileChecksum(finalPath, ChecksumSHA256)
	if err != nil {
		t.Fatalf("Failed to calculate final checksum: %v", err)
	}

	if finalChecksum.Value != op.ChecksumInfo.Value {
		t.Errorf("Final file checksum mismatch: expected %s, got %s",
			op.ChecksumInfo.Value, finalChecksum.Value)
	}
}

func TestChecksumCorruptionDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "corruption_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage file
	testContent := []byte("corruption detection test")
	err = manager.StageFile(tx, "corrupt_test.txt", testContent)
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Corrupt the staged file
	stagePath := filepath.Join(tx.StageDir, "corrupt_test.txt")
	corruptedContent := []byte("corrupted content")
	err = os.WriteFile(stagePath, corruptedContent, 0644)
	if err != nil {
		t.Fatalf("Failed to corrupt staged file: %v", err)
	}

	// Mark intent
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Commit should fail due to checksum mismatch
	err = manager.Commit(tx, tempDir, nil)
	if err == nil {
		t.Error("Commit should fail when staged file is corrupted")
	}

	expectedError := "staged file checksum validation failed"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestUnsupportedChecksumAlgorithm(t *testing.T) {
	testData := []byte("algorithm test")

	_, err := CalculateDataChecksum(testData, "md5")
	if err == nil {
		t.Error("Unsupported algorithm should return error")
	}

	expectedError := "unsupported checksum algorithm"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestCompareFileChecksums(t *testing.T) {
	checksum1 := &FileChecksum{
		Algorithm: ChecksumSHA256,
		Value:     "abc123",
		Size:      100,
	}

	checksum2 := &FileChecksum{
		Algorithm: ChecksumSHA256,
		Value:     "abc123",
		Size:      100,
	}

	checksum3 := &FileChecksum{
		Algorithm: ChecksumSHA256,
		Value:     "def456",
		Size:      100,
	}

	// Test equal checksums
	if !CompareFileChecksums(checksum1, checksum2) {
		t.Error("Identical checksums should be equal")
	}

	// Test different checksums
	if CompareFileChecksums(checksum1, checksum3) {
		t.Error("Different checksums should not be equal")
	}

	// Test nil checksums
	if !CompareFileChecksums(nil, nil) {
		t.Error("Two nil checksums should be equal")
	}

	if CompareFileChecksums(checksum1, nil) {
		t.Error("Checksum and nil should not be equal")
	}
}

// Note: contains helper function is defined in recovery_test.go and available package-wide

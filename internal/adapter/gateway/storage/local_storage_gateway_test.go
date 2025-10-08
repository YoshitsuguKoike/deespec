package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageGateway_SaveAndLoadArtifact(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Test data
	taskID := "test-task-001"
	content := []byte("test artifact content")

	// Save artifact
	req := output.SaveArtifactRequest{
		TaskID:       taskID,
		ArtifactType: output.ArtifactTypeCode,
		Content:      content,
		ContentType:  "text/plain",
		Metadata: map[string]string{
			"language": "go",
			"file":     "main.go",
		},
	}

	metadata, err := gateway.SaveArtifact(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, metadata.ID)
	assert.Equal(t, taskID, metadata.TaskID)
	assert.Equal(t, output.ArtifactTypeCode, metadata.Type)
	assert.Equal(t, int64(len(content)), metadata.Size)
	assert.Equal(t, "text/plain", metadata.ContentType)

	// Load artifact
	artifact, err := gateway.LoadArtifact(ctx, metadata.ID)
	require.NoError(t, err)
	assert.Equal(t, metadata.ID, artifact.ID)
	assert.Equal(t, content, artifact.Content)
	assert.Equal(t, taskID, artifact.Metadata.TaskID)
	assert.Equal(t, "go", artifact.Metadata.Metadata["language"])
}

func TestLocalStorageGateway_ListArtifacts(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	ctx := context.Background()
	taskID := "test-task-002"

	// Save multiple artifacts
	for i := 0; i < 3; i++ {
		req := output.SaveArtifactRequest{
			TaskID:       taskID,
			ArtifactType: output.ArtifactTypeCode,
			Content:      []byte("content " + string(rune('A'+i))),
			ContentType:  "text/plain",
		}
		_, err := gateway.SaveArtifact(ctx, req)
		require.NoError(t, err)
	}

	// List artifacts
	metadataList, err := gateway.ListArtifacts(ctx, taskID)
	require.NoError(t, err)
	assert.Len(t, metadataList, 3)

	// Verify all artifacts belong to the same task
	for _, metadata := range metadataList {
		assert.Equal(t, taskID, metadata.TaskID)
	}
}

func TestLocalStorageGateway_ListArtifacts_EmptyTask(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	ctx := context.Background()

	// List artifacts for non-existent task
	metadataList, err := gateway.ListArtifacts(ctx, "non-existent-task")
	require.NoError(t, err)
	assert.Empty(t, metadataList)
}

func TestLocalStorageGateway_LoadInstruction_AbsolutePath(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create instruction file
	instructionContent := "# Test Instruction\n\nThis is a test instruction."
	instructionPath := filepath.Join(tempDir, "test-instruction.md")
	err = os.WriteFile(instructionPath, []byte(instructionContent), 0644)
	require.NoError(t, err)

	// Load instruction with absolute path
	loadedContent, err := gateway.LoadInstruction(ctx, instructionPath)
	require.NoError(t, err)
	assert.Equal(t, instructionContent, loadedContent)
}

func TestLocalStorageGateway_LoadInstruction_RelativePath(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create instruction file
	instructionContent := "# Test Instruction\n\nRelative path test."
	instructionPath := "instructions/test.md"
	fullPath := filepath.Join(tempDir, instructionPath)
	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(instructionContent), 0644)
	require.NoError(t, err)

	// Load instruction with relative path
	loadedContent, err := gateway.LoadInstruction(ctx, instructionPath)
	require.NoError(t, err)
	assert.Equal(t, instructionContent, loadedContent)
}

func TestLocalStorageGateway_DeleteArtifact(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	ctx := context.Background()
	taskID := "test-task-003"

	// Save artifact
	req := output.SaveArtifactRequest{
		TaskID:       taskID,
		ArtifactType: output.ArtifactTypeCode,
		Content:      []byte("to be deleted"),
		ContentType:  "text/plain",
	}
	metadata, err := gateway.SaveArtifact(ctx, req)
	require.NoError(t, err)

	// Verify artifact exists
	_, err = gateway.LoadArtifact(ctx, metadata.ID)
	require.NoError(t, err)

	// Delete artifact
	err = gateway.DeleteArtifact(ctx, taskID, metadata.ID)
	require.NoError(t, err)

	// Verify artifact is deleted
	_, err = gateway.LoadArtifact(ctx, metadata.ID)
	assert.Error(t, err)
}

func TestLocalStorageGateway_EnsureInstructionFile(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	instructionPath := "instructions/new-instruction.md"
	instructionContent := "# New Instruction\n\nThis is a new instruction."

	// Ensure instruction file (should create it)
	err = gateway.EnsureInstructionFile(instructionPath, instructionContent)
	require.NoError(t, err)

	// Verify file exists
	fullPath := filepath.Join(tempDir, instructionPath)
	content, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, instructionContent, string(content))

	// Ensure instruction file again (should not overwrite)
	err = gateway.EnsureInstructionFile(instructionPath, "different content")
	require.NoError(t, err)

	// Verify original content is preserved
	content, err = os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, instructionContent, string(content))
}

func TestLocalStorageGateway_CopyFile(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	// Create source file
	srcPath := "source/file.txt"
	srcContent := "source content"
	srcFullPath := filepath.Join(tempDir, srcPath)
	err = os.MkdirAll(filepath.Dir(srcFullPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(srcFullPath, []byte(srcContent), 0644)
	require.NoError(t, err)

	// Copy file
	dstPath := "destination/file.txt"
	err = gateway.CopyFile(srcPath, dstPath)
	require.NoError(t, err)

	// Verify destination file
	dstFullPath := filepath.Join(tempDir, dstPath)
	content, err := os.ReadFile(dstFullPath)
	require.NoError(t, err)
	assert.Equal(t, srcContent, string(content))
}

func TestLocalStorageGateway_GetStoragePath(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()

	// Create gateway
	gateway, err := NewLocalStorageGateway(tempDir)
	require.NoError(t, err)

	taskID := "test-task-004"
	artifactID := "test-artifact-001"

	// Get storage path
	storagePath := gateway.GetStoragePath(taskID, artifactID)
	expectedPath := filepath.Join(tempDir, "artifacts", taskID, artifactID)
	assert.Equal(t, expectedPath, storagePath)
}

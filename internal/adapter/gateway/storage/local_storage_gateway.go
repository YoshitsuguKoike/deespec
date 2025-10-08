package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// LocalStorageGateway implements StorageGateway using local filesystem
// Directory structure: <baseDir>/artifacts/<taskID>/<artifactID>/
//   - content: actual artifact content
//   - metadata.json: artifact metadata
type LocalStorageGateway struct {
	baseDir string // Base directory for artifact storage (e.g., ~/.deespec)
}

// NewLocalStorageGateway creates a new local filesystem-based storage gateway
func NewLocalStorageGateway(baseDir string) (*LocalStorageGateway, error) {
	// Ensure base directory exists
	artifactsDir := filepath.Join(baseDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return nil, fmt.Errorf("create artifacts directory: %w", err)
	}

	return &LocalStorageGateway{
		baseDir: baseDir,
	}, nil
}

// SaveArtifact saves an artifact to local filesystem
func (g *LocalStorageGateway) SaveArtifact(ctx context.Context, req output.SaveArtifactRequest) (*output.ArtifactMetadata, error) {
	// Generate unique artifact ID based on content hash + timestamp
	artifactID := g.generateArtifactID(req.Content)

	// Create artifact directory: <baseDir>/artifacts/<taskID>/<artifactID>/
	artifactDir := filepath.Join(g.baseDir, "artifacts", req.TaskID, artifactID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("create artifact directory: %w", err)
	}

	// Save artifact content
	contentPath := filepath.Join(artifactDir, "content")
	if err := os.WriteFile(contentPath, req.Content, 0644); err != nil {
		return nil, fmt.Errorf("write artifact content: %w", err)
	}

	// Create metadata
	metadata := output.ArtifactMetadata{
		ID:          artifactID,
		TaskID:      req.TaskID,
		Type:        req.ArtifactType,
		StoragePath: contentPath,
		ContentType: req.ContentType,
		Size:        int64(len(req.Content)),
		UploadedAt:  time.Now(),
		Metadata:    req.Metadata,
	}

	// Save metadata as JSON
	metadataPath := filepath.Join(artifactDir, "metadata.json")
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	return &metadata, nil
}

// LoadArtifact retrieves an artifact from local filesystem
func (g *LocalStorageGateway) LoadArtifact(ctx context.Context, artifactID string) (*output.Artifact, error) {
	// Search for artifact in all task directories
	// Pattern: <baseDir>/artifacts/*/<artifactID>/
	artifactsDir := filepath.Join(g.baseDir, "artifacts")

	var foundArtifactDir string
	err := filepath.WalkDir(artifactsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == artifactID {
			foundArtifactDir = path
			return filepath.SkipAll // Stop walking
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search artifact: %w", err)
	}

	if foundArtifactDir == "" {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	// Load metadata
	metadataPath := filepath.Join(foundArtifactDir, "metadata.json")
	metadataJSON, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	var metadata output.ArtifactMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	// Load content
	contentPath := filepath.Join(foundArtifactDir, "content")
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return &output.Artifact{
		ID:       artifactID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

// LoadInstruction loads an instruction/specification document from filesystem
func (g *LocalStorageGateway) LoadInstruction(ctx context.Context, instructionPath string) (string, error) {
	// If instructionPath is absolute, use it directly
	// Otherwise, treat it as relative to baseDir
	var fullPath string
	if filepath.IsAbs(instructionPath) {
		fullPath = instructionPath
	} else {
		fullPath = filepath.Join(g.baseDir, instructionPath)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read instruction file: %w", err)
	}

	return string(content), nil
}

// ListArtifacts lists artifacts for a given task
func (g *LocalStorageGateway) ListArtifacts(ctx context.Context, taskID string) ([]*output.ArtifactMetadata, error) {
	taskArtifactsDir := filepath.Join(g.baseDir, "artifacts", taskID)

	// Check if task artifacts directory exists
	if _, err := os.Stat(taskArtifactsDir); os.IsNotExist(err) {
		return []*output.ArtifactMetadata{}, nil // No artifacts for this task
	}

	var metadataList []*output.ArtifactMetadata

	// Iterate through artifact directories
	entries, err := os.ReadDir(taskArtifactsDir)
	if err != nil {
		return nil, fmt.Errorf("read task artifacts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Load metadata for each artifact
		metadataPath := filepath.Join(taskArtifactsDir, entry.Name(), "metadata.json")
		metadataJSON, err := os.ReadFile(metadataPath)
		if err != nil {
			// Skip artifacts with missing metadata
			continue
		}

		var metadata output.ArtifactMetadata
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			// Skip artifacts with invalid metadata
			continue
		}

		metadataList = append(metadataList, &metadata)
	}

	return metadataList, nil
}

// generateArtifactID generates a unique artifact ID based on content hash
func (g *LocalStorageGateway) generateArtifactID(content []byte) string {
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%d", hashStr, timestamp)
}

// DeleteArtifact removes an artifact from local filesystem (utility method)
func (g *LocalStorageGateway) DeleteArtifact(ctx context.Context, taskID, artifactID string) error {
	artifactDir := filepath.Join(g.baseDir, "artifacts", taskID, artifactID)
	if err := os.RemoveAll(artifactDir); err != nil {
		return fmt.Errorf("delete artifact directory: %w", err)
	}
	return nil
}

// GetStoragePath returns the full path to an artifact's storage location
func (g *LocalStorageGateway) GetStoragePath(taskID, artifactID string) string {
	return filepath.Join(g.baseDir, "artifacts", taskID, artifactID)
}

// EnsureInstructionFile ensures an instruction file exists at the given path
// If content is provided and file doesn't exist, creates it
func (g *LocalStorageGateway) EnsureInstructionFile(instructionPath, content string) error {
	fullPath := filepath.Join(g.baseDir, instructionPath)

	// Check if file already exists
	if _, err := os.Stat(fullPath); err == nil {
		return nil // File exists, nothing to do
	}

	// Create parent directory
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create instruction directory: %w", err)
	}

	// Write content
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write instruction file: %w", err)
	}

	return nil
}

// CopyFile copies a file from src to dst within the storage
func (g *LocalStorageGateway) CopyFile(src, dst string) error {
	srcPath := filepath.Join(g.baseDir, src)
	dstPath := filepath.Join(g.baseDir, dst)

	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination directory
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	// Create destination file
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy file content: %w", err)
	}

	return nil
}

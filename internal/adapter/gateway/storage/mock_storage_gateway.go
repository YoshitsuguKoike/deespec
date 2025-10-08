package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// MockStorageGateway is a mock implementation of StorageGateway
// Stores artifacts in memory for testing purposes
// Will be replaced with real S3/Local implementation in Phase 6
type MockStorageGateway struct {
	mu           sync.RWMutex
	artifacts    map[string]*output.Artifact
	instructions map[string]string
	nextID       int
}

// NewMockStorageGateway creates a new mock storage gateway
func NewMockStorageGateway() *MockStorageGateway {
	return &MockStorageGateway{
		artifacts:    make(map[string]*output.Artifact),
		instructions: make(map[string]string),
		nextID:       1,
	}
}

// SaveArtifact saves an artifact to mock storage
func (g *MockStorageGateway) SaveArtifact(ctx context.Context, req output.SaveArtifactRequest) (*output.ArtifactMetadata, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Generate artifact ID
	artifactID := fmt.Sprintf("mock-artifact-%d", g.nextID)
	g.nextID++

	// Store artifact
	artifact := &output.Artifact{
		ID:      artifactID,
		Content: req.Content,
		Metadata: output.ArtifactMetadata{
			ID:          artifactID,
			TaskID:      req.TaskID,
			Type:        req.ArtifactType,
			StoragePath: "mock://artifacts/" + artifactID,
			ContentType: req.ContentType,
			Size:        int64(len(req.Content)),
			UploadedAt:  time.Now(),
			Metadata:    req.Metadata,
		},
	}

	g.artifacts[artifactID] = artifact

	return &artifact.Metadata, nil
}

// LoadArtifact retrieves an artifact from mock storage
func (g *MockStorageGateway) LoadArtifact(ctx context.Context, artifactID string) (*output.Artifact, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	artifact, exists := g.artifacts[artifactID]
	if !exists {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	return artifact, nil
}

// LoadInstruction loads an instruction/specification document
func (g *MockStorageGateway) LoadInstruction(ctx context.Context, instructionPath string) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check if instruction exists in mock storage
	if instruction, exists := g.instructions[instructionPath]; exists {
		return instruction, nil
	}

	// Return mock instruction
	return fmt.Sprintf("[Mock Instruction] Content from %s\n\nThis is a placeholder instruction document.", instructionPath), nil
}

// ListArtifacts lists artifacts for a given task
func (g *MockStorageGateway) ListArtifacts(ctx context.Context, taskID string) ([]*output.ArtifactMetadata, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var metadata []*output.ArtifactMetadata
	for _, artifact := range g.artifacts {
		if artifact.Metadata.TaskID == taskID {
			md := artifact.Metadata
			metadata = append(metadata, &md)
		}
	}

	return metadata, nil
}

// SetInstruction sets a mock instruction (for testing)
func (g *MockStorageGateway) SetInstruction(path, content string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.instructions[path] = content
}

// GetArtifactCount returns the number of stored artifacts (for testing)
func (g *MockStorageGateway) GetArtifactCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.artifacts)
}

// Clear clears all stored artifacts and instructions (for testing)
func (g *MockStorageGateway) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.artifacts = make(map[string]*output.Artifact)
	g.instructions = make(map[string]string)
	g.nextID = 1
}

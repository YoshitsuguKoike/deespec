package output

import (
	"context"
	"time"
)

// StorageGateway is the interface for external storage operations
// Supports both local filesystem and cloud storage (S3, GCS, etc.)
type StorageGateway interface {
	// SaveArtifact persists an artifact to storage
	SaveArtifact(ctx context.Context, req SaveArtifactRequest) (*ArtifactMetadata, error)

	// LoadArtifact retrieves an artifact from storage
	LoadArtifact(ctx context.Context, artifactID string) (*Artifact, error)

	// LoadInstruction loads an instruction/specification document
	LoadInstruction(ctx context.Context, instructionPath string) (string, error)

	// ListArtifacts lists artifacts for a given task
	ListArtifacts(ctx context.Context, taskID string) ([]*ArtifactMetadata, error)
}

// SaveArtifactRequest represents a request to save an artifact
type SaveArtifactRequest struct {
	TaskID       string            // Associated task ID
	ArtifactType ArtifactType      // Type of artifact
	Content      []byte            // Artifact content
	Metadata     map[string]string // Additional metadata
	ContentType  string            // MIME type (optional)
}

// ArtifactType represents the type of artifact
type ArtifactType string

const (
	ArtifactTypeCode ArtifactType = "code" // Generated code files
	ArtifactTypeSpec ArtifactType = "spec" // Specification documents
	ArtifactTypeData ArtifactType = "data" // Data files
	ArtifactTypeLog  ArtifactType = "log"  // Execution logs
)

// Artifact represents a stored artifact
type Artifact struct {
	ID       string           // Unique artifact ID
	Content  []byte           // Artifact content
	Metadata ArtifactMetadata // Artifact metadata
}

// ArtifactMetadata contains information about an artifact
type ArtifactMetadata struct {
	ID          string            // Unique artifact ID
	TaskID      string            // Associated task ID
	Type        ArtifactType      // Artifact type
	StoragePath string            // Storage path (e.g., s3://bucket/key)
	ContentType string            // MIME type
	Size        int64             // Size in bytes
	UploadedAt  time.Time         // Upload timestamp
	Metadata    map[string]string // Additional metadata
}

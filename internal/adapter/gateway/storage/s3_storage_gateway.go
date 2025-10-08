package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// S3StorageGateway implements StorageGateway using AWS S3
// Bucket structure: s3://<bucket>/<prefix>/artifacts/<taskID>/<artifactID>/
//   - content: actual artifact content
//   - metadata.json: artifact metadata (stored as S3 object metadata)
type S3StorageGateway struct {
	client     S3API // Use interface for testability
	bucketName string
	prefix     string // Optional prefix for all keys (e.g., "deespec/prod")
}

// S3Config holds S3 storage gateway configuration
type S3Config struct {
	BucketName string // S3 bucket name
	Prefix     string // Optional key prefix
	Region     string // AWS region (optional, uses default if empty)
}

// NewS3StorageGateway creates a new S3-based storage gateway
func NewS3StorageGateway(cfg S3Config) (*S3StorageGateway, error) {
	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Override region if specified
	if cfg.Region != "" {
		awsCfg.Region = cfg.Region
	}

	client := s3.NewFromConfig(awsCfg)

	return &S3StorageGateway{
		client:     client,
		bucketName: cfg.BucketName,
		prefix:     cfg.Prefix,
	}, nil
}

// NewS3StorageGatewayWithClient creates a new S3-based storage gateway with custom S3 client
// This is primarily used for testing with mock S3 clients
func NewS3StorageGatewayWithClient(client S3API, bucketName, prefix string) *S3StorageGateway {
	return &S3StorageGateway{
		client:     client,
		bucketName: bucketName,
		prefix:     prefix,
	}
}

// SaveArtifact saves an artifact to S3
func (g *S3StorageGateway) SaveArtifact(ctx context.Context, req output.SaveArtifactRequest) (*output.ArtifactMetadata, error) {
	// Generate unique artifact ID
	artifactID := g.generateArtifactID(req.Content)

	// Build S3 key: <prefix>/artifacts/<taskID>/<artifactID>/content
	contentKey := g.buildKey("artifacts", req.TaskID, artifactID, "content")

	// Prepare metadata as S3 object metadata
	s3Metadata := make(map[string]string)
	s3Metadata["artifact-id"] = artifactID
	s3Metadata["task-id"] = req.TaskID
	s3Metadata["artifact-type"] = string(req.ArtifactType)
	s3Metadata["uploaded-at"] = time.Now().Format(time.RFC3339)

	// Add custom metadata
	for k, v := range req.Metadata {
		s3Metadata[k] = v
	}

	// Upload content to S3
	_, err := g.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(g.bucketName),
		Key:         aws.String(contentKey),
		Body:        bytes.NewReader(req.Content),
		ContentType: aws.String(req.ContentType),
		Metadata:    s3Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("upload to S3: %w", err)
	}

	// Create metadata structure
	metadata := output.ArtifactMetadata{
		ID:          artifactID,
		TaskID:      req.TaskID,
		Type:        req.ArtifactType,
		StoragePath: fmt.Sprintf("s3://%s/%s", g.bucketName, contentKey),
		ContentType: req.ContentType,
		Size:        int64(len(req.Content)),
		UploadedAt:  time.Now(),
		Metadata:    req.Metadata,
	}

	// Also save metadata as separate JSON object for easier querying
	metadataKey := g.buildKey("artifacts", req.TaskID, artifactID, "metadata.json")
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = g.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(g.bucketName),
		Key:         aws.String(metadataKey),
		Body:        bytes.NewReader(metadataJSON),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("upload metadata to S3: %w", err)
	}

	return &metadata, nil
}

// LoadArtifact retrieves an artifact from S3
func (g *S3StorageGateway) LoadArtifact(ctx context.Context, artifactID string) (*output.Artifact, error) {
	// To load artifact, we need to search for it
	// List all task directories and find the artifact
	prefix := g.buildKey("artifacts") + "/"

	// List objects with prefix
	listOutput, err := g.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(g.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("list S3 objects: %w", err)
	}

	// Find metadata file for this artifact
	var metadataKey string
	for _, obj := range listOutput.Contents {
		key := aws.ToString(obj.Key)
		// Check if this is the metadata file for our artifact
		// Pattern: <prefix>/artifacts/<taskID>/<artifactID>/metadata.json
		if contains(key, artifactID) && hasSuffix(key, "metadata.json") {
			metadataKey = key
			break
		}
	}

	if metadataKey == "" {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	// Download metadata
	metadataObj, err := g.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(g.bucketName),
		Key:    aws.String(metadataKey),
	})
	if err != nil {
		return nil, fmt.Errorf("download metadata from S3: %w", err)
	}
	defer metadataObj.Body.Close()

	metadataJSON, err := io.ReadAll(metadataObj.Body)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	var metadata output.ArtifactMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	// Download content
	// Replace metadata.json with content in the key
	contentKey := metadataKey[:len(metadataKey)-len("metadata.json")] + "content"

	contentObj, err := g.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(g.bucketName),
		Key:    aws.String(contentKey),
	})
	if err != nil {
		return nil, fmt.Errorf("download content from S3: %w", err)
	}
	defer contentObj.Body.Close()

	content, err := io.ReadAll(contentObj.Body)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return &output.Artifact{
		ID:       artifactID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

// LoadInstruction loads an instruction/specification document from S3
func (g *S3StorageGateway) LoadInstruction(ctx context.Context, instructionPath string) (string, error) {
	// Build S3 key for instruction
	key := g.buildKey(instructionPath)

	// Download instruction from S3
	result, err := g.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(g.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("download instruction from S3: %w", err)
	}
	defer result.Body.Close()

	content, err := io.ReadAll(result.Body)
	if err != nil {
		return "", fmt.Errorf("read instruction: %w", err)
	}

	return string(content), nil
}

// ListArtifacts lists artifacts for a given task
func (g *S3StorageGateway) ListArtifacts(ctx context.Context, taskID string) ([]*output.ArtifactMetadata, error) {
	// Build prefix for task artifacts
	prefix := g.buildKey("artifacts", taskID) + "/"

	// List objects with prefix
	listOutput, err := g.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(g.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("list S3 objects: %w", err)
	}

	var metadataList []*output.ArtifactMetadata

	// Filter for metadata.json files
	for _, obj := range listOutput.Contents {
		key := aws.ToString(obj.Key)
		if !hasSuffix(key, "metadata.json") {
			continue
		}

		// Download metadata
		metadataObj, err := g.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(g.bucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			// Skip artifacts with download errors
			continue
		}

		metadataJSON, err := io.ReadAll(metadataObj.Body)
		metadataObj.Body.Close()
		if err != nil {
			continue
		}

		var metadata output.ArtifactMetadata
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			continue
		}

		metadataList = append(metadataList, &metadata)
	}

	return metadataList, nil
}

// DeleteArtifact removes an artifact from S3 (utility method)
func (g *S3StorageGateway) DeleteArtifact(ctx context.Context, taskID, artifactID string) error {
	// Delete both content and metadata
	contentKey := g.buildKey("artifacts", taskID, artifactID, "content")
	metadataKey := g.buildKey("artifacts", taskID, artifactID, "metadata.json")

	// Delete content
	_, err := g.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(g.bucketName),
		Key:    aws.String(contentKey),
	})
	if err != nil {
		return fmt.Errorf("delete content from S3: %w", err)
	}

	// Delete metadata
	_, err = g.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(g.bucketName),
		Key:    aws.String(metadataKey),
	})
	if err != nil {
		return fmt.Errorf("delete metadata from S3: %w", err)
	}

	return nil
}

// buildKey builds an S3 key with the configured prefix
func (g *S3StorageGateway) buildKey(parts ...string) string {
	if g.prefix != "" {
		allParts := append([]string{g.prefix}, parts...)
		return joinPath(allParts...)
	}
	return joinPath(parts...)
}

// generateArtifactID generates a unique artifact ID based on content hash
func (g *S3StorageGateway) generateArtifactID(content []byte) string {
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%d", hashStr, timestamp)
}

// Helper functions

func joinPath(parts ...string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "/"
		}
		result += part
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

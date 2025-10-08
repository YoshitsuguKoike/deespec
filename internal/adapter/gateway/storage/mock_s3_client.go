package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// MockS3Client is a mock implementation of S3API for testing
// It stores objects in memory and simulates S3 behavior
type MockS3Client struct {
	mu      sync.RWMutex
	objects map[string]*mockS3Object // key -> object
}

// mockS3Object represents an S3 object stored in memory
type mockS3Object struct {
	content     []byte
	contentType string
	metadata    map[string]string
}

// NewMockS3Client creates a new mock S3 client
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		objects: make(map[string]*mockS3Object),
	}
}

// PutObject simulates uploading an object to S3
func (m *MockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read body content
	content, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Store object
	key := aws.ToString(params.Key)
	m.objects[key] = &mockS3Object{
		content:     content,
		contentType: aws.ToString(params.ContentType),
		metadata:    params.Metadata,
	}

	return &s3.PutObjectOutput{}, nil
}

// GetObject simulates retrieving an object from S3
func (m *MockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.ToString(params.Key)
	obj, exists := m.objects[key]
	if !exists {
		return nil, &types.NoSuchKey{
			Message: aws.String(fmt.Sprintf("The specified key does not exist: %s", key)),
		}
	}

	return &s3.GetObjectOutput{
		Body:        io.NopCloser(bytes.NewReader(obj.content)),
		ContentType: aws.String(obj.contentType),
		Metadata:    obj.metadata,
	}, nil
}

// ListObjectsV2 simulates listing objects in S3
func (m *MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := aws.ToString(params.Prefix)
	var contents []types.Object

	// Filter objects by prefix
	for key := range m.objects {
		if strings.HasPrefix(key, prefix) {
			contents = append(contents, types.Object{
				Key: aws.String(key),
			})
		}
	}

	// Simple implementation without pagination
	// For comprehensive testing, pagination support could be added
	return &s3.ListObjectsV2Output{
		Contents:    contents,
		IsTruncated: aws.Bool(false),
	}, nil
}

// DeleteObject simulates deleting an object from S3
func (m *MockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(params.Key)
	delete(m.objects, key)

	return &s3.DeleteObjectOutput{}, nil
}

// GetObjectCount returns the number of stored objects (for testing)
func (m *MockS3Client) GetObjectCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.objects)
}

// Clear removes all stored objects (for testing)
func (m *MockS3Client) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects = make(map[string]*mockS3Object)
}

// GetObject retrieves an object for inspection (for testing)
func (m *MockS3Client) GetObjectForTest(key string) (*mockS3Object, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, exists := m.objects[key]
	return obj, exists
}

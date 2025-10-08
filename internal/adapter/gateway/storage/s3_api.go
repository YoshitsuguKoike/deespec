package storage

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3API defines the interface for S3 operations used by S3StorageGateway
// This interface allows for mocking in tests without requiring actual S3 connection
type S3API interface {
	// PutObject uploads an object to S3
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

	// GetObject retrieves an object from S3
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)

	// ListObjectsV2 lists objects in S3
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)

	// DeleteObject deletes an object from S3
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// Ensure *s3.Client implements S3API
var _ S3API = (*s3.Client)(nil)

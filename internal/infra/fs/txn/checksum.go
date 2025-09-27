package txn

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

// ChecksumAlgorithm represents the hashing algorithm used
type ChecksumAlgorithm string

const (
	// ChecksumSHA256 is the default checksum algorithm
	ChecksumSHA256 ChecksumAlgorithm = "sha256"
)

// FileChecksum represents a file's checksum information
type FileChecksum struct {
	// Algorithm used for checksum calculation
	Algorithm ChecksumAlgorithm `json:"algorithm"`

	// Hex-encoded checksum value
	Value string `json:"value"`

	// File size in bytes
	Size int64 `json:"size"`

	// File path (for validation)
	Path string `json:"path"`
}

// CalculateFileChecksum computes checksum for a file
func CalculateFileChecksum(filePath string, algorithm ChecksumAlgorithm) (*FileChecksum, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()

	// Get file info for size
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file for checksum: %w", err)
	}

	// Calculate checksum based on algorithm
	var checksum string
	switch algorithm {
	case ChecksumSHA256:
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return nil, fmt.Errorf("calculate sha256: %w", err)
		}
		checksum = fmt.Sprintf("%x", hash.Sum(nil))

	default:
		return nil, fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
	}

	return &FileChecksum{
		Algorithm: algorithm,
		Value:     checksum,
		Size:      info.Size(),
		Path:      filePath,
	}, nil
}

// ValidateFileChecksum verifies a file against expected checksum
func ValidateFileChecksum(filePath string, expected *FileChecksum) error {
	startTime := time.Now()

	if expected == nil {
		fmt.Fprintf(os.Stderr, "ERROR: Checksum validation failed %s=%s error=%s\n",
			MetricChecksumValidationFailed, filePath, "no_expected_checksum")
		return fmt.Errorf("no expected checksum provided")
	}

	// Calculate current checksum
	current, err := CalculateFileChecksum(filePath, expected.Algorithm)
	if err != nil {
		duration := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "ERROR: Checksum validation failed %s=%s %s=%dms error=%v\n",
			MetricChecksumValidationFailed, filePath, MetricChecksumCalculationTime, duration.Milliseconds(), err)
		return fmt.Errorf("calculate current checksum: %w", err)
	}

	// Validate algorithm matches
	if current.Algorithm != expected.Algorithm {
		duration := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "ERROR: Checksum validation failed %s=%s %s=%dms error=%s\n",
			MetricChecksumValidationFailed, filePath, MetricChecksumCalculationTime, duration.Milliseconds(), "algorithm_mismatch")
		return fmt.Errorf("checksum algorithm mismatch: expected %s, got %s",
			expected.Algorithm, current.Algorithm)
	}

	// Validate size matches
	if current.Size != expected.Size {
		duration := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "ERROR: Checksum validation failed %s=%s %s=%dms error=%s expected_size=%d actual_size=%d\n",
			MetricChecksumValidationFailed, filePath, MetricChecksumCalculationTime, duration.Milliseconds(), "size_mismatch", expected.Size, current.Size)
		return fmt.Errorf("file size mismatch: expected %d bytes, got %d bytes",
			expected.Size, current.Size)
	}

	// Validate checksum matches
	if current.Value != expected.Value {
		duration := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "ERROR: Checksum validation failed %s=%s %s=%dms error=%s\n",
			MetricChecksumValidationFailed, filePath, MetricChecksumCalculationTime, duration.Milliseconds(), "checksum_mismatch")
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expected.Value, current.Value)
	}

	// Log successful validation
	duration := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "INFO: Checksum validation successful %s=%s %s=%s %s=%dms\n",
		MetricChecksumValidationSuccess, filePath, MetricChecksumAlgorithm, expected.Algorithm, MetricChecksumCalculationTime, duration.Milliseconds())

	return nil
}

// CalculateDataChecksum computes checksum for in-memory data
func CalculateDataChecksum(data []byte, algorithm ChecksumAlgorithm) (*FileChecksum, error) {
	var checksum string

	switch algorithm {
	case ChecksumSHA256:
		hash := sha256.Sum256(data)
		checksum = fmt.Sprintf("%x", hash)

	default:
		return nil, fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
	}

	return &FileChecksum{
		Algorithm: algorithm,
		Value:     checksum,
		Size:      int64(len(data)),
		Path:      "", // No path for in-memory data
	}, nil
}

// ValidateDataChecksum verifies data against expected checksum
func ValidateDataChecksum(data []byte, expected *FileChecksum) error {
	if expected == nil {
		return fmt.Errorf("no expected checksum provided")
	}

	// Calculate current checksum
	current, err := CalculateDataChecksum(data, expected.Algorithm)
	if err != nil {
		return fmt.Errorf("calculate current checksum: %w", err)
	}

	// Validate algorithm matches
	if current.Algorithm != expected.Algorithm {
		return fmt.Errorf("checksum algorithm mismatch: expected %s, got %s",
			expected.Algorithm, current.Algorithm)
	}

	// Validate size matches
	if current.Size != expected.Size {
		return fmt.Errorf("data size mismatch: expected %d bytes, got %d bytes",
			expected.Size, current.Size)
	}

	// Validate checksum matches
	if current.Value != expected.Value {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expected.Value, current.Value)
	}

	return nil
}

// CompareFileChecksums compares two file checksums for equality
func CompareFileChecksums(a, b *FileChecksum) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Algorithm == b.Algorithm &&
		a.Value == b.Value &&
		a.Size == b.Size
}

package txn

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"runtime"
	"sync"
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

// TeeHashWriter writes to an underlying writer while computing checksum
type TeeHashWriter struct {
	writer io.Writer
	hasher hash.Hash
	size   int64
}

// NewTeeHashWriter creates a new TeeHashWriter for stream checksum calculation
func NewTeeHashWriter(w io.Writer, algorithm ChecksumAlgorithm) (*TeeHashWriter, error) {
	var hasher hash.Hash
	switch algorithm {
	case ChecksumSHA256:
		hasher = sha256.New()
	default:
		return nil, fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
	}

	return &TeeHashWriter{
		writer: w,
		hasher: hasher,
		size:   0,
	}, nil
}

// Write implements io.Writer, writing to both the underlying writer and hasher
func (t *TeeHashWriter) Write(p []byte) (n int, err error) {
	n, err = t.writer.Write(p)
	if err != nil {
		return n, err
	}

	// Write to hasher for checksum calculation
	t.hasher.Write(p[:n])
	t.size += int64(n)

	return n, nil
}

// Checksum returns the final checksum result
func (t *TeeHashWriter) Checksum(algorithm ChecksumAlgorithm) *FileChecksum {
	return &FileChecksum{
		Algorithm: algorithm,
		Value:     fmt.Sprintf("%x", t.hasher.Sum(nil)),
		Size:      t.size,
		Path:      "", // Set by caller if needed
	}
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
		fmt.Fprintf(os.Stderr, "ERROR: Checksum validation failed op=commit file=%s expected=%s actual=%s %s=%s %s=%dms\n",
			filePath, expected.Value, current.Value, MetricChecksumValidationFailed, filePath, MetricChecksumCalculationTime, duration.Milliseconds())
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

// ChecksumJob represents a checksum calculation job for parallel processing
type ChecksumJob struct {
	FilePath  string
	Algorithm ChecksumAlgorithm
	Result    chan ChecksumResult
}

// ChecksumResult contains the result of a checksum calculation
type ChecksumResult struct {
	FilePath string
	Checksum *FileChecksum
	Error    error
}

// ChecksumWorkerPool manages parallel checksum calculations
type ChecksumWorkerPool struct {
	workerCount int
	jobs        chan ChecksumJob
	wg          sync.WaitGroup
}

// NewChecksumWorkerPool creates a new worker pool for parallel checksum calculation
func NewChecksumWorkerPool(workerCount int) *ChecksumWorkerPool {
	if workerCount <= 0 {
		workerCount = 4 // Default worker count
	}

	pool := &ChecksumWorkerPool{
		workerCount: workerCount,
		jobs:        make(chan ChecksumJob, workerCount*2), // Buffered channel
	}

	// Start workers
	for i := 0; i < workerCount; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker processes checksum jobs
func (p *ChecksumWorkerPool) worker() {
	defer p.wg.Done()

	for job := range p.jobs {
		startTime := time.Now()
		checksum, err := CalculateFileChecksum(job.FilePath, job.Algorithm)

		if err != nil {
			duration := time.Since(startTime)
			fmt.Fprintf(os.Stderr, "WARN: Parallel checksum calculation failed %s=%s %s=%dms error=%v\n",
				MetricChecksumCalculationFailed, job.FilePath, MetricChecksumCalculationTime, duration.Milliseconds(), err)
		} else {
			duration := time.Since(startTime)
			fmt.Fprintf(os.Stderr, "INFO: Parallel checksum calculation completed %s=%s %s=%s %s=%dms\n",
				MetricChecksumCalculationSuccess, job.FilePath, MetricChecksumAlgorithm, job.Algorithm, MetricChecksumCalculationTime, duration.Milliseconds())
		}

		job.Result <- ChecksumResult{
			FilePath: job.FilePath,
			Checksum: checksum,
			Error:    err,
		}
	}
}

// CalculateChecksums calculates checksums for multiple files in parallel
func (p *ChecksumWorkerPool) CalculateChecksums(filePaths []string, algorithm ChecksumAlgorithm) map[string]ChecksumResult {
	results := make(map[string]ChecksumResult)

	if len(filePaths) == 0 {
		return results
	}

	// Create result channels
	resultChans := make([]chan ChecksumResult, len(filePaths))
	for i := range resultChans {
		resultChans[i] = make(chan ChecksumResult, 1)
	}

	// Submit jobs
	for i, filePath := range filePaths {
		job := ChecksumJob{
			FilePath:  filePath,
			Algorithm: algorithm,
			Result:    resultChans[i],
		}
		p.jobs <- job
	}

	// Collect results
	for i, resultChan := range resultChans {
		result := <-resultChan
		results[filePaths[i]] = result
		close(resultChan)
	}

	return results
}

// Close shuts down the worker pool
func (p *ChecksumWorkerPool) Close() {
	close(p.jobs)
	p.wg.Wait()
}

// CalculateOptimalWorkerCount determines optimal worker count based on system resources
func CalculateOptimalWorkerCount(fileCount int) int {
	// Get CPU core count, fallback to 1 if unavailable
	coreCount := runtime.GOMAXPROCS(0)
	if coreCount <= 0 {
		coreCount = 1
	}

	// Base worker count on file count and CPU cores
	// Formula: min(fileCount, min(coreCount, 4))
	// Rationale:
	// - No point having more workers than files
	// - Respect GOMAXPROCS setting for CPU-bound work
	// - Cap at 4 to prevent excessive I/O contention
	workerCount := fileCount
	if workerCount > coreCount {
		workerCount = coreCount
	}
	if workerCount > 4 {
		workerCount = 4
	}

	// Minimum of 1 worker for any work
	if workerCount < 1 {
		workerCount = 1
	}

	return workerCount
}

// CalculateChecksumsParallel is a convenience function for parallel checksum calculation
func CalculateChecksumsParallel(filePaths []string, algorithm ChecksumAlgorithm, workerCount int) map[string]ChecksumResult {
	pool := NewChecksumWorkerPool(workerCount)
	defer pool.Close()

	return pool.CalculateChecksums(filePaths, algorithm)
}

// CalculateChecksumsOptimal calculates checksums with automatically optimized worker count
func CalculateChecksumsOptimal(filePaths []string, algorithm ChecksumAlgorithm) map[string]ChecksumResult {
	optimalWorkers := CalculateOptimalWorkerCount(len(filePaths))

	fmt.Fprintf(os.Stderr, "INFO: Checksum calculation with optimal parallelism files=%d workers=%d cores=%d\n",
		len(filePaths), optimalWorkers, runtime.GOMAXPROCS(0))

	return CalculateChecksumsParallel(filePaths, algorithm, optimalWorkers)
}

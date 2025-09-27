package txn

// Metrics keys reserved for Step 12 doctor/metrics integration
// These keys provide a standard interface for monitoring and diagnostics
// METRICS KEYS (Step 7 feedback): Fixed key names for Step 12 integration
const (
	// Transaction commit metrics
	MetricCommitSuccess    = "txn.commit.success"
	MetricCommitFailed     = "txn.commit.failed"
	MetricCommitIdempotent = "txn.commit.idempotent"

	// Transaction rollback metrics (Step 7 feedback)
	MetricRegisterRollbackCount = "txn.register.rollback.count"
	MetricRollbackSuccess       = "txn.rollback.success"
	MetricRollbackFailed        = "txn.rollback.failed"

	// Transaction recovery metrics
	MetricRecoverForwardCount   = "txn.recover.forward.count"
	MetricRecoverForwardSuccess = "txn.recover.forward.success"
	MetricRecoverForwardFailed  = "txn.recover.forward.failed"

	// Recovery retry metrics
	MetricRecoveryRetryAttempt = "txn.retry.attempt"
	MetricRecoveryRetrySuccess = "txn.retry.success"
	MetricRecoveryRetryTimeout = "txn.retry.timeout"
	MetricRecoveryRetryDelay   = "txn.retry.delay_ms"

	// Transaction scan metrics
	MetricScanTotal      = "txn.scan.total"
	MetricScanIntentOnly = "txn.scan.intent_only_count"
	MetricScanIncomplete = "txn.scan.incomplete_count"
	MetricScanAbandoned  = "txn.scan.abandoned_count"
	MetricScanCommitted  = "txn.scan.committed_count"

	// Transaction cleanup metrics
	MetricCleanupSuccess = "txn.cleanup.success"
	MetricCleanupFailed  = "txn.cleanup.failed"
	MetricCleanupPending = "txn.cleanup.pending"

	// Transaction stage metrics
	MetricStageSuccess = "txn.stage.success"
	MetricStageFailed  = "txn.stage.failed"
	MetricStageEXDEV   = "txn.stage.exdev_detected"

	// Transaction performance metrics
	MetricCommitDurationMs  = "txn.commit.duration_ms"
	MetricRecoverDurationMs = "txn.recover.duration_ms"
	MetricScanDurationMs    = "txn.scan.duration_ms"

	// Fsync audit metrics (standardized format)
	MetricFsyncFileCount = "fsync.file.count"
	MetricFsyncDirCount  = "fsync.dir.count"
	MetricFsyncFilePath  = "fsync.file.path"
	MetricFsyncDirPath   = "fsync.dir.path"
	MetricAtomicRename   = "fsync.atomic.rename"
	MetricWriteFileSync  = "fsync.write.file.sync"

	// Checksum validation metrics (Step 11)
	MetricChecksumValidationSuccess = "txn.checksum.validation.success"
	MetricChecksumValidationFailed  = "txn.checksum.validation.failed"
	MetricChecksumCalculationTime   = "txn.checksum.calculation.duration_ms"
	MetricChecksumAlgorithm         = "txn.checksum.algorithm"
)

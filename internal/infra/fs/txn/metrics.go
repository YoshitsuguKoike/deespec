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
)

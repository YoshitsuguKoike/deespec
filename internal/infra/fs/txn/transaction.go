package txn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

// Manager handles transaction lifecycle
type Manager struct {
	baseDir string // Base directory for all transactions (.deespec/var/txn)
}

// NewManager creates a new transaction manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
	}
}

// Begin starts a new transaction
func (m *Manager) Begin(ctx context.Context) (*Transaction, error) {
	// Generate unique transaction ID
	txnID := generateTxnID()

	// Create transaction directories
	txnDir := filepath.Join(m.baseDir, string(txnID))
	stageDir := filepath.Join(txnDir, "stage")
	undoDir := filepath.Join(txnDir, "undo")

	// Create directories with proper permissions
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return nil, fmt.Errorf("create stage directory: %w", err)
	}

	if err := os.MkdirAll(undoDir, 0755); err != nil {
		return nil, fmt.Errorf("create undo directory: %w", err)
	}

	// Fsync parent directory to ensure directories are persisted
	if err := fs.FsyncDir(m.baseDir); err != nil {
		// Log warning but continue (as per architecture doc)
		fmt.Fprintf(os.Stderr, "WARN: fsync base directory failed: %v\n", err)
	}

	// Create initial manifest
	manifest := &Manifest{
		ID:          txnID,
		Description: "Transaction " + string(txnID),
		Files:       []FileOperation{},
		CreatedAt:   time.Now().UTC(),
	}

	// Create transaction object
	tx := &Transaction{
		Manifest: manifest,
		Status:   StatusPending,
		BaseDir:  txnDir,
		StageDir: stageDir,
		UndoDir:  undoDir,
	}

	// Save initial manifest
	if err := m.saveManifest(tx); err != nil {
		return nil, fmt.Errorf("save manifest: %w", err)
	}

	return tx, nil
}

// StageFile stages a file for the transaction
func (m *Manager) StageFile(tx *Transaction, dst string, content []byte) error {
	if tx.Status != StatusPending {
		return fmt.Errorf("cannot stage file: transaction status is %s", tx.Status)
	}

	// EXDEV 検出は Commit で destRoot を元に実施するため、ここでは行わない
	stagePath := filepath.Join(tx.StageDir, dst)

	stageDir := filepath.Dir(stagePath)

	// Create parent directories if needed
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return fmt.Errorf("create stage parent directory: %w", err)
	}

	// Optimized I/O: Write file and calculate checksum in single pass
	file, err := os.Create(stagePath)
	if err != nil {
		return fmt.Errorf("create staged file: %w", err)
	}
	defer file.Close()

	// Create TeeHashWriter to calculate checksum during write
	teeWriter, err := NewTeeHashWriter(file, ChecksumSHA256)
	if err != nil {
		return fmt.Errorf("create tee hash writer: %w", err)
	}

	// Write content to both file and hasher simultaneously
	if _, err := teeWriter.Write(content); err != nil {
		return fmt.Errorf("write staged file with checksum: %w", err)
	}

	// Sync file to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync staged file: %w", err)
	}

	// Get checksum result (no additional I/O required)
	checksumInfo := teeWriter.Checksum(ChecksumSHA256)
	checksumInfo.Path = stagePath

	// Verify staged file checksum (integrity check)
	if err := ValidateFileChecksum(stagePath, checksumInfo); err != nil {
		return fmt.Errorf("staged file checksum validation failed: %w", err)
	}

	// Update manifest with file operation including checksum
	op := FileOperation{
		Type:         "create",
		Destination:  dst,
		Size:         int64(len(content)),
		Mode:         0644,
		Checksum:     checksumInfo.Value, // Legacy field
		ChecksumInfo: checksumInfo,       // Detailed checksum info
	}

	tx.Manifest.Files = append(tx.Manifest.Files, op)

	// Save updated manifest
	if err := m.saveManifest(tx); err != nil {
		return fmt.Errorf("update manifest: %w", err)
	}

	return nil
}

// MarkIntent marks the transaction as ready to commit
func (m *Manager) MarkIntent(tx *Transaction) error {
	if tx.Status != StatusPending {
		return fmt.Errorf("cannot mark intent: transaction status is %s", tx.Status)
	}

	// Validate manifest before marking intent
	if err := tx.Manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// Create intent marker
	intent := &Intent{
		TxnID:     tx.Manifest.ID,
		MarkedAt:  time.Now().UTC(),
		Checksums: make(map[string]string),
		Ready:     true,
	}

	// TODO: Calculate checksums for staged files (Step 11)
	// For now, we'll use empty checksums
	for _, op := range tx.Manifest.Files {
		intent.Checksums[op.Destination] = ""
	}

	// Save intent marker
	intentPath := filepath.Join(tx.BaseDir, "status.intent")
	intentData, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal intent: %w", err)
	}

	if err := fs.WriteFileSync(intentPath, intentData, 0644); err != nil {
		return fmt.Errorf("write intent marker: %w", err)
	}

	// Update transaction state
	tx.Status = StatusIntent
	tx.Intent = intent

	return nil
}

// Commit commits the transaction
// The destRoot parameter specifies the root directory for final file destinations
//
// IDEMPOTENCY GUARANTEE: This method is fully idempotent. If status.commit already exists,
// it returns success immediately without any action (no-op return). This makes forward
// recovery completely safe for double execution.
func (m *Manager) Commit(tx *Transaction, destRoot string, withJournal func() error) error {
	// 0) 早期 EXDEV 検出（stage と最終反映先が別デバイスなら安全失敗）
	if ok, err := sameDevice(tx.StageDir, destRoot); err == nil && !ok {
		return fmt.Errorf("commit aborted: stage(%s) and destRoot(%s) are on different filesystems (EXDEV)", tx.StageDir, destRoot)
	}
	// IDEMPOTENT CHECK: If status.commit exists, this is a no-op (safe for forward recovery)
	commitPath := filepath.Join(tx.BaseDir, "status.commit")
	if _, err := os.Stat(commitPath); err == nil {
		// Already committed - no-op return for complete idempotency
		tx.Status = StatusCommit
		// Log for metrics tracking (Step 12 preparation)
		fmt.Fprintf(os.Stderr, "INFO: Transaction already committed (no-op) txn.commit.idempotent=true txn.id=%s\n", tx.Manifest.ID)
		return nil
	}

	if tx.Status != StatusIntent {
		return fmt.Errorf("cannot commit: transaction status is %s", tx.Status)
	}

	// Phase 1: Validate staged file checksums before commit (parallel for large transactions)
	if len(tx.Manifest.Files) > 4 {
		// Use parallel checksum validation for large transactions
		var filePaths []string
		checksumMap := make(map[string]*FileChecksum)

		for _, op := range tx.Manifest.Files {
			if op.ChecksumInfo != nil {
				stagePath := filepath.Join(tx.StageDir, op.Destination)
				filePaths = append(filePaths, stagePath)
				checksumMap[stagePath] = op.ChecksumInfo
			}
		}

		if len(filePaths) > 0 {
			// Calculate checksums in parallel (worker count = min(files, 4))
			workerCount := len(filePaths)
			if workerCount > 4 {
				workerCount = 4
			}

			fmt.Fprintf(os.Stderr, "INFO: Using parallel checksum validation txn.checksum.parallel=true txn.files=%d txn.workers=%d\n",
				len(filePaths), workerCount)

			results := CalculateChecksumsParallel(filePaths, ChecksumSHA256, workerCount)

			// Validate results
			for filePath, result := range results {
				if result.Error != nil {
					fmt.Fprintf(os.Stderr, "ERROR: Parallel checksum calculation failed op=commit file=%s error=%v\n", filePath, result.Error)
					return fmt.Errorf("parallel checksum calculation failed for %s: %w", filePath, result.Error)
				}

				expected := checksumMap[filePath]
				if !CompareFileChecksums(result.Checksum, expected) {
					fmt.Fprintf(os.Stderr, "ERROR: Parallel checksum validation failed op=commit file=%s expected=%s actual=%s\n",
						filePath, expected.Value, result.Checksum.Value)
					return fmt.Errorf("parallel checksum validation failed for %s: expected %s, got %s",
						filePath, expected.Value, result.Checksum.Value)
				}
			}
		}
	} else {
		// Sequential validation for small transactions
		for _, op := range tx.Manifest.Files {
			stagePath := filepath.Join(tx.StageDir, op.Destination)

			// Validate checksum if available
			if op.ChecksumInfo != nil {
				if err := ValidateFileChecksum(stagePath, op.ChecksumInfo); err != nil {
					return fmt.Errorf("staged file checksum validation failed for %s: %w", op.Destination, err)
				}
			}
		}
	}

	// Phase 2: Rename staged files to final destinations
	for _, op := range tx.Manifest.Files {
		stagePath := filepath.Join(tx.StageDir, op.Destination)
		finalPath := filepath.Join(destRoot, op.Destination)

		// Ensure parent directory exists
		finalDir := filepath.Dir(finalPath)
		if err := os.MkdirAll(finalDir, 0755); err != nil {
			return fmt.Errorf("create final directory: %w", err)
		}

		// 親ディレクトリを永続化
		if err := fs.FsyncDir(finalDir); err != nil {
			return fmt.Errorf("fsync final directory (pre-rename): %w", err)
		}

		// Atomic rename
		if err := fs.AtomicRename(stagePath, finalPath); err != nil {
			return fmt.Errorf("rename %s to %s: %w", stagePath, finalPath, err)
		}

		// rename 後も親を fsync（ポリシーどおり“都度”）
		if err := fs.FsyncDir(finalDir); err != nil {
			return fmt.Errorf("fsync final directory (post-rename): %w", err)
		}

		// Verify final file checksum after rename
		if op.ChecksumInfo != nil {
			if err := ValidateFileChecksum(finalPath, op.ChecksumInfo); err != nil {
				return fmt.Errorf("final file checksum validation failed for %s: %w", op.Destination, err)
			}
		}
	}

	// Phase 3: Execute journal operation
	// CLEANUP ORDER (Step 7 feedback): Journal must be successfully appended BEFORE
	// creating status.commit marker. This ensures journal durability before marking
	// the transaction as complete.
	if withJournal != nil {
		if err := withJournal(); err != nil {
			return fmt.Errorf("journal operation failed: %w", err)
		}
	} else {
		// journal コールバック無し（前方回復シナリオ等）
		fmt.Fprintf(os.Stderr, "WARN: Forward recovery without journal callback for %s\n", tx.Manifest.ID)
	}

	// Phase 4: Mark commit complete
	// This creates status.commit AFTER successful journal append
	commit := &Commit{
		TxnID:       tx.Manifest.ID,
		CommittedAt: time.Now().UTC(),
		CommittedFiles: func() []string {
			files := make([]string, len(tx.Manifest.Files))
			for i, op := range tx.Manifest.Files {
				files[i] = op.Destination
			}
			return files
		}(),
		Success: true,
	}

	// Save commit marker
	commitPathForSave := filepath.Join(tx.BaseDir, "status.commit")
	commitData, err := json.MarshalIndent(commit, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal commit: %w", err)
	}

	if err := fs.WriteFileSync(commitPathForSave, commitData, 0644); err != nil {
		return fmt.Errorf("write commit marker: %w", err)
	}

	// Update transaction state
	tx.Status = StatusCommit
	tx.Commit = commit

	return nil
}

// Rollback aborts transaction and optionally restores original files
// Rollback can be called at any stage of the transaction lifecycle
func (m *Manager) Rollback(tx *Transaction, reason string) error {
	startTime := time.Now()

	// Log rollback attempt with metrics
	fmt.Fprintf(os.Stderr, "INFO: Rolling back transaction %s=%s %s=%s\n",
		MetricRegisterRollbackCount, tx.Manifest.ID, "reason", reason)

	// Check if transaction is already committed
	if tx.Status == StatusCommit {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot rollback committed transaction %s=%s\n",
			MetricRollbackFailed, tx.Manifest.ID)
		return fmt.Errorf("cannot rollback committed transaction %s", tx.Manifest.ID)
	}

	// Phase 1: Restore original files if undo information exists
	undoPerformed := false
	if tx.Undo != nil && len(tx.Undo.RestoreOps) > 0 {
		for _, restore := range tx.Undo.RestoreOps {
			if err := m.performRestore(restore); err != nil {
				fmt.Fprintf(os.Stderr, "WARN: Failed to restore %s during rollback: %v\n",
					restore.TargetPath, err)
				// Continue with other restore operations
			} else {
				undoPerformed = true
			}
		}
	}

	// Phase 2: Clean up transaction files (stage, undo, manifest)
	if err := os.RemoveAll(tx.BaseDir); err != nil {
		duration := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to cleanup transaction during rollback %s=%s %s=%dms error=%v\n",
			MetricRollbackFailed, tx.Manifest.ID, "duration_ms", duration.Milliseconds(), err)
		return fmt.Errorf("cleanup transaction directory: %w", err)
	}

	// Phase 3: Fsync parent directory to ensure cleanup is persisted
	if err := fs.FsyncDir(m.baseDir); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: fsync after rollback cleanup failed: %v\n", err)
	}

	// Update transaction state
	tx.Status = StatusAborted

	// Log successful rollback with metrics
	duration := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "INFO: Transaction rollback completed %s=%s %s=%t %s=%dms\n",
		MetricRollbackSuccess, tx.Manifest.ID, "undo_performed", undoPerformed,
		"duration_ms", duration.Milliseconds())

	return nil
}

// performRestore restores a single file from undo information
func (m *Manager) performRestore(restore RestoreOp) error {
	switch restore.Type {
	case "overwrite":
		// Restore original content from undo directory
		return fs.AtomicRename(restore.UndoPath, restore.TargetPath)

	case "delete":
		// Remove file that was created during transaction
		if err := os.Remove(restore.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove created file: %w", err)
		}
		// Fsync parent directory
		return fs.FsyncDir(filepath.Dir(restore.TargetPath))

	case "create":
		// Recreate file that was deleted during transaction
		return fs.AtomicRename(restore.UndoPath, restore.TargetPath)

	default:
		return fmt.Errorf("unknown restore operation type: %s", restore.Type)
	}
}

// Cleanup removes transaction work directory
// CLEANUP ORDER (Step 7 feedback): This method should only be called AFTER
// successful journal append and commit marker creation. For failed transactions,
// cleanup happens through Step 8's recovery flow.
func (m *Manager) Cleanup(tx *Transaction) error {
	// Only cleanup committed or aborted transactions
	if tx.Status != StatusCommit && tx.Status != StatusAborted {
		return fmt.Errorf("cannot cleanup: transaction status is %s", tx.Status)
	}

	// Remove entire transaction directory
	if err := os.RemoveAll(tx.BaseDir); err != nil {
		return fmt.Errorf("remove transaction directory: %w", err)
	}

	// Fsync parent to ensure removal is persisted
	if err := fs.FsyncDir(m.baseDir); err != nil {
		// Log warning but continue
		fmt.Fprintf(os.Stderr, "WARN: fsync after cleanup failed: %v\n", err)
	}

	return nil
}

// saveManifest saves the transaction manifest to disk
func (m *Manager) saveManifest(tx *Transaction) error {
	manifestPath := filepath.Join(tx.BaseDir, "manifest.json")
	data, err := json.MarshalIndent(tx.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := fs.WriteFileSync(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// generateTxnID generates a unique transaction ID
func generateTxnID() TxnID {
	// Format: txn_<timestamp>_<random>
	// For simplicity, using timestamp + nanoseconds
	now := time.Now().UTC()
	return TxnID(fmt.Sprintf("txn_%d_%d",
		now.Unix(),
		now.Nanosecond()))
}

// resolveAbsDst resolves destination to absolute path to eliminate cwd dependency
func (t *Transaction) resolveAbsDst(dst string) string {
	// .deespec/ 前置きを誤って含むケースを除去（防御）
	if strings.HasPrefix(dst, ".deespec"+string(os.PathSeparator)) {
		dst = strings.TrimPrefix(dst, ".deespec"+string(os.PathSeparator))
	}
	// 絶対は使わない前提（Validateで弾く）。念のため残すならそのまま返す
	if filepath.IsAbs(dst) {
		return dst
	}

	if home := os.Getenv("DEE_HOME"); home != "" {
		return filepath.Join(home, dst) // ← 期待する出力は常に tmp/.deespec/xxx
	}
	// txn の作業ディレクトリ(BaseDir)は“作業用”なので最終物のフォールバックには使わない
	// 最後の保険：cwd。テストでは必ず DEE_HOME を設定する想定
	return filepath.Join(".", dst)
}

// sameDevice は 2 つのパスが同一デバイス上かを判定する
func sameDevice(p1, p2 string) (bool, error) {
	// 代表として各ディレクトリ自身を stat
	s1, err1 := os.Stat(p1)
	s2, err2 := os.Stat(p2)
	if err1 != nil || err2 != nil {
		// 判定不能な場合は true 扱い（安全側に倒すなら false にしてもよい）
		if err1 != nil {
			return true, err1
		}
		return true, err2
	}
	// Unix系: Stat_t.Dev を比較（Windows は true にフォールバック）
	if st1, ok1 := s1.Sys().(*syscall.Stat_t); ok1 {
		if st2, ok2 := s2.Sys().(*syscall.Stat_t); ok2 {
			return st1.Dev == st2.Dev, nil
		}
	}
	return true, nil
}

package txn

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestTxnStatus(t *testing.T) {
	// Test status constants are defined
	statuses := []Status{
		StatusPending,
		StatusIntent,
		StatusCommit,
		StatusAborted,
		StatusFailed,
	}

	expectedValues := []string{
		"PENDING",
		"INTENT",
		"COMMIT",
		"ABORTED",
		"FAILED",
	}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Status mismatch: got %s, want %s", status, expectedValues[i])
		}
	}
}

func TestStatusIsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Valid PENDING", StatusPending, true},
		{"Valid INTENT", StatusIntent, true},
		{"Valid COMMIT", StatusCommit, true},
		{"Valid ABORTED", StatusAborted, true},
		{"Valid FAILED", StatusFailed, true},
		{"Invalid empty", Status(""), false},
		{"Invalid unknown", Status("UNKNOWN"), false},
		{"Invalid lowercase", Status("pending"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    Status
		wantErr bool
	}{
		{"Valid PENDING", `"PENDING"`, StatusPending, false},
		{"Valid INTENT", `"INTENT"`, StatusIntent, false},
		{"Valid COMMIT", `"COMMIT"`, StatusCommit, false},
		{"Valid ABORTED", `"ABORTED"`, StatusAborted, false},
		{"Valid FAILED", `"FAILED"`, StatusFailed, false},
		{"Invalid unknown", `"UNKNOWN"`, Status(""), true},
		{"Invalid lowercase", `"pending"`, Status(""), true},
		{"Invalid format", `123`, Status(""), true},
		{"Invalid empty", `""`, Status(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Status
			err := json.Unmarshal([]byte(tt.json), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Status.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Status.UnmarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileOperation(t *testing.T) {
	// Test FileOperation struct
	op := FileOperation{
		Type:        "create",
		Destination: "testdata/path/to/file.txt",
		Checksum:    "abc123",
		Size:        1024,
		Mode:        0644,
	}

	if op.Type != "create" {
		t.Errorf("Type mismatch: got %s, want create", op.Type)
	}
	if op.Destination != "testdata/path/to/file.txt" {
		t.Errorf("Destination mismatch: got %s, want testdata/path/to/file.txt", op.Destination)
	}
	if op.Size != 1024 {
		t.Errorf("Size mismatch: got %d, want 1024", op.Size)
	}

	// Test rename operation
	renameOp := FileOperation{
		Type:        "rename",
		Source:      "testdata/old/path.txt",
		Destination: "testdata/new/path.txt",
	}

	if renameOp.Source != "testdata/old/path.txt" {
		t.Errorf("Source mismatch: got %s, want testdata/old/path.txt", renameOp.Source)
	}
}

func TestFileOperationValidate(t *testing.T) {
	tests := []struct {
		name    string
		op      FileOperation
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid create operation",
			op: FileOperation{
				Type:        "create",
				Destination: "/path/to/file.txt",
			},
			wantErr: false,
		},
		{
			name: "Valid rename operation",
			op: FileOperation{
				Type:        "rename",
				Source:      "/old/path.txt",
				Destination: "/new/path.txt",
			},
			wantErr: false,
		},
		{
			name:    "Missing type",
			op:      FileOperation{Destination: "/path/to/file.txt"},
			wantErr: true,
			errMsg:  "file operation type is required",
		},
		{
			name:    "Invalid type",
			op:      FileOperation{Type: "invalid", Destination: "/path"},
			wantErr: true,
			errMsg:  "invalid file operation type",
		},
		{
			name:    "Missing destination",
			op:      FileOperation{Type: "create"},
			wantErr: true,
			errMsg:  "destination path is required",
		},
		{
			name:    "Rename missing source",
			op:      FileOperation{Type: "rename", Destination: "/new/path.txt"},
			wantErr: true,
			errMsg:  "source path is required for rename operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("FileOperation.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg && !contains(err.Error(), tt.errMsg) {
					t.Errorf("FileOperation.Validate() error = %v, want containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestManifest(t *testing.T) {
	now := time.Now()
	deadline := now.Add(1 * time.Hour)

	manifest := Manifest{
		ID:          TxnID("txn_12345_abcdef"),
		Description: "Test transaction",
		Files: []FileOperation{
			{
				Type:        "create",
				Destination: "testdata/test/file1.txt",
				Size:        100,
			},
			{
				Type:        "update",
				Destination: "testdata/test/file2.txt",
				Size:        200,
			},
		},
		CreatedAt: now,
		Deadline:  &deadline,
		Meta: map[string]interface{}{
			"user":    "testuser",
			"command": "test-command",
		},
	}

	if manifest.ID != "txn_12345_abcdef" {
		t.Errorf("ID mismatch: got %s, want txn_12345_abcdef", manifest.ID)
	}
	if len(manifest.Files) != 2 {
		t.Errorf("Files count mismatch: got %d, want 2", len(manifest.Files))
	}
	if manifest.Meta["user"] != "testuser" {
		t.Errorf("Meta user mismatch: got %v, want testuser", manifest.Meta["user"])
	}
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid manifest",
			m: Manifest{
				ID:          TxnID("txn_valid"),
				Description: "Valid transaction",
				Files: []FileOperation{
					{Type: "create", Destination: "/path/to/file"},
				},
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "Missing ID",
			m: Manifest{
				Files: []FileOperation{
					{Type: "create", Destination: "/path/to/file"},
				},
				CreatedAt: time.Now(),
			},
			wantErr: true,
			errMsg:  "transaction ID is required",
		},
		{
			name: "No file operations",
			m: Manifest{
				ID:        TxnID("txn_empty"),
				Files:     []FileOperation{},
				CreatedAt: time.Now(),
			},
			wantErr: true,
			errMsg:  "at least one file operation is required",
		},
		{
			name: "Invalid file operation",
			m: Manifest{
				ID: TxnID("txn_invalid_op"),
				Files: []FileOperation{
					{Type: "", Destination: "/path"},
				},
				CreatedAt: time.Now(),
			},
			wantErr: true,
			errMsg:  "file operation type is required",
		},
		{
			name: "Zero timestamp",
			m: Manifest{
				ID: TxnID("txn_no_time"),
				Files: []FileOperation{
					{Type: "create", Destination: "/path"},
				},
			},
			wantErr: true,
			errMsg:  "creation timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Manifest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Manifest.Validate() error = %v, want containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestIntent(t *testing.T) {
	intent := Intent{
		TxnID:    TxnID("txn_test_123"),
		MarkedAt: time.Now(),
		Checksums: map[string]string{
			"testdata/file1": "checksum1",
			"testdata/file2": "checksum2",
		},
		Ready: true,
	}

	if !intent.Ready {
		t.Error("Intent should be ready")
	}
	if len(intent.Checksums) != 2 {
		t.Errorf("Checksums count mismatch: got %d, want 2", len(intent.Checksums))
	}
	if intent.Checksums["testdata/file1"] != "checksum1" {
		t.Errorf("Checksum mismatch: got %s, want checksum1", intent.Checksums["testdata/file1"])
	}
}

func TestCommit(t *testing.T) {
	commit := Commit{
		TxnID:       TxnID("txn_commit_test"),
		CommittedAt: time.Now(),
		JournalSeq:  42,
		CommittedFiles: []string{
			"testdata/path/file1",
			"testdata/path/file2",
			"testdata/path/file3",
		},
		Success: true,
	}

	if !commit.Success {
		t.Error("Commit should be successful")
	}
	if commit.JournalSeq != 42 {
		t.Errorf("JournalSeq mismatch: got %d, want 42", commit.JournalSeq)
	}
	if len(commit.CommittedFiles) != 3 {
		t.Errorf("CommittedFiles count mismatch: got %d, want 3", len(commit.CommittedFiles))
	}
}

func TestTransaction(t *testing.T) {
	tx := Transaction{
		Manifest: &Manifest{
			ID:          TxnID("txn_full_test"),
			Description: "Full transaction test",
			Files:       []FileOperation{},
			CreatedAt:   time.Now(),
		},
		Status:   StatusPending,
		BaseDir:  ".deespec/var/txn/txn_full_test/",
		StageDir: ".deespec/var/txn/txn_full_test/stage/",
		UndoDir:  ".deespec/var/txn/txn_full_test/undo/",
	}

	if tx.Status != StatusPending {
		t.Errorf("Status mismatch: got %s, want %s", tx.Status, StatusPending)
	}
	if tx.Manifest.ID != "txn_full_test" {
		t.Errorf("Manifest ID mismatch: got %s, want txn_full_test", tx.Manifest.ID)
	}
	if tx.BaseDir != ".deespec/var/txn/txn_full_test/" {
		t.Errorf("BaseDir mismatch: got %s", tx.BaseDir)
	}
}

func TestRecoveryInfo(t *testing.T) {
	recovery := RecoveryInfo{
		IncompleteTxns: []TxnID{
			"txn_incomplete_1",
			"txn_incomplete_2",
		},
		IntentOnly: []TxnID{
			"txn_intent_1",
		},
		PartialStage: []TxnID{
			"txn_partial_1",
		},
		CleanupReady: []TxnID{
			"txn_cleanup_1",
			"txn_cleanup_2",
			"txn_cleanup_3",
		},
		ScannedAt: time.Now(),
	}

	if len(recovery.IncompleteTxns) != 2 {
		t.Errorf("IncompleteTxns count mismatch: got %d, want 2", len(recovery.IncompleteTxns))
	}
	if len(recovery.IntentOnly) != 1 {
		t.Errorf("IntentOnly count mismatch: got %d, want 1", len(recovery.IntentOnly))
	}
	if len(recovery.CleanupReady) != 3 {
		t.Errorf("CleanupReady count mismatch: got %d, want 3", len(recovery.CleanupReady))
	}
}

func TestTxnError(t *testing.T) {
	baseErr := errors.New("disk full")
	txnErr := &TxnError{
		TxnID:       TxnID("txn_error_test"),
		Operation:   "stage_file",
		Err:         baseErr,
		Recoverable: false,
	}

	// Test Error() method
	errStr := txnErr.Error()
	expectedStr := "transaction txn_error_test: stage_file failed (unrecoverable): disk full"
	if errStr != expectedStr {
		t.Errorf("Error string mismatch:\ngot:  %s\nwant: %s", errStr, expectedStr)
	}

	// Test recoverable error
	recoverableErr := &TxnError{
		TxnID:       TxnID("txn_retry"),
		Operation:   "acquire_lock",
		Err:         errors.New("lock busy"),
		Recoverable: true,
	}

	errStr2 := recoverableErr.Error()
	if !contains(errStr2, "recoverable") {
		t.Errorf("Expected 'recoverable' in error string: %s", errStr2)
	}

	// Test Unwrap
	if txnErr.Unwrap() != baseErr {
		t.Error("Unwrap should return the base error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

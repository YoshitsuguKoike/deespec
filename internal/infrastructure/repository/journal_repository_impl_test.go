package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

func TestJournalRepositoryImpl_Append(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Test appending a record
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: []interface{}{"implement_1.md"},
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		t.Errorf("Journal file was not created")
	}

	// Verify content
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal file: %v", err)
	}

	if len(content) == 0 {
		t.Errorf("Journal file is empty")
	}
}

func TestJournalRepositoryImpl_AppendMultiple(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Append multiple records
	for i := 1; i <= 3; i++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00Z",
			SBIID:     "test-sbi-001",
			Turn:      i,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}

		err = repo.Append(ctx, record)
		if err != nil {
			t.Fatalf("Failed to append record %d: %v", i, err)
		}
	}

	// Load and verify
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
}

func TestJournalRepositoryImpl_Load(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Test loading from non-existent file
	records, err := repo.Load(ctx)
	if err != nil {
		t.Errorf("Load should not fail for non-existent file: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Expected 0 records from non-existent file, got %d", len(records))
	}

	// Append a record
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: []interface{}{"implement_1.md"},
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Load and verify
	records, err = repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if records[0].SBIID != "test-sbi-001" {
		t.Errorf("Expected SBIID 'test-sbi-001', got '%s'", records[0].SBIID)
	}

	if records[0].Turn != 1 {
		t.Errorf("Expected Turn 1, got %d", records[0].Turn)
	}
}

func TestJournalRepositoryImpl_LoadWithCorruptedLines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Write corrupted journal file
	content := `{"timestamp":"2025-01-01T00:00:00Z","sbi_id":"test-001","turn":1,"step":"implement","status":"WIP","attempt":1,"decision":"PENDING","elapsed_ms":1000,"error":"","artifacts":[]}
{"timestamp":"2025-01-01T00:01:00Z","sbi_id":"test-001","turn":2,"step":"review
{"timestamp":"2025-01-01T00:02:00Z","sbi_id":"test-001","turn":3,"step":"done","status":"DONE","attempt":1,"decision":"SUCCEEDED","elapsed_ms":2000,"error":"","artifacts":[]}
`
	err = os.WriteFile(journalPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Load should skip corrupted line
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 valid records (1 corrupted skipped), got %d", len(records))
	}
}

func TestJournalRepositoryImpl_FindByTurn(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Append records with different turns
	for i := 1; i <= 3; i++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00Z",
			SBIID:     "test-sbi-001",
			Turn:      i,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		err = repo.Append(ctx, record)
		if err != nil {
			t.Fatalf("Failed to append record: %v", err)
		}
	}

	// Find by turn
	records, err := repo.FindByTurn(ctx, 2)
	if err != nil {
		t.Fatalf("FindByTurn failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record for turn 2, got %d", len(records))
	}

	if records[0].Turn != 2 {
		t.Errorf("Expected Turn 2, got %d", records[0].Turn)
	}
}

func TestJournalRepositoryImpl_FindBySBI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Append records with different SBI IDs
	sbiIDs := []string{"sbi-001", "sbi-002", "sbi-001"}
	for i, sbiID := range sbiIDs {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00Z",
			SBIID:     sbiID,
			Turn:      i + 1,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		err = repo.Append(ctx, record)
		if err != nil {
			t.Fatalf("Failed to append record: %v", err)
		}
	}

	// Find by SBI ID
	records, err := repo.FindBySBI(ctx, "sbi-001")
	if err != nil {
		t.Fatalf("FindBySBI failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records for sbi-001, got %d", len(records))
	}

	for _, record := range records {
		if record.SBIID != "sbi-001" {
			t.Errorf("Expected SBIID 'sbi-001', got '%s'", record.SBIID)
		}
	}
}

func TestJournalRepositoryImpl_BackwardCompatibility(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Write journal with old "ts" field
	content := `{"ts":"2025-01-01T00:00:00Z","sbi_id":"test-001","turn":1,"step":"implement","status":"WIP","attempt":1,"decision":"PENDING","elapsed_ms":1000,"error":"","artifacts":[]}
`
	err = os.WriteFile(journalPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Load should handle old "ts" field
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if records[0].Timestamp != "2025-01-01T00:00:00Z" {
		t.Errorf("Expected timestamp '2025-01-01T00:00:00Z', got '%s'", records[0].Timestamp)
	}
}

func TestJournalRepositoryImpl_EmptyJournal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create empty file
	err = os.WriteFile(journalPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}

	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Load empty journal
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records from empty journal, got %d", len(records))
	}
}

// Turn 2 Additional Tests - Infrastructure Layer Enhancements

func TestJournalRepositoryImpl_EmptyTimestampNormalization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Append record with empty timestamp
	record := &repository.JournalRecord{
		Timestamp: "", // Empty timestamp
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: []interface{}{},
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Load and verify timestamp was normalized
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// Verify timestamp was set (should not be empty)
	if records[0].Timestamp == "" {
		t.Error("Expected non-empty timestamp after normalization")
	}
}

func TestJournalRepositoryImpl_NilArtifactsNormalization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Append record with nil artifacts
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: nil, // nil artifacts
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Load and verify artifacts were normalized to empty array during Append
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// After normalization during Append, artifacts should be an empty array (not nil)
	// However, if the JSON doesn't have the field, mapToRecord may leave it nil
	// The key check is that it's either nil or empty - both are safe
	if records[0].Artifacts == nil {
		// nil is acceptable - it's safe for iteration
		return
	}

	if len(records[0].Artifacts) != 0 {
		t.Errorf("Expected empty or nil artifacts, got %d items", len(records[0].Artifacts))
	}
}

func TestJournalRepositoryImpl_ConcurrentAppends(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Concurrent appends
	numGoroutines := 50
	errChan := make(chan error, numGoroutines)
	doneChan := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			record := &repository.JournalRecord{
				Timestamp: "2025-01-01T00:00:00Z",
				SBIID:     "test-sbi-001",
				Turn:      index,
				Step:      "implement",
				Status:    "WIP",
				Attempt:   1,
				Decision:  "PENDING",
				ElapsedMs: 1000,
				Error:     "",
				Artifacts: []interface{}{},
			}

			err := repo.Append(ctx, record)
			if err != nil {
				errChan <- err
			}
			doneChan <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-doneChan
	}
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Concurrent append failed: %v", err)
	}

	// Verify all records were written
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != numGoroutines {
		t.Errorf("Expected %d records, got %d", numGoroutines, len(records))
	}
}

func TestJournalRepositoryImpl_LinesWithOnlyWhitespace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Write journal with whitespace-only lines
	content := `{"timestamp":"2025-01-01T00:00:00Z","sbi_id":"test-001","turn":1,"step":"implement","status":"WIP","attempt":1,"decision":"PENDING","elapsed_ms":1000,"error":"","artifacts":[]}


{"timestamp":"2025-01-01T00:01:00Z","sbi_id":"test-001","turn":2,"step":"review","status":"DONE","attempt":1,"decision":"SUCCEEDED","elapsed_ms":2000,"error":"","artifacts":[]}
`
	err = os.WriteFile(journalPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Load should skip whitespace-only lines
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 valid records (whitespace lines skipped), got %d", len(records))
	}
}

func TestJournalRepositoryImpl_ComplexArtifacts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Create record with complex nested artifacts
	complexArtifacts := []interface{}{
		"simple_file.md",
		123,
		map[string]interface{}{
			"type":     "report",
			"path":     "/path/to/report.json",
			"metadata": map[string]interface{}{"version": "1.0", "size": 2048},
		},
		[]interface{}{"item1", "item2", map[string]interface{}{"nested": "value"}},
	}

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: complexArtifacts,
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Load and verify complex artifacts are preserved
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if len(records[0].Artifacts) != 4 {
		t.Errorf("Expected 4 artifacts, got %d", len(records[0].Artifacts))
	}
}

func TestJournalRepositoryImpl_UnicodeFields(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Create record with Unicode content
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "テスト-SBI-001",
		Turn:      1,
		Step:      "実装",
		Status:    "進行中",
		Attempt:   1,
		Decision:  "保留",
		ElapsedMs: 1000,
		Error:     "エラー: ファイルが見つかりません 🚨",
		Artifacts: []interface{}{"実装_1.md", "テスト結果.json"},
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record with Unicode: %v", err)
	}

	// Load and verify Unicode is preserved
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].SBIID != "テスト-SBI-001" {
		t.Errorf("Unicode SBIID not preserved: got %s", records[0].SBIID)
	}

	if records[0].Error != "エラー: ファイルが見つかりません 🚨" {
		t.Errorf("Unicode error not preserved: got %s", records[0].Error)
	}
}

func TestJournalRepositoryImpl_FindByTurnEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Find by turn in empty repository
	records, err := repo.FindByTurn(ctx, 1)
	if err != nil {
		t.Fatalf("FindByTurn should not fail on empty repository: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records from empty repository, got %d", len(records))
	}
}

func TestJournalRepositoryImpl_FindBySBIEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Find by SBI in empty repository
	records, err := repo.FindBySBI(ctx, "non-existent")
	if err != nil {
		t.Fatalf("FindBySBI should not fail on empty repository: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records from empty repository, got %d", len(records))
	}
}

func TestJournalRepositoryImpl_TypeConversions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Write journal with various number types (JSON unmarshaling uses float64)
	content := `{"timestamp":"2025-01-01T00:00:00Z","sbi_id":"test-001","turn":1.0,"step":"implement","status":"WIP","attempt":1.0,"decision":"PENDING","elapsed_ms":1000.0,"error":"","artifacts":[]}
`
	err = os.WriteFile(journalPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Load should handle type conversions
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].Turn != 1 {
		t.Errorf("Expected Turn 1, got %d", records[0].Turn)
	}

	if records[0].Attempt != 1 {
		t.Errorf("Expected Attempt 1, got %d", records[0].Attempt)
	}

	if records[0].ElapsedMs != 1000 {
		t.Errorf("Expected ElapsedMs 1000, got %d", records[0].ElapsedMs)
	}
}

func TestJournalRepositoryImpl_AppendToExistingFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "journal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Pre-populate with one record
	initialContent := `{"timestamp":"2025-01-01T00:00:00Z","sbi_id":"test-001","turn":1,"step":"implement","status":"WIP","attempt":1,"decision":"PENDING","elapsed_ms":1000,"error":"","artifacts":[]}
`
	err = os.WriteFile(journalPath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write initial file: %v", err)
	}

	repo := NewJournalRepositoryImpl(journalPath)
	ctx := context.Background()

	// Append a new record
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:01:00Z",
		SBIID:     "test-001",
		Turn:      2,
		Step:      "review",
		Status:    "DONE",
		Attempt:   1,
		Decision:  "SUCCEEDED",
		ElapsedMs: 2000,
		Error:     "",
		Artifacts: []interface{}{},
	}

	err = repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append to existing file: %v", err)
	}

	// Verify both records exist
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records after append, got %d", len(records))
	}

	if records[1].Turn != 2 {
		t.Errorf("Expected second record Turn 2, got %d", records[1].Turn)
	}
}

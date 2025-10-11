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

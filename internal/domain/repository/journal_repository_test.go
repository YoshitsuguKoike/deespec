package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockJournalRepository is a mock implementation of JournalRepository for testing
type MockJournalRepository struct {
	mu      sync.RWMutex
	records []*repository.JournalRecord
}

// NewMockJournalRepository creates a new mock journal repository
func NewMockJournalRepository() *MockJournalRepository {
	return &MockJournalRepository{
		records: make([]*repository.JournalRecord, 0),
	}
}

// Append adds a new record to the journal
func (m *MockJournalRepository) Append(ctx context.Context, record *repository.JournalRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record == nil {
		return errors.New("record cannot be nil")
	}

	// Create a copy to avoid external modifications
	recordCopy := &repository.JournalRecord{
		Timestamp: record.Timestamp,
		SBIID:     record.SBIID,
		Turn:      record.Turn,
		Step:      record.Step,
		Status:    record.Status,
		Attempt:   record.Attempt,
		Decision:  record.Decision,
		ElapsedMs: record.ElapsedMs,
		Error:     record.Error,
		Artifacts: make([]interface{}, len(record.Artifacts)),
	}
	copy(recordCopy.Artifacts, record.Artifacts)

	m.records = append(m.records, recordCopy)
	return nil
}

// Load retrieves all journal records
func (m *MockJournalRepository) Load(ctx context.Context) ([]*repository.JournalRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make([]*repository.JournalRecord, len(m.records))
	for i, record := range m.records {
		result[i] = &repository.JournalRecord{
			Timestamp: record.Timestamp,
			SBIID:     record.SBIID,
			Turn:      record.Turn,
			Step:      record.Step,
			Status:    record.Status,
			Attempt:   record.Attempt,
			Decision:  record.Decision,
			ElapsedMs: record.ElapsedMs,
			Error:     record.Error,
			Artifacts: make([]interface{}, len(record.Artifacts)),
		}
		copy(result[i].Artifacts, record.Artifacts)
	}

	return result, nil
}

// FindByTurn retrieves records for a specific turn
func (m *MockJournalRepository) FindByTurn(ctx context.Context, turn int) ([]*repository.JournalRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*repository.JournalRecord
	for _, record := range m.records {
		if record.Turn == turn {
			recordCopy := &repository.JournalRecord{
				Timestamp: record.Timestamp,
				SBIID:     record.SBIID,
				Turn:      record.Turn,
				Step:      record.Step,
				Status:    record.Status,
				Attempt:   record.Attempt,
				Decision:  record.Decision,
				ElapsedMs: record.ElapsedMs,
				Error:     record.Error,
				Artifacts: make([]interface{}, len(record.Artifacts)),
			}
			copy(recordCopy.Artifacts, record.Artifacts)
			result = append(result, recordCopy)
		}
	}

	return result, nil
}

// FindBySBI retrieves records for a specific SBI
func (m *MockJournalRepository) FindBySBI(ctx context.Context, sbiID string) ([]*repository.JournalRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*repository.JournalRecord
	for _, record := range m.records {
		if record.SBIID == sbiID {
			recordCopy := &repository.JournalRecord{
				Timestamp: record.Timestamp,
				SBIID:     record.SBIID,
				Turn:      record.Turn,
				Step:      record.Step,
				Status:    record.Status,
				Attempt:   record.Attempt,
				Decision:  record.Decision,
				ElapsedMs: record.ElapsedMs,
				Error:     record.Error,
				Artifacts: make([]interface{}, len(record.Artifacts)),
			}
			copy(recordCopy.Artifacts, record.Artifacts)
			result = append(result, recordCopy)
		}
	}

	return result, nil
}

// Test Suite for JournalRepository

func TestJournalRepository_Append(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
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

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Verify record was added
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if records[0].SBIID != "test-sbi-001" {
		t.Errorf("Expected SBIID 'test-sbi-001', got '%s'", records[0].SBIID)
	}
}

func TestJournalRepository_AppendNilRecord(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	err := repo.Append(ctx, nil)
	if err == nil {
		t.Error("Expected error when appending nil record")
	}
}

func TestJournalRepository_AppendMultiple(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Append multiple records
	for i := 1; i <= 5; i++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
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

		err := repo.Append(ctx, record)
		if err != nil {
			t.Fatalf("Failed to append record %d: %v", i, err)
		}
	}

	// Verify all records were added
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("Expected 5 records, got %d", len(records))
	}

	// Verify records are in order
	for i, record := range records {
		if record.Turn != i+1 {
			t.Errorf("Expected Turn %d, got %d", i+1, record.Turn)
		}
	}
}

func TestJournalRepository_Load(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Load from empty repository
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records from empty repository, got %d", len(records))
	}

	// Append records
	for i := 1; i <= 3; i++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
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
		repo.Append(ctx, record)
	}

	// Load all records
	records, err = repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
}

func TestJournalRepository_FindByTurn(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Append records with different turns
	turns := []int{1, 2, 2, 3, 3, 3}
	for _, turn := range turns {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
			SBIID:     "test-sbi-001",
			Turn:      turn,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Find records for turn 2
	records, err := repo.FindByTurn(ctx, 2)
	if err != nil {
		t.Fatalf("FindByTurn failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records for turn 2, got %d", len(records))
	}

	for _, record := range records {
		if record.Turn != 2 {
			t.Errorf("Expected Turn 2, got %d", record.Turn)
		}
	}

	// Find records for turn 3
	records, err = repo.FindByTurn(ctx, 3)
	if err != nil {
		t.Fatalf("FindByTurn failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records for turn 3, got %d", len(records))
	}
}

func TestJournalRepository_FindByTurnNoMatch(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Append records with turn 1
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
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
	repo.Append(ctx, record)

	// Find records for turn 99 (doesn't exist)
	records, err := repo.FindByTurn(ctx, 99)
	if err != nil {
		t.Fatalf("FindByTurn failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records for non-existent turn, got %d", len(records))
	}
}

func TestJournalRepository_FindBySBI(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Append records with different SBI IDs
	sbiIDs := []string{"sbi-001", "sbi-002", "sbi-001", "sbi-003", "sbi-001"}
	for i, sbiID := range sbiIDs {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
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
		repo.Append(ctx, record)
	}

	// Find records for sbi-001
	records, err := repo.FindBySBI(ctx, "sbi-001")
	if err != nil {
		t.Fatalf("FindBySBI failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records for sbi-001, got %d", len(records))
	}

	for _, record := range records {
		if record.SBIID != "sbi-001" {
			t.Errorf("Expected SBIID 'sbi-001', got '%s'", record.SBIID)
		}
	}

	// Find records for sbi-002
	records, err = repo.FindBySBI(ctx, "sbi-002")
	if err != nil {
		t.Fatalf("FindBySBI failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record for sbi-002, got %d", len(records))
	}
}

func TestJournalRepository_FindBySBINoMatch(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Append records with sbi-001
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
		SBIID:     "sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: []interface{}{},
	}
	repo.Append(ctx, record)

	// Find records for non-existent SBI
	records, err := repo.FindBySBI(ctx, "non-existent-sbi")
	if err != nil {
		t.Fatalf("FindBySBI failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records for non-existent SBI, got %d", len(records))
	}
}

func TestJournalRepository_RecordFields(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Create record with all fields populated
	record := &repository.JournalRecord{
		Timestamp: "2025-01-15T10:30:45.123456789Z",
		SBIID:     "sbi-comprehensive",
		Turn:      5,
		Step:      "review",
		Status:    "DONE",
		Attempt:   3,
		Decision:  "SUCCEEDED",
		ElapsedMs: 5000,
		Error:     "timeout error",
		Artifacts: []interface{}{"review_5.md", "test_results.json", map[string]string{"key": "value"}},
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Load and verify all fields
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	r := records[0]

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Timestamp", r.Timestamp, "2025-01-15T10:30:45.123456789Z"},
		{"SBIID", r.SBIID, "sbi-comprehensive"},
		{"Turn", r.Turn, 5},
		{"Step", r.Step, "review"},
		{"Status", r.Status, "DONE"},
		{"Attempt", r.Attempt, 3},
		{"Decision", r.Decision, "SUCCEEDED"},
		{"ElapsedMs", r.ElapsedMs, int64(5000)},
		{"Error", r.Error, "timeout error"},
		{"Artifacts length", len(r.Artifacts), 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Expected %s to be %v, got %v", tt.name, tt.expected, tt.got)
			}
		})
	}
}

func TestJournalRepository_WorkflowSteps(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test all workflow steps
	steps := []string{"plan", "implement", "review", "done"}

	for i, step := range steps {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
			SBIID:     "test-sbi-001",
			Turn:      1,
			Step:      step,
			Status:    "WIP",
			Attempt:   i + 1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Verify all steps were recorded
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 4 {
		t.Errorf("Expected 4 records, got %d", len(records))
	}

	for i, record := range records {
		if record.Step != steps[i] {
			t.Errorf("Expected Step '%s', got '%s'", steps[i], record.Step)
		}
	}
}

func TestJournalRepository_StatusValues(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test various status values
	statuses := []string{"WIP", "DONE", "FAILED", "PENDING"}

	for i, status := range statuses {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
			SBIID:     "test-sbi-001",
			Turn:      i + 1,
			Step:      "implement",
			Status:    status,
			Attempt:   1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Verify all statuses were recorded
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	for i, record := range records {
		if record.Status != statuses[i] {
			t.Errorf("Expected Status '%s', got '%s'", statuses[i], record.Status)
		}
	}
}

func TestJournalRepository_DecisionValues(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test various decision values
	decisions := []string{"PENDING", "SUCCEEDED", "FAILED", "NEEDS_REVISION"}

	for i, decision := range decisions {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
			SBIID:     "test-sbi-001",
			Turn:      i + 1,
			Step:      "review",
			Status:    "DONE",
			Attempt:   1,
			Decision:  decision,
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Verify all decisions were recorded
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	for i, record := range records {
		if record.Decision != decisions[i] {
			t.Errorf("Expected Decision '%s', got '%s'", decisions[i], record.Decision)
		}
	}
}

func TestJournalRepository_Concurrency(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	errorChan := make(chan error, 100)

	// Concurrent appends
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			record := &repository.JournalRecord{
				Timestamp: "2025-01-01T00:00:00.000000000Z",
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
				errorChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent append failed: %v", err)
	}

	// Verify all records were added
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 100 {
		t.Errorf("Expected 100 records, got %d", len(records))
	}
}

func TestJournalRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Pre-populate with some records
	for i := 0; i < 10; i++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
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
		repo.Append(ctx, record)
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 50)

	// Concurrent reads
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := repo.Load(ctx)
			if err != nil {
				errorChan <- err
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			record := &repository.JournalRecord{
				Timestamp: "2025-01-01T00:00:00.000000000Z",
				SBIID:     "test-sbi-001",
				Turn:      index + 10,
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
				errorChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify final state
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 35 {
		t.Errorf("Expected 35 records after concurrent operations, got %d", len(records))
	}
}

func TestJournalRepository_EmptyArtifacts(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
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

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records[0].Artifacts) != 0 {
		t.Errorf("Expected empty artifacts, got %d items", len(records[0].Artifacts))
	}
}

func TestJournalRepository_ComplexArtifacts(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test with various artifact types
	artifacts := []interface{}{
		"simple_string.md",
		123,
		map[string]interface{}{
			"type": "file",
			"path": "/path/to/file.txt",
			"size": 1024,
		},
		[]string{"item1", "item2", "item3"},
	}

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "",
		Artifacts: artifacts,
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records[0].Artifacts) != 4 {
		t.Errorf("Expected 4 artifacts, got %d", len(records[0].Artifacts))
	}
}

func TestJournalRepository_MultipleAttempts(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Simulate multiple attempts for the same turn
	for attempt := 1; attempt <= 3; attempt++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00.000000000Z",
			SBIID:     "test-sbi-001",
			Turn:      1,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   attempt,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Find all records for turn 1
	records, err := repo.FindByTurn(ctx, 1)
	if err != nil {
		t.Fatalf("FindByTurn failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 attempts for turn 1, got %d", len(records))
	}

	// Verify attempt numbers
	for i, record := range records {
		if record.Attempt != i+1 {
			t.Errorf("Expected Attempt %d, got %d", i+1, record.Attempt)
		}
	}
}

func TestJournalRepository_EmptyStringFields(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
		SBIID:     "",
		Turn:      1,
		Step:      "",
		Status:    "",
		Attempt:   1,
		Decision:  "",
		ElapsedMs: 0,
		Error:     "",
		Artifacts: []interface{}{},
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record with empty fields: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.SBIID != "" || r.Step != "" || r.Status != "" || r.Decision != "" || r.Error != "" {
		t.Error("Empty string fields should remain empty")
	}
}

func TestJournalRepository_LargeElapsedTime(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00.000000000Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "DONE",
		Attempt:   1,
		Decision:  "SUCCEEDED",
		ElapsedMs: 999999999, // Large elapsed time
		Error:     "",
		Artifacts: []interface{}{},
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if records[0].ElapsedMs != 999999999 {
		t.Errorf("Expected ElapsedMs 999999999, got %d", records[0].ElapsedMs)
	}
}

// Turn 2 Additional Tests - Enhanced Coverage

func TestJournalRepository_TimestampFormats(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test various RFC3339Nano timestamp formats
	testCases := []struct {
		name      string
		timestamp string
	}{
		{
			name:      "RFC3339Nano with full precision",
			timestamp: "2025-01-15T10:30:45.123456789Z",
		},
		{
			name:      "RFC3339Nano with milliseconds",
			timestamp: "2025-01-15T10:30:45.123Z",
		},
		{
			name:      "RFC3339 without nanoseconds",
			timestamp: "2025-01-15T10:30:45Z",
		},
		{
			name:      "RFC3339 with timezone offset",
			timestamp: "2025-01-15T10:30:45+09:00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			record := &repository.JournalRecord{
				Timestamp: tc.timestamp,
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

			err := repo.Append(ctx, record)
			if err != nil {
				t.Fatalf("Failed to append record with timestamp %s: %v", tc.timestamp, err)
			}

			records, err := repo.FindByTurn(ctx, 1)
			if err != nil {
				t.Fatalf("Failed to load records: %v", err)
			}

			if len(records) == 0 {
				t.Fatal("No records found")
			}

			if records[len(records)-1].Timestamp != tc.timestamp {
				t.Errorf("Expected timestamp %s, got %s", tc.timestamp, records[len(records)-1].Timestamp)
			}
		})
	}
}

func TestJournalRepository_NilArtifactsSafety(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test that nil artifacts are handled safely
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
		Artifacts: nil, // Explicitly nil
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record with nil artifacts: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// Verify artifacts is not nil but empty
	if records[0].Artifacts == nil {
		t.Error("Expected non-nil artifacts slice, got nil")
	}

	if len(records[0].Artifacts) != 0 {
		t.Errorf("Expected empty artifacts, got %d items", len(records[0].Artifacts))
	}
}

func TestJournalRepository_LongErrorMessages(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test with very long error message
	longError := ""
	for i := 0; i < 1000; i++ {
		longError += "Error detail " + string(rune('0'+i%10)) + ". "
	}

	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "test-sbi-001",
		Turn:      1,
		Step:      "implement",
		Status:    "FAILED",
		Attempt:   1,
		Decision:  "FAILED",
		ElapsedMs: 1000,
		Error:     longError,
		Artifacts: []interface{}{},
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record with long error: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if records[0].Error != longError {
		t.Error("Error message was not preserved correctly")
	}
}

func TestJournalRepository_NegativeValues(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test with negative turn and attempt (edge case)
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "test-sbi-001",
		Turn:      -1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   -1,
		Decision:  "PENDING",
		ElapsedMs: -1000,
		Error:     "",
		Artifacts: []interface{}{},
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record with negative values: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if records[0].Turn != -1 {
		t.Errorf("Expected Turn -1, got %d", records[0].Turn)
	}

	if records[0].Attempt != -1 {
		t.Errorf("Expected Attempt -1, got %d", records[0].Attempt)
	}

	if records[0].ElapsedMs != -1000 {
		t.Errorf("Expected ElapsedMs -1000, got %d", records[0].ElapsedMs)
	}
}

func TestJournalRepository_UnicodeContent(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Test with Unicode content in various fields
	record := &repository.JournalRecord{
		Timestamp: "2025-01-01T00:00:00Z",
		SBIID:     "テスト-SBI-001",
		Turn:      1,
		Step:      "implement",
		Status:    "WIP",
		Attempt:   1,
		Decision:  "PENDING",
		ElapsedMs: 1000,
		Error:     "エラー: ファイルが見つかりません 🚨",
		Artifacts: []interface{}{"実装_1.md", "テスト結果.json"},
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record with Unicode content: %v", err)
	}

	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if records[0].SBIID != "テスト-SBI-001" {
		t.Errorf("Unicode SBIID not preserved: got %s", records[0].SBIID)
	}

	if records[0].Error != "エラー: ファイルが見つかりません 🚨" {
		t.Errorf("Unicode error not preserved: got %s", records[0].Error)
	}
}

func TestJournalRepository_FindByTurnMultipleAttempts(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Add multiple records for the same turn with different attempts
	for attempt := 1; attempt <= 5; attempt++ {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00Z",
			SBIID:     "test-sbi-001",
			Turn:      1,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   attempt,
			Decision:  "PENDING",
			ElapsedMs: int64(1000 * attempt),
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Find all records for turn 1
	records, err := repo.FindByTurn(ctx, 1)
	if err != nil {
		t.Fatalf("FindByTurn failed: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("Expected 5 attempts for turn 1, got %d", len(records))
	}

	// Verify attempts are in order
	for i, record := range records {
		expectedAttempt := i + 1
		if record.Attempt != expectedAttempt {
			t.Errorf("Expected attempt %d, got %d", expectedAttempt, record.Attempt)
		}
	}
}

func TestJournalRepository_FindBySBIMultipleTurns(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Add records for multiple SBIs with multiple turns
	sbiIDs := []string{"sbi-001", "sbi-002", "sbi-001", "sbi-003", "sbi-001", "sbi-002"}
	for i, sbiID := range sbiIDs {
		record := &repository.JournalRecord{
			Timestamp: "2025-01-01T00:00:00Z",
			SBIID:     sbiID,
			Turn:      (i % 3) + 1,
			Step:      "implement",
			Status:    "WIP",
			Attempt:   1,
			Decision:  "PENDING",
			ElapsedMs: 1000,
			Error:     "",
			Artifacts: []interface{}{},
		}
		repo.Append(ctx, record)
	}

	// Find all records for sbi-001
	records, err := repo.FindBySBI(ctx, "sbi-001")
	if err != nil {
		t.Fatalf("FindBySBI failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records for sbi-001, got %d", len(records))
	}

	// Verify all records belong to sbi-001
	for _, record := range records {
		if record.SBIID != "sbi-001" {
			t.Errorf("Expected SBIID 'sbi-001', got '%s'", record.SBIID)
		}
	}
}

func TestJournalRepository_DataIsolation(t *testing.T) {
	repo := NewMockJournalRepository()
	ctx := context.Background()

	// Create and append a record
	originalArtifacts := []interface{}{"file1.md", "file2.md"}
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
		Artifacts: originalArtifacts,
	}

	err := repo.Append(ctx, record)
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Modify original artifacts after appending
	originalArtifacts[0] = "modified.md"
	originalArtifacts = append(originalArtifacts, "new.md")

	// Load and verify the stored data was not affected
	records, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if len(records[0].Artifacts) != 2 {
		t.Errorf("Expected 2 artifacts (isolation failed), got %d", len(records[0].Artifacts))
	}

	if records[0].Artifacts[0] != "file1.md" {
		t.Errorf("Expected 'file1.md', got '%v' (isolation failed)", records[0].Artifacts[0])
	}
}

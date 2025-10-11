package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

// JournalRepositoryImpl implements repository.JournalRepository using NDJSON file-based storage
type JournalRepositoryImpl struct {
	journalPath string
}

// NewJournalRepositoryImpl creates a new NDJSON-based journal repository
func NewJournalRepositoryImpl(journalPath string) *JournalRepositoryImpl {
	return &JournalRepositoryImpl{
		journalPath: journalPath,
	}
}

// Append adds a new record to the journal using NDJSON format with file locking
func (r *JournalRepositoryImpl) Append(ctx context.Context, record *repository.JournalRecord) error {
	entry := map[string]interface{}{
		"timestamp":  record.Timestamp,
		"sbi_id":     record.SBIID,
		"turn":       record.Turn,
		"step":       record.Step,
		"status":     record.Status,
		"attempt":    record.Attempt,
		"decision":   record.Decision,
		"elapsed_ms": record.ElapsedMs,
		"error":      record.Error,
		"artifacts":  record.Artifacts,
	}

	// Normalize timestamps
	if entry["timestamp"] == "" {
		entry["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	}

	// Normalize artifacts to ensure it's always an array
	if entry["artifacts"] == nil {
		entry["artifacts"] = []interface{}{}
	}

	// Use NDJSON append with file locking
	if err := fs.AppendNDJSONLine(r.journalPath, entry); err != nil {
		return fmt.Errorf("failed to append journal entry: %w", err)
	}

	return nil
}

// Load retrieves all journal records from NDJSON file
func (r *JournalRepositoryImpl) Load(ctx context.Context) ([]*repository.JournalRecord, error) {
	// Check if file exists
	if _, err := os.Stat(r.journalPath); os.IsNotExist(err) {
		return []*repository.JournalRecord{}, nil
	}

	// Open file for reading
	file, err := os.Open(r.journalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open journal file: %w", err)
	}
	defer file.Close()

	// Read line by line (NDJSON format)
	var records []*repository.JournalRecord
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse JSON line
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Log warning but continue processing (skip corrupted lines)
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Skipping corrupted journal line %d: %v\n", lineNum, err)
			continue
		}

		// Convert to JournalRecord
		record := r.mapToRecord(entry)
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read journal file: %w", err)
	}

	return records, nil
}

// FindByTurn retrieves records for a specific turn
func (r *JournalRepositoryImpl) FindByTurn(ctx context.Context, turn int) ([]*repository.JournalRecord, error) {
	all, err := r.Load(ctx)
	if err != nil {
		return nil, err
	}

	var result []*repository.JournalRecord
	for _, rec := range all {
		if rec.Turn == turn {
			result = append(result, rec)
		}
	}

	return result, nil
}

// FindBySBI retrieves records for a specific SBI
func (r *JournalRepositoryImpl) FindBySBI(ctx context.Context, sbiID string) ([]*repository.JournalRecord, error) {
	all, err := r.Load(ctx)
	if err != nil {
		return nil, err
	}

	var result []*repository.JournalRecord
	for _, rec := range all {
		if rec.SBIID == sbiID {
			result = append(result, rec)
		}
	}

	return result, nil
}

// mapToRecord converts a map entry to a JournalRecord
func (r *JournalRepositoryImpl) mapToRecord(entry map[string]interface{}) *repository.JournalRecord {
	record := &repository.JournalRecord{}

	// Support both "timestamp" and "ts" (for backward compatibility)
	if ts, ok := entry["timestamp"].(string); ok {
		record.Timestamp = ts
	} else if ts, ok := entry["ts"].(string); ok {
		record.Timestamp = ts
	}

	if sbiID, ok := entry["sbi_id"].(string); ok {
		record.SBIID = sbiID
	}

	if turn, ok := entry["turn"].(float64); ok {
		record.Turn = int(turn)
	} else if turn, ok := entry["turn"].(int); ok {
		record.Turn = turn
	}

	if step, ok := entry["step"].(string); ok {
		record.Step = step
	}

	if status, ok := entry["status"].(string); ok {
		record.Status = status
	}

	if attempt, ok := entry["attempt"].(float64); ok {
		record.Attempt = int(attempt)
	} else if attempt, ok := entry["attempt"].(int); ok {
		record.Attempt = attempt
	}

	if decision, ok := entry["decision"].(string); ok {
		record.Decision = decision
	}

	if elapsedMs, ok := entry["elapsed_ms"].(float64); ok {
		record.ElapsedMs = int64(elapsedMs)
	} else if elapsedMs, ok := entry["elapsed_ms"].(int64); ok {
		record.ElapsedMs = elapsedMs
	}

	if errMsg, ok := entry["error"].(string); ok {
		record.Error = errMsg
	}

	if artifacts, ok := entry["artifacts"].([]interface{}); ok {
		record.Artifacts = artifacts
	}

	return record
}

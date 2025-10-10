package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

// JournalRepositoryImpl implements repository.JournalRepository using file-based storage
type JournalRepositoryImpl struct {
	journalPath string
}

// NewJournalRepositoryImpl creates a new file-based journal repository
func NewJournalRepositoryImpl(journalPath string) *JournalRepositoryImpl {
	return &JournalRepositoryImpl{
		journalPath: journalPath,
	}
}

// Append adds a new record to the journal
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

	return r.appendMap(entry)
}

// appendMap appends a map entry to the journal file
func (r *JournalRepositoryImpl) appendMap(entry map[string]interface{}) error {
	// Read existing journal
	var entries []map[string]interface{}
	if data, err := os.ReadFile(r.journalPath); err == nil {
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("failed to parse existing journal: %w", err)
		}
	}

	// Append new entry
	entries = append(entries, entry)

	// Write back atomically
	if err := fs.AtomicWriteJSON(r.journalPath, entries); err != nil {
		return fmt.Errorf("failed to write journal: %w", err)
	}

	return nil
}

// Load retrieves all journal records
func (r *JournalRepositoryImpl) Load(ctx context.Context) ([]*repository.JournalRecord, error) {
	data, err := os.ReadFile(r.journalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*repository.JournalRecord{}, nil
		}
		return nil, fmt.Errorf("failed to read journal: %w", err)
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse journal: %w", err)
	}

	records := make([]*repository.JournalRecord, 0, len(entries))
	for _, entry := range entries {
		record := r.mapToRecord(entry)
		records = append(records, record)
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

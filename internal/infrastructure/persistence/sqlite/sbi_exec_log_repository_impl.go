package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// SBIExecLogRepositoryImpl implements SBIExecLogRepository using SQLite
type SBIExecLogRepositoryImpl struct {
	db *sql.DB
}

// NewSBIExecLogRepository creates a new SBIExecLogRepository implementation
func NewSBIExecLogRepository(db *sql.DB) repository.SBIExecLogRepository {
	return &SBIExecLogRepositoryImpl{db: db}
}

// Save saves a new execution log entry
func (r *SBIExecLogRepositoryImpl) Save(ctx context.Context, log *repository.SBIExecLog) error {
	query := `
		INSERT INTO sbi_exec_logs (sbi_id, turn, step, decision, report_path, executed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err := r.db.ExecContext(ctx, query,
		log.SBIID,
		log.Turn,
		log.Step,
		log.Decision,
		log.ReportPath,
		log.ExecutedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save SBI exec log: %w", err)
	}

	return nil
}

// FindBySBIID retrieves all execution logs for a specific SBI, ordered by turn ASC
func (r *SBIExecLogRepositoryImpl) FindBySBIID(ctx context.Context, sbiID string) ([]*repository.SBIExecLog, error) {
	query := `
		SELECT id, sbi_id, turn, step, decision, report_path, executed_at, created_at
		FROM sbi_exec_logs
		WHERE sbi_id = ?
		ORDER BY turn ASC, step ASC
	`

	rows, err := r.db.QueryContext(ctx, query, sbiID)
	if err != nil {
		return nil, fmt.Errorf("failed to query SBI exec logs: %w", err)
	}
	defer rows.Close()

	var logs []*repository.SBIExecLog
	for rows.Next() {
		log := &repository.SBIExecLog{}
		var decision sql.NullString

		err := rows.Scan(
			&log.ID,
			&log.SBIID,
			&log.Turn,
			&log.Step,
			&decision,
			&log.ReportPath,
			&log.ExecutedAt,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan SBI exec log: %w", err)
		}

		if decision.Valid {
			log.Decision = &decision.String
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating SBI exec logs: %w", err)
	}

	return logs, nil
}

// FindBySBIIDAndTurn retrieves a specific execution log entry
func (r *SBIExecLogRepositoryImpl) FindBySBIIDAndTurn(ctx context.Context, sbiID string, turn int, step string) (*repository.SBIExecLog, error) {
	query := `
		SELECT id, sbi_id, turn, step, decision, report_path, executed_at, created_at
		FROM sbi_exec_logs
		WHERE sbi_id = ? AND turn = ? AND step = ?
	`

	log := &repository.SBIExecLog{}
	var decision sql.NullString

	err := r.db.QueryRowContext(ctx, query, sbiID, turn, step).Scan(
		&log.ID,
		&log.SBIID,
		&log.Turn,
		&log.Step,
		&decision,
		&log.ReportPath,
		&log.ExecutedAt,
		&log.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find SBI exec log: %w", err)
	}

	if decision.Valid {
		log.Decision = &decision.String
	}

	return log, nil
}

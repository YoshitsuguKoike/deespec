package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// SBIRepositoryImpl implements repository.SBIRepository with SQLite
type SBIRepositoryImpl struct {
	db *sql.DB
}

// getDB returns the appropriate database executor from context
func (r *SBIRepositoryImpl) getDB(ctx context.Context) dbExecutor {
	if tx, ok := transaction.GetTxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NewSBIRepository creates a new SQLite-based SBI repository
func NewSBIRepository(db *sql.DB) repository.SBIRepository {
	return &SBIRepositoryImpl{db: db}
}

// Find retrieves an SBI by its ID
func (r *SBIRepositoryImpl) Find(ctx context.Context, id repository.SBIID) (*sbi.SBI, error) {
	query := `
		SELECT id, title, description, status, current_step, parent_pbi_id,
		       estimated_hours, priority, sequence, registered_at, started_at, completed_at,
		       labels, assigned_agent, file_paths,
		       current_turn, current_attempt, max_turns, max_attempts, last_error, artifact_paths,
		       created_at, updated_at
		FROM sbis
		WHERE id = ?
	`

	db := r.getDB(ctx)
	return r.scanSBI(db.QueryRowContext(ctx, query, string(id)))
}

// Save persists an SBI entity
func (r *SBIRepositoryImpl) Save(ctx context.Context, s *sbi.SBI) error {
	metadata := s.Metadata()
	execution := s.ExecutionState()

	// Marshal JSON arrays
	labelsJSON, err := json.Marshal(metadata.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels failed: %w", err)
	}

	filePathsJSON, err := json.Marshal(metadata.FilePaths)
	if err != nil {
		return fmt.Errorf("marshal file paths failed: %w", err)
	}

	artifactPathsJSON, err := json.Marshal(execution.ArtifactPaths)
	if err != nil {
		return fmt.Errorf("marshal artifact paths failed: %w", err)
	}

	// Handle optional parent PBI ID
	var parentPBIID interface{}
	if s.ParentTaskID() != nil {
		parentPBIID = s.ParentTaskID().String()
	}

	// Handle registered_at (NULL if zero value)
	var registeredAt interface{}
	if !metadata.RegisteredAt.IsZero() {
		registeredAt = metadata.RegisteredAt
	}

	// Handle sequence (NULL if zero)
	var sequence interface{}
	if metadata.Sequence > 0 {
		sequence = metadata.Sequence
	}

	// Handle started_at (NULL if not set)
	var startedAt interface{}
	if metadata.StartedAt != nil {
		startedAt = *metadata.StartedAt
	}

	// Handle completed_at (NULL if not set)
	var completedAt interface{}
	if metadata.CompletedAt != nil {
		completedAt = *metadata.CompletedAt
	}

	query := `
		INSERT INTO sbis (id, title, description, status, current_step, parent_pbi_id,
		                  estimated_hours, priority, sequence, registered_at, started_at, completed_at,
		                  labels, assigned_agent, file_paths,
		                  current_turn, current_attempt, max_turns, max_attempts, last_error, artifact_paths,
		                  created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			status = excluded.status,
			current_step = excluded.current_step,
			parent_pbi_id = excluded.parent_pbi_id,
			estimated_hours = excluded.estimated_hours,
			priority = excluded.priority,
			sequence = excluded.sequence,
			registered_at = excluded.registered_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			labels = excluded.labels,
			assigned_agent = excluded.assigned_agent,
			file_paths = excluded.file_paths,
			current_turn = excluded.current_turn,
			current_attempt = excluded.current_attempt,
			max_turns = excluded.max_turns,
			max_attempts = excluded.max_attempts,
			last_error = excluded.last_error,
			artifact_paths = excluded.artifact_paths,
			updated_at = excluded.updated_at
	`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query,
		s.ID().String(), s.Title(), s.Description(),
		string(s.Status()), string(s.CurrentStep()), parentPBIID,
		metadata.EstimatedHours, metadata.Priority, sequence, registeredAt, startedAt, completedAt,
		string(labelsJSON), metadata.AssignedAgent, string(filePathsJSON),
		execution.CurrentTurn.Value(), execution.CurrentAttempt.Value(), execution.MaxTurns, execution.MaxAttempts,
		execution.LastError, string(artifactPathsJSON),
		s.CreatedAt().Value(), s.UpdatedAt().Value(),
	)
	if err != nil {
		return fmt.Errorf("save SBI failed: %w", err)
	}

	return nil
}

// Delete removes an SBI
func (r *SBIRepositoryImpl) Delete(ctx context.Context, id repository.SBIID) error {
	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, "DELETE FROM sbis WHERE id = ?", string(id))
	if err != nil {
		return fmt.Errorf("delete SBI failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected failed: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("SBI not found: %s", id)
	}

	return nil
}

// List retrieves SBIs by filter
func (r *SBIRepositoryImpl) List(ctx context.Context, filter repository.SBIFilter) ([]*sbi.SBI, error) {
	query := `
		SELECT id, title, description, status, current_step, parent_pbi_id,
		       estimated_hours, priority, sequence, registered_at, started_at, completed_at,
		       labels, assigned_agent, file_paths,
		       current_turn, current_attempt, max_turns, max_attempts, last_error, artifact_paths,
		       created_at, updated_at
		FROM sbis
		WHERE 1=1
	`
	args := []interface{}{}

	// Add status filter
	if len(filter.Statuses) > 0 {
		query += " AND status IN ("
		for i, status := range filter.Statuses {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, string(status))
		}
		query += ")"
	}

	// Add parent PBI filter
	if filter.PBIID != nil {
		query += " AND parent_pbi_id = ?"
		args = append(args, string(*filter.PBIID))
	}

	// Add ordering and pagination
	// IMPORTANT: Order by priority DESC, registered_at ASC, sequence ASC for correct task execution order
	query += " ORDER BY priority DESC, registered_at ASC, sequence ASC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list SBIs failed: %w", err)
	}
	defer rows.Close()

	var sbis []*sbi.SBI
	for rows.Next() {
		s, err := r.scanSBIFromRows(rows, ctx)
		if err != nil {
			return nil, err
		}
		sbis = append(sbis, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SBIs failed: %w", err)
	}

	return sbis, nil
}

// FindByPBIID retrieves SBIs that belong to a PBI
func (r *SBIRepositoryImpl) FindByPBIID(ctx context.Context, pbiID repository.PBIID) ([]*sbi.SBI, error) {
	query := `
		SELECT id, title, description, status, current_step, parent_pbi_id,
		       estimated_hours, priority, sequence, registered_at, started_at, completed_at,
		       labels, assigned_agent, file_paths,
		       current_turn, current_attempt, max_turns, max_attempts, last_error, artifact_paths,
		       created_at, updated_at
		FROM sbis
		WHERE parent_pbi_id = ?
		ORDER BY priority DESC, registered_at ASC, sequence ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, string(pbiID))
	if err != nil {
		return nil, fmt.Errorf("find SBIs by PBI ID failed: %w", err)
	}
	defer rows.Close()

	var sbis []*sbi.SBI
	for rows.Next() {
		s, err := r.scanSBIFromRows(rows, ctx)
		if err != nil {
			return nil, err
		}
		sbis = append(sbis, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SBIs failed: %w", err)
	}

	return sbis, nil
}

// scanSBI scans a single SBI from a row
func (r *SBIRepositoryImpl) scanSBI(row *sql.Row) (*sbi.SBI, error) {
	var (
		sbiID             string
		title             string
		description       sql.NullString
		status            string
		currentStep       string
		parentPBIID       sql.NullString
		estimatedHours    float64
		priority          int
		sequence          sql.NullInt64
		registeredAt      sql.NullString
		startedAt         sql.NullString
		completedAt       sql.NullString
		labelsJSON        sql.NullString
		assignedAgent     sql.NullString
		filePathsJSON     sql.NullString
		currentTurn       int
		currentAttempt    int
		maxTurns          int
		maxAttempts       int
		lastError         sql.NullString
		artifactPathsJSON sql.NullString
		createdAt         string
		updatedAt         string
	)

	err := row.Scan(
		&sbiID, &title, &description, &status, &currentStep, &parentPBIID,
		&estimatedHours, &priority, &sequence, &registeredAt, &startedAt, &completedAt,
		&labelsJSON, &assignedAgent, &filePathsJSON,
		&currentTurn, &currentAttempt, &maxTurns, &maxAttempts, &lastError, &artifactPathsJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("SBI not found")
		}
		return nil, fmt.Errorf("scan SBI failed: %w", err)
	}

	// Parse timestamps
	createdAtTime, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updatedAtTime, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}

	return r.reconstructSBI(sbiID, title, description, status, currentStep, parentPBIID,
		estimatedHours, priority, sequence, registeredAt, startedAt, completedAt,
		labelsJSON, assignedAgent, filePathsJSON,
		currentTurn, currentAttempt, maxTurns, maxAttempts, lastError, artifactPathsJSON,
		createdAtTime, updatedAtTime)
}

// scanSBIFromRows scans a single SBI from rows
func (r *SBIRepositoryImpl) scanSBIFromRows(rows *sql.Rows, ctx context.Context) (*sbi.SBI, error) {
	var (
		sbiID             string
		title             string
		description       sql.NullString
		status            string
		currentStep       string
		parentPBIID       sql.NullString
		estimatedHours    float64
		priority          int
		sequence          sql.NullInt64
		registeredAt      sql.NullString
		startedAt         sql.NullString
		completedAt       sql.NullString
		labelsJSON        sql.NullString
		assignedAgent     sql.NullString
		filePathsJSON     sql.NullString
		currentTurn       int
		currentAttempt    int
		maxTurns          int
		maxAttempts       int
		lastError         sql.NullString
		artifactPathsJSON sql.NullString
		createdAt         string
		updatedAt         string
	)

	err := rows.Scan(
		&sbiID, &title, &description, &status, &currentStep, &parentPBIID,
		&estimatedHours, &priority, &sequence, &registeredAt, &startedAt, &completedAt,
		&labelsJSON, &assignedAgent, &filePathsJSON,
		&currentTurn, &currentAttempt, &maxTurns, &maxAttempts, &lastError, &artifactPathsJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan SBI failed: %w", err)
	}

	// Parse timestamps
	createdAtTime, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updatedAtTime, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}

	return r.reconstructSBI(sbiID, title, description, status, currentStep, parentPBIID,
		estimatedHours, priority, sequence, registeredAt, startedAt, completedAt,
		labelsJSON, assignedAgent, filePathsJSON,
		currentTurn, currentAttempt, maxTurns, maxAttempts, lastError, artifactPathsJSON,
		createdAtTime, updatedAtTime)
}

// reconstructSBI reconstructs an SBI entity from database values
func (r *SBIRepositoryImpl) reconstructSBI(
	sbiID, title string,
	description sql.NullString,
	status, currentStep string,
	parentPBIID sql.NullString,
	estimatedHours float64,
	priority int,
	sequence sql.NullInt64,
	registeredAt, startedAt, completedAt sql.NullString,
	labelsJSON, assignedAgent, filePathsJSON sql.NullString,
	currentTurn, currentAttempt, maxTurns, maxAttempts int,
	lastError, artifactPathsJSON sql.NullString,
	createdAt, updatedAt time.Time,
) (*sbi.SBI, error) {
	// Unmarshal JSON arrays
	var labels []string
	if labelsJSON.Valid && labelsJSON.String != "" {
		if err := json.Unmarshal([]byte(labelsJSON.String), &labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels failed: %w", err)
		}
	}

	var filePaths []string
	if filePathsJSON.Valid && filePathsJSON.String != "" {
		if err := json.Unmarshal([]byte(filePathsJSON.String), &filePaths); err != nil {
			return nil, fmt.Errorf("unmarshal file paths failed: %w", err)
		}
	}

	var artifactPaths []string
	if artifactPathsJSON.Valid && artifactPathsJSON.String != "" {
		if err := json.Unmarshal([]byte(artifactPathsJSON.String), &artifactPaths); err != nil {
			return nil, fmt.Errorf("unmarshal artifact paths failed: %w", err)
		}
	}

	// Convert string ID to TaskID
	taskID, err := model.NewTaskIDFromString(sbiID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	// Handle optional parent PBI ID
	var parentPBITaskID *model.TaskID
	if parentPBIID.Valid && parentPBIID.String != "" {
		pbiID, err := model.NewTaskIDFromString(parentPBIID.String)
		if err != nil {
			return nil, fmt.Errorf("invalid parent PBI ID: %w", err)
		}
		parentPBITaskID = &pbiID
	}

	// Parse registered_at timestamp
	var registeredAtTime time.Time
	if registeredAt.Valid && registeredAt.String != "" {
		t, err := parseTime(registeredAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse registered_at failed: %w", err)
		}
		registeredAtTime = t
	}

	// Parse started_at timestamp (nullable)
	var startedAtTime *time.Time
	if startedAt.Valid && startedAt.String != "" {
		t, err := parseTime(startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse started_at failed: %w", err)
		}
		startedAtTime = &t
	}

	// Parse completed_at timestamp (nullable)
	var completedAtTime *time.Time
	if completedAt.Valid && completedAt.String != "" {
		t, err := parseTime(completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at failed: %w", err)
		}
		completedAtTime = &t
	}

	// Reconstruct SBI metadata
	metadata := sbi.SBIMetadata{
		EstimatedHours: estimatedHours,
		Priority:       priority,
		Sequence:       int(sequence.Int64),
		RegisteredAt:   registeredAtTime,
		StartedAt:      startedAtTime,
		CompletedAt:    completedAtTime,
		Labels:         labels,
		AssignedAgent:  assignedAgent.String,
		FilePaths:      filePaths,
	}

	// Reconstruct execution state
	turn, err := model.NewTurnFromInt(currentTurn)
	if err != nil {
		return nil, fmt.Errorf("invalid current turn: %w", err)
	}

	attempt, err := model.NewAttemptFromInt(currentAttempt)
	if err != nil {
		return nil, fmt.Errorf("invalid current attempt: %w", err)
	}

	execution := &sbi.ExecutionState{
		CurrentTurn:    turn,
		CurrentAttempt: attempt,
		MaxTurns:       maxTurns,
		MaxAttempts:    maxAttempts,
		LastError:      lastError.String,
		ArtifactPaths:  artifactPaths,
	}

	return sbi.ReconstructSBI(
		taskID,
		title,
		description.String,
		model.Status(status),
		model.Step(currentStep),
		parentPBITaskID,
		metadata,
		execution,
		createdAt,
		updatedAt,
	), nil
}

// GetNextSequence returns the next sequence number for SBI registration
// This method is used to guarantee registration order
func (r *SBIRepositoryImpl) GetNextSequence(ctx context.Context) (int, error) {
	query := `SELECT COALESCE(MAX(sequence), 0) + 1 FROM sbis`

	db := r.getDB(ctx)
	var nextSeq int
	err := db.QueryRowContext(ctx, query).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("get next sequence failed: %w", err)
	}

	return nextSeq, nil
}

// ResetSBIState resets an SBI to allow re-execution
func (r *SBIRepositoryImpl) ResetSBIState(ctx context.Context, id repository.SBIID, toStatus string) error {
	query := `UPDATE sbis SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, toStatus, string(id))
	if err != nil {
		return fmt.Errorf("reset SBI state failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected failed: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("SBI not found: %s", id)
	}

	return nil
}

// GetDependencies retrieves the list of SBI IDs that the given SBI depends on
func (r *SBIRepositoryImpl) GetDependencies(ctx context.Context, sbiID repository.SBIID) ([]string, error) {
	query := `
		SELECT depends_on_sbi_id
		FROM sbi_dependencies
		WHERE sbi_id = ?
		ORDER BY created_at ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, string(sbiID))
	if err != nil {
		return nil, fmt.Errorf("get dependencies failed: %w", err)
	}
	defer rows.Close()

	var dependencies []string
	for rows.Next() {
		var dependsOnID string
		if err := rows.Scan(&dependsOnID); err != nil {
			return nil, fmt.Errorf("scan dependency failed: %w", err)
		}
		dependencies = append(dependencies, dependsOnID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies failed: %w", err)
	}

	return dependencies, nil
}

// GetDependents retrieves the list of SBI IDs that depend on the given SBI
func (r *SBIRepositoryImpl) GetDependents(ctx context.Context, sbiID repository.SBIID) ([]string, error) {
	query := `
		SELECT sbi_id
		FROM sbi_dependencies
		WHERE depends_on_sbi_id = ?
		ORDER BY created_at ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, string(sbiID))
	if err != nil {
		return nil, fmt.Errorf("get dependents failed: %w", err)
	}
	defer rows.Close()

	var dependents []string
	for rows.Next() {
		var sbiIDStr string
		if err := rows.Scan(&sbiIDStr); err != nil {
			return nil, fmt.Errorf("scan dependent failed: %w", err)
		}
		dependents = append(dependents, sbiIDStr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependents failed: %w", err)
	}

	return dependents, nil
}

// SaveDependencies persists the dependencies for an SBI
// This replaces all existing dependencies with the provided list
func (r *SBIRepositoryImpl) SaveDependencies(ctx context.Context, sbiID repository.SBIID, dependsOn []string) error {
	db := r.getDB(ctx)

	// Delete existing dependencies first
	deleteQuery := `DELETE FROM sbi_dependencies WHERE sbi_id = ?`
	_, err := db.ExecContext(ctx, deleteQuery, string(sbiID))
	if err != nil {
		return fmt.Errorf("delete existing dependencies failed: %w", err)
	}

	// Insert new dependencies
	if len(dependsOn) > 0 {
		insertQuery := `INSERT INTO sbi_dependencies (sbi_id, depends_on_sbi_id) VALUES (?, ?)`
		for _, dependsOnID := range dependsOn {
			_, err := db.ExecContext(ctx, insertQuery, string(sbiID), dependsOnID)
			if err != nil {
				return fmt.Errorf("insert dependency failed: %w", err)
			}
		}
	}

	return nil
}

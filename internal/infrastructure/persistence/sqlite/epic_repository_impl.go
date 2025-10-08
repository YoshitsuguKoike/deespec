package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// dbExecutor is an interface for executing database queries
// Both *sql.DB and *sql.Tx implement this interface
type dbExecutor interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// EPICRepositoryImpl implements repository.EPICRepository with SQLite
type EPICRepositoryImpl struct {
	db *sql.DB
}

// getDB returns the appropriate database executor from context
// If a transaction exists in the context, it returns the transaction
// Otherwise, it returns the database connection
func (r *EPICRepositoryImpl) getDB(ctx context.Context) dbExecutor {
	if tx, ok := transaction.GetTxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NewEPICRepository creates a new SQLite-based EPIC repository
func NewEPICRepository(db *sql.DB) repository.EPICRepository {
	return &EPICRepositoryImpl{db: db}
}

// Find retrieves an EPIC by its ID
func (r *EPICRepositoryImpl) Find(ctx context.Context, id repository.EPICID) (*epic.EPIC, error) {
	query := `
		SELECT id, title, description, status, current_step,
		       estimated_story_points, priority, labels, assigned_agent,
		       created_at, updated_at
		FROM epics
		WHERE id = ?
	`

	db := r.getDB(ctx)
	return r.scanEPIC(db.QueryRowContext(ctx, query, string(id)))
}

// Save persists an EPIC entity
func (r *EPICRepositoryImpl) Save(ctx context.Context, e *epic.EPIC) error {
	metadata := e.Metadata()

	// Marshal JSON arrays
	labelsJSON, err := json.Marshal(metadata.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels failed: %w", err)
	}

	query := `
		INSERT INTO epics (id, title, description, status, current_step,
		                   estimated_story_points, priority, labels, assigned_agent,
		                   created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			status = excluded.status,
			current_step = excluded.current_step,
			estimated_story_points = excluded.estimated_story_points,
			priority = excluded.priority,
			labels = excluded.labels,
			assigned_agent = excluded.assigned_agent,
			updated_at = excluded.updated_at
	`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query,
		e.ID().String(), e.Title(), e.Description(),
		string(e.Status()), string(e.CurrentStep()),
		metadata.EstimatedStoryPoints, metadata.Priority, string(labelsJSON), metadata.AssignedAgent,
		e.CreatedAt().Value(), e.UpdatedAt().Value(),
	)
	if err != nil {
		return fmt.Errorf("save EPIC failed: %w", err)
	}

	// Update PBI relationships
	if err := r.updatePBIRelationships(ctx, e); err != nil {
		return fmt.Errorf("update PBI relationships failed: %w", err)
	}

	return nil
}

// Delete removes an EPIC
func (r *EPICRepositoryImpl) Delete(ctx context.Context, id repository.EPICID) error {
	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, "DELETE FROM epics WHERE id = ?", string(id))
	if err != nil {
		return fmt.Errorf("delete EPIC failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected failed: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("EPIC not found: %s", id)
	}

	return nil
}

// List retrieves EPICs by filter
func (r *EPICRepositoryImpl) List(ctx context.Context, filter repository.EPICFilter) ([]*epic.EPIC, error) {
	query := `
		SELECT id, title, description, status, current_step,
		       estimated_story_points, priority, labels, assigned_agent,
		       created_at, updated_at
		FROM epics
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

	// Add ordering and pagination
	query += " ORDER BY created_at DESC"
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
		return nil, fmt.Errorf("list EPICs failed: %w", err)
	}
	defer rows.Close()

	var epics []*epic.EPIC
	for rows.Next() {
		e, err := r.scanEPICFromRows(rows, ctx)
		if err != nil {
			return nil, err
		}
		epics = append(epics, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate EPICs failed: %w", err)
	}

	return epics, nil
}

// FindByPBIID retrieves the parent EPIC of a PBI
func (r *EPICRepositoryImpl) FindByPBIID(ctx context.Context, pbiID repository.PBIID) (*epic.EPIC, error) {
	query := `
		SELECT e.id, e.title, e.description, e.status, e.current_step,
		       e.estimated_story_points, e.priority, e.labels, e.assigned_agent,
		       e.created_at, e.updated_at
		FROM epics e
		INNER JOIN epic_pbis ep ON e.id = ep.epic_id
		WHERE ep.pbi_id = ?
	`

	db := r.getDB(ctx)
	return r.scanEPIC(db.QueryRowContext(ctx, query, string(pbiID)))
}

// scanEPIC scans a single EPIC from a row
func (r *EPICRepositoryImpl) scanEPIC(row *sql.Row) (*epic.EPIC, error) {
	var (
		epicID               string
		title                string
		description          sql.NullString
		status               string
		currentStep          string
		estimatedStoryPoints int
		priority             int
		labelsJSON           sql.NullString
		assignedAgent        sql.NullString
		createdAt            string
		updatedAt            string
	)

	err := row.Scan(
		&epicID, &title, &description, &status, &currentStep,
		&estimatedStoryPoints, &priority, &labelsJSON, &assignedAgent,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("EPIC not found")
		}
		return nil, fmt.Errorf("scan EPIC failed: %w", err)
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

	return r.reconstructEPIC(epicID, title, description, status, currentStep,
		estimatedStoryPoints, priority, labelsJSON, assignedAgent,
		createdAtTime, updatedAtTime, context.Background())
}

// scanEPICFromRows scans a single EPIC from rows
func (r *EPICRepositoryImpl) scanEPICFromRows(rows *sql.Rows, ctx context.Context) (*epic.EPIC, error) {
	var (
		epicID               string
		title                string
		description          sql.NullString
		status               string
		currentStep          string
		estimatedStoryPoints int
		priority             int
		labelsJSON           sql.NullString
		assignedAgent        sql.NullString
		createdAt            string
		updatedAt            string
	)

	err := rows.Scan(
		&epicID, &title, &description, &status, &currentStep,
		&estimatedStoryPoints, &priority, &labelsJSON, &assignedAgent,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan EPIC failed: %w", err)
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

	return r.reconstructEPIC(epicID, title, description, status, currentStep,
		estimatedStoryPoints, priority, labelsJSON, assignedAgent,
		createdAtTime, updatedAtTime, ctx)
}

// reconstructEPIC reconstructs an EPIC entity from database values
func (r *EPICRepositoryImpl) reconstructEPIC(
	epicID, title string,
	description sql.NullString,
	status, currentStep string,
	estimatedStoryPoints, priority int,
	labelsJSON, assignedAgent sql.NullString,
	createdAt, updatedAt time.Time,
	ctx context.Context,
) (*epic.EPIC, error) {
	// Unmarshal JSON arrays
	var labels []string
	if labelsJSON.Valid && labelsJSON.String != "" {
		if err := json.Unmarshal([]byte(labelsJSON.String), &labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels failed: %w", err)
		}
	}

	// Convert string ID to TaskID
	taskID, err := model.NewTaskIDFromString(epicID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	// Reconstruct EPIC metadata
	metadata := epic.EPICMetadata{
		EstimatedStoryPoints: estimatedStoryPoints,
		Priority:             priority,
		Labels:               labels,
		AssignedAgent:        assignedAgent.String,
	}

	// Query child PBI IDs
	pbiIDs, err := r.queryPBIIDsByEPIC(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("query PBI IDs failed: %w", err)
	}

	return epic.ReconstructEPIC(
		taskID,
		title,
		description.String,
		model.Status(status),
		model.Step(currentStep),
		pbiIDs,
		metadata,
		createdAt,
		updatedAt,
	), nil
}

// parseTime parses a time string in RFC3339 format
func parseTime(timeStr string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Try SQLite datetime format
		t, err = time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse time failed: %w", err)
		}
	}
	return t, nil
}

// queryPBIIDsByEPIC queries PBI IDs that belong to an EPIC
func (r *EPICRepositoryImpl) queryPBIIDsByEPIC(ctx context.Context, epicID string) ([]model.TaskID, error) {
	query := `
		SELECT pbi_id FROM epic_pbis
		WHERE epic_id = ?
		ORDER BY position
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, epicID)
	if err != nil {
		return nil, fmt.Errorf("query PBI IDs failed: %w", err)
	}
	defer rows.Close()

	var pbiIDs []model.TaskID
	for rows.Next() {
		var pbiIDStr string
		if err := rows.Scan(&pbiIDStr); err != nil {
			return nil, fmt.Errorf("scan PBI ID failed: %w", err)
		}

		pbiID, err := model.NewTaskIDFromString(pbiIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid PBI ID: %w", err)
		}
		pbiIDs = append(pbiIDs, pbiID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PBI IDs failed: %w", err)
	}

	return pbiIDs, nil
}

// updatePBIRelationships updates the epic_pbis relationship table
func (r *EPICRepositoryImpl) updatePBIRelationships(ctx context.Context, e *epic.EPIC) error {
	db := r.getDB(ctx)

	// Delete existing relationships
	_, err := db.ExecContext(ctx, "DELETE FROM epic_pbis WHERE epic_id = ?", e.ID().String())
	if err != nil {
		return fmt.Errorf("delete old relationships failed: %w", err)
	}

	// Insert new relationships
	pbiIDs := e.PBIIDs()
	for i, pbiID := range pbiIDs {
		_, err := db.ExecContext(ctx,
			"INSERT INTO epic_pbis (epic_id, pbi_id, position) VALUES (?, ?, ?)",
			e.ID().String(), pbiID.String(), i,
		)
		if err != nil {
			return fmt.Errorf("insert relationship failed: %w", err)
		}
	}

	return nil
}

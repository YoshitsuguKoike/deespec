package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// PBIRepositoryImpl implements repository.PBIRepository with SQLite
type PBIRepositoryImpl struct {
	db *sql.DB
}

// getDB returns the appropriate database executor from context
func (r *PBIRepositoryImpl) getDB(ctx context.Context) dbExecutor {
	if tx, ok := transaction.GetTxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NewPBIRepository creates a new SQLite-based PBI repository
func NewPBIRepository(db *sql.DB) repository.PBIRepository {
	return &PBIRepositoryImpl{db: db}
}

// Find retrieves a PBI by its ID
func (r *PBIRepositoryImpl) Find(ctx context.Context, id repository.PBIID) (*pbi.PBI, error) {
	query := `
		SELECT id, title, description, status, current_step, parent_epic_id,
		       story_points, priority, labels, assigned_agent, acceptance_criteria,
		       created_at, updated_at
		FROM pbis
		WHERE id = ?
	`

	db := r.getDB(ctx)
	return r.scanPBI(db.QueryRowContext(ctx, query, string(id)))
}

// Save persists a PBI entity
func (r *PBIRepositoryImpl) Save(ctx context.Context, p *pbi.PBI) error {
	metadata := p.Metadata()

	// Marshal JSON arrays
	labelsJSON, err := json.Marshal(metadata.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels failed: %w", err)
	}

	acceptanceCriteriaJSON, err := json.Marshal(metadata.AcceptanceCriteria)
	if err != nil {
		return fmt.Errorf("marshal acceptance criteria failed: %w", err)
	}

	// Handle optional parent EPIC ID
	var parentEPICID interface{}
	if p.ParentTaskID() != nil {
		parentEPICID = p.ParentTaskID().String()
	}

	query := `
		INSERT INTO pbis (id, title, description, status, current_step, parent_epic_id,
		                  story_points, priority, labels, assigned_agent, acceptance_criteria,
		                  created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			status = excluded.status,
			current_step = excluded.current_step,
			parent_epic_id = excluded.parent_epic_id,
			story_points = excluded.story_points,
			priority = excluded.priority,
			labels = excluded.labels,
			assigned_agent = excluded.assigned_agent,
			acceptance_criteria = excluded.acceptance_criteria,
			updated_at = excluded.updated_at
	`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query,
		p.ID().String(), p.Title(), p.Description(),
		string(p.Status()), string(p.CurrentStep()), parentEPICID,
		metadata.StoryPoints, metadata.Priority, string(labelsJSON), metadata.AssignedAgent, string(acceptanceCriteriaJSON),
		p.CreatedAt().Value(), p.UpdatedAt().Value(),
	)
	if err != nil {
		return fmt.Errorf("save PBI failed: %w", err)
	}

	// Update SBI relationships
	if err := r.updateSBIRelationships(ctx, p); err != nil {
		return fmt.Errorf("update SBI relationships failed: %w", err)
	}

	return nil
}

// Delete removes a PBI
func (r *PBIRepositoryImpl) Delete(ctx context.Context, id repository.PBIID) error {
	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, "DELETE FROM pbis WHERE id = ?", string(id))
	if err != nil {
		return fmt.Errorf("delete PBI failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected failed: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("PBI not found: %s", id)
	}

	return nil
}

// List retrieves PBIs by filter
func (r *PBIRepositoryImpl) List(ctx context.Context, filter repository.PBIFilter) ([]*pbi.PBI, error) {
	query := `
		SELECT id, title, description, status, current_step, parent_epic_id,
		       story_points, priority, labels, assigned_agent, acceptance_criteria,
		       created_at, updated_at
		FROM pbis
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

	// Add parent EPIC filter
	if filter.EPICID != nil {
		query += " AND parent_epic_id = ?"
		args = append(args, string(*filter.EPICID))
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
		return nil, fmt.Errorf("list PBIs failed: %w", err)
	}
	defer rows.Close()

	var pbis []*pbi.PBI
	for rows.Next() {
		p, err := r.scanPBIFromRows(rows, ctx)
		if err != nil {
			return nil, err
		}
		pbis = append(pbis, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PBIs failed: %w", err)
	}

	return pbis, nil
}

// FindByEPICID retrieves PBIs that belong to an EPIC
func (r *PBIRepositoryImpl) FindByEPICID(ctx context.Context, epicID repository.EPICID) ([]*pbi.PBI, error) {
	query := `
		SELECT id, title, description, status, current_step, parent_epic_id,
		       story_points, priority, labels, assigned_agent, acceptance_criteria,
		       created_at, updated_at
		FROM pbis
		WHERE parent_epic_id = ?
		ORDER BY created_at ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, string(epicID))
	if err != nil {
		return nil, fmt.Errorf("find PBIs by EPIC ID failed: %w", err)
	}
	defer rows.Close()

	var pbis []*pbi.PBI
	for rows.Next() {
		p, err := r.scanPBIFromRows(rows, ctx)
		if err != nil {
			return nil, err
		}
		pbis = append(pbis, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PBIs failed: %w", err)
	}

	return pbis, nil
}

// FindBySBIID retrieves the parent PBI of an SBI
func (r *PBIRepositoryImpl) FindBySBIID(ctx context.Context, sbiID repository.SBIID) (*pbi.PBI, error) {
	query := `
		SELECT p.id, p.title, p.description, p.status, p.current_step, p.parent_epic_id,
		       p.story_points, p.priority, p.labels, p.assigned_agent, p.acceptance_criteria,
		       p.created_at, p.updated_at
		FROM pbis p
		INNER JOIN pbi_sbis ps ON p.id = ps.pbi_id
		WHERE ps.sbi_id = ?
	`

	db := r.getDB(ctx)
	return r.scanPBI(db.QueryRowContext(ctx, query, string(sbiID)))
}

// scanPBI scans a single PBI from a row
func (r *PBIRepositoryImpl) scanPBI(row *sql.Row) (*pbi.PBI, error) {
	var (
		pbiID                  string
		title                  string
		description            sql.NullString
		status                 string
		currentStep            string
		parentEPICID           sql.NullString
		storyPoints            int
		priority               int
		labelsJSON             sql.NullString
		assignedAgent          sql.NullString
		acceptanceCriteriaJSON sql.NullString
		createdAt              string
		updatedAt              string
	)

	err := row.Scan(
		&pbiID, &title, &description, &status, &currentStep, &parentEPICID,
		&storyPoints, &priority, &labelsJSON, &assignedAgent, &acceptanceCriteriaJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("PBI not found")
		}
		return nil, fmt.Errorf("scan PBI failed: %w", err)
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

	return r.reconstructPBI(pbiID, title, description, status, currentStep, parentEPICID,
		storyPoints, priority, labelsJSON, assignedAgent, acceptanceCriteriaJSON,
		createdAtTime, updatedAtTime, context.Background())
}

// scanPBIFromRows scans a single PBI from rows
func (r *PBIRepositoryImpl) scanPBIFromRows(rows *sql.Rows, ctx context.Context) (*pbi.PBI, error) {
	var (
		pbiID                  string
		title                  string
		description            sql.NullString
		status                 string
		currentStep            string
		parentEPICID           sql.NullString
		storyPoints            int
		priority               int
		labelsJSON             sql.NullString
		assignedAgent          sql.NullString
		acceptanceCriteriaJSON sql.NullString
		createdAt              string
		updatedAt              string
	)

	err := rows.Scan(
		&pbiID, &title, &description, &status, &currentStep, &parentEPICID,
		&storyPoints, &priority, &labelsJSON, &assignedAgent, &acceptanceCriteriaJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan PBI failed: %w", err)
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

	return r.reconstructPBI(pbiID, title, description, status, currentStep, parentEPICID,
		storyPoints, priority, labelsJSON, assignedAgent, acceptanceCriteriaJSON,
		createdAtTime, updatedAtTime, ctx)
}

// reconstructPBI reconstructs a PBI entity from database values
func (r *PBIRepositoryImpl) reconstructPBI(
	pbiID, title string,
	description sql.NullString,
	status, currentStep string,
	parentEPICID sql.NullString,
	storyPoints, priority int,
	labelsJSON, assignedAgent, acceptanceCriteriaJSON sql.NullString,
	createdAt, updatedAt time.Time,
	ctx context.Context,
) (*pbi.PBI, error) {
	// Unmarshal JSON arrays
	var labels []string
	if labelsJSON.Valid && labelsJSON.String != "" {
		if err := json.Unmarshal([]byte(labelsJSON.String), &labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels failed: %w", err)
		}
	}

	var acceptanceCriteria []string
	if acceptanceCriteriaJSON.Valid && acceptanceCriteriaJSON.String != "" {
		if err := json.Unmarshal([]byte(acceptanceCriteriaJSON.String), &acceptanceCriteria); err != nil {
			return nil, fmt.Errorf("unmarshal acceptance criteria failed: %w", err)
		}
	}

	// Convert string ID to TaskID
	taskID, err := model.NewTaskIDFromString(pbiID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	// Handle optional parent EPIC ID
	var parentEPICTaskID *model.TaskID
	if parentEPICID.Valid && parentEPICID.String != "" {
		epicID, err := model.NewTaskIDFromString(parentEPICID.String)
		if err != nil {
			return nil, fmt.Errorf("invalid parent EPIC ID: %w", err)
		}
		parentEPICTaskID = &epicID
	}

	// Reconstruct PBI metadata
	metadata := pbi.PBIMetadata{
		StoryPoints:        storyPoints,
		Priority:           priority,
		Labels:             labels,
		AssignedAgent:      assignedAgent.String,
		AcceptanceCriteria: acceptanceCriteria,
	}

	// Query child SBI IDs
	sbiIDs, err := r.querySBIIDsByPBI(ctx, pbiID)
	if err != nil {
		return nil, fmt.Errorf("query SBI IDs failed: %w", err)
	}

	return pbi.ReconstructPBI(
		taskID,
		title,
		description.String,
		model.Status(status),
		model.Step(currentStep),
		parentEPICTaskID,
		sbiIDs,
		metadata,
		createdAt,
		updatedAt,
	), nil
}

// querySBIIDsByPBI queries SBI IDs that belong to a PBI
func (r *PBIRepositoryImpl) querySBIIDsByPBI(ctx context.Context, pbiID string) ([]model.TaskID, error) {
	query := `
		SELECT sbi_id FROM pbi_sbis
		WHERE pbi_id = ?
		ORDER BY position
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, pbiID)
	if err != nil {
		return nil, fmt.Errorf("query SBI IDs failed: %w", err)
	}
	defer rows.Close()

	var sbiIDs []model.TaskID
	for rows.Next() {
		var sbiIDStr string
		if err := rows.Scan(&sbiIDStr); err != nil {
			return nil, fmt.Errorf("scan SBI ID failed: %w", err)
		}

		sbiID, err := model.NewTaskIDFromString(sbiIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SBI ID: %w", err)
		}
		sbiIDs = append(sbiIDs, sbiID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SBI IDs failed: %w", err)
	}

	return sbiIDs, nil
}

// updateSBIRelationships updates the pbi_sbis relationship table
func (r *PBIRepositoryImpl) updateSBIRelationships(ctx context.Context, p *pbi.PBI) error {
	db := r.getDB(ctx)

	// Delete existing relationships
	_, err := db.ExecContext(ctx, "DELETE FROM pbi_sbis WHERE pbi_id = ?", p.ID().String())
	if err != nil {
		return fmt.Errorf("delete old relationships failed: %w", err)
	}

	// Insert new relationships
	sbiIDs := p.SBIIDs()
	for i, sbiID := range sbiIDs {
		_, err := db.ExecContext(ctx,
			"INSERT INTO pbi_sbis (pbi_id, sbi_id, position) VALUES (?, ?, ?)",
			p.ID().String(), sbiID.String(), i,
		)
		if err != nil {
			return fmt.Errorf("insert relationship failed: %w", err)
		}
	}

	return nil
}

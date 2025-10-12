package persistence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	_ "github.com/mattn/go-sqlite3"
)

// PBISQLiteRepository implements pbi.Repository with SQLite + Markdown files
type PBISQLiteRepository struct {
	db       *sql.DB
	rootPath string
}

// NewPBISQLiteRepository creates a new PBISQLiteRepository
func NewPBISQLiteRepository(db *sql.DB, rootPath string) *PBISQLiteRepository {
	return &PBISQLiteRepository{
		db:       db,
		rootPath: rootPath,
	}
}

// Save saves a PBI with its Markdown body
func (r *PBISQLiteRepository) Save(p *pbi.PBI, body string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Save metadata to database
	_, err = tx.Exec(`
		INSERT INTO pbis (
			id, title, status, story_points, priority,
			parent_epic_id, current_step, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			story_points = excluded.story_points,
			priority = excluded.priority,
			parent_epic_id = excluded.parent_epic_id,
			updated_at = excluded.updated_at
	`,
		p.ID, p.Title, string(p.Status), p.EstimatedStoryPoints,
		p.Priority, nullString(p.ParentEpicID), "planning",
		p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to save PBI metadata: %w", err)
	}

	// 2. Save Markdown file
	pbiDir := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", p.ID)
	if err := os.MkdirAll(pbiDir, 0755); err != nil {
		return fmt.Errorf("failed to create PBI directory: %w", err)
	}

	mdPath := filepath.Join(pbiDir, "pbi.md")
	if err := os.WriteFile(mdPath, []byte(body), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown file: %w", err)
	}

	return tx.Commit()
}

// FindByID retrieves a PBI by ID (metadata only)
func (r *PBISQLiteRepository) FindByID(id string) (*pbi.PBI, error) {
	var p pbi.PBI
	var status string
	var priority int
	var parentEpicID sql.NullString
	var createdAt, updatedAt string

	err := r.db.QueryRow(`
		SELECT id, title, status, story_points, priority,
		       parent_epic_id, created_at, updated_at
		FROM pbis
		WHERE id = ?
	`, id).Scan(
		&p.ID, &p.Title, &status, &p.EstimatedStoryPoints,
		&priority, &parentEpicID, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("PBI not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find PBI: %w", err)
	}

	// Parse status
	p.Status = pbi.Status(status)

	// Parse priority
	p.Priority = pbi.Priority(priority)

	// Parse parent epic ID
	if parentEpicID.Valid {
		p.ParentEpicID = parentEpicID.String
	}

	// Parse timestamps
	p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	p.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return &p, nil
}

// GetBody retrieves the Markdown body from file
func (r *PBISQLiteRepository) GetBody(id string) (string, error) {
	mdPath := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", id, "pbi.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Markdown file: %w", err)
	}
	return string(data), nil
}

// FindAll retrieves all PBIs (metadata only)
func (r *PBISQLiteRepository) FindAll() ([]*pbi.PBI, error) {
	rows, err := r.db.Query(`
		SELECT id, title, status, story_points, priority,
		       parent_epic_id, created_at, updated_at
		FROM pbis
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all PBIs: %w", err)
	}
	defer rows.Close()

	return r.scanPBIs(rows)
}

// FindByStatus retrieves PBIs by status (metadata only)
func (r *PBISQLiteRepository) FindByStatus(status pbi.Status) ([]*pbi.PBI, error) {
	rows, err := r.db.Query(`
		SELECT id, title, status, story_points, priority,
		       parent_epic_id, created_at, updated_at
		FROM pbis
		WHERE status = ?
		ORDER BY created_at DESC
	`, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to query PBIs by status: %w", err)
	}
	defer rows.Close()

	return r.scanPBIs(rows)
}

// Delete deletes a PBI (both database and Markdown file)
func (r *PBISQLiteRepository) Delete(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Delete from database
	_, err = tx.Exec(`DELETE FROM pbis WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete PBI from database: %w", err)
	}

	// 2. Delete Markdown directory
	pbiDir := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", id)
	if err := os.RemoveAll(pbiDir); err != nil {
		return fmt.Errorf("failed to delete PBI directory: %w", err)
	}

	return tx.Commit()
}

// Exists checks if a PBI exists
func (r *PBISQLiteRepository) Exists(id string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM pbis WHERE id = ?`, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check PBI existence: %w", err)
	}
	return count > 0, nil
}

// scanPBIs scans multiple PBIs from rows
func (r *PBISQLiteRepository) scanPBIs(rows *sql.Rows) ([]*pbi.PBI, error) {
	var pbis []*pbi.PBI

	for rows.Next() {
		var p pbi.PBI
		var status string
		var priority int
		var parentEpicID sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(
			&p.ID, &p.Title, &status, &p.EstimatedStoryPoints,
			&priority, &parentEpicID, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan PBI: %w", err)
		}

		// Parse status
		p.Status = pbi.Status(status)

		// Parse priority
		p.Priority = pbi.Priority(priority)

		// Parse parent epic ID
		if parentEpicID.Valid {
			p.ParentEpicID = parentEpicID.String
		}

		// Parse timestamps
		p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		p.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}

		pbis = append(pbis, &p)
	}

	return pbis, rows.Err()
}

// nullString returns a sql.NullString
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

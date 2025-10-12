package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/YoshitsuguKoike/deespec/internal/app/config"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// LabelRepositoryImpl implements repository.LabelRepository with SQLite
type LabelRepositoryImpl struct {
	db          *sql.DB
	labelConfig appconfig.LabelConfig
}

// getDB returns the appropriate database executor from context
func (r *LabelRepositoryImpl) getDB(ctx context.Context) dbExecutor {
	if tx, ok := transaction.GetTxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NewLabelRepository creates a new SQLite-based Label repository
func NewLabelRepository(db *sql.DB, labelConfig appconfig.LabelConfig) repository.LabelRepository {
	return &LabelRepositoryImpl{
		db:          db,
		labelConfig: labelConfig,
	}
}

// Save persists a Label entity
func (r *LabelRepositoryImpl) Save(ctx context.Context, lbl *label.Label) error {
	// Marshal JSON fields
	templatePathsJSON, err := json.Marshal(lbl.TemplatePaths())
	if err != nil {
		return fmt.Errorf("marshal template_paths failed: %w", err)
	}

	contentHashesJSON, err := json.Marshal(lbl.ContentHashes())
	if err != nil {
		return fmt.Errorf("marshal content_hashes failed: %w", err)
	}

	query := `
		INSERT INTO labels (name, description, template_paths, content_hashes,
		                    parent_label_id, color, priority, is_active,
		                    line_count, last_synced_at, metadata,
		                    created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query,
		lbl.Name(), lbl.Description(), string(templatePathsJSON), string(contentHashesJSON),
		lbl.ParentLabelID(), lbl.Color(), lbl.Priority(), lbl.IsActive(),
		lbl.LineCount(), lbl.LastSyncedAt(), lbl.Metadata(),
		lbl.CreatedAt(), lbl.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("save label failed: %w", err)
	}

	// Set the generated ID back to the label
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id failed: %w", err)
	}
	lbl.SetID(int(id))

	return nil
}

// FindByID retrieves a Label by its ID
func (r *LabelRepositoryImpl) FindByID(ctx context.Context, id int) (*label.Label, error) {
	query := `
		SELECT id, name, description, template_paths, content_hashes,
		       parent_label_id, color, priority, is_active,
		       line_count, last_synced_at, metadata,
		       created_at, updated_at
		FROM labels
		WHERE id = ?
	`

	db := r.getDB(ctx)
	return r.scanLabel(db.QueryRowContext(ctx, query, id))
}

// FindByName retrieves a Label by its name
func (r *LabelRepositoryImpl) FindByName(ctx context.Context, name string) (*label.Label, error) {
	query := `
		SELECT id, name, description, template_paths, content_hashes,
		       parent_label_id, color, priority, is_active,
		       line_count, last_synced_at, metadata,
		       created_at, updated_at
		FROM labels
		WHERE name = ?
	`

	db := r.getDB(ctx)
	return r.scanLabel(db.QueryRowContext(ctx, query, name))
}

// FindAll retrieves all Labels
func (r *LabelRepositoryImpl) FindAll(ctx context.Context) ([]*label.Label, error) {
	query := `
		SELECT id, name, description, template_paths, content_hashes,
		       parent_label_id, color, priority, is_active,
		       line_count, last_synced_at, metadata,
		       created_at, updated_at
		FROM labels
		ORDER BY priority DESC, name ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all labels failed: %w", err)
	}
	defer rows.Close()

	return r.scanLabels(rows)
}

// FindActive retrieves all active Labels
func (r *LabelRepositoryImpl) FindActive(ctx context.Context) ([]*label.Label, error) {
	query := `
		SELECT id, name, description, template_paths, content_hashes,
		       parent_label_id, color, priority, is_active,
		       line_count, last_synced_at, metadata,
		       created_at, updated_at
		FROM labels
		WHERE is_active = 1
		ORDER BY priority DESC, name ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query active labels failed: %w", err)
	}
	defer rows.Close()

	return r.scanLabels(rows)
}

// Update updates a Label entity
func (r *LabelRepositoryImpl) Update(ctx context.Context, lbl *label.Label) error {
	// Marshal JSON fields
	templatePathsJSON, err := json.Marshal(lbl.TemplatePaths())
	if err != nil {
		return fmt.Errorf("marshal template_paths failed: %w", err)
	}

	contentHashesJSON, err := json.Marshal(lbl.ContentHashes())
	if err != nil {
		return fmt.Errorf("marshal content_hashes failed: %w", err)
	}

	query := `
		UPDATE labels
		SET name = ?, description = ?, template_paths = ?, content_hashes = ?,
		    parent_label_id = ?, color = ?, priority = ?, is_active = ?,
		    line_count = ?, last_synced_at = ?, metadata = ?, updated_at = ?
		WHERE id = ?
	`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query,
		lbl.Name(), lbl.Description(), string(templatePathsJSON), string(contentHashesJSON),
		lbl.ParentLabelID(), lbl.Color(), lbl.Priority(), lbl.IsActive(),
		lbl.LineCount(), lbl.LastSyncedAt(), lbl.Metadata(), lbl.UpdatedAt(),
		lbl.ID(),
	)
	if err != nil {
		return fmt.Errorf("update label failed: %w", err)
	}

	return nil
}

// Delete removes a Label entity
func (r *LabelRepositoryImpl) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM labels WHERE id = ?`

	db := r.getDB(ctx)
	_, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete label failed: %w", err)
	}

	return nil
}

// FindChildren retrieves child labels of a parent label
func (r *LabelRepositoryImpl) FindChildren(ctx context.Context, parentID int) ([]*label.Label, error) {
	query := `
		SELECT id, name, description, template_paths, content_hashes,
		       parent_label_id, color, priority, is_active,
		       line_count, last_synced_at, metadata,
		       created_at, updated_at
		FROM labels
		WHERE parent_label_id = ?
		ORDER BY name ASC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("query child labels failed: %w", err)
	}
	defer rows.Close()

	return r.scanLabels(rows)
}

// FindByParentID retrieves labels by parent ID (nil for root labels)
func (r *LabelRepositoryImpl) FindByParentID(ctx context.Context, parentID *int) ([]*label.Label, error) {
	var query string
	var args []interface{}

	if parentID == nil {
		query = `
			SELECT id, name, description, template_paths, content_hashes,
			       parent_label_id, color, priority, is_active,
			       line_count, last_synced_at, metadata,
			       created_at, updated_at
			FROM labels
			WHERE parent_label_id IS NULL
			ORDER BY name ASC
		`
	} else {
		query = `
			SELECT id, name, description, template_paths, content_hashes,
			       parent_label_id, color, priority, is_active,
			       line_count, last_synced_at, metadata,
			       created_at, updated_at
			FROM labels
			WHERE parent_label_id = ?
			ORDER BY name ASC
		`
		args = append(args, *parentID)
	}

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query labels by parent ID failed: %w", err)
	}
	defer rows.Close()

	return r.scanLabels(rows)
}

// ValidateIntegrity validates the integrity of a single label's templates
func (r *LabelRepositoryImpl) ValidateIntegrity(ctx context.Context, labelID int) (*repository.ValidationResult, error) {
	lbl, err := r.FindByID(ctx, labelID)
	if err != nil {
		return nil, fmt.Errorf("find label failed: %w", err)
	}

	result := &repository.ValidationResult{
		LabelID:   labelID,
		LabelName: lbl.Name(),
		Status:    repository.ValidationOK,
	}

	// Check each template file
	for _, templatePath := range lbl.TemplatePaths() {
		// Resolve file path using template directories
		resolvedPath, err := r.resolveTemplatePath(templatePath)
		if err != nil {
			result.Status = repository.ValidationMissing
			result.FilePath = templatePath
			return result, nil
		}

		// Calculate current hash
		currentHash, err := calculateFileHash(resolvedPath)
		if err != nil {
			result.Status = repository.ValidationMissing
			result.FilePath = templatePath
			return result, nil
		}

		// Compare with stored hash
		expectedHash, exists := lbl.GetContentHash(templatePath)
		if exists && expectedHash != currentHash {
			result.Status = repository.ValidationModified
			result.ExpectedHash = expectedHash
			result.ActualHash = currentHash
			result.FilePath = templatePath
			return result, nil
		}
	}

	return result, nil
}

// ValidateAllLabels validates the integrity of all labels
func (r *LabelRepositoryImpl) ValidateAllLabels(ctx context.Context) ([]*repository.ValidationResult, error) {
	labels, err := r.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all labels failed: %w", err)
	}

	results := make([]*repository.ValidationResult, 0, len(labels))
	for _, lbl := range labels {
		result, err := r.ValidateIntegrity(ctx, lbl.ID())
		if err != nil {
			return nil, fmt.Errorf("validate label %d failed: %w", lbl.ID(), err)
		}
		results = append(results, result)
	}

	return results, nil
}

// SyncFromFile re-reads template files and updates the label's hash and line count
func (r *LabelRepositoryImpl) SyncFromFile(ctx context.Context, labelID int) error {
	lbl, err := r.FindByID(ctx, labelID)
	if err != nil {
		return fmt.Errorf("find label failed: %w", err)
	}

	totalLines := 0
	lbl.ClearContentHashes()

	for _, templatePath := range lbl.TemplatePaths() {
		// Resolve file path
		resolvedPath, err := r.resolveTemplatePath(templatePath)
		if err != nil {
			return fmt.Errorf("resolve template path %s failed: %w", templatePath, err)
		}

		// Calculate hash
		hash, err := calculateFileHash(resolvedPath)
		if err != nil {
			return fmt.Errorf("calculate hash for %s failed: %w", resolvedPath, err)
		}
		lbl.SetContentHash(templatePath, hash)

		// Count lines
		lines, err := countFileLines(resolvedPath)
		if err != nil {
			return fmt.Errorf("count lines for %s failed: %w", resolvedPath, err)
		}
		totalLines += lines
	}

	lbl.SetLineCount(totalLines)
	lbl.UpdateSyncTime()

	// Update in database
	return r.Update(ctx, lbl)
}

// Helper methods

// scanLabel scans a single label from a SQL row
func (r *LabelRepositoryImpl) scanLabel(row *sql.Row) (*label.Label, error) {
	var id int
	var name, description string
	var templatePathsJSON, contentHashesJSON string
	var parentLabelID sql.NullInt64
	var color string
	var priority int
	var isActive bool
	var lineCount int
	var lastSyncedAt time.Time
	var metadata string
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&id, &name, &description, &templatePathsJSON, &contentHashesJSON,
		&parentLabelID, &color, &priority, &isActive,
		&lineCount, &lastSyncedAt, &metadata,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("label not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan label failed: %w", err)
	}

	// Unmarshal JSON fields
	var templatePaths []string
	if err := json.Unmarshal([]byte(templatePathsJSON), &templatePaths); err != nil {
		return nil, fmt.Errorf("unmarshal template_paths failed: %w", err)
	}

	var contentHashes map[string]string
	if err := json.Unmarshal([]byte(contentHashesJSON), &contentHashes); err != nil {
		return nil, fmt.Errorf("unmarshal content_hashes failed: %w", err)
	}

	var parentID *int
	if parentLabelID.Valid {
		pid := int(parentLabelID.Int64)
		parentID = &pid
	}

	return label.ReconstructLabel(
		id, name, description, templatePaths, contentHashes,
		parentID, color, priority, isActive, lineCount, lastSyncedAt,
		metadata, createdAt, updatedAt,
	), nil
}

// scanLabels scans multiple labels from SQL rows
func (r *LabelRepositoryImpl) scanLabels(rows *sql.Rows) ([]*label.Label, error) {
	var labels []*label.Label

	for rows.Next() {
		var id int
		var name, description string
		var templatePathsJSON, contentHashesJSON string
		var parentLabelID sql.NullInt64
		var color string
		var priority int
		var isActive bool
		var lineCount int
		var lastSyncedAt time.Time
		var metadata string
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &name, &description, &templatePathsJSON, &contentHashesJSON,
			&parentLabelID, &color, &priority, &isActive,
			&lineCount, &lastSyncedAt, &metadata,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan label failed: %w", err)
		}

		// Unmarshal JSON fields
		var templatePaths []string
		if err := json.Unmarshal([]byte(templatePathsJSON), &templatePaths); err != nil {
			return nil, fmt.Errorf("unmarshal template_paths failed: %w", err)
		}

		var contentHashes map[string]string
		if err := json.Unmarshal([]byte(contentHashesJSON), &contentHashes); err != nil {
			return nil, fmt.Errorf("unmarshal content_hashes failed: %w", err)
		}

		var parentID *int
		if parentLabelID.Valid {
			pid := int(parentLabelID.Int64)
			parentID = &pid
		}

		lbl := label.ReconstructLabel(
			id, name, description, templatePaths, contentHashes,
			parentID, color, priority, isActive, lineCount, lastSyncedAt,
			metadata, createdAt, updatedAt,
		)
		labels = append(labels, lbl)
	}

	return labels, nil
}

// resolveTemplatePath resolves a template path to an absolute path
// If the path is absolute, it uses it directly (after verification).
// If the path is relative, it searches in the configured template directories in priority order.
func (r *LabelRepositoryImpl) resolveTemplatePath(templatePath string) (string, error) {
	// If the path is absolute, verify it exists and return it
	if filepath.IsAbs(templatePath) {
		if _, err := os.Stat(templatePath); err == nil {
			return templatePath, nil
		}
		return "", fmt.Errorf("absolute template file not found: %s", templatePath)
	}

	// For relative paths, search in configured template directories
	for _, dir := range r.labelConfig.TemplateDirs {
		fullPath := filepath.Join(dir, templatePath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("template file not found in any configured directory: %s", templatePath)
}

// calculateFileHash calculates SHA256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("calculate hash failed: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// countFileLines counts the number of lines in a file
func countFileLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("read file failed: %w", err)
	}

	lines := strings.Count(string(content), "\n")
	// Add 1 if file doesn't end with newline
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}

	return lines, nil
}

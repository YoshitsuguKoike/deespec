package sqlite

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed schema.sql
var schemaSQL string

//go:embed migrations/004_add_ordering_fields.sql
var migration004SQL string

//go:embed migrations/005_add_sbi_dependencies.sql
var migration005SQL string

//go:embed migrations/006_add_sbi_work_timestamps.sql
var migration006SQL string

// Migrator manages database schema migrations
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a new database migrator
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// Migrate applies all pending database migrations
func (m *Migrator) Migrate() error {
	// Create schema_migrations table if it doesn't exist
	if err := m.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("create migrations table failed: %w", err)
	}

	// Check if initial schema has been applied
	applied, err := m.isInitialSchemaApplied()
	if err != nil {
		return fmt.Errorf("check schema version failed: %w", err)
	}

	if !applied {
		// Apply initial schema
		if err := m.applyInitialSchema(); err != nil {
			return fmt.Errorf("apply initial schema failed: %w", err)
		}
	}

	// Always apply incremental migrations (for both new and existing databases)
	// This ensures new migrations added after schema.sql are always applied
	if err := m.applyIncrementalMigrations(); err != nil {
		return fmt.Errorf("apply incremental migrations failed: %w", err)
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist
func (m *Migrator) ensureMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		);
	`
	_, err := m.db.Exec(query)
	return err
}

// isInitialSchemaApplied checks if the initial schema has been applied
func (m *Migrator) isInitialSchemaApplied() (bool, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", 1).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// applyInitialSchema applies the initial database schema
func (m *Migrator) applyInitialSchema() error {
	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	defer tx.Rollback()

	// Split schema into individual statements
	statements := splitSQLStatements(schemaSQL)

	// Execute each statement
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Skip schema_migrations table creation (already handled by ensureMigrationsTable)
		if strings.Contains(stmt, "CREATE TABLE") && strings.Contains(stmt, "schema_migrations") {
			continue
		}

		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("execute statement %d failed: %w\nStatement: %s", i, err, stmt)
		}
	}

	// Record migration (skip if already exists - handled by schema.sql INSERTs)
	// The schema.sql file contains its own migration records
	// We only record the initial schema application here if no records exist

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction failed: %w", err)
	}

	return nil
}

// splitSQLStatements splits a SQL file into individual statements
func splitSQLStatements(sql string) []string {
	// Remove comments
	lines := strings.Split(sql, "\n")
	var cleanLines []string
	for _, line := range lines {
		// Remove single-line comments
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}

	cleanSQL := strings.Join(cleanLines, "\n")

	// Split by semicolon
	statements := strings.Split(cleanSQL, ";")

	var result []string
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}

	return result
}

// applyIncrementalMigrations applies incremental migrations for existing databases
func (m *Migrator) applyIncrementalMigrations() error {
	// Check current version
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("get current version failed: %w", err)
	}

	// Define migrations to apply
	migrations := []struct {
		version int
		sql     string
		desc    string
	}{
		{4, migration004SQL, "Add sequence and registered_at fields to sbis table"},
		{5, migration005SQL, "Add SBI dependencies table"},
		{6, migration006SQL, "Add started_at and completed_at timestamps to sbis table"},
	}

	// Apply each migration if not already applied
	for _, migration := range migrations {
		if currentVersion >= migration.version {
			// Migration already applied
			continue
		}

		if err := m.applyMigration(migration.version, migration.sql, migration.desc); err != nil {
			return fmt.Errorf("apply migration %d failed: %w", migration.version, err)
		}
	}

	return nil
}

// getCurrentVersion returns the current schema version
func (m *Migrator) getCurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// applyMigration applies a single migration
func (m *Migrator) applyMigration(version int, migrationSQL, description string) error {
	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	defer tx.Rollback()

	// Split migration into individual statements
	statements := splitSQLStatements(migrationSQL)

	// Execute each statement
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Skip schema_migrations INSERT (we'll add it separately)
		if strings.Contains(stmt, "INSERT") && strings.Contains(stmt, "schema_migrations") {
			continue
		}

		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("execute statement %d failed: %w\nStatement: %s", i, err, stmt)
		}
	}

	// Record migration version (if not already recorded)
	_, err = tx.Exec("INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)", version, description)
	if err != nil {
		return fmt.Errorf("record migration failed: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction failed: %w", err)
	}

	return nil
}

// Version returns the current schema version
func (m *Migrator) Version() (string, error) {
	var version string
	err := m.db.QueryRow("SELECT version FROM schema_migrations ORDER BY applied_at DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		return "none", nil
	}
	if err != nil {
		return "", err
	}
	return version, nil
}

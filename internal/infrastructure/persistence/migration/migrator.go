package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Migrator handles database migrations
type Migrator struct {
	db            *sql.DB
	migrationsDir string
}

// NewMigrator creates a new Migrator
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: ".deespec/migrations",
	}
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
	// 1. Ensure schema_migrations table exists
	if err := m.ensureMigrationTable(); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// 2. Get applied migrations
	applied, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// 3. Load migration files
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// 4. Apply pending migrations
	for _, migration := range migrations {
		if applied[migration.Version] {
			continue // Skip already applied
		}

		fmt.Printf("Applying migration %s: %s\n", migration.Version, migration.Name)

		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.Version, err)
		}

		fmt.Printf("✓ Applied migration %s\n", migration.Version)
	}

	fmt.Println("All migrations applied successfully")
	return nil
}

// ensureMigrationTable creates the schema_migrations table if it doesn't exist
func (m *Migrator) ensureMigrationTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)
	`)
	return err
}

// getAppliedMigrations returns a map of applied migration versions
func (m *Migrator) getAppliedMigrations() (map[string]bool, error) {
	rows, err := m.db.Query(`
		SELECT version FROM schema_migrations
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// Migration represents a database migration
type Migration struct {
	Version string
	Name    string
	SQL     string
}

// loadMigrations loads all migration files from the migrations directory
func (m *Migrator) loadMigrations() ([]Migration, error) {
	files, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, err
	}

	var migrations []Migration

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		// Parse version from filename (e.g., "001_create_pbis.sql" -> "001")
		parts := strings.SplitN(file.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}

		version := parts[0]
		name := strings.TrimSuffix(parts[1], ".sql")

		// Read migration SQL
		sqlPath := filepath.Join(m.migrationsDir, file.Name())
		sqlBytes, err := os.ReadFile(sqlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(sqlBytes),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration applies a single migration in a transaction
func (m *Migrator) applyMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.Exec(migration.SQL); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	// Record migration
	_, err = tx.Exec(`
		INSERT INTO schema_migrations (version, name, applied_at)
		VALUES (?, ?, ?)
	`, migration.Version, migration.Name, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

package sqlite

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigration_NewDatabase(t *testing.T) {
	// Create temporary database
	tmpDB := "/tmp/test_new_db.db"
	defer os.Remove(tmpDB)

	db, err := sql.Open("sqlite3", tmpDB)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create migrator and apply migrations
	migrator := NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Verify schema_migrations table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}

	if count < 6 {
		t.Errorf("Expected at least 6 migration records (004, 005, 006), got %d", count)
	}

	// Verify sbis table has new fields (from migrations 004, 005, 006)
	var hasSequence, hasRegisteredAt, hasStartedAt, hasCompletedAt bool

	rows, err := db.Query("PRAGMA table_info(sbis)")
	if err != nil {
		t.Fatalf("Failed to get table info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		if name == "sequence" {
			hasSequence = true
		}
		if name == "registered_at" {
			hasRegisteredAt = true
		}
		if name == "started_at" {
			hasStartedAt = true
		}
		if name == "completed_at" {
			hasCompletedAt = true
		}
	}

	if !hasSequence {
		t.Error("sbis table missing 'sequence' field")
	}
	if !hasRegisteredAt {
		t.Error("sbis table missing 'registered_at' field")
	}
	if !hasStartedAt {
		t.Error("sbis table missing 'started_at' field from migration 006")
	}
	if !hasCompletedAt {
		t.Error("sbis table missing 'completed_at' field from migration 006")
	}

	// Verify ordering index exists
	var indexCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_sbis_ordering'").Scan(&indexCount)
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}

	if indexCount != 1 {
		t.Error("idx_sbis_ordering index not found")
	}
}

func TestMigration_ExistingDatabase(t *testing.T) {
	// Create temporary database with old schema (version 3)
	tmpDB := "/tmp/test_existing_db.db"
	defer os.Remove(tmpDB)

	db, err := sql.Open("sqlite3", tmpDB)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create schema_migrations table
	_, err = db.Exec(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create schema_migrations: %v", err)
	}

	// Create old sbis table without sequence and registered_at
	_, err = db.Exec(`
		CREATE TABLE sbis (
			id TEXT PRIMARY KEY,
			parent_pbi_id TEXT,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL,
			current_step TEXT NOT NULL,
			estimated_hours REAL,
			priority INTEGER NOT NULL DEFAULT 3,
			labels TEXT,
			assigned_agent TEXT,
			file_paths TEXT,
			current_turn INTEGER NOT NULL DEFAULT 1,
			current_attempt INTEGER NOT NULL DEFAULT 1,
			max_turns INTEGER NOT NULL DEFAULT 10,
			max_attempts INTEGER NOT NULL DEFAULT 3,
			last_error TEXT,
			artifact_paths TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create sbis table: %v", err)
	}

	// Insert migration records up to version 3
	_, err = db.Exec("INSERT INTO schema_migrations (version, description) VALUES (1, 'Initial schema')")
	if err != nil {
		t.Fatalf("Failed to insert migration record 1: %v", err)
	}
	_, err = db.Exec("INSERT INTO schema_migrations (version, description) VALUES (2, 'Lock tables')")
	if err != nil {
		t.Fatalf("Failed to insert migration record 2: %v", err)
	}
	_, err = db.Exec("INSERT INTO schema_migrations (version, description) VALUES (3, 'Label system')")
	if err != nil {
		t.Fatalf("Failed to insert migration record 3: %v", err)
	}

	// Insert test SBI data
	_, err = db.Exec(`
		INSERT INTO sbis (id, title, status, current_step, priority)
		VALUES ('SBI-001', 'Test SBI', 'pending', 'design', 0)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Apply migrations
	migrator := NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Verify version 6 was applied (migration 004, 005, 006)
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query version: %v", err)
	}

	if version != 6 {
		t.Errorf("Expected version 6, got %d", version)
	}

	// Verify new fields exist (from migrations 004, 005, 006)
	var hasSequence, hasRegisteredAt, hasStartedAt, hasCompletedAt bool
	rows, err := db.Query("PRAGMA table_info(sbis)")
	if err != nil {
		t.Fatalf("Failed to get table info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		if name == "sequence" {
			hasSequence = true
		}
		if name == "registered_at" {
			hasRegisteredAt = true
		}
		if name == "started_at" {
			hasStartedAt = true
		}
		if name == "completed_at" {
			hasCompletedAt = true
		}
	}

	if !hasSequence {
		t.Error("sbis table missing 'sequence' field after migration")
	}
	if !hasRegisteredAt {
		t.Error("sbis table missing 'registered_at' field after migration")
	}
	if !hasStartedAt {
		t.Error("sbis table missing 'started_at' field after migration 006")
	}
	if !hasCompletedAt {
		t.Error("sbis table missing 'completed_at' field after migration 006")
	}

	// Verify existing data was backfilled
	var sequence sql.NullInt64
	var registeredAt sql.NullString
	err = db.QueryRow("SELECT sequence, registered_at FROM sbis WHERE id = 'SBI-001'").Scan(&sequence, &registeredAt)
	if err != nil {
		t.Fatalf("Failed to query test data: %v", err)
	}

	if !sequence.Valid {
		t.Error("sequence was not backfilled for existing data")
	}
	if !registeredAt.Valid {
		t.Error("registered_at was not backfilled for existing data")
	}

	t.Logf("Backfilled data: sequence=%d, registered_at=%s", sequence.Int64, registeredAt.String)
}

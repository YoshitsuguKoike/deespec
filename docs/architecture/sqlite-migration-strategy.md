# SQLite Migration Strategy for DeeSpec

## Overview

This document outlines the strategy for migrating DeeSpec from file-based storage to SQLite, with emphasis on backward compatibility, data migration, and handling schema evolution.

## Current Storage System

### File-Based Structure
```
.deespec/
├── specs/
│   └── sbi/
│       └── SBI-{ID}/
│           ├── spec.md
│           ├── state.json
│           ├── implement_{n}.md
│           ├── test_{n}.md
│           ├── review_{n}.md
│           └── done_{n}.md
├── workflow.json
├── setting.json
└── journal/
    └── {date}/
        └── {txid}/
```

### Data Characteristics
- Structured metadata (JSON)
- Large text content (Markdown)
- Binary attachments (future)
- Audit trail requirements
- Concurrent access patterns

## SQLite Database Design

### Schema Version Management

```sql
-- schema_version table
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL,
    description TEXT,
    checksum TEXT NOT NULL
);

-- Current version marker
CREATE TABLE schema_info (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO schema_info (key, value) VALUES ('version', '1');
```

### Core Tables

```sql
-- SBI tasks
CREATE TABLE sbi (
    id TEXT PRIMARY KEY,           -- SBI-01K6FRP24VXAE1C24TYMZ95G6S
    task_id TEXT NOT NULL,
    label TEXT,
    priority INTEGER DEFAULT 0,
    status TEXT NOT NULL,           -- READY, WIP, REVIEW, DONE
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    metadata JSON                   -- Extensible metadata
);

-- Execution history
CREATE TABLE execution (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sbi_id TEXT NOT NULL,
    turn INTEGER NOT NULL,
    step TEXT NOT NULL,              -- plan, implement, test, review
    status TEXT NOT NULL,
    decision TEXT,                   -- SUCCEEDED, NEEDS_CHANGES, FAILED
    attempt INTEGER DEFAULT 1,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    lease_expires_at TIMESTAMP,
    content TEXT,                    -- Large markdown content
    FOREIGN KEY (sbi_id) REFERENCES sbi(id),
    UNIQUE(sbi_id, turn)
);

-- Configuration
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value JSON NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Audit log
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TIMESTAMP NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action TEXT NOT NULL,
    old_value JSON,
    new_value JSON,
    user TEXT,
    session_id TEXT
);
```

### Indexes

```sql
CREATE INDEX idx_sbi_status ON sbi(status);
CREATE INDEX idx_sbi_created ON sbi(created_at);
CREATE INDEX idx_execution_sbi ON execution(sbi_id);
CREATE INDEX idx_execution_turn ON execution(sbi_id, turn);
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_timestamp ON audit_log(timestamp);
```

## Migration Strategy

### Phase 1: Dual Storage (Months 1-2)

```go
// Dual storage adapter
type DualStorageRepository struct {
    fileRepo   FileRepository      // Existing
    sqliteRepo SQLiteRepository    // New
    mode       StorageMode         // FILE_ONLY, DUAL_WRITE, SQLITE_PRIMARY
}

func (r *DualStorageRepository) Save(sbi *SBI) error {
    switch r.mode {
    case FILE_ONLY:
        return r.fileRepo.Save(sbi)
    case DUAL_WRITE:
        // Write to both, file is source of truth
        if err := r.fileRepo.Save(sbi); err != nil {
            return err
        }
        _ = r.sqliteRepo.Save(sbi) // Log but don't fail
        return nil
    case SQLITE_PRIMARY:
        // Write to SQLite first, file as backup
        if err := r.sqliteRepo.Save(sbi); err != nil {
            return err
        }
        _ = r.fileRepo.Save(sbi)
        return nil
    }
}
```

### Phase 2: Data Migration Tools

```go
// Migration command
type MigrationCommand struct {
    source FileRepository
    dest   SQLiteRepository
    validator DataValidator
}

func (m *MigrationCommand) Migrate(dryRun bool) error {
    // 1. Scan all file-based SBIs
    sbis, err := m.source.ListAll()

    // 2. Validate data integrity
    for _, sbi := range sbis {
        if err := m.validator.Validate(sbi); err != nil {
            log.Errorf("Invalid SBI %s: %v", sbi.ID, err)
            continue
        }
    }

    // 3. Begin transaction
    tx, err := m.dest.Begin()

    // 4. Migrate in batches
    for batch := range batches(sbis, 100) {
        if err := m.migrateBatch(tx, batch); err != nil {
            tx.Rollback()
            return err
        }
    }

    // 5. Verify migration
    if err := m.verify(); err != nil {
        tx.Rollback()
        return err
    }

    if !dryRun {
        tx.Commit()
    }
    return nil
}
```

### Phase 3: Cutover Process

```bash
#!/bin/bash
# Migration script

# 1. Stop all running processes
deespec stop --all

# 2. Backup existing data
tar -czf backup_$(date +%Y%m%d).tar.gz .deespec/

# 3. Run migration with validation
deespec migrate --validate --dry-run
if [ $? -ne 0 ]; then
    echo "Dry run failed"
    exit 1
fi

# 4. Actual migration
deespec migrate --validate

# 5. Verify data integrity
deespec verify --check-all

# 6. Switch to SQLite mode
echo '{"storage_mode": "sqlite"}' > .deespec/setting.json

# 7. Restart services
deespec start
```

## Schema Evolution Strategy

### Forward Compatibility

```sql
-- Add new columns with defaults
ALTER TABLE sbi ADD COLUMN new_field TEXT DEFAULT '';

-- Use JSON for extensibility
UPDATE sbi SET metadata = json_set(metadata, '$.new_property', 'value');
```

### Migration Scripts

```go
// migrations/001_initial_schema.sql
// migrations/002_add_priority.sql
// migrations/003_add_audit_log.sql

type Migration struct {
    Version     int
    Description string
    Up          string  // SQL to apply
    Down        string  // SQL to rollback
    Checksum    string
}

func ApplyMigrations(db *sql.DB, migrations []Migration) error {
    current := getCurrentVersion(db)

    for _, m := range migrations {
        if m.Version <= current {
            continue
        }

        tx, _ := db.Begin()
        if err := executeMigration(tx, m); err != nil {
            tx.Rollback()
            return err
        }
        tx.Commit()
    }
    return nil
}
```

## Backward Compatibility

### Data Export

```go
// Export to legacy format
func ExportToFiles(repo SQLiteRepository, path string) error {
    sbis, _ := repo.ListAll()

    for _, sbi := range sbis {
        // Create directory structure
        sbiPath := filepath.Join(path, "specs/sbi", sbi.ID)
        os.MkdirAll(sbiPath, 0755)

        // Export spec.md
        writeFile(filepath.Join(sbiPath, "spec.md"), sbi.Spec)

        // Export state.json
        state := convertToLegacyState(sbi)
        writeJSON(filepath.Join(sbiPath, "state.json"), state)

        // Export execution files
        execs, _ := repo.GetExecutions(sbi.ID)
        for _, exec := range execs {
            filename := fmt.Sprintf("%s_%d.md", exec.Step, exec.Turn)
            writeFile(filepath.Join(sbiPath, filename), exec.Content)
        }
    }
    return nil
}
```

### Rollback Procedure

```bash
#!/bin/bash
# Rollback to file-based storage

# 1. Export current SQLite data
deespec export --format=files --output=.deespec_export

# 2. Stop services
deespec stop --all

# 3. Backup SQLite database
cp .deespec/deespec.db .deespec/deespec.db.backup

# 4. Restore file-based data
rm -rf .deespec/specs
mv .deespec_export/specs .deespec/

# 5. Update configuration
jq '.storage_mode = "file"' .deespec/setting.json > tmp.json
mv tmp.json .deespec/setting.json

# 6. Restart with file mode
deespec start
```

## Data Integrity

### Validation Rules

```go
type DataValidator struct {
    rules []ValidationRule
}

type ValidationRule interface {
    Validate(data interface{}) error
}

// Example rules
type SBIIDFormatRule struct{}
func (r *SBIIDFormatRule) Validate(data interface{}) error {
    sbi := data.(*SBI)
    if !regexp.MustCompile(`^SBI-[0-9A-Z]{26}$`).MatchString(sbi.ID) {
        return fmt.Errorf("invalid SBI ID format: %s", sbi.ID)
    }
    return nil
}

type TurnSequenceRule struct{}
func (r *TurnSequenceRule) Validate(data interface{}) error {
    execs := data.([]*Execution)
    for i, exec := range execs {
        if exec.Turn != i+1 {
            return fmt.Errorf("turn sequence broken at %d", exec.Turn)
        }
    }
    return nil
}
```

### Consistency Checks

```sql
-- Check for orphaned executions
SELECT e.* FROM execution e
LEFT JOIN sbi s ON e.sbi_id = s.id
WHERE s.id IS NULL;

-- Check for duplicate turns
SELECT sbi_id, turn, COUNT(*) as cnt
FROM execution
GROUP BY sbi_id, turn
HAVING cnt > 1;

-- Check status consistency
SELECT s.* FROM sbi s
WHERE s.status = 'DONE'
AND NOT EXISTS (
    SELECT 1 FROM execution e
    WHERE e.sbi_id = s.id
    AND e.decision = 'SUCCEEDED'
);
```

## Performance Considerations

### Query Optimization

```sql
-- Use covering indexes
CREATE INDEX idx_sbi_list ON sbi(status, created_at)
    INCLUDE (id, task_id, label);

-- Partition large tables (using views)
CREATE VIEW recent_executions AS
SELECT * FROM execution
WHERE started_at > datetime('now', '-30 days');

-- Use prepared statements
PREPARE get_sbi_stmt AS
SELECT * FROM sbi WHERE id = ?;
```

### Connection Management

```go
type ConnectionPool struct {
    writeDB *sql.DB  // Single writer
    readDBs []*sql.DB // Multiple readers
}

func NewConnectionPool(dbPath string) *ConnectionPool {
    // Writer connection
    writeDB, _ := sql.Open("sqlite3", dbPath)
    writeDB.SetMaxOpenConns(1)

    // Reader connections
    readDBs := make([]*sql.DB, 4)
    for i := range readDBs {
        db, _ := sql.Open("sqlite3", dbPath+"?mode=ro")
        db.SetMaxOpenConns(5)
        readDBs[i] = db
    }

    return &ConnectionPool{writeDB, readDBs}
}
```

## Monitoring and Maintenance

### Health Checks

```go
func HealthCheck(db *sql.DB) error {
    // Check connection
    if err := db.Ping(); err != nil {
        return err
    }

    // Check schema version
    var version int
    db.QueryRow("SELECT value FROM schema_info WHERE key = 'version'").Scan(&version)
    if version != EXPECTED_VERSION {
        return fmt.Errorf("schema version mismatch")
    }

    // Check data integrity
    var count int
    db.QueryRow("PRAGMA integrity_check").Scan(&count)

    return nil
}
```

### Backup Strategy

```bash
#!/bin/bash
# Automated backup script

DB_PATH=".deespec/deespec.db"
BACKUP_DIR=".deespec/backups"

# Create backup directory
mkdir -p $BACKUP_DIR

# Perform online backup
sqlite3 $DB_PATH ".backup '$BACKUP_DIR/backup_$(date +%Y%m%d_%H%M%S).db'"

# Compress older backups
find $BACKUP_DIR -name "*.db" -mtime +7 -exec gzip {} \;

# Remove old backups (keep 30 days)
find $BACKUP_DIR -name "*.gz" -mtime +30 -delete
```

## Risk Mitigation

| Risk | Mitigation Strategy |
|------|-------------------|
| Data loss during migration | Comprehensive backups, dry-run validation |
| Performance degradation | Benchmark before/after, query optimization |
| Schema evolution issues | Version management, rollback scripts |
| Concurrent access problems | WAL mode, proper locking |
| Large file handling | BLOB streaming, external file storage |
| Corruption | Regular integrity checks, backup rotation |

## Success Criteria

- Zero data loss during migration
- Query performance ≤ 10ms for common operations
- Successful rollback capability demonstrated
- All existing features work without modification
- Backup/restore time < 1 minute for 10GB database

## Timeline

- **Month 1**: Implement dual storage adapter
- **Month 2**: Create migration tools and scripts
- **Month 3**: Beta testing with subset of data
- **Month 4**: Production migration with rollback plan
- **Month 5**: Monitor and optimize
- **Month 6**: Deprecate file-based storage

## Conclusion

This migration strategy ensures a smooth transition from file-based to SQLite storage while maintaining data integrity, backward compatibility, and system reliability. The phased approach minimizes risk and provides multiple rollback points.
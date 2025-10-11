-- Migration: 005
-- Description: Add SBI dependency management tables
-- Dependencies between SBIs are stored in a separate table to enable
-- proper task ordering in parallel execution workflows.

-- SBI dependency table
-- This table stores dependencies between SBIs (e.g., SBI-002 depends on SBI-001)
CREATE TABLE IF NOT EXISTS sbi_dependencies (
    sbi_id TEXT NOT NULL,              -- The SBI that has the dependency
    depends_on_sbi_id TEXT NOT NULL,   -- The SBI that must be completed first
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (sbi_id, depends_on_sbi_id),
    FOREIGN KEY (sbi_id) REFERENCES sbis(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_sbi_id) REFERENCES sbis(id) ON DELETE CASCADE
);

-- Index for efficient dependency lookups
-- Used by TaskPickerService to check if dependencies are met
CREATE INDEX IF NOT EXISTS idx_sbi_deps_sbi_id ON sbi_dependencies(sbi_id);

-- Reverse lookup index for finding dependent tasks
-- Used to find all tasks that depend on a specific SBI
CREATE INDEX IF NOT EXISTS idx_sbi_deps_depends_on ON sbi_dependencies(depends_on_sbi_id);

-- Record migration
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (5, 'Add sbi_dependencies table for dependency management');

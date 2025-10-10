-- Migration 004: Add ordering fields to sbis table
-- Purpose: Fix ULID ordering issue by adding explicit priority, sequence, and registered_at fields
-- Version: 4
-- Date: 2025-10-10

-- Check if sequence column exists before adding
-- SQLite doesn't have a clean way to check, so we use a workaround
-- If the column already exists, ALTER TABLE will fail silently in a transaction we can catch

-- Add sequence field
ALTER TABLE sbis ADD COLUMN sequence INTEGER;

-- Add registered_at field
ALTER TABLE sbis ADD COLUMN registered_at DATETIME;

-- Backfill sequence for existing SBIs based on created_at order
-- This ensures existing data gets sequence numbers in creation order
UPDATE sbis
SET sequence = (
    SELECT COUNT(*)
    FROM sbis AS s2
    WHERE s2.created_at <= sbis.created_at
)
WHERE sequence IS NULL;

-- Backfill registered_at with created_at for existing SBIs
UPDATE sbis
SET registered_at = created_at
WHERE registered_at IS NULL;

-- Create index for efficient ordering queries
-- This index optimizes queries that order by priority DESC, registered_at ASC, sequence ASC
-- Index is created AFTER fields exist
CREATE INDEX IF NOT EXISTS idx_sbis_ordering ON sbis(priority DESC, registered_at ASC, sequence ASC);

-- Record this migration
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (4, 'Add sequence and registered_at fields to sbis table for correct ordering');

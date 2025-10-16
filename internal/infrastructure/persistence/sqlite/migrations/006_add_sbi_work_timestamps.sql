-- Migration: 006
-- Description: Add work start and completion timestamps to SBIs
-- These timestamps enable statistical analysis of task completion times
-- and help track actual work duration vs estimated hours.

-- Add started_at column to track when work begins (PENDING → PICKED)
ALTER TABLE sbis ADD COLUMN started_at DATETIME;

-- Add completed_at column to track when work finishes (DONE/FAILED)
ALTER TABLE sbis ADD COLUMN completed_at DATETIME;

-- Create index for completed_at to enable efficient time-range queries
CREATE INDEX IF NOT EXISTS idx_sbis_completed_at ON sbis(completed_at);

-- Create index for started_at to enable efficient time-range queries
CREATE INDEX IF NOT EXISTS idx_sbis_started_at ON sbis(started_at);

-- Record migration
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (6, 'Add started_at and completed_at timestamps to sbis table');

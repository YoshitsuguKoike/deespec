-- Migration 008: Add only_implement flag to SBIs table
-- This flag controls SBI workflow behavior:
--   false (default): full cycle with IMPLEMENT → REVIEW steps
--   true: implementation-only mode, skips REVIEW step

ALTER TABLE sbis ADD COLUMN only_implement BOOLEAN DEFAULT 0;

-- Record migration
INSERT INTO schema_migrations (version, description)
VALUES (8, 'Add only_implement flag to sbis table for workflow control');

-- Migration 007: Create SBI execution logs table
-- This table records the execution history of each SBI turn (IMPLEMENT and REVIEW steps)

CREATE TABLE IF NOT EXISTS sbi_exec_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sbi_id TEXT NOT NULL,
    turn INTEGER NOT NULL,
    step TEXT NOT NULL,  -- 'IMPLEMENT' or 'REVIEW'
    decision TEXT,  -- NULL for IMPLEMENT, 'SUCCEEDED'/'NEEDS_CHANGES'/'FAILED' for REVIEW
    report_path TEXT,  -- Path to implement_N.md or review_N.md
    executed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(sbi_id, turn, step),
    FOREIGN KEY (sbi_id) REFERENCES sbis(id) ON DELETE CASCADE
);

-- Index for efficient querying by SBI ID
CREATE INDEX IF NOT EXISTS idx_sbi_exec_logs_sbi_id ON sbi_exec_logs(sbi_id);

-- Index for querying by SBI ID and turn
CREATE INDEX IF NOT EXISTS idx_sbi_exec_logs_sbi_turn ON sbi_exec_logs(sbi_id, turn);

-- Record migration
INSERT INTO schema_migrations (version, description)
VALUES (7, 'Create SBI execution logs table');

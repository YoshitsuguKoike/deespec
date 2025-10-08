-- DeeSpec SQLite Schema
-- Clean Architecture Infrastructure Layer
-- Version: 1.0

-- EPIC テーブル (Large Feature Group)
CREATE TABLE IF NOT EXISTS epics (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    estimated_story_points INTEGER,
    priority INTEGER NOT NULL DEFAULT 3,
    labels TEXT, -- JSON array
    assigned_agent TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- PBI テーブル (Product Backlog Item)
CREATE TABLE IF NOT EXISTS pbis (
    id TEXT PRIMARY KEY,
    parent_epic_id TEXT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    story_points INTEGER,
    priority INTEGER NOT NULL DEFAULT 3,
    labels TEXT, -- JSON array
    assigned_agent TEXT,
    acceptance_criteria TEXT, -- JSON array
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_epic_id) REFERENCES epics(id) ON DELETE SET NULL
);

-- SBI テーブル (Spec Backlog Item)
CREATE TABLE IF NOT EXISTS sbis (
    id TEXT PRIMARY KEY,
    parent_pbi_id TEXT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    estimated_hours REAL,
    priority INTEGER NOT NULL DEFAULT 3,
    labels TEXT, -- JSON array
    assigned_agent TEXT,
    file_paths TEXT, -- JSON array
    current_turn INTEGER NOT NULL DEFAULT 1,
    current_attempt INTEGER NOT NULL DEFAULT 1,
    max_turns INTEGER NOT NULL DEFAULT 10,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error TEXT,
    artifact_paths TEXT, -- JSON array
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_pbi_id) REFERENCES pbis(id) ON DELETE SET NULL
);

-- EPIC-PBI 関連テーブル (多対多関係の管理)
CREATE TABLE IF NOT EXISTS epic_pbis (
    epic_id TEXT NOT NULL,
    pbi_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (epic_id, pbi_id),
    FOREIGN KEY (epic_id) REFERENCES epics(id) ON DELETE CASCADE,
    FOREIGN KEY (pbi_id) REFERENCES pbis(id) ON DELETE CASCADE
);

-- PBI-SBI 関連テーブル (多対多関係の管理)
CREATE TABLE IF NOT EXISTS pbi_sbis (
    pbi_id TEXT NOT NULL,
    sbi_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (pbi_id, sbi_id),
    FOREIGN KEY (pbi_id) REFERENCES pbis(id) ON DELETE CASCADE,
    FOREIGN KEY (sbi_id) REFERENCES sbis(id) ON DELETE CASCADE
);

-- パフォーマンス最適化用インデックス
CREATE INDEX IF NOT EXISTS idx_pbis_parent_epic_id ON pbis(parent_epic_id);
CREATE INDEX IF NOT EXISTS idx_sbis_parent_pbi_id ON sbis(parent_pbi_id);
CREATE INDEX IF NOT EXISTS idx_epics_status ON epics(status);
CREATE INDEX IF NOT EXISTS idx_pbis_status ON pbis(status);
CREATE INDEX IF NOT EXISTS idx_sbis_status ON sbis(status);
CREATE INDEX IF NOT EXISTS idx_epics_created_at ON epics(created_at);
CREATE INDEX IF NOT EXISTS idx_pbis_created_at ON pbis(created_at);
CREATE INDEX IF NOT EXISTS idx_sbis_created_at ON sbis(created_at);

-- スキーマバージョン管理テーブル
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

-- RunLock テーブル (SBI実行の排他制御)
CREATE TABLE IF NOT EXISTS run_locks (
    lock_id TEXT PRIMARY KEY,        -- Resource identifier (e.g., SBI ID)
    pid INTEGER NOT NULL,             -- Process ID
    hostname TEXT NOT NULL,           -- Host name
    acquired_at DATETIME NOT NULL,    -- Lock acquisition time
    expires_at DATETIME NOT NULL,     -- Lock expiration time
    heartbeat_at DATETIME NOT NULL,   -- Last heartbeat time
    metadata TEXT,                    -- JSON metadata (optional)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- StateLock テーブル (状態ファイルの排他制御)
CREATE TABLE IF NOT EXISTS state_locks (
    lock_id TEXT PRIMARY KEY,        -- Resource identifier
    pid INTEGER NOT NULL,             -- Process ID
    hostname TEXT NOT NULL,           -- Host name
    acquired_at DATETIME NOT NULL,    -- Lock acquisition time
    expires_at DATETIME NOT NULL,     -- Lock expiration time
    heartbeat_at DATETIME NOT NULL,   -- Last heartbeat time
    lock_type TEXT NOT NULL,          -- Lock type: "read" or "write"
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Lock パフォーマンス最適化用インデックス
CREATE INDEX IF NOT EXISTS idx_run_locks_expires_at ON run_locks(expires_at);
CREATE INDEX IF NOT EXISTS idx_state_locks_expires_at ON state_locks(expires_at);
CREATE INDEX IF NOT EXISTS idx_run_locks_heartbeat_at ON run_locks(heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_state_locks_heartbeat_at ON state_locks(heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_run_locks_pid ON run_locks(pid);
CREATE INDEX IF NOT EXISTS idx_state_locks_pid ON state_locks(pid);

-- 初期バージョン記録
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (1, 'Initial schema - EPIC/PBI/SBI tables');

-- Lock systemバージョン記録
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (2, 'Add Lock tables - run_locks and state_locks');

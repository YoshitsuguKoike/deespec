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
    sequence INTEGER,                    -- 登録順序番号 (自動採番で設定)
    registered_at DATETIME,              -- 明示的な登録タイムスタンプ
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

-- SBI順序管理用インデックス (優先度→登録順で最適化)
CREATE INDEX IF NOT EXISTS idx_sbis_ordering ON sbis(priority DESC, registered_at ASC, sequence ASC);

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

-- Labels テーブル (Phase 9.1: Label System with File Integrity)
CREATE TABLE IF NOT EXISTS labels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,              -- ラベル名 (例: "security", "perspective:designer")
    description TEXT,                       -- 説明文
    template_paths TEXT,                    -- JSON array: 相対パス ["perspectives/designer.md"]
    content_hashes TEXT,                    -- JSON object: {"path": "sha256hash"}
    parent_label_id INTEGER,                -- 親ラベルID（階層化対応）
    color TEXT,                             -- UI表示用カラー
    priority INTEGER DEFAULT 0,             -- 指示書マージ優先度（高い方が優先）
    is_active BOOLEAN DEFAULT 1,            -- 有効/無効フラグ
    line_count INTEGER,                     -- 総行数（1000行制限チェック用）
    last_synced_at DATETIME,                -- 最終同期日時
    metadata TEXT,                          -- JSON: 将来の拡張用
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_label_id) REFERENCES labels(id) ON DELETE SET NULL
);

-- Labels パフォーマンス最適化用インデックス
CREATE INDEX IF NOT EXISTS idx_labels_name ON labels(name);
CREATE INDEX IF NOT EXISTS idx_labels_parent ON labels(parent_label_id);
CREATE INDEX IF NOT EXISTS idx_labels_is_active ON labels(is_active);
CREATE INDEX IF NOT EXISTS idx_labels_last_synced ON labels(last_synced_at);

-- Label systemバージョン記録
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (3, 'Add label management system with integrity check');

-- SBI順序管理フィールド追加
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (4, 'Add sequence and registered_at fields to sbis table for correct ordering');

-- SBI dependency table (Version 5)
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
CREATE INDEX IF NOT EXISTS idx_sbi_deps_sbi_id ON sbi_dependencies(sbi_id);
CREATE INDEX IF NOT EXISTS idx_sbi_deps_depends_on ON sbi_dependencies(depends_on_sbi_id);

-- SBI dependency management version
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (5, 'Add sbi_dependencies table for dependency management');

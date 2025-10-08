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

-- 初期バージョン記録
INSERT OR IGNORE INTO schema_migrations (version, description)
VALUES (1, 'Initial schema - EPIC/PBI/SBI tables');

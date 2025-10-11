# deespec データベーススキーマ設計

> PBI/SBI管理のためのデータベーススキーマ設計書

---

## 目次

1. [データベース選択](#1-データベース選択)
2. [テーブル設計](#2-テーブル設計)
3. [ER図](#3-er図)
4. [インデックス戦略](#4-インデックス戦略)
5. [マイグレーション戦略](#5-マイグレーション戦略)
6. [パフォーマンス最適化](#6-パフォーマンス最適化)
7. [バックアップとリカバリ](#7-バックアップとリカバリ)
8. [将来の拡張性](#8-将来の拡張性)

---

## 1. データベース選択

### 1.1 推奨: SQLite 3

**選定理由**:

| 要件 | SQLite | PostgreSQL | MySQL |
|------|--------|------------|-------|
| **ローカルツール** | ✅ ファイルベース | ❌ サーバー必要 | ❌ サーバー必要 |
| **セットアップ** | ✅ 不要 | ❌ 複雑 | ❌ 複雑 |
| **パフォーマンス** | ✅ 十分 | ✅ 高速 | ✅ 高速 |
| **トランザクション** | ✅ ACID準拠 | ✅ ACID準拠 | ✅ ACID準拠 |
| **移植性** | ✅ 単一ファイル | ❌ ダンプ必要 | ❌ ダンプ必要 |
| **Go対応** | ✅ 標準的 | ✅ 良好 | ✅ 良好 |

**ファイルパス**: `.deespec/var/deespec.db`

### 1.2 スケーリング戦略

```
Phase 1: SQLite（〜1000件のPBI）
  ↓ 十分なパフォーマンス
  ↓
Phase 2: SQLite最適化（〜10,000件のPBI）
  ↓ インデックス、クエリ最適化
  ↓
Phase 3: PostgreSQL移行（10,000件〜）
  ↓ 必要に応じて
  ↓
Phase 4: 分散DB（100,000件〜）
```

**結論**: Phase 1ではSQLiteで十分。ほとんどのユースケースで問題なし。

---

## 2. テーブル設計

### 2.1 PBIテーブル（pbis）

**目的**: Product Backlog Itemのメイン情報を格納

**ファイル保存形式**:
```
.deespec/specs/pbi/
├── PBI-001/
│   └── pbi.md          # Markdown形式（YAMLは使わない）
├── PBI-002/
│   └── pbi.md
```

**設計思想**:
- ✅ **Markdownで保存**: YAMLは使わず、pbi.mdとして保存
- ✅ **DBはメタデータのみ**: 本文はファイルシステムに保存
- ✅ **titleはH1から抽出**: Markdownの最初の`# Title`から取得（検索用にDBに保存）
- ✅ **bodyはDB不要**: `.deespec/specs/pbi/{id}/pbi.md`に保存
- ✅ **source_documentは不要**: IDが決まれば必ず`.deespec/specs/pbi/{id}/pbi.md`

**ファイルとDBの役割分担**:
```
.deespec/specs/pbi/PBI-001/pbi.md  ← 真実の源（Markdown本文）
                    ↓ title/metadata抽出
DB (pbis table)                     ← メタデータのみ（検索・フィルタ用）
```

```sql
CREATE TABLE pbis (
    -- プライマリキー
    id TEXT PRIMARY KEY,                    -- PBI-001, PBI-002, ... (自動生成)

    -- 基本情報（検索用）
    title TEXT NOT NULL,                    -- pbi.mdのH1から抽出（検索用）

    -- ステータス管理（構造体で管理）
    status TEXT NOT NULL DEFAULT 'pending', -- pending | planning | planed | in_progress | done

    -- 見積もりと優先度
    estimated_story_points INTEGER,         -- フィボナッチ数（1,2,3,5,8,13）またはNULL
    priority INTEGER NOT NULL DEFAULT 0,    -- 0=通常, 1=高, 2=緊急

    -- 階層構造（将来拡張）
    parent_epic_id TEXT,                    -- 親EPICのID（NULL可）

    -- タイムスタンプ
    created_at TEXT NOT NULL,               -- ISO 8601形式
    updated_at TEXT NOT NULL,               -- ISO 8601形式

    -- 制約
    CHECK (priority >= 0 AND priority <= 2),
    CHECK (estimated_story_points IS NULL OR estimated_story_points > 0),
    CHECK (status IN ('pending', 'planning', 'planed', 'in_progress', 'done')),
    FOREIGN KEY (parent_epic_id) REFERENCES pbis(id) ON DELETE SET NULL
);
```

**カラム詳細**:

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|---|------|-----------|------|
| `id` | TEXT | NO | - | PBI-XXX形式の一意識別子 |
| `title` | TEXT | NO | - | pbi.mdのH1から抽出（検索用） |
| `status` | TEXT | NO | 'pending' | 実行ステータス（5段階） |
| `estimated_story_points` | INTEGER | YES | NULL | 見積もりポイント |
| `priority` | INTEGER | NO | 0 | 優先度（0-2） |
| `parent_epic_id` | TEXT | YES | NULL | 親EPIC ID |
| `created_at` | TEXT | NO | - | 作成日時 |
| `updated_at` | TEXT | NO | - | 更新日時 |

**Note**: 本文（body）は`.deespec/specs/pbi/{id}/pbi.md`に保存。DBには保存しない。

**ステータス構造体**:
```go
type PBIStatus string

const (
    PBIStatusPending    PBIStatus = "pending"      // 未着手
    PBIStatusPlanning   PBIStatus = "planning"     // 計画中
    PBIStatusPlaned     PBIStatus = "planed"       // 計画完了
    PBIStatusInProgress PBIStatus = "in_progress"  // 実行中
    PBIStatusDone       PBIStatus = "done"         // 完了
)
```

---

### 2.2 SBIテーブル（sbis）

**目的**: Small Backlog Item（PBIの分解単位）を管理

```sql
CREATE TABLE sbis (
    -- プライマリキー
    id TEXT PRIMARY KEY,                    -- SBI-001, SBI-002, ...

    -- メタデータ
    version TEXT NOT NULL DEFAULT '1.0',
    type TEXT NOT NULL DEFAULT 'sbi',

    -- 基本情報
    title TEXT NOT NULL,
    description TEXT,

    -- ステータス
    status TEXT NOT NULL DEFAULT 'PENDING',
    current_step TEXT,                      -- PICK | IMPLEMENT | REVIEW | DONE

    -- 関連PBI
    parent_pbi_id TEXT NOT NULL,            -- 親PBI（必須）

    -- プロンプトパス
    implement_prompt_path TEXT,             -- IMPLEMENTプロンプトへのパス
    review_prompt_path TEXT,                -- REVIEWプロンプトへのパス

    -- タイムスタンプ
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    started_at DATETIME,                    -- 実行開始日時
    completed_at DATETIME,                  -- 完了日時

    -- 制約
    CHECK (status IN ('PENDING', 'PICKED', 'IMPLEMENTING', 'REVIEWING', 'DONE', 'FAILED')),
    CHECK (current_step IN ('PICK', 'IMPLEMENT', 'REVIEW', 'DONE') OR current_step IS NULL),
    FOREIGN KEY (parent_pbi_id) REFERENCES pbis(id) ON DELETE CASCADE
);
```

---

### 2.3 実行履歴テーブル（sbi_executions）

**目的**: SBIの実行履歴（Turn記録）を管理

```sql
CREATE TABLE sbi_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sbi_id TEXT NOT NULL,                   -- SBI ID
    turn_number INTEGER NOT NULL,           -- ターン番号（1, 2, 3, ...）
    step TEXT NOT NULL,                     -- PICK | IMPLEMENT | REVIEW | DONE

    -- 実行情報
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    duration_seconds INTEGER,               -- 実行時間（秒）

    -- 結果
    status TEXT NOT NULL,                   -- success | failure | interrupted
    output_path TEXT,                       -- 出力ファイルパス
    error_message TEXT,                     -- エラーメッセージ（失敗時）

    -- Agent情報
    agent_name TEXT,                        -- 実行したAgent名
    agent_command TEXT,                     -- 実行コマンド

    -- 制約
    FOREIGN KEY (sbi_id) REFERENCES sbis(id) ON DELETE CASCADE,
    UNIQUE (sbi_id, turn_number, step)      -- 同じSBI+Turn+Stepは一意
);
```

---

### 2.4 Agentテーブル（agents）

**目的**: Agent設定を管理（agents.yamlの代替またはキャッシュ）

```sql
CREATE TABLE agents (
    name TEXT PRIMARY KEY,                  -- Agent名（一意識別子）
    command TEXT NOT NULL,                  -- 実行コマンド
    type TEXT NOT NULL,                     -- interactive | api | local
    description TEXT,                       -- 説明

    -- 環境変数（JSON形式）
    env_vars TEXT,                          -- {"KEY": "value", ...}

    -- ステータス
    is_default BOOLEAN NOT NULL DEFAULT 0,  -- デフォルトAgent
    is_active BOOLEAN NOT NULL DEFAULT 1,   -- 有効/無効

    -- タイムスタンプ
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,

    -- 制約
    CHECK (type IN ('interactive', 'api', 'local'))
);
```

**注意**: Phase 1では`.deespec/config/agents.yaml`を使用。Phase 2以降でDB化を検討。

---

## 3. ER図

### 3.1 エンティティ関係

```
┌─────────────────┐
│     EPICs       │
│  (将来拡張)      │
└────────┬────────┘
         │ 1
         │
         │ *
┌────────▼────────┐
│      PBIs       │
│  (pbis)         │
└────────┬────────┘
         │ 1
         │
         │ *
┌────────▼────────┐
│      SBIs       │         ┌──────────────────┐
│  (sbis)         │◄────────│ sbi_executions   │
└─────────────────┘  1    * │                  │
                            └──────────────────┘

┌─────────────────┐
│     Agents      │
│  (agents)       │
└─────────────────┘
```

### 3.2 リレーションシップ詳細

| From | To | 関係 | カーディナリティ | カスケード |
|------|----|----|----------------|----------|
| PBIs | SBIs | 1:N | 1つのPBIは複数のSBI | CASCADE |
| PBIs | PBIs (self) | 1:N | EPIC → PBI階層 | SET NULL |
| SBIs | sbi_executions | 1:N | 1つのSBIは複数の実行履歴 | CASCADE |
| Agents | - | 独立 | - | - |

---

## 4. インデックス戦略

### 4.1 必須インデックス

```sql
-- pbisテーブル
CREATE INDEX idx_pbis_status ON pbis(status);
CREATE INDEX idx_pbis_priority ON pbis(priority);
CREATE INDEX idx_pbis_created_at ON pbis(created_at);
CREATE INDEX idx_pbis_updated_at ON pbis(updated_at);
CREATE INDEX idx_pbis_parent_epic_id ON pbis(parent_epic_id);

-- sbisテーブル
CREATE INDEX idx_sbis_status ON sbis(status);
CREATE INDEX idx_sbis_parent_pbi_id ON sbis(parent_pbi_id);
CREATE INDEX idx_sbis_current_step ON sbis(current_step);

-- sbi_executionsテーブル
CREATE INDEX idx_sbi_exec_sbi_id ON sbi_executions(sbi_id);
CREATE INDEX idx_sbi_exec_started_at ON sbi_executions(started_at);
```

### 4.2 複合インデックス（クエリ最適化用）

```sql
-- ステータス + 優先度での検索用
CREATE INDEX idx_pbis_status_priority ON pbis(status, priority);

-- 日付範囲 + ステータスでの検索用
CREATE INDEX idx_pbis_created_status ON pbis(created_at, status);
```

### 4.3 EXPLAIN ANALYZEによる検証

```sql
-- クエリプラン確認
EXPLAIN QUERY PLAN
SELECT * FROM pbis
WHERE status = 'PENDING'
ORDER BY priority DESC, created_at DESC;

-- 期待される実行計画:
-- SEARCH TABLE pbis USING INDEX idx_pbis_status_priority (status=? AND priority>?)
```

---

## 5. マイグレーション戦略

### 5.1 マイグレーション管理テーブル

```sql
CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY,               -- マイグレーションバージョン（001, 002, ...）
    name TEXT NOT NULL,                     -- マイグレーション名
    applied_at DATETIME NOT NULL,           -- 適用日時
    checksum TEXT                           -- ファイルのチェックサム（整合性確認）
);
```

### 5.2 マイグレーションファイル構成

```
.deespec/migrations/
├── 001_create_pbis.sql                    # PBIテーブル作成
├── 002_create_sbis.sql                    # SBIテーブル作成
├── 003_create_sbi_executions.sql          # 実行履歴テーブル
├── 004_create_agents.sql                  # Agentテーブル（オプション）
└── 999_initial_data.sql                   # 初期データ投入
```

### 5.3 マイグレーション実行フロー

```go
// internal/infrastructure/persistence/migration/migrator.go

type Migration struct {
    Version  string
    Name     string
    SQL      string
    Checksum string
}

func (m *Migrator) Migrate() error {
    // 1. schema_migrationsテーブル作成
    if err := m.ensureMigrationTable(); err != nil {
        return err
    }

    // 2. 適用済みマイグレーションを取得
    applied, err := m.getAppliedMigrations()
    if err != nil {
        return err
    }

    // 3. 未適用のマイグレーションを実行
    migrations := m.loadMigrations()
    for _, migration := range migrations {
        if applied[migration.Version] {
            continue // スキップ
        }

        // トランザクション内で実行
        if err := m.applyMigration(migration); err != nil {
            return fmt.Errorf("migration %s failed: %w", migration.Version, err)
        }

        log.Printf("Applied migration %s: %s", migration.Version, migration.Name)
    }

    return nil
}

func (m *Migrator) applyMigration(migration Migration) error {
    tx, err := m.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // マイグレーションSQL実行
    if _, err := tx.Exec(migration.SQL); err != nil {
        return err
    }

    // schema_migrationsに記録
    _, err = tx.Exec(`
        INSERT INTO schema_migrations (version, name, applied_at, checksum)
        VALUES (?, ?, ?, ?)
    `, migration.Version, migration.Name, time.Now(), migration.Checksum)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### 5.4 ロールバック戦略

```
.deespec/migrations/
├── 001_create_pbis.up.sql                 # アップマイグレーション
├── 001_create_pbis.down.sql               # ダウンマイグレーション（ロールバック）
├── 002_create_sbis.up.sql
├── 002_create_sbis.down.sql
...
```

**001_create_pbis.down.sql**:
```sql
-- Rollback: Drop pbis table
DROP INDEX IF EXISTS idx_pbis_status;
DROP INDEX IF EXISTS idx_pbis_priority;
DROP INDEX IF EXISTS idx_pbis_created_at;
DROP TABLE IF EXISTS pbis;
```

---

## 6. パフォーマンス最適化

### 6.1 クエリ最適化パターン

#### パターン1: N+1問題の回避

**悪い例**:
```go
// N+1クエリ（遅い）
sbis, _ := repo.FindAll()
for _, sbi := range sbis {
    pbi, _ := repo.FindPBIByID(sbi.ParentPBIID)  // N回のクエリ
    sbi.ParentPBI = pbi
}
```

**良い例**:
```go
// JOINで一度に取得（速い）
rows, _ := db.Query(`
    SELECT s.*, p.id, p.title, p.status
    FROM sbis s
    LEFT JOIN pbis p ON s.parent_pbi_id = p.id
    ORDER BY s.id
`)

// Goでマッピング
sbis := mapSBIsWithPBIs(rows)
```

#### パターン2: ページネーション

```sql
-- LIMIT + OFFSET
SELECT * FROM pbis
ORDER BY created_at DESC
LIMIT 20 OFFSET 40;  -- 3ページ目

-- Keyset Pagination（より高速）
SELECT * FROM pbis
WHERE created_at < ?  -- 前ページの最後のcreated_at
ORDER BY created_at DESC
LIMIT 20;
```

#### パターン3: カウントクエリの最適化

```sql
-- 遅い
SELECT COUNT(*) FROM pbis WHERE status = 'PENDING';

-- 速い（概算でよい場合）
SELECT estimated_count
FROM sqlite_stat1
WHERE tbl = 'pbis';
```

### 6.2 VACUUM戦略

```sql
-- 定期的なVACUUM（週次推奨）
VACUUM;

-- AUTO_VACUUM設定
PRAGMA auto_vacuum = INCREMENTAL;
```

### 6.3 WALモード（Write-Ahead Logging）

```sql
-- 読み込みパフォーマンス向上
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
```

---

## 7. バックアップとリカバリ

### 7.1 バックアップ戦略

```bash
# 方法1: ファイルコピー（シンプル）
cp .deespec/var/deespec.db .deespec/var/backups/deespec_$(date +%Y%m%d_%H%M%S).db

# 方法2: SQLiteのBACKUPコマンド（推奨）
sqlite3 .deespec/var/deespec.db "BACKUP TO '.deespec/var/backups/deespec_backup.db'"

# 方法3: SQLダンプ
sqlite3 .deespec/var/deespec.db .dump > backup.sql
```

### 7.2 自動バックアップ

```go
// internal/infrastructure/persistence/backup/backup.go

func ScheduleBackup(dbPath string, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for range ticker.C {
        if err := createBackup(dbPath); err != nil {
            log.Printf("Backup failed: %v", err)
        }
    }
}

func createBackup(dbPath string) error {
    timestamp := time.Now().Format("20060102_150405")
    backupPath := fmt.Sprintf(".deespec/var/backups/deespec_%s.db", timestamp)

    // SQLiteのBACKUPコマンド実行
    cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf("BACKUP TO '%s'", backupPath))
    return cmd.Run()
}
```

### 7.3 リストア

```bash
# バックアップから復元
cp .deespec/var/backups/deespec_20251011_100000.db .deespec/var/deespec.db

# SQLダンプから復元
sqlite3 .deespec/var/deespec.db < backup.sql
```

---

## 8. 将来の拡張性

### 8.1 Phase 2: EPIC階層

```sql
-- EPICテーブル（Phase 3）
CREATE TABLE epics (
    id TEXT PRIMARY KEY,                  -- EPIC-001
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- PBIsテーブルに親EPIC追加（既存）
ALTER TABLE pbis ADD COLUMN parent_epic_id TEXT REFERENCES epics(id);
```

### 8.2 Phase 3: タグ付けシステム

```sql
-- 汎用タグテーブル
CREATE TABLE tags (
    name TEXT PRIMARY KEY,
    category TEXT,                        -- 'label', 'skill', 'domain' など
    description TEXT,
    color TEXT                            -- 表示用の色
);

-- PBIとタグの関連
CREATE TABLE pbi_tags (
    pbi_id TEXT NOT NULL,
    tag_name TEXT NOT NULL,
    PRIMARY KEY (pbi_id, tag_name),
    FOREIGN KEY (pbi_id) REFERENCES pbis(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_name) REFERENCES tags(name) ON DELETE CASCADE
);
```

### 8.3 Phase 4: 全文検索

```sql
-- FTS5仮想テーブル（全文検索）
CREATE VIRTUAL TABLE pbis_fts USING fts5(
    id UNINDEXED,
    title,
    description,
    content='pbis',
    content_rowid='rowid'
);

-- トリガーで自動同期
CREATE TRIGGER pbis_ai AFTER INSERT ON pbis BEGIN
    INSERT INTO pbis_fts(rowid, id, title, description)
    VALUES (new.rowid, new.id, new.title, new.description);
END;

-- 全文検索クエリ
SELECT * FROM pbis
WHERE id IN (
    SELECT id FROM pbis_fts WHERE pbis_fts MATCH 'coverage AND test'
);
```

---

## まとめ

### Phase 1実装チェックリスト

- [ ] `pbis`テーブル作成
- [ ] `sbis`テーブル作成
- [ ] `sbi_executions`テーブル作成
- [ ] 基本インデックス作成
- [ ] マイグレーション機構実装
- [ ] Markdown → SQLiteインポート機能
- [ ] SQLite → Markdownエクスポート機能（互換性維持）

### パフォーマンス目標

| 操作 | 目標レスポンス時間 | 条件 |
|------|------------------|------|
| PBI登録 | < 10ms | 1件 |
| PBI一覧取得 | < 50ms | 100件 |
| ステータス検索 | < 100ms | 1000件のPBI |
| 全文検索 | < 200ms | 10,000件のPBI（FTS5使用） |

### 移行戦略

```
Phase 1: Markdownファイルのみ（2週間）
  ↓
Phase 1.5: Markdown + SQLite並行運用（1週間）
  ↓
Phase 2: SQLiteメイン、Markdownは export/import（2週間）
  ↓
Phase 3: SQLiteのみ（Markdownは deprecated）
```

**結論**: SQLiteは十分なパフォーマンスと拡張性を提供。Phase 1ではMarkdownで開始し、Phase 2でSQLite移行を検討。

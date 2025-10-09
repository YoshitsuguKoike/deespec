# Phase 9.1: ラベルシステムSQLite化 実装計画（改訂版）

**作成日**: 2025-10-09
**改訂日**: 2025-10-09
**ステータス**: 設計完了・実装待ち
**優先度**: 高
**推定工数**: 10-12時間

---

## 目次

1. [概要](#概要)
2. [背景と目的](#背景と目的)
3. [現状の問題点](#現状の問題点)
4. [新設計](#新設計)
5. [スキーマ設計](#スキーマ設計)
6. [setting.json拡張](#settingjson拡張)
7. [アーキテクチャ設計](#アーキテクチャ設計)
8. [実装計画](#実装計画)
9. [移行戦略](#移行戦略)
10. [テスト計画](#テスト計画)
11. [リスクと対策](#リスクと対策)

---

## 概要

ラベルシステムを旧ファイルベース実装からClean Architecture + SQLiteベースに完全移行する。
**ファイルをSource of Truth**として、ユーザーが直接編集可能としつつ、SQLiteでメタデータ管理と整合性チェックを実現する。

### 主要な変更点

| 項目 | Before（旧実装） | After（新実装） |
|-----|-----------------|----------------|
| **データ保存** | `.deespec/var/labels.json` | SQLite `labels`, `task_labels` テーブル |
| **CLI実装** | `label_cmd.go`（meta.yml直接操作） | DI Container経由のRepository利用 |
| **テンプレート管理** | ファイルシステムのみ | **ファイル優先 + DBでメタ管理** |
| **テンプレート数** | 1ラベル1ファイル | 1ラベル複数ファイル対応 |
| **バリデーション** | なし（typo可能） | 登録済みラベルのみ + **整合性チェック** |
| **ディレクトリ** | `.deespec/prompts/labels/` 固定 | **`.claude/` 等を`setting.json`で設定可能** |
| **ファイル編集** | ❌ 編集すると不整合 | ✅ **ユーザー編集可能 + ハッシュ検証** |

### 新機能

1. **複数テンプレート対応**: 1ラベルに複数の指示書ファイルを関連付け
2. **ワイルドカード展開**: `security/*.md` で全ファイル自動読み込み
3. **整合性チェック**: SHA256ハッシュでファイル変更を検出
4. **一括インポート**: `.claude/` 等のディレクトリを一括登録
5. **設定可能なパス**: `setting.json` でテンプレートディレクトリを変更可能

---

## 背景と目的

### Why: なぜこの実装が必要か

#### 現状の課題

1. **二重管理の問題**
   - ラベル情報が `.deespec/var/labels.json`（独自インデックス）と `meta.yml`（各SBI）に分散
   - SQLiteの `sbis.labels` フィールド（JSON）とファイルベース管理が並存

2. **typo防止の欠如**
   - ラベル名の自由入力により、`fronted` vs `frontend` のような誤りが発生
   - 未使用・廃止ラベルの検出が困難

3. **指示書管理の脆弱性**
   - テンプレートファイルパスがハードコード（`.deespec/prompts/labels/<label>.md`）
   - ファイル名変更時の追跡不可
   - 階層化ラベル（`frontend/admin`）の親子関係が非体系的

4. **拡張性の限界**
   - 1ラベル1ファイルの制約（複数の指示書を組み合わせたい場合に対応できない）
   - ディレクトリ配下の全ファイルを一括読み込みする機能がない

5. **ファイル編集の制約**（★ 新規追加）
   - ユーザーが直接`.md`ファイルを編集できない
   - 編集すると`.deespec/var/labels.json`との不整合が発生
   - 既存の`.claude/`ディレクトリ構造を活用できない

### 目的

1. **データ一元管理**: SQLiteをメタデータ管理に利用、ファイルをSource of Truthに
2. **typo防止**: 事前登録されたラベルのみ使用可能にする
3. **複数テンプレート対応**: 1つのラベルに複数の指示書を関連付け
4. **ワイルドカード展開**: `prompts/labels/security/*.md` で全ファイル自動読み込み
5. **階層化サポート**: 親ラベルの指示書を自動継承
6. **ファイル編集可能**: ユーザーが直接`.md`ファイルを編集可能（★ 新規）
7. **整合性管理**: SHA256ハッシュでファイル変更を検出・警告（★ 新規）
8. **柔軟なディレクトリ**: `.claude/`等を`setting.json`で設定（★ 新規）

---

## 現状の問題点

### 1. データフロー（Before）

```
┌─────────────────────────────────────────────────┐
│ SBI作成時                                       │
├─────────────────────────────────────────────────┤
│ User: deespec sbi register --labels security    │
│   ↓                                             │
│ CLI: meta.yml に labels: [security] を直接書込  │
│   ↓                                             │
│ CLI: .deespec/var/labels.json を更新（独自実装）│
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│ Agent実行時                                     │
├─────────────────────────────────────────────────┤
│ Workflow: state.json から WIP SBI読み取り       │
│   ↓                                             │
│ CLI: meta.yml から labels 読み取り              │
│   ↓                                             │
│ EnrichTaskWithLabels:                           │
│   .deespec/prompts/labels/<label>.md を探索    │
│   （ハードコードされたパス）                    │
└─────────────────────────────────────────────────┘

問題:
- SQLite（sbis.labels）とファイル（meta.yml, labels.json）の不整合リスク
- テンプレートパスの変更追跡不可
- typo時のエラー検出なし
- ユーザーがファイルを直接編集できない
```

### 2. 既存実装の問題箇所

#### internal/interface/cli/label_cmd.go

```go
// 問題1: meta.yml直接操作（Repository層を経由していない）
metaPath := filepath.Join(".deespec", st.WIP, "meta.yml")
meta, err := loadSBIMeta(metaPath)  // ❌ ファイル直接読み込み

// 問題2: 独自インデックス管理
if err := updateLabelIndex(); err != nil {  // ❌ .deespec/var/labels.json 更新
    return fmt.Errorf("failed to update label index: %w", err)
}
```

#### internal/interface/cli/claude_prompt.go

```go
// 問題3: テンプレートパスがハードコード
labelPath := filepath.Join(".deespec", "prompts", "labels", label+".md")
if content, err := os.ReadFile(labelPath); err == nil {  // ❌ 固定パス
    // ...
}

// 問題4: ファイルシステムベースの探索（DB未利用）
lines := strings.Split(string(data), "\n")
// YAML手動パース ❌
```

---

## 新設計

### 設計原則

1. **File as Source of Truth**: ファイルが正、DBはインデックス+検証
2. **Clean Architecture準拠**: Domain層・Repository層・UseCase層の分離
3. **ユーザー編集可能**: `.md`ファイルを直接編集OK、整合性チェックで検出
4. **設定可能なパス**: `setting.json`でテンプレートディレクトリを変更可能
5. **段階的移行**: 旧データの自動マイグレーション機能提供

### アーキテクチャ方針

```
┌──────────────────────────────────────────────────────┐
│ Source of Truth: ファイルシステム                     │
│ .claude/ (setting.jsonで設定可能)                    │
│ .deespec/prompts/labels/ (デフォルト)                │
└──────────────────────────────────────────────────────┘
              ↓ インデックス化
┌──────────────────────────────────────────────────────┐
│ SQLite: メタデータ + ハッシュ管理                     │
│ - labels テーブル: パス、SHA256ハッシュ、メタ情報     │
│ - task_labels テーブル: タスクとラベルの関連          │
└──────────────────────────────────────────────────────┘
              ↓ 整合性チェック
┌──────────────────────────────────────────────────────┐
│ 検証機能 (deespec label validate)                    │
│ - ハッシュ値比較でファイル変更を検出                   │
│ - 不一致時に警告・自動同期オプション                   │
└──────────────────────────────────────────────────────┘
```

### データフロー（After）

```
┌──────────────────────────────────────────────────────┐
│ ラベル登録（プロジェクト初期化時）                     │
├──────────────────────────────────────────────────────┤
│ User: deespec label import .claude --recursive        │
│   ↓                                                  │
│ CLI: .claude/ 配下の.mdファイルをスキャン             │
│   → 行数チェック（1000行以内）                        │
│   → SHA256ハッシュ計算                                │
│   ↓                                                  │
│ Repository → SQLite                                   │
│   labels テーブルに保存:                              │
│   - template_paths: 相対パス                          │
│   - content_hashes: {"path": "hash"}                  │
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│ SBI作成時                                             │
├──────────────────────────────────────────────────────┤
│ User: deespec sbi register --labels security          │
│   ↓                                                  │
│ CLI → TaskUseCase.CreateSBI()                         │
│   ↓                                                  │
│ LabelRepository.FindByName("security")                │
│   ✓ ラベル存在確認（typo防止）                        │
│   ↓                                                  │
│ TaskRepository.Save(sbi)                              │
│   ↓                                                  │
│ LabelRepository.AttachLabel(sbiID, labelID)           │
│   → task_labels テーブルに関連保存                   │
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│ Agent実行時                                           │
├──────────────────────────────────────────────────────┤
│ Workflow: TaskRepository.Find(sbiID)                  │
│   ↓                                                  │
│ LabelRepository.GetTaskLabels(sbiID)                  │
│   → labels JOIN task_labels (priority順)             │
│   ↓                                                  │
│ EnrichTaskWithLabels:                                 │
│   LabelRepository.ExpandTemplates(labelID)            │
│   → setting.jsonのlabel_template_dirsから解決        │
│   → ワイルドカード展開（*.md → 実ファイルリスト）     │
│   → 各ファイルを読み込んでマージ                      │
│   → ★ ハッシュ不一致時に警告                         │
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│ ファイル編集後（ユーザーが直接.mdを編集）             │
├──────────────────────────────────────────────────────┤
│ User: vim .claude/perspectives/designer.md (手動編集) │
│   ↓                                                  │
│ User: deespec label validate                          │
│   ↓                                                  │
│ Repository: 全ラベルのハッシュチェック                │
│   ✓ OK: ハッシュ一致                                 │
│   ⚠ MODIFIED: ハッシュ不一致 → 警告                  │
│   ↓                                                  │
│ User: deespec label validate --sync                   │
│   → DBのハッシュを更新（ファイルを正とする）          │
└──────────────────────────────────────────────────────┘
```

---

## スキーマ設計

### 1. labels テーブル（改訂版）

```sql
CREATE TABLE IF NOT EXISTS labels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,              -- ラベル名 (例: "security", "perspective:designer")
    description TEXT,                       -- 説明文
    template_paths TEXT,                    -- JSON array: 相対パス ["perspectives/designer.md"]
    content_hashes TEXT,                    -- ★ JSON object: {"path": "sha256hash"}
    parent_label_id INTEGER,                -- 親ラベルID（階層化対応）
    color TEXT,                             -- UI表示用カラー
    priority INTEGER DEFAULT 0,             -- 指示書マージ優先度（高い方が優先）
    is_active BOOLEAN DEFAULT 1,            -- 有効/無効フラグ
    line_count INTEGER,                     -- ★ 総行数（1000行制限チェック用）
    last_synced_at DATETIME,                -- ★ 最終同期日時
    metadata TEXT,                          -- JSON: 将来の拡張用
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_label_id) REFERENCES labels(id) ON DELETE SET NULL
);

-- インデックス
CREATE INDEX IF NOT EXISTS idx_labels_name ON labels(name);
CREATE INDEX IF NOT EXISTS idx_labels_parent ON labels(parent_label_id);
CREATE INDEX IF NOT EXISTS idx_labels_is_active ON labels(is_active);
CREATE INDEX IF NOT EXISTS idx_labels_last_synced ON labels(last_synced_at);
```

**★ 新規フィールド:**

| フィールド | 型 | 用途 | 例 |
|-----------|---|------|---|
| `content_hashes` | TEXT (JSON) | 各テンプレートファイルのSHA256ハッシュ値 | `{"perspectives/designer.md": "a3f5b8c9..."}` |
| `line_count` | INTEGER | 総行数（1000行制限チェック） | `145` |
| `last_synced_at` | DATETIME | 最終同期日時 | `2025-10-09 12:00:00` |

**template_paths の例:**

```json
["perspectives/designer.md", "perspectives/engineer.md"]
```

- 相対パス（`setting.json`の`label_template_dirs`を基準に解決）
- ワイルドカード対応: `["security/*.md"]`

**content_hashes の例:**

```json
{
  "perspectives/designer.md": "a3f5b8c9d2e1f4a7b6c5d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9",
  "perspectives/engineer.md": "7d6e5f4c3b2a1098f7e6d5c4b3a29018e7d6c5b4a39281f0e9d8c7b6a5948372"
}
```

### 2. task_labels テーブル（変更なし）

```sql
CREATE TABLE IF NOT EXISTS task_labels (
    task_id TEXT NOT NULL,                  -- SBI-xxx, PBI-xxx, EPIC-xxx
    label_id INTEGER NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,    -- ラベル表示順序
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (task_id, label_id),
    FOREIGN KEY (label_id) REFERENCES labels(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_labels_task_id ON task_labels(task_id);
CREATE INDEX IF NOT EXISTS idx_task_labels_label_id ON task_labels(label_id);
```

---

## setting.json拡張

### 新規フィールド

```json
{
  // 既存の設定（維持）
  "agent": "claude-code",
  "model": "claude-3-5-sonnet-20250101",
  "timeout": 60,
  "max_turns": 10,
  "max_attempts": 3,
  "log_level": "info",

  // ★ 新規: ラベルシステム設定
  "label_config": {
    // テンプレートファイル探索ディレクトリ（優先度順）
    "template_dirs": [
      ".claude",                          // 優先度1: .claude/
      ".deespec/prompts/labels"           // 優先度2: .deespec/prompts/labels/
    ],

    // インポート設定
    "import": {
      "auto_prefix_from_dir": true,       // ディレクトリ名を自動プレフィックス化
      "max_line_count": 1000,             // 1ファイルの最大行数
      "exclude_patterns": [                // 除外パターン
        "*.secret.md",
        "settings.*.json",
        "tmp/**"
      ]
    },

    // 検証設定
    "validation": {
      "auto_sync_on_mismatch": false,     // 不一致時に自動同期するか
      "warn_on_large_files": true         // 大きいファイル（500行以上）で警告
    }
  },

  // 既存の設定（維持）
  "storage": {
    "type": "local",
    "path": ".deespec/storage"
  }
}
```

### フィールド詳細

#### label_config.template_dirs

- **型**: `string[]`
- **用途**: テンプレートファイルを探索するディレクトリのリスト（優先度順）
- **例**:
  ```json
  ["./claude", ".deespec/prompts/labels", "docs/instructions"]
  ```
- **動作**:
  - ラベルのテンプレートパス（相対パス）を解決する際、この順番でディレクトリを探索
  - 最初に見つかったファイルを使用

#### label_config.import.auto_prefix_from_dir

- **型**: `boolean`
- **用途**: インポート時にディレクトリ名を自動的にラベルプレフィックスにするか
- **例**:
  - `true`: `.claude/perspectives/designer.md` → `perspective:designer`
  - `false`: `.claude/perspectives/designer.md` → `designer`

#### label_config.import.max_line_count

- **型**: `integer`
- **用途**: 1ファイルの最大行数（これを超えるファイルはインポート時にスキップ）
- **デフォルト**: `1000`

#### label_config.import.exclude_patterns

- **型**: `string[]`
- **用途**: インポート時に除外するファイルパターン（glob形式）
- **例**: `["*.secret.md", "settings.*.json", "tmp/**"]`

---

## アーキテクチャ設計

### レイヤー構成

```
┌─────────────────────────────────────────────────────────┐
│ Interface Layer (CLI)                                    │
│ - label_cmd.go (全面書き換え: DI Container経由)          │
│ - label_import.go (新規: 一括インポート)                 │
│ - label_validate.go (新規: 整合性チェック)               │
│ - claude_prompt.go (改善: Repository経由+ハッシュ警告)   │
└─────────────────────────────────────────────────────────┘
                          ↓ 依存
┌─────────────────────────────────────────────────────────┐
│ Application Layer (UseCase)                              │
│ - label_use_case.go (新規作成)                           │
│   · RegisterLabel(name, desc, templates, priority)       │
│   · AttachLabelToTask(taskID, labelName)                 │
│   · GetTaskLabels(taskID)                                │
│   · ExpandTemplates(labelID)                             │
│   · ValidateIntegrity(labelID)                           │
│   · SyncFromFile(labelID)                                │
└─────────────────────────────────────────────────────────┘
                          ↓ 依存
┌─────────────────────────────────────────────────────────┐
│ Domain Layer                                             │
│ - model/label/label.go (新規作成)                        │
│   · Label エンティティ (content_hashes, line_count追加)  │
│   · LabelID 値オブジェクト                               │
│ - repository/label_repository.go (インターフェース)      │
│   · ValidateIntegrity(labelID) 追加                      │
│   · SyncFromFile(labelID) 追加                           │
└─────────────────────────────────────────────────────────┘
                          ↑ 実装
┌─────────────────────────────────────────────────────────┐
│ Infrastructure Layer                                     │
│ - persistence/sqlite/label_repository_impl.go (新規作成) │
│   · SQLiteベース実装                                     │
│   · ワイルドカード展開ロジック                           │
│   · SHA256ハッシュ計算・検証                             │
│   · 複数ディレクトリ解決（setting.json連携）            │
│ - config/settings.go (拡張)                              │
│   · LabelConfig構造体追加                                │
└─────────────────────────────────────────────────────────┘
```

### 主要コンポーネント

#### 1. Domain層

**internal/domain/model/label/label.go**

```go
package label

type Label struct {
    id            int
    name          string
    description   string
    templatePaths []string                  // 複数パス対応
    contentHashes map[string]string         // ★ path → SHA256 hash
    parentLabelID *int
    color         string
    priority      int
    isActive      bool
    lineCount     int                       // ★ 総行数
    lastSyncedAt  time.Time                 // ★ 最終同期日時
    metadata      map[string]interface{}
    createdAt     time.Time
    updatedAt     time.Time
}

// コンストラクタ
func NewLabel(name, description string, templatePaths []string, priority int) (*Label, error)

// ★ 新規メソッド
func (l *Label) SetContentHash(path, hash string)
func (l *Label) GetContentHash(path string) string
func (l *Label) SetLineCount(count int)
func (l *Label) UpdateSyncTime()
```

**internal/domain/repository/label_repository.go**

```go
package repository

type LabelRepository interface {
    // 基本CRUD
    Save(ctx context.Context, l *label.Label) error
    Find(ctx context.Context, id label.LabelID) (*label.Label, error)
    FindByName(ctx context.Context, name string) (*label.Label, error)
    List(ctx context.Context, activeOnly bool) ([]*label.Label, error)
    Delete(ctx context.Context, id label.LabelID) error

    // タスク関連操作
    AttachLabel(ctx context.Context, taskID string, labelID label.LabelID, position int) error
    DetachLabel(ctx context.Context, taskID string, labelID label.LabelID) error
    GetTaskLabels(ctx context.Context, taskID string) ([]*label.Label, error)

    // テンプレート展開
    ExpandTemplates(ctx context.Context, labelID label.LabelID) ([]string, error)
    GetTemplateFilenames(ctx context.Context, labelID label.LabelID) ([]string, error)

    // ★ 新規: 整合性チェック
    ValidateIntegrity(ctx context.Context, labelID label.LabelID) (*ValidationResult, error)
    ValidateAllLabels(ctx context.Context) ([]*ValidationResult, error)

    // ★ 新規: 同期
    SyncFromFile(ctx context.Context, labelID label.LabelID) error
}

type ValidationResult struct {
    LabelID       int
    LabelName     string
    Status        ValidationStatus  // OK, MODIFIED, MISSING
    ExpectedHash  string
    ActualHash    string
    FilePath      string
    Message       string
}

type ValidationStatus int
const (
    ValidationOK ValidationStatus = iota
    ValidationModified
    ValidationMissing
)
```

#### 2. Infrastructure層

**internal/infrastructure/persistence/sqlite/label_repository_impl.go**

主要メソッド:

1. **ExpandTemplates(labelID)**: ワイルドカード展開 + 複数ディレクトリ解決
   ```go
   func (r *LabelRepositoryImpl) ExpandTemplates(ctx context.Context, labelID label.LabelID) ([]string, error) {
       lbl, _ := r.Find(ctx, labelID)

       var expandedPaths []string

       for _, templatePath := range lbl.TemplatePaths() {
           // ワイルドカード展開
           if strings.Contains(templatePath, "*") {
               // setting.jsonの各template_dirで展開試行
               for _, baseDir := range r.templateDirs {
                   pattern := filepath.Join(baseDir, templatePath)
                   matches, _ := filepath.Glob(pattern)
                   expandedPaths = append(expandedPaths, matches...)
               }
           } else {
               // 通常のパス解決（複数ディレクトリから探索）
               resolved, err := r.resolveTemplatePath(templatePath)
               if err == nil {
                   expandedPaths = append(expandedPaths, resolved)
               }
           }
       }

       return uniqueAndSort(expandedPaths), nil
   }
   ```

2. **resolveTemplatePath(relativePath)**: 複数ディレクトリから探索
   ```go
   func (r *LabelRepositoryImpl) resolveTemplatePath(relativePath string) (string, error) {
       // setting.jsonのtemplate_dirsを順番に探索
       for _, baseDir := range r.templateDirs {
           fullPath := filepath.Join(baseDir, relativePath)
           if _, err := os.Stat(fullPath); err == nil {
               return fullPath, nil  // 見つかった
           }
       }
       return "", fmt.Errorf("template not found: %s", relativePath)
   }
   ```

3. **ValidateIntegrity(labelID)**: ハッシュ検証
   ```go
   func (r *LabelRepositoryImpl) ValidateIntegrity(ctx context.Context, labelID label.LabelID) (*ValidationResult, error) {
       lbl, _ := r.Find(ctx, labelID)

       for filePath, expectedHash := range lbl.ContentHashes() {
           fullPath, err := r.resolveTemplatePath(filePath)
           if err != nil {
               return &ValidationResult{Status: ValidationMissing, FilePath: filePath}, nil
           }

           content, _ := os.ReadFile(fullPath)
           actualHash := calculateSHA256(content)

           if actualHash != expectedHash {
               return &ValidationResult{
                   Status:       ValidationModified,
                   ExpectedHash: expectedHash,
                   ActualHash:   actualHash,
                   FilePath:     filePath,
               }, nil
           }
       }

       return &ValidationResult{Status: ValidationOK}, nil
   }
   ```

4. **SyncFromFile(labelID)**: ファイルからDBを更新
   ```go
   func (r *LabelRepositoryImpl) SyncFromFile(ctx context.Context, labelID label.LabelID) error {
       lbl, _ := r.Find(ctx, labelID)

       // 各テンプレートファイルのハッシュを再計算
       for _, filePath := range lbl.TemplatePaths() {
           fullPath, _ := r.resolveTemplatePath(filePath)
           content, _ := os.ReadFile(fullPath)

           newHash := calculateSHA256(content)
           lbl.SetContentHash(filePath, newHash)

           lineCount := bytes.Count(content, []byte("\n"))
           lbl.SetLineCount(lineCount)
       }

       lbl.UpdateSyncTime()
       return r.Save(ctx, lbl)
   }
   ```

**internal/infra/config/settings.go（拡張）**

```go
package config

type LabelConfig struct {
    TemplateDirs      []string          `json:"template_dirs"`
    ImportConfig      LabelImportConfig `json:"import"`
    ValidationConfig  LabelValidationConfig `json:"validation"`
}

type LabelImportConfig struct {
    AutoPrefixFromDir bool     `json:"auto_prefix_from_dir"`
    MaxLineCount      int      `json:"max_line_count"`
    ExcludePatterns   []string `json:"exclude_patterns"`
}

type LabelValidationConfig struct {
    AutoSyncOnMismatch bool `json:"auto_sync_on_mismatch"`
    WarnOnLargeFiles   bool `json:"warn_on_large_files"`
}

type AppConfig struct {
    // 既存フィールド
    Agent      string `json:"agent"`
    Model      string `json:"model"`
    Timeout    int    `json:"timeout"`
    MaxTurns   int    `json:"max_turns"`
    MaxAttempts int   `json:"max_attempts"`
    LogLevel   string `json:"log_level"`

    // ★ 新規
    LabelConfig LabelConfig `json:"label_config"`

    // 既存フィールド
    Storage StorageConfig `json:"storage"`
}

// デフォルト値
func NewDefaultLabelConfig() LabelConfig {
    return LabelConfig{
        TemplateDirs: []string{".claude", ".deespec/prompts/labels"},
        ImportConfig: LabelImportConfig{
            AutoPrefixFromDir: true,
            MaxLineCount:      1000,
            ExcludePatterns:   []string{"settings.*.json", "*.secret.md"},
        },
        ValidationConfig: LabelValidationConfig{
            AutoSyncOnMismatch: false,
            WarnOnLargeFiles:   true,
        },
    }
}
```

#### 3. Interface層（CLI）

**新規コマンド一覧:**

```bash
# ラベル登録（ファイルから）
deespec label register <name> --file <path> [--description <desc>] [--priority <n>]

# 一括インポート
deespec label import <directory> [--recursive] [--prefix-from-dir] [--dry-run]

# ラベル一覧
deespec label list [--json] [--show-inactive]

# ラベル詳細表示
deespec label show <name> [--check-integrity]

# ラベル更新
deespec label update <name> [--description <desc>] [--priority <n>] [--add-template <path>]

# ラベル削除
deespec label delete <name>

# タスクへラベル付与
deespec label attach --task <id> --labels <label1,label2,...>

# タスクからラベル削除
deespec label detach --task <id> --labels <label1,label2,...>

# テンプレート展開確認
deespec label templates --label <name> [--content]

# ★ 整合性チェック
deespec label validate [--sync] [--label <name>]

# 旧データ移行
deespec label migrate [--dry-run]
```

**internal/interface/cli/label_validate.go（新規）**

```go
package cli

func newLabelValidateCmd() *cobra.Command {
    var sync bool
    var labelName string

    cmd := &cobra.Command{
        Use:   "validate",
        Short: "Validate label file integrity",
        Long: `Check if label template files match database hashes.

This command compares SHA256 hashes of template files with stored values.
If files have been modified, it warns you and optionally syncs the database.

Examples:
  # Validate all labels
  deespec label validate

  # Validate specific label
  deespec label validate --label security

  # Auto-sync modified files
  deespec label validate --sync`,
        RunE: func(cmd *cobra.Command, args []string) error {
            return runLabelValidate(labelName, sync)
        },
    }

    cmd.Flags().BoolVar(&sync, "sync", false, "Auto-sync modified files (update DB from files)")
    cmd.Flags().StringVar(&labelName, "label", "", "Validate specific label only")

    return cmd
}

func runLabelValidate(labelName string, sync bool) error {
    container, _ := initializeContainer()
    defer container.Close()

    labelRepo := container.GetLabelRepository()
    ctx := context.Background()

    var results []*repository.ValidationResult

    if labelName != "" {
        // 特定ラベルのみ検証
        lbl, err := labelRepo.FindByName(ctx, labelName)
        if err != nil {
            return fmt.Errorf("label not found: %s", labelName)
        }

        labelID, _ := label.NewLabelID(lbl.ID())
        result, _ := labelRepo.ValidateIntegrity(ctx, labelID)
        results = []*repository.ValidationResult{result}
    } else {
        // 全ラベル検証
        results, _ = labelRepo.ValidateAllLabels(ctx)
    }

    // 結果表示
    var okCount, modifiedCount, missingCount int

    for _, result := range results {
        switch result.Status {
        case repository.ValidationOK:
            okCount++
            fmt.Printf("✓ %s - OK\n", result.LabelName)
        case repository.ValidationModified:
            modifiedCount++
            fmt.Printf("⚠ %s - MODIFIED\n", result.LabelName)
            fmt.Printf("  File: %s\n", result.FilePath)
            fmt.Printf("  Expected: %s\n", result.ExpectedHash[:16]+"...")
            fmt.Printf("  Actual:   %s\n", result.ActualHash[:16]+"...")

            if sync {
                labelID, _ := label.NewLabelID(result.LabelID)
                labelRepo.SyncFromFile(ctx, labelID)
                fmt.Printf("  ✓ Synced\n")
            }
        case repository.ValidationMissing:
            missingCount++
            fmt.Printf("❌ %s - MISSING FILE\n", result.LabelName)
            fmt.Printf("  Expected: %s\n", result.FilePath)
        }
    }

    fmt.Printf("\nSummary:\n")
    fmt.Printf("  OK:       %d\n", okCount)
    fmt.Printf("  Modified: %d\n", modifiedCount)
    fmt.Printf("  Missing:  %d\n", missingCount)

    if modifiedCount > 0 && !sync {
        fmt.Printf("\nRun with --sync to update database from files.\n")
    }

    return nil
}
```

**internal/interface/cli/label_import.go（新規）**

```go
package cli

func newLabelImportCmd() *cobra.Command {
    var recursive bool
    var prefixFromDir bool
    var dryRun bool

    cmd := &cobra.Command{
        Use:   "import <directory>",
        Short: "Import labels from a directory",
        Long: `Import all .md files from a directory as labels.

This command scans a directory for markdown files and registers them as labels.
It respects exclude patterns from setting.json.

Examples:
  # Import .claude/ directory
  deespec label import .claude --recursive --prefix-from-dir

  # Dry run
  deespec label import .claude --recursive --dry-run`,
        Args: cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            dir := args[0]
            return importLabelsFromDirectory(dir, recursive, prefixFromDir, dryRun)
        },
    }

    cmd.Flags().BoolVar(&recursive, "recursive", false, "Import recursively")
    cmd.Flags().BoolVar(&prefixFromDir, "prefix-from-dir", false, "Use directory name as label prefix")
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be imported")

    return cmd
}

func importLabelsFromDirectory(dir string, recursive, prefixFromDir, dryRun bool) error {
    container, _ := initializeContainer()
    defer container.Close()

    labelRepo := container.GetLabelRepository()
    cfg := container.GetConfig().LabelConfig
    ctx := context.Background()

    // 除外判定関数
    shouldExclude := func(path string) bool {
        for _, pattern := range cfg.ImportConfig.ExcludePatterns {
            matched, _ := filepath.Match(pattern, filepath.Base(path))
            if matched {
                return true
            }
        }
        return false
    }

    // ファイル走査
    var files []string
    if recursive {
        filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
            if filepath.Ext(path) == ".md" && !shouldExclude(path) {
                files = append(files, path)
            }
            return nil
        })
    } else {
        entries, _ := os.ReadDir(dir)
        for _, e := range entries {
            if filepath.Ext(e.Name()) == ".md" {
                path := filepath.Join(dir, e.Name())
                if !shouldExclude(path) {
                    files = append(files, path)
                }
            }
        }
    }

    fmt.Printf("Found %d markdown files:\n\n", len(files))

    var registered int
    for _, filePath := range files {
        // ラベル名生成
        labelName := generateLabelName(filePath, dir, prefixFromDir)

        // ファイル読み込み
        content, _ := os.ReadFile(filePath)
        lines := bytes.Count(content, []byte("\n"))

        // 行数チェック
        if lines > cfg.ImportConfig.MaxLineCount {
            Warn("Skipping %s: exceeds %d lines (%d)", filePath, cfg.ImportConfig.MaxLineCount, lines)
            continue
        }

        if dryRun {
            fmt.Printf("Would register: %-30s (%s, %d lines)\n", labelName, filePath, lines)
            continue
        }

        // 相対パス計算（template_dirsの最初のディレクトリからの相対パス）
        baseDir := cfg.TemplateDirs[0]
        relPath, _ := filepath.Rel(baseDir, filePath)

        // ラベル作成
        lbl, _ := label.NewLabel(labelName, "", []string{relPath}, 0)

        // ハッシュ計算
        hash := calculateSHA256(content)
        lbl.SetContentHash(relPath, hash)
        lbl.SetLineCount(lines)

        // 保存
        if err := labelRepo.Save(ctx, lbl); err != nil {
            Warn("Failed to register %s: %v", labelName, err)
            continue
        }

        fmt.Printf("✓ %-30s (%d lines)\n", labelName, lines)
        registered++
    }

    if !dryRun {
        fmt.Printf("\nTotal: %d labels registered\n", registered)
    }

    return nil
}

func generateLabelName(filePath, baseDir string, prefixFromDir bool) string {
    relPath, _ := filepath.Rel(baseDir, filePath)

    if prefixFromDir {
        // .claude/perspectives/designer.md → "perspective:designer"
        parts := strings.Split(relPath, string(filepath.Separator))
        if len(parts) > 1 {
            dir := parts[0]
            file := strings.TrimSuffix(parts[len(parts)-1], ".md")
            return fmt.Sprintf("%s:%s", dir, strings.ToLower(file))
        }
    }

    // デフォルト: ファイル名のみ
    return strings.ToLower(strings.TrimSuffix(filepath.Base(filePath), ".md"))
}

func calculateSHA256(content []byte) string {
    hash := sha256.Sum256(content)
    return hex.EncodeToString(hash[:])
}
```

---

## 実装計画

### Phase 9.1a: スキーマ拡張（1-2時間）

#### タスク

1. **schema.sql更新**
   - `labels`テーブルに3フィールド追加:
     - `content_hashes TEXT`
     - `line_count INTEGER`
     - `last_synced_at DATETIME`
   - インデックス追加: `idx_labels_last_synced`

2. **マイグレーションバージョン追加**
   ```sql
   INSERT OR IGNORE INTO schema_migrations (version, description)
   VALUES (3, 'Add label management system with integrity check');
   ```

3. **初期データ投入（オプション）**
   ```sql
   -- サンプルラベル（.claude/が存在する場合は不要）
   INSERT OR IGNORE INTO labels (name, description, template_paths, priority) VALUES
       ('architecture', 'システムアーキテクチャ', '["architecture.md"]', 5);
   ```

#### 成果物

- ✅ `schema.sql` 更新（+60行）
- ✅ マイグレーション動作確認
- ✅ 既存DBへの適用テスト

---

### Phase 9.1b: setting.json拡張（1時間）

#### タスク

1. **設定構造体定義**
   - `internal/infra/config/settings.go` 拡張
   - `LabelConfig`, `LabelImportConfig`, `LabelValidationConfig` 追加

2. **デフォルト値設定**
   - `.claude`, `.deespec/prompts/labels` をデフォルトディレクトリに

3. **設定読み込みテスト**
   - `setting.json`のパース確認
   - デフォルト値のフォールバック確認

#### 成果物

- ✅ `settings.go` 拡張（+80行）
- ✅ 設定読み込みテスト（+50行）
- ✅ サンプル`setting.json`更新

---

### Phase 9.1c: Domain層・Repository層実装（3-4時間）

#### タスク

1. **Label ドメインモデル拡張**
   - `internal/domain/model/label/label.go`
   - `contentHashes`, `lineCount`, `lastSyncedAt` フィールド追加
   - メソッド追加: `SetContentHash`, `SetLineCount`, `UpdateSyncTime`

2. **Repository インターフェース拡張**
   - `internal/domain/repository/label_repository.go`
   - `ValidateIntegrity`, `ValidateAllLabels`, `SyncFromFile` 追加

3. **SQLite Repository実装**
   - `internal/infrastructure/persistence/sqlite/label_repository_impl.go`
   - CRUD操作（既存）
   - ワイルドカード展開ロジック
   - **複数ディレクトリ解決ロジック**（新規）
   - **SHA256ハッシュ計算・検証**（新規）
   - **整合性チェック・同期**（新規）

4. **DI Container統合**
   - `internal/infrastructure/di/container.go` 更新
   ```go
   func (c *Container) GetLabelRepository() repository.LabelRepository {
       cfg := c.config.LabelConfig
       return sqlite.NewLabelRepository(c.db, cfg)
   }
   ```

#### 成果物

- ✅ `label.go` 拡張（+100行）
- ✅ `label_repository.go` 拡張（+50行）
- ✅ `label_repository_impl.go` 実装（+700行）
- ✅ 単体テスト（+400行、90%以上カバレッジ）

---

### Phase 9.1d: CLI層実装（3-4時間）

#### タスク

1. **label_cmd.go リファクタリング**
   - 旧実装削除（~470行削除）
   - 新コマンド実装: `register`, `list`, `show`, `update`, `delete`, `attach`, `detach`, `templates`

2. **label_import.go 新規作成**
   - `import` コマンド実装
   - 除外パターン対応
   - 行数制限チェック
   - Dry-run対応

3. **label_validate.go 新規作成**
   - `validate` コマンド実装
   - ハッシュ検証ロジック
   - `--sync` オプション

4. **旧関数削除**
   ```go
   // 削除対象（~200行）
   func loadSBIMeta(path string) (*SBIMeta, error)
   func saveSBIMeta(path string, meta *SBIMeta) error
   func updateLabelIndex() error
   func loadLabelIndex() (*LabelIndex, error)
   func saveLabelIndex(index *LabelIndex) error
   ```

#### 成果物

- ✅ `label_cmd.go` 全面刷新（~500行）
- ✅ `label_import.go` 新規（~300行）
- ✅ `label_validate.go` 新規（~200行）
- ✅ 統合テスト（+500行）

---

### Phase 9.1e: EnrichTaskWithLabels改善（1-2時間）

#### タスク

1. **claude_prompt.go リファクタリング**
   - `EnrichTaskWithLabels()` 書き換え
   - Repository経由のデータ取得
   - 複数ディレクトリ対応
   - ハッシュ不一致時の警告追加

2. **テンプレート読み込みロジック改善**
   ```go
   // Before: ハードコードパス
   labelPath := filepath.Join(".deespec", "prompts", "labels", label+".md")

   // After: Repository経由 + 複数ディレクトリ解決
   templatePaths := labelRepo.ExpandTemplates(ctx, labelID)
   for _, path := range templatePaths {
       content, _ := os.ReadFile(path)
       // ハッシュ検証（警告のみ）
       if !validateHash(path, content) {
           Warn("Template file modified: %s (run 'deespec label validate --sync')", path)
       }
       // マージ処理
   }
   ```

3. **エラーハンドリング強化**
   - テンプレートファイル不在時の graceful fallback
   - ワイルドカード展開失敗時の警告

#### 成果物

- ✅ `claude_prompt.go` 改善（~200行変更）
- ✅ テスト追加（+250行）

---

### Phase 9.1f: テスト・ドキュメント（2時間）

#### タスク

1. **統合テスト実施**
   - 全フロー動作確認（import → attach → validate → agent実行）
   - エラーケーステスト
   - パフォーマンステスト

2. **ドキュメント作成**
   - ユーザーガイド（`docs/user-guide/labels.md`）
   - マイグレーションガイド（`docs/migration/phase-9.1.md`）

3. **サンプル作成**
   - `.claude/` サンプルディレクトリ
   - `setting.json` サンプル

4. **CHANGELOG更新**

#### 成果物

- ✅ ドキュメント2件
- ✅ サンプルファイル
- ✅ CHANGELOG.md更新

---

## 移行戦略

### 1. 旧データ移行フロー

```
┌──────────────────────────────────────────────────────┐
│ Step 1: .claude/ 一括インポート                       │
├──────────────────────────────────────────────────────┤
│ deespec label import .claude --recursive \            │
│   --prefix-from-dir --dry-run                         │
│   ↓                                                  │
│ 27件の.mdファイル検出、プレビュー表示                │
└──────────────────────────────────────────────────────┘
                        ↓
┌──────────────────────────────────────────────────────┐
│ Step 2: 実行                                          │
├──────────────────────────────────────────────────────┤
│ deespec label import .claude --recursive \            │
│   --prefix-from-dir                                   │
│   ↓                                                  │
│ 27ラベル登録完了                                      │
└──────────────────────────────────────────────────────┘
                        ↓
┌──────────────────────────────────────────────────────┐
│ Step 3: 旧データ移行（オプション）                     │
├──────────────────────────────────────────────────────┤
│ deespec label migrate                                 │
│   ↓                                                  │
│ .deespec/var/labels.json から既存ラベル・関連を移行  │
│ .deespec/var/labels.json.bak にバックアップ         │
└──────────────────────────────────────────────────────┘
                        ↓
┌──────────────────────────────────────────────────────┐
│ Step 4: 整合性チェック                                │
├──────────────────────────────────────────────────────┤
│ deespec label validate                                │
│   ↓                                                  │
│ 全ラベルのハッシュ検証OK                              │
└──────────────────────────────────────────────────────┘
```

### 2. 段階的移行スケジュール

| フェーズ | 期間 | 内容 |
|---------|------|------|
| **Phase 9.1a-f** | Week 1 | 新実装リリース、import/validateコマンド提供 |
| **移行期間** | Week 2 | `.claude/`等の一括インポート、動作確認 |
| **旧実装削除** | Week 3 | `.deespec/var/labels.json`関連コード削除 |

---

## テスト計画

### 1. 単体テスト

#### Label ドメインモデル

```go
func TestLabel_SetContentHash(t *testing.T)
func TestLabel_ValidateTemplatePath(t *testing.T)
func TestLabel_LineCountLimit(t *testing.T)
```

#### LabelRepository

```go
func TestLabelRepository_ExpandTemplates_MultipleDirectories(t *testing.T)
func TestLabelRepository_ValidateIntegrity_HashMismatch(t *testing.T)
func TestLabelRepository_SyncFromFile(t *testing.T)
func TestLabelRepository_ExpandWildcard(t *testing.T)
```

### 2. 統合テスト

#### CLI コマンド

```go
func TestLabelImportCommand_ExcludePatterns(t *testing.T)
func TestLabelValidateCommand_HashMismatch(t *testing.T)
func TestLabelValidateCommand_Sync(t *testing.T)
```

#### EnrichTaskWithLabels

```go
func TestEnrichTaskWithLabels_MultipleDirectories(t *testing.T)
func TestEnrichTaskWithLabels_HashMismatchWarning(t *testing.T)
```

### 3. E2Eテスト

シナリオ:

```bash
# 1. .claude/インポート
deespec label import .claude --recursive --prefix-from-dir

# 2. SBI作成とラベル付与
deespec sbi register --title "ログイン機能" \
  --labels perspective:designer,perspective:engineer

# 3. ファイル編集
vim .claude/perspectives/designer.md  # 手動編集

# 4. 整合性チェック
deespec label validate
# ⚠ perspective:designer - MODIFIED

# 5. 同期
deespec label validate --sync

# 6. Agent実行
deespec run  # → 指示書が正しく読み込まれるか確認
```

---

## リスクと対策

### リスク1: ハッシュ計算のパフォーマンス

**リスク内容:**
- 大量ラベル（100+）の検証で遅延

**対策:**
1. 並行ハッシュ計算（goroutine利用）
2. 最終同期日時でスキップ（最近同期済みは検証不要）
3. 増分検証（変更されたファイルのみ）

### リスク2: 複数ディレクトリでの重複ファイル

**リスク内容:**
- `.claude/security.md` と `.deespec/prompts/labels/security.md` が両方存在

**対策:**
1. 優先度順（`template_dirs`の順番）で解決
2. 警告表示（重複検出時）
3. `--strict`モードで重複エラー化（オプション）

### リスク3: setting.json編集ミス

**リスク内容:**
- JSON構文エラーで起動不可

**対策:**
1. バリデーション強化
2. デフォルト値へのフォールバック
3. `deespec config validate` コマンド提供

### リスク4: ファイル編集後の未同期

**リスク内容:**
- ユーザーがファイル編集後、`validate --sync`を忘れる

**対策:**
1. Agent実行時に自動チェック（警告のみ）
2. `auto_sync_on_mismatch: true` オプション（setting.json）
3. pre-commit hook提供（オプション）

---

## 実装チェックリスト

### Phase 9.1a: スキーマ拡張

- [ ] schema.sql に3フィールド追加
- [ ] インデックス追加
- [ ] マイグレーションバージョン3記録
- [ ] 既存DBへの適用テスト

### Phase 9.1b: setting.json拡張

- [ ] LabelConfig構造体定義
- [ ] デフォルト値実装
- [ ] 設定読み込みテスト
- [ ] サンプルsetting.json作成

### Phase 9.1c: Domain/Repository層

- [ ] Label エンティティ拡張（3フィールド+メソッド）
- [ ] LabelRepository インターフェース拡張
- [ ] LabelRepositoryImpl 実装
  - [ ] 複数ディレクトリ解決
  - [ ] SHA256ハッシュ計算
  - [ ] ValidateIntegrity実装
  - [ ] SyncFromFile実装
- [ ] DI Container統合
- [ ] 単体テスト（90%以上カバレッジ）

### Phase 9.1d: CLI層

- [ ] label_cmd.go 全面書き換え
  - [ ] register, list, show, update, delete
  - [ ] attach, detach, templates
- [ ] label_import.go 実装
  - [ ] 再帰的スキャン
  - [ ] 除外パターン
  - [ ] 行数制限
  - [ ] Dry-run
- [ ] label_validate.go 実装
  - [ ] ハッシュ検証
  - [ ] --sync オプション
- [ ] 旧関数削除（5個）
- [ ] 統合テスト

### Phase 9.1e: Prompt層

- [ ] EnrichTaskWithLabels改善
  - [ ] Repository経由
  - [ ] 複数ディレクトリ対応
  - [ ] ハッシュ不一致警告
- [ ] エラーハンドリング強化
- [ ] テスト追加

### Phase 9.1f: テスト・ドキュメント

- [ ] 統合テスト全Pass
- [ ] E2Eシナリオテスト
- [ ] ユーザーガイド作成
- [ ] マイグレーションガイド作成
- [ ] サンプルファイル作成
- [ ] CHANGELOG更新

### 最終確認

- [ ] 全テスト合格
- [ ] ドキュメント完備
- [ ] `.claude/`インポート動作確認
- [ ] 整合性チェック動作確認
- [ ] コードレビュー完了
- [ ] git commit & push
- [ ] Phase 9.1完了報告

---

## 参考資料

### 関連ドキュメント

- [Clean Architecture設計書](./clean-architecture-design.md)
- [SQLiteマイグレーション戦略](./sqlite-migration-strategy.md)
- [リファクタリング計画](./refactoring-plan.md)

### 外部リンク

- [SQLite JSON関数](https://www.sqlite.org/json1.html)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [SHA-256](https://en.wikipedia.org/wiki/SHA-2)

---

## 変更履歴

| 日付 | バージョン | 変更内容 |
|-----|----------|---------|
| 2025-10-09 | 1.0 | 初版作成 |
| 2025-10-09 | 2.0 | **改訂版**: ファイル優先アーキテクチャ、setting.json拡張、整合性チェック追加 |

---

**作成者**: Claude (Claude Code)
**レビュー**: 未実施
**承認**: 未実施
**最終更新**: 2025-10-09

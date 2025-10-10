# DeeSpec 機能改善候補リスト

このドキュメントは、今後実装すべき機能改善をトラッキングします。

**最終更新日**: 2025-10-10
**ステータス**: Phase 3完了後、Phase 8進行中

---

## 優先度の定義

- 🔴 **High**: ユーザビリティに直接影響、早急に実装すべき
- 🟡 **Medium**: 利便性向上、次のフェーズで実装検討
- 🟢 **Low**: Nice-to-have、余裕があれば実装

---

## 🔴 優先度: High

### 1. SBI/PBI/EPIC 一覧表示コマンド

**現状の問題:**
- 登録したSBI/PBI/EPICを確認するコマンドが存在しない
- ファイルシステムを直接探索するしかない
- 一覧性がなく、管理が困難

**提案する機能:**

```bash
# SBI一覧表示
deespec sbi list
deespec sbi list --format json
deespec sbi list --format table
deespec sbi list --filter status=draft
deespec sbi list --sort created_at

# SBI詳細表示
deespec sbi show <id-or-uuid>

# PBI一覧（将来）
deespec pbi list

# EPIC一覧（将来）
deespec epic list
```

**実装方針:**

#### Option A: ファイルシステムベース（簡易実装）
- `.deespec/specs/sbi/` 配下をスキャン
- 各UUIDディレクトリの `spec.md` を解析
- メタデータを抽出して表示

**利点:**
- 実装が簡単
- SQLite不要で即座に動作

**欠点:**
- パフォーマンス問題（大量のSBIで遅い）
- フィルタリング・ソート機能が限定的

#### Option B: SQLiteベース（推奨）
- `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go` を活用
- SQLiteにメタデータを保存・クエリ
- 高速なフィルタリング・ソート

**利点:**
- 高速なクエリ
- 複雑な検索条件に対応可能
- Clean Architectureに準拠

**欠点:**
- SQLiteスキーマの整備が必要
- 登録時にDBへの保存処理が必要

**推奨実装順序:**
1. Phase 8.3: SQLiteリポジトリの完全実装
2. Phase 8.4: `sbi list` コマンド実装
3. Phase 8.5: フィルタリング・ソート機能追加

**参考実装場所:**
- CLI: `internal/interface/cli/sbi/list.go` (新規作成)
- UseCase: `internal/application/usecase/sbi/list_sbi.go` (新規作成)
- Repository: `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go` (既存拡張)

**期待される出力例:**

```bash
$ deespec sbi list --format table

UUID                                  ID              Title                      Status    Created
e520e775-f36f-4edc-8519-19fb20449ecc  SBI-001         User Authentication        draft     2025-10-10 16:07
a1b2c3d4-e5f6-7890-abcd-ef1234567890  SBI-002         Database Migration         in_progress 2025-10-09 14:32
...
```

**関連Issue:**
- #N/A (新規作成予定)

---

### 2. meta.yml の完全廃止とSQLiteへの移行

**現状:**
- `meta.yml` は既に使用されていない（Phase 3で廃止）
- ファイルベース: `<uuid>/spec.md` のみ
- SQLiteリポジトリは実装済みだが、まだ完全移行していない

**提案する改善:**

1. **登録時のSQLite保存**
   - `register_sbi_usecase.go` でSQLiteに保存
   - spec.mdとSQLiteの両方に書き込み

2. **一覧表示・検索はSQLiteから**
   - `sbi list` はSQLiteをクエリ
   - ファイルシステムは読まない

3. **spec.md はバックアップ的位置づけ**
   - 人間が読める形式として保持
   - Gitで管理しやすい

**メリット:**
- 高速なクエリ
- 複雑な検索条件に対応
- スケーラブル

**実装場所:**
- `internal/application/usecase/register_sbi_usecase.go` - SQLite保存処理追加
- `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go` - Save/Find実装

---

## 🟡 優先度: Medium

### 3. SBI検索・フィルタリング機能

**提案する機能:**

```bash
# ラベルでフィルタリング
deespec sbi list --label backend --label security

# ステータスでフィルタリング
deespec sbi list --status draft

# タイトルで検索
deespec sbi list --search "authentication"

# 作成日でフィルタリング
deespec sbi list --created-after 2025-10-01

# 組み合わせ
deespec sbi list --label backend --status in_progress --sort created_at
```

**実装方針:**
- SQLiteのWHERE句とORDER BYを活用
- `SBIFilter` 構造体を拡張
- Cobraのフラグで条件を受け取る

**参考:**
```go
type SBIFilter struct {
    Labels       []string
    Status       *string
    SearchQuery  *string
    CreatedAfter *time.Time
    CreatedBefore *time.Time
    Limit        int
    Offset       int
    SortBy       string  // "created_at", "updated_at", "title"
    SortOrder    string  // "asc", "desc"
}
```

---

### 4. SBI詳細表示コマンド

**提案する機能:**

```bash
# UUIDまたはIDで詳細表示
deespec sbi show e520e775-f36f-4edc-8519-19fb20449ecc
deespec sbi show SBI-001

# JSON形式で出力
deespec sbi show SBI-001 --format json

# ファイルパスも表示
deespec sbi show SBI-001 --show-path
```

**期待される出力:**

```
SBI Details
===========

UUID:       e520e775-f36f-4edc-8519-19fb20449ecc
ID:         SBI-001
Title:      User Authentication
Status:     draft
Labels:     backend, security
Created:    2025-10-10 16:07:23 UTC
Updated:    2025-10-10 16:07:23 UTC
Path:       .deespec/specs/sbi/e520e775-f36f-4edc-8519-19fb20449ecc/spec.md

Description:
------------
[spec.mdの内容を表示]
```

**実装場所:**
- CLI: `internal/interface/cli/sbi/show.go` (新規作成)
- UseCase: `internal/application/usecase/sbi/get_sbi.go` (新規作成)

---

### 5. journal.ndjson の自動作成

**現状の問題:**
- SBI登録時に `journal.ndjson` が作成されない
- ジャーナル機能が動作していない可能性

**調査項目:**
1. `register_sbi_usecase.go` でジャーナル書き込みが実装されているか確認
2. `internal/infrastructure/transaction/register_transaction_service.go` の実装確認
3. ジャーナル機能の有効化フラグ確認

**期待される動作:**
```bash
# SBI登録後
cat .deespec/journal.ndjson | tail -1 | jq .
{
  "ts": "2025-10-10T16:07:23.123Z",
  "step": "register",
  "decision": "DONE",
  "artifacts": [
    {
      "type": "sbi",
      "id": "SBI-001",
      "uuid": "e520e775-f36f-4edc-8519-19fb20449ecc",
      "spec_path": ".deespec/specs/sbi/e520e775-f36f-4edc-8519-19fb20449ecc"
    }
  ]
}
```

**実装場所:**
- `internal/infrastructure/transaction/register_transaction_service.go`
- `internal/application/usecase/register_sbi_usecase.go`

---

## 🟢 優先度: Low

### 6. SBI編集コマンド

**提案する機能:**

```bash
# エディタで編集
deespec sbi edit SBI-001

# タイトル変更
deespec sbi update SBI-001 --title "New Title"

# ラベル追加
deespec sbi update SBI-001 --add-label new-label

# ステータス変更
deespec sbi update SBI-001 --status in_progress
```

---

### 7. SBI削除コマンド

**提案する機能:**

```bash
# SBI削除
deespec sbi delete SBI-001

# 確認なし削除
deespec sbi delete SBI-001 --force

# 複数削除
deespec sbi delete SBI-001 SBI-002 SBI-003
```

**実装方針:**
- SQLiteから削除
- ファイルシステムは `.deespec/archive/` に移動（完全削除ではない）

---

### 8. エクスポート・インポート機能

**提案する機能:**

```bash
# JSON形式でエクスポート
deespec sbi export --output sbi-backup.json

# CSVエクスポート
deespec sbi export --format csv --output sbi-list.csv

# インポート
deespec sbi import --input sbi-backup.json
```

**ユースケース:**
- バックアップ・リストア
- 他のプロジェクトへの移行
- Excel/Google Sheetsでの管理

---

### 9. バージョン情報の充実化

**現状:**
```bash
$ deespec version
deespec version dev
  Go version:    go1.23.0
  OS/Arch:       darwin/arm64
  Compiler:      gc
```

**提案する追加情報:**

```bash
$ deespec version --verbose
deespec version v1.0.0
  Build Date:    2025-10-10 16:00:00 UTC
  Git Commit:    a00cffe
  Git Branch:    main
  Go version:    go1.23.0
  OS/Arch:       darwin/arm64
  Compiler:      gc

Database:
  SQLite:        enabled
  Schema:        v1.2.0

Features:
  Label System:  enabled
  Lock System:   SQLite-based
  Journal:       enabled
```

**実装方針:**
- build時に `-ldflags` で埋め込み
- `internal/buildinfo/version.go` に追加フィールド

---

## 実装の進め方

### Phase 8.3: SBI管理機能（推奨）

```bash
# 実装順序
1. SQLiteリポジトリの完全実装
   - Save, Find, List, Delete メソッド
   - テスト追加

2. sbi list コマンド実装
   - CLI: sbi/list.go
   - UseCase: sbi/list_sbi.go
   - 基本的な一覧表示

3. sbi show コマンド実装
   - CLI: sbi/show.go
   - UseCase: sbi/get_sbi.go
   - 詳細表示

4. フィルタリング機能追加
   - --label, --status, --search フラグ
   - SQLiteクエリ拡張

5. ジャーナル機能の修正
   - register時のjournal.ndjson書き込み確認
   - 必要に応じて修正
```

### Phase 9: 高度な管理機能

```bash
1. sbi update コマンド
2. sbi delete コマンド
3. export/import機能
4. バージョン情報の充実化
```

---

## 関連ドキュメント

- [Clean Architecture設計](./architecture/clean-architecture-design.md)
- [リファクタリング計画](./architecture/refactoring-plan.md)
- [SQLite移行戦略](./architecture/sqlite-migration-strategy.md)
- [CLI層ファイル分類](./architecture/cli-files-classification.md)

---

## 変更履歴

| 日付 | 変更内容 | 担当 |
|------|---------|------|
| 2025-10-10 | 初版作成（Phase 3完了後） | Claude |

---

## フィードバック

機能改善の提案や優先度の変更がある場合は、このドキュメントを更新してください。

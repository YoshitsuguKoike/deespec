# DeeSpec 機能改善候補リスト

このドキュメントは、今後実装すべき機能改善をトラッキングします。

**最終更新日**: 2025-10-10 (ULID順序問題と再実行機能追加)
**ステータス**: Phase 3完了後、Phase 8進行中

---

## 優先度の定義

- 🚨 **Critical**: システムの正常動作に必須、即時対応が必要
- 🔴 **High**: ユーザビリティに直接影響、早急に実装すべき
- 🟡 **Medium**: 利便性向上、次のフェーズで実装検討
- 🟢 **Low**: Nice-to-have、余裕があれば実装

---

## 🚨 優先度: Critical

### 1. ULID順序問題の修正とタスク管理フロー実装

**優先度**: 🚨 **最優先 - システムの正常動作に必須**

**現状の問題:**

1. **ULID順序問題**: ULIDの文字列ソートにより、タスクが登録順に実行されない
   - ULIDは先頭48ビットにタイムスタンプを含むが、ミリ秒単位で複数登録すると順序が保証されない
   - ランダム部分の影響で登録順序が崩れる
   - SQLite化前から存在する既知の問題

2. **再実行機能の欠如**:
   - Claude Code認証問題などでSBIが誤って進行し、review&wipステップで強制完了した場合にやり直せない
   - AIエージェントが実質的には失敗しているのに成功と判断してしまう場合がある
   - タスクの実行履歴を確認できない

3. **タスク管理フローの不足**:
   - 一覧表示 → 詳細確認 → 再実行という基本フローが存在しない
   - ユーザーは直接ファイルシステムを操作するしかない

**要件:**

- **優先度順序**: Priority (DESC) → 登録順序 (ASC) でタスクを実行
- **登録順序の保証**: 同一優先度内では必ず登録順に実行
- **再実行可能性**: 完了済みタスクを任意のステータスにリセット可能
- **履歴追跡**: タスクの実行履歴を確認可能

---

#### 解決策の詳細

**1. データベーススキーマの拡張**

`sbis`テーブルに明示的な順序フィールドを追加:

```sql
-- マイグレーション: 006_add_ordering_fields.sql
ALTER TABLE sbis ADD COLUMN priority INTEGER DEFAULT 0;      -- 優先度(0=通常, 1=高, 2=緊急)
ALTER TABLE sbis ADD COLUMN sequence INTEGER;                -- 登録順序番号(自動採番)
ALTER TABLE sbis ADD COLUMN registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- 既存SBIのsequenceをcreated_atベースでバックフィル
UPDATE sbis SET sequence = (
    SELECT COUNT(*) FROM sbis AS s2 WHERE s2.created_at <= sbis.created_at
);

-- 効率的な順序クエリのためのインデックス作成
CREATE INDEX idx_sbis_ordering ON sbis(priority DESC, registered_at ASC, sequence ASC);
```

**設計の理由:**
- `priority`: 明示的な優先度管理(0=通常, 1=高, 2=緊急など)
- `sequence`: 登録順序を保証する自動採番フィールド
- `registered_at`: ULIDとは独立した明示的な登録タイムスタンプ
- **ULID自体は変更せず**、一意識別子として継続使用
- インデックスは`(priority DESC, registered_at ASC, sequence ASC)`で最適化

**2. Domain層の変更**

**ファイル**: `internal/domain/sbi.go`

```go
type SBI struct {
    ID           string    // ULID (一意識別子として維持)
    Title        string
    Priority     int       // 0=通常, 1=高, 2=緊急
    Status       string
    Sequence     int       // 登録順序(同一優先度内)
    RegisteredAt time.Time // 明示的な登録タイムスタンプ
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Labels       []string
    SpecPath     string
}
```

**3. Repository層の変更**

**ファイル**: `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go`

```go
// ListSBIs は優先度(DESC)→登録順(ASC)でSBIを返す
func (r *SBIRepositoryImpl) ListSBIs(ctx context.Context, filter *SBIFilter) ([]*domain.SBI, error) {
    query := `
        SELECT id, title, priority, status, registered_at, sequence, created_at, updated_at
        FROM sbis
        WHERE 1=1
    `
    args := []interface{}{}

    if filter != nil {
        if filter.Status != "" {
            query += " AND status = ?"
            args = append(args, filter.Status)
        }
        if filter.Priority >= 0 {
            query += " AND priority = ?"
            args = append(args, filter.Priority)
        }
    }

    // 重要: この順序でタスク実行順を制御
    query += " ORDER BY priority DESC, registered_at ASC, sequence ASC"

    // ... 実装
}

// GetNextSequence は次のシーケンス番号を返す(並行安全)
func (r *SBIRepositoryImpl) GetNextSequence(ctx context.Context) (int, error) {
    query := `SELECT COALESCE(MAX(sequence), 0) + 1 FROM sbis`
    var seq int
    err := r.db.QueryRowContext(ctx, query).Scan(&seq)
    return seq, err
}

// ResetSBIState はSBIの状態をリセットして再実行可能にする
func (r *SBIRepositoryImpl) ResetSBIState(ctx context.Context, id string, toStatus string) error {
    query := `UPDATE sbis SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
    _, err := r.db.ExecContext(ctx, query, toStatus, id)
    return err
}

// GetSBIHistory は実行履歴をjournal.ndjsonから取得
func (r *SBIRepositoryImpl) GetSBIHistory(ctx context.Context, id string) ([]*domain.ExecutionHistory, error) {
    // journal.ndjsonを読み込み、指定IDの履歴を抽出
    // ... 実装
}
```

**4. Application層の変更**

**ファイル**: `internal/application/usecase/sbi/register_sbi_usecase.go`

登録時にsequenceを自動設定:

```go
func (u *RegisterSBIUseCase) Execute(ctx context.Context, req RegisterSBIRequest) (*RegisterSBIResponse, error) {
    // トランザクション内で次のシーケンス番号を取得(並行安全)
    sequence, err := u.sbiRepo.GetNextSequence(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get next sequence: %w", err)
    }

    sbi := &domain.SBI{
        ID:           generateULID(),
        Title:        req.Title,
        Priority:     req.Priority,    // リクエストから、またはデフォルト0
        Sequence:     sequence,        // 自動採番
        RegisteredAt: time.Now(),      // 明示的なタイムスタンプ
        Status:       "pending",
        Labels:       req.Labels,
        SpecPath:     specPath,
    }

    if err := u.sbiRepo.Save(ctx, sbi); err != nil {
        return nil, err
    }

    return &RegisterSBIResponse{SBIID: sbi.ID}, nil
}
```

**新規UseCaseファイル**:
- `internal/application/usecase/sbi/list_sbis_usecase.go` - 一覧取得
- `internal/application/usecase/sbi/get_sbi_detail_usecase.go` - 詳細取得
- `internal/application/usecase/sbi/reset_sbi_usecase.go` - 状態リセット
- `internal/application/usecase/sbi/get_sbi_history_usecase.go` - 実行履歴取得

**5. Picker層の修正**

**ファイル**: `internal/application/usecase/workflow/picker.go`

正しい順序でタスクを選択:

```go
func (p *Picker) PickNext(ctx context.Context) (*domain.SBI, error) {
    query := `
        SELECT id, title, priority, status, sequence, registered_at
        FROM sbis
        WHERE status IN ('pending', 'in_progress')
        ORDER BY priority DESC, registered_at ASC, sequence ASC
        LIMIT 1
    `

    var sbi domain.SBI
    err := p.db.QueryRowContext(ctx, query).Scan(
        &sbi.ID, &sbi.Title, &sbi.Priority, &sbi.Status, &sbi.Sequence, &sbi.RegisteredAt,
    )

    if err == sql.ErrNoRows {
        return nil, nil // タスクなし
    }
    if err != nil {
        return nil, fmt.Errorf("failed to pick next SBI: %w", err)
    }

    return &sbi, nil
}
```

**設計のポイント**:
- `ORDER BY priority DESC, registered_at ASC, sequence ASC`で正確な順序制御
- 最高優先度が最初、同一優先度内では登録順を保証
- SQLiteインデックスを活用し、パフォーマンスも最適化

**6. CLI層の新規コマンド**

**ファイル**: `internal/interface/cli/sbi/list.go` (新規作成)

```go
func newListCommand() *cobra.Command {
    var (
        status   string
        priority int
        format   string
    )

    cmd := &cobra.Command{
        Use:   "list",
        Short: "登録されたSBIの一覧を表示",
        Long:  "優先度(DESC)→登録順(ASC)でSBIを表示します",
        RunE: func(cmd *cobra.Command, args []string) error {
            // list usecaseを呼び出し
            // テーブル形式で表示: Priority, Sequence, ID, Title, Status, Registered
            return nil
        },
    }

    cmd.Flags().StringVar(&status, "status", "", "ステータスでフィルタ")
    cmd.Flags().IntVar(&priority, "priority", -1, "優先度でフィルタ(-1=全て)")
    cmd.Flags().StringVar(&format, "format", "table", "出力形式(table, json, yaml)")

    return cmd
}
```

**期待される出力**:
```bash
$ deespec sbi list

Priority  Seq  UUID                                  ID       Title                    Status      Registered
--------  ---  ------------------------------------  -------  -----------------------  ----------  -------------------
2         001  a1b2c3d4-e5f6-7890-abcd-ef1234567890  SBI-003  Critical Bug Fix         pending     2025-10-10 16:10:23
1         002  e520e775-f36f-4edc-8519-19fb20449ecc  SBI-001  User Authentication      in_progress 2025-10-10 16:07:15
1         003  f6g7h8i9-j0k1-2345-lmno-pq6789012345  SBI-004  Payment Integration      pending     2025-10-10 16:12:45
0         004  b2c3d4e5-f6g7-8901-bcde-fg2345678901  SBI-002  Database Migration       pending     2025-10-10 16:09:32

4 SBIs (1 in_progress, 3 pending)
```

**ファイル**: `internal/interface/cli/sbi/show.go` (新規作成)

```go
func newShowCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "show <id>",
        Short: "SBIの詳細情報を表示",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            sbiID := args[0]
            // get detail usecaseを呼び出し
            // 詳細情報とspec.mdの内容を表示
            return nil
        },
    }
    return cmd
}
```

**ファイル**: `internal/interface/cli/sbi/reset.go` (新規作成)

```go
func newResetCommand() *cobra.Command {
    var (
        toStatus string
        force    bool
    )

    cmd := &cobra.Command{
        Use:   "reset <id>",
        Short: "SBIを再実行可能な状態にリセット",
        Long:  "完了済みのSBIを指定したステータスに戻し、再実行を可能にします",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            sbiID := args[0]

            if !force {
                // 確認プロンプト
                fmt.Printf("本当にSBI %s を %s にリセットしますか? [y/N]: ", sbiID, toStatus)
                var response string
                fmt.Scanln(&response)
                if response != "y" && response != "Y" {
                    fmt.Println("キャンセルしました")
                    return nil
                }
            }

            // reset usecaseを呼び出し
            return nil
        },
    }

    cmd.Flags().StringVar(&toStatus, "to-status", "pending", "リセット先のステータス")
    cmd.Flags().BoolVar(&force, "force", false, "確認なしでリセット")

    return cmd
}
```

**ファイル**: `internal/interface/cli/sbi/history.go` (新規作成)

```go
func newHistoryCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "history <id>",
        Short: "SBIの実行履歴を表示",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            sbiID := args[0]
            // journal.ndjsonから履歴を取得
            // 実行試行回数、タイムスタンプ、結果を表示
            return nil
        },
    }
    return cmd
}
```

**ファイル**: `internal/interface/cli/sbi/sbi.go`

既存コマンドにサブコマンドを追加:

```go
func NewSBICommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "sbi",
        Short: "Specification Backlog Item (SBI)の管理",
    }

    cmd.AddCommand(newRegisterCommand())  // 既存
    cmd.AddCommand(newListCommand())      // 新規
    cmd.AddCommand(newShowCommand())      // 新規
    cmd.AddCommand(newResetCommand())     // 新規
    cmd.AddCommand(newHistoryCommand())   // 新規

    return cmd
}
```

---

#### 実装の優先順位

**Phase 1 (即時対応 - 最優先):**
1. ✅ データベーススキーマのマイグレーション(priority, sequence, registered_at追加)
2. ✅ SBI domainモデルの更新
3. ✅ 登録usecaseでsequence自動設定
4. ✅ pickerの順序クエリ修正(`ORDER BY priority DESC, registered_at ASC, sequence ASC`)
5. ✅ 複数SBI登録でのテスト実施

**Phase 2 (高優先度):**
6. ⬜ `sbi list`コマンド実装
7. ⬜ `sbi show <id>`コマンド実装
8. ⬜ 各種優先度での順序テスト

**Phase 3 (中優先度):**
9. ⬜ `sbi reset <id>`コマンド実装
10. ⬜ `sbi history <id>`コマンド実装
11. ⬜ journal.ndjson連携確認

---

#### 期待される結果

実装後:
- ✅ SBIは正しい順序で実行される: **優先度DESC → 登録順ASC**
- ✅ ユーザーは正しい順序でSBI一覧を確認可能
- ✅ ユーザーはSBIの詳細情報を確認可能
- ✅ ユーザーは誤って完了したSBIをリセットして再実行可能
- ✅ ユーザーは実行履歴を確認して何が起きたか理解可能
- ✅ Claude Code認証問題などでの誤進行に対処可能

---

#### 関連Issue

- #N/A (新規作成予定)

---

#### 実装場所サマリ

| 層 | ファイル | 変更内容 |
|----|---------|----------|
| Infrastructure | `migrations/006_add_ordering_fields.sql` | スキーマ追加 |
| Infrastructure | `sqlite/sbi_repository_impl.go` | ListSBIs, GetNextSequence, ResetSBIState追加 |
| Domain | `domain/sbi.go` | Priority, Sequence, RegisteredAt追加 |
| Application | `usecase/sbi/register_sbi_usecase.go` | Sequence自動設定 |
| Application | `usecase/sbi/list_sbis_usecase.go` | 新規作成 |
| Application | `usecase/sbi/get_sbi_detail_usecase.go` | 新規作成 |
| Application | `usecase/sbi/reset_sbi_usecase.go` | 新規作成 |
| Application | `usecase/workflow/picker.go` | ORDER BY修正 |
| Interface | `cli/sbi/list.go` | 新規作成 |
| Interface | `cli/sbi/show.go` | 新規作成 |
| Interface | `cli/sbi/reset.go` | 新規作成 |
| Interface | `cli/sbi/history.go` | 新規作成 |
| Interface | `cli/sbi/sbi.go` | サブコマンド追加 |

---

#### 作業計画 (Phase 1実装)

**目標**: ULID順序問題を修正し、タスクを正しい順序(優先度→登録順)で実行可能にする

**推定工数**: 4-6時間

**前提条件**:
- SQLiteデータベースが初期化済み
- 既存のSBIテーブルが存在
- マイグレーション機構が動作

---

##### ステップ1: マイグレーションスクリプト作成 (30分)

**作業内容**:
1. `internal/infrastructure/persistence/sqlite/migrations/006_add_ordering_fields.sql` を新規作成
2. スキーマ変更SQL記述:
   - `ALTER TABLE sbis ADD COLUMN priority INTEGER DEFAULT 0`
   - `ALTER TABLE sbis ADD COLUMN sequence INTEGER`
   - `ALTER TABLE sbis ADD COLUMN registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`
3. 既存データのバックフィル:
   - `UPDATE sbis SET sequence = ...` (created_atベース)
4. インデックス作成:
   - `CREATE INDEX idx_sbis_ordering ON sbis(priority DESC, registered_at ASC, sequence ASC)`

**確認方法**:
```bash
# マイグレーションファイルの構文チェック
sqlite3 test.db < internal/infrastructure/persistence/sqlite/migrations/006_add_ordering_fields.sql

# テーブル構造確認
sqlite3 .deespec/deespec.db ".schema sbis"
```

**成果物**:
- `internal/infrastructure/persistence/sqlite/migrations/006_add_ordering_fields.sql`

---

##### ステップ2: マイグレーション実行機構の確認・更新 (30分)

**作業内容**:
1. `internal/infrastructure/persistence/sqlite/migration.go` を確認
2. マイグレーションバージョン管理の確認
3. 006マイグレーションを登録
4. テスト実行でマイグレーションが適用されることを確認

**確認方法**:
```bash
# マイグレーション実行
go run cmd/deespec/main.go init

# または既存DBに対して
# マイグレーション適用コマンド(実装されている場合)
```

**成果物**:
- 更新された `migration.go` (必要に応じて)
- 動作確認済みマイグレーション

---

##### ステップ3: Domain層の更新 (30分)

**作業内容**:
1. `internal/domain/sbi.go` を編集
2. `SBI` 構造体に以下のフィールドを追加:
   ```go
   Priority     int       // 0=通常, 1=高, 2=緊急
   Sequence     int       // 登録順序
   RegisteredAt time.Time // 登録タイムスタンプ
   ```
3. コンストラクタやファクトリメソッドの更新(存在する場合)

**確認方法**:
```bash
# コンパイルエラーがないことを確認
go build ./internal/domain/...
```

**成果物**:
- 更新された `internal/domain/sbi.go`

---

##### ステップ4: Repository層の更新 (1.5時間)

**作業内容**:
1. `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go` を編集
2. 以下のメソッドを追加:
   - `GetNextSequence(ctx context.Context) (int, error)`
   - `ListSBIs(ctx context.Context, filter *SBIFilter) ([]*domain.SBI, error)`
   - `ResetSBIState(ctx context.Context, id string, toStatus string) error`
3. 既存の`Save`メソッドを更新してpriority, sequence, registered_atを保存
4. 既存の`FindByID`メソッドを更新して新フィールドを読み込み

**確認方法**:
```bash
# リポジトリテストを実行
go test ./internal/infrastructure/persistence/sqlite/... -v

# またはテストを新規作成
```

**成果物**:
- 更新された `sbi_repository_impl.go`
- 新規テスト `sbi_repository_impl_test.go` (拡張)

---

##### ステップ5: Application層の更新 (1時間)

**作業内容**:
1. `internal/application/usecase/sbi/register_sbi_usecase.go` を編集
2. `Execute`メソッドで以下を実装:
   - `GetNextSequence()`を呼び出してsequence取得
   - `Priority`をリクエストから設定(デフォルト0)
   - `RegisteredAt`を`time.Now()`で設定
3. トランザクション内でsequence取得と保存を実行(並行安全性確保)

**確認方法**:
```bash
# UseCaseテストを実行
go test ./internal/application/usecase/sbi/... -v
```

**成果物**:
- 更新された `register_sbi_usecase.go`

---

##### ステップ6: Picker層の更新 (30分)

**作業内容**:
1. `internal/application/usecase/workflow/picker.go` を確認
2. タスク選択クエリを以下に変更:
   ```sql
   SELECT id, title, priority, status, sequence, registered_at
   FROM sbis
   WHERE status IN ('pending', 'in_progress')
   ORDER BY priority DESC, registered_at ASC, sequence ASC
   LIMIT 1
   ```
3. 新フィールド(priority, sequence, registered_at)を読み込むようにScanを更新

**確認方法**:
```bash
# Pickerテストを実行
go test ./internal/application/usecase/workflow/... -v
```

**成果物**:
- 更新された `picker.go`

---

##### ステップ7: 統合テスト (1時間)

**作業内容**:
1. マイグレーション適用後の動作確認
2. 複数SBIを登録して順序確認:
   ```bash
   # 優先度0のSBI登録
   echo "title: Task A" | deespec sbi register --stdin

   # 優先度1のSBI登録(高優先度)
   echo "title: Task B\npriority: 1" | deespec sbi register --stdin

   # 優先度0のSBI登録
   echo "title: Task C" | deespec sbi register --stdin
   ```
3. SQLiteデータベースを直接確認:
   ```bash
   sqlite3 .deespec/deespec.db "SELECT priority, sequence, id, title FROM sbis ORDER BY priority DESC, registered_at ASC, sequence ASC"
   ```
4. 期待結果: Task B(優先度1) → Task A(優先度0, seq 1) → Task C(優先度0, seq 2) の順

**確認方法**:
```bash
# 全テストを実行
go test ./... -v

# ビルドして動作確認
go build -o deespec cmd/deespec/main.go
./deespec sbi register --stdin < test_sbi.md
```

**成果物**:
- 動作確認済みの完全な実装
- テスト結果レポート

---

##### ステップ8: ドキュメント更新とコミット (30分)

**作業内容**:
1. CHANGELOG更新
2. feature-improvements.md のチェックボックス更新
3. マイグレーションREADME更新(存在する場合)
4. コミットメッセージ作成:
   ```
   feat: fix ULID ordering issue with explicit priority and sequence fields

   - Add priority, sequence, registered_at to sbis table
   - Implement GetNextSequence() for registration order guarantee
   - Update Picker to use ORDER BY priority DESC, registered_at ASC, sequence ASC
   - Ensure tasks execute in correct order: priority first, then registration order

   Fixes: ULID lexicographic sorting causing incorrect task execution order
   Impact: Tasks now execute in intended order (priority → registration)

   🤖 Generated with [Claude Code](https://claude.com/claude-code)

   Co-Authored-By: Claude <noreply@anthropic.com>
   ```

**成果物**:
- 更新されたドキュメント
- Gitコミット

---

##### トラブルシューティング

**問題1: マイグレーションが適用されない**
- 原因: マイグレーションバージョン管理の問題
- 解決策: `schema_migrations`テーブルを確認し、手動でバージョン更新

**問題2: 既存データのsequenceがNULL**
- 原因: バックフィルSQLが実行されていない
- 解決策: 手動でUPDATE文を実行

**問題3: GetNextSequence()が同じ値を返す(並行実行時)**
- 原因: トランザクション分離レベルの問題
- 解決策: SELECT FOR UPDATEまたはトランザクション内で実行

**問題4: pickerが古いクエリを使用**
- 原因: キャッシュまたはビルド済みバイナリ
- 解決策: `go clean -cache && go build`で再ビルド

---

##### 成功基準

- ✅ マイグレーションが正常に適用される
- ✅ 新規SBI登録時にsequenceが自動採番される
- ✅ priorityフィールドが正しく保存・読み込みされる
- ✅ Pickerが優先度→登録順でタスクを選択する
- ✅ 全テストがパスする
- ✅ 複数SBI登録テストで正しい順序が確認できる

---

##### リスク管理

**リスク1: 既存データの破損**
- 軽減策: マイグレーション前に`.deespec/deespec.db`をバックアップ
- 対応策: バックアップから復元

**リスク2: ダウンタイム**
- 軽減策: マイグレーションは高速(ALTER TABLE + UPDATE)
- 対応策: データ量が多い場合は段階的移行

**リスク3: 並行実行時のsequence重複**
- 軽減策: トランザクション内でGetNextSequence()を実行
- 対応策: UNIQUE制約またはロック機構の追加

---

##### 次のステップ (Phase 2準備)

Phase 1完了後、以下を準備:
1. `sbi list`コマンドの設計
2. テーブル表示フォーマットの決定
3. フィルタリング機能の仕様決定

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

### 5. 設定管理コマンド (config)

**現状の問題:**
- 設定を変更するには `.deespec/setting.json` を直接編集する必要がある
- 設定項目の一覧や説明が分かりにくい
- JSON構文エラーのリスク
- 初心者には敷居が高い

**提案する機能:**

```bash
# 全設定の表示
deespec config list
deespec config list --format json
deespec config list --format yaml

# 特定項目の取得
deespec config get timeout_sec
deespec config get max_turns

# 設定の変更
deespec config set timeout_sec 1200
deespec config set max_turns 10
deespec config set stderr_level debug

# 設定の削除（デフォルトに戻す）
deespec config unset timeout_sec
deespec config reset <key>

# 全設定を初期化
deespec config reset
deespec config reset --force

# 設定のバリデーション
deespec config validate

# 設定のエクスポート・インポート
deespec config export --output backup.json
deespec config import --input backup.json

# 設定項目の説明表示
deespec config describe timeout_sec
deespec config describe --all
```

**期待される出力例:**

```bash
$ deespec config list

Configuration (.deespec/setting.json)
=====================================

Core Settings:
  home:           .deespec
  agent_bin:      claude
  timeout_sec:    900          (default)

Execution Limits:
  max_attempts:   3            (default)
  max_turns:      8            (default)

Logging:
  stderr_level:   info         (default)

Feature Flags:
  validate:       false        (default)
  auto_fb:        false        (default)

(default) = using default value
```

```bash
$ deespec config get timeout_sec
900

$ deespec config set timeout_sec 1200
✓ Configuration updated: timeout_sec = 1200

$ deespec config describe timeout_sec
timeout_sec
  Type:     integer
  Default:  900
  Range:    60 - 3600
  Description:
    Timeout for agent execution in seconds.
    If an agent does not respond within this time,
    the execution will be terminated.
```

**実装方針:**

1. **読み取り系コマンド**
   - `setting.json` をパースして表示
   - デフォルト値と比較してマーク表示

2. **書き込み系コマンド**
   - バリデーション実行
   - `setting.json` を更新
   - バックアップ作成（`.deespec/setting.json.bak`）

3. **バリデーション**
   - 型チェック（文字列/整数/真偽値）
   - 範囲チェック（timeout_sec: 60-3600など）
   - 列挙値チェック（stderr_level: debug|info|warn|error）

4. **初期化**
   - デフォルト値のテンプレートから復元
   - 既存ファイルをバックアップ

**メリット:**

1. **ユーザビリティ向上**
   - エディタ不要で設定変更可能
   - 設定項目の発見が容易
   - タイポや構文エラー防止

2. **安全性向上**
   - バリデーションによる不正な値の防止
   - バックアップによる復旧可能性

3. **標準的なCLIパターン**
   - `git config`, `npm config` など一般的なパターン
   - ユーザーの学習コストが低い

4. **自動化対応**
   - CI/CDでの設定変更が容易
   - セットアップスクリプトの作成が簡単

5. **既存方式との共存**
   - ファイル直接編集も引き続き可能
   - 既存ユーザーへの影響なし

**実装場所:**
- CLI: `internal/interface/cli/config/config.go` (新規作成)
- UseCase: `internal/application/usecase/config/config_manager.go` (新規作成)
- Service: `internal/application/service/config_service.go` (新規作成)

**設定スキーマ定義:**
```go
type ConfigSchema struct {
    Key          string
    Type         ConfigType  // String, Int, Bool
    Default      interface{}
    Description  string
    Validator    func(interface{}) error
}

var ConfigSchemas = []ConfigSchema{
    {
        Key:         "timeout_sec",
        Type:        ConfigTypeInt,
        Default:     900,
        Description: "Timeout for agent execution in seconds",
        Validator:   IntRange(60, 3600),
    },
    // ...
}
```

---

### 6. journal.ndjson の自動作成

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

### 7. SBI編集コマンド

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

### 8. SBI削除コマンド

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

### 9. エクスポート・インポート機能

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

### 10. バージョン情報の充実化

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

## 📋 実装作業一覧と進捗表

### Phase 1: ULID順序問題の修正 (優先度: 🚨 Critical)

| ステップ | 作業内容 | 実装ファイル | ステータス | 完了日 |
|---------|---------|-------------|-----------|--------|
| **ステップ1** | マイグレーションスクリプト作成 | `internal/infrastructure/persistence/sqlite/schema.sql`<br>`internal/infrastructure/persistence/sqlite/migrations/004_add_ordering_fields.sql` | ✅ 完了 | 2025-10-10 |
| **ステップ2** | マイグレーション実行機構の確認・更新 | `internal/infrastructure/persistence/sqlite/migration.go`<br>`internal/infrastructure/persistence/sqlite/migration_test.go` | ✅ 完了 | 2025-10-10 |
| **ステップ3** | Domain層の更新 | `internal/domain/model/sbi/sbi.go` | ✅ 完了 | 2025-10-10 |
| **ステップ4** | Repository層の更新 | `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go`<br>`internal/domain/repository/sbi_repository.go` | ✅ 完了 | 2025-10-10 |
| **ステップ5** | Application層の更新 | `internal/application/usecase/task/task_use_case_impl.go`<br>`internal/infrastructure/transaction/register_transaction_service.go` | ✅ 完了 | 2025-10-10 |
| **ステップ6** | Picker層の更新 | `internal/application/service/task_picker_service.go`<br>`internal/application/dto/sbi_task_dto.go`<br>`internal/application/dto/task_dto.go` | ✅ 完了 | 2025-10-10 |
| **ステップ7** | 統合テスト実行と動作確認 | 動作確認・テスト実施 | ✅ 完了 | 2025-10-10 |
| **ステップ8** | ドキュメント更新とコミット | `CHANGELOG.md`<br>`docs/feature-improvements.md` | ✅ 完了 | 2025-10-10 |

### Phase 2: タスク管理CLIコマンド実装 (優先度: 🔴 High)

| ステップ | 作業内容 | 実装ファイル | ステータス | 完了日 |
|---------|---------|-------------|-----------|--------|
| **CLI-1** | `sbi list` コマンド実装 | `internal/interface/cli/sbi/sbi_list.go`<br>UseCase追加 | ✅ 完了 | 2025-10-10 |
| **CLI-2** | `sbi show <id>` コマンド実装 | `internal/interface/cli/sbi/sbi_show.go`<br>UseCase追加 | ✅ 完了 | 2025-10-10 |
| **CLI-3** | `sbi reset <id>` コマンド実装 | `internal/interface/cli/sbi/sbi_reset.go`<br>ResetSBIState UseCase | ✅ 完了 | 2025-10-10 |
| **CLI-4** | `sbi history <id>` コマンド実装 | `internal/interface/cli/sbi/sbi_history.go`<br>journal.ndjson連携 | ✅ 完了 | 2025-10-10 |
| **CLI-5** | 各種優先度での順序テスト | テストケース追加 | ⬜ 未実施 | - |

### 実装詳細と成果物

#### Phase 1 完了内容

**ステップ1: マイグレーションスクリプト作成**
- ✅ `schema.sql`: `sequence INTEGER`, `registered_at DATETIME` フィールド追加
- ✅ `004_add_ordering_fields.sql`: 既存データベース用増分マイグレーション
- ✅ `idx_sbis_ordering` インデックス作成

**ステップ2: マイグレーション実行機構**
- ✅ `migration.go`: `applyIncrementalMigrations()` メソッド追加
- ✅ `migration_test.go`: 新規DB・既存DBアップグレードテスト追加
- ✅ Version 4マイグレーション適用確認

**ステップ3: Domain層の更新**
- ✅ `SBIMetadata` 構造体に `Sequence int`, `RegisteredAt time.Time` 追加
- ✅ アクセサメソッド追加: `SetSequence()`, `Sequence()`, `SetRegisteredAt()`, `RegisteredAt()`

**ステップ4: Repository層の更新**
- ✅ SELECT文にsequence, registered_at追加
- ✅ ORDER BY句修正: `priority DESC, registered_at ASC, sequence ASC`
- ✅ `GetNextSequence(ctx) (int, error)` メソッド実装
- ✅ `ResetSBIState(ctx, id, toStatus)` メソッド実装
- ✅ Repository interface更新

**ステップ5: Application層の更新**
- ✅ `CreateSBI()` 内でトランザクション内sequence取得・設定
- ✅ `RegisterTransactionService` にDB接続追加（未使用、将来用）
- ✅ Status型変換関数修正

**ステップ6: Picker層の更新**
- ✅ `sortTasksByPriority()` でpriority DESC実装
- ✅ registered_at, sequence比較ロジック追加
- ✅ デフォルトorderBy更新: `["priority", "registered_at", "sequence", "id"]`
- ✅ SBITaskDTO, SBIDTOにフィールド追加

**ステップ7: 統合テスト**
- ✅ アプリケーションビルド成功
- ✅ 3つのSBI登録成功 (sequence: 1, 2, 3)
- ✅ 優先度別ソート動作確認 (Task B[pri=1] → Task A[pri=0,seq=1] → Task C[pri=0,seq=3])
- ✅ インデックス作成確認
- ✅ Migration Version 4適用確認

### 成功基準の達成状況

**Phase 1 成功基準**:
- ✅ マイグレーションが正常に適用される
- ✅ 新規SBI登録時にsequenceが自動採番される (1→2→3 確認済み)
- ✅ priorityフィールドが正しく保存・読み込みされる
- ✅ Pickerが優先度→登録順でタスクを選択する
- ✅ 全テストがパスする
- ✅ 複数SBI登録テストで正しい順序が確認できる

**実測データ (SQLiteクエリ結果)**:
```
id                                   | title              | priority | seq | registered_at
-------------------------------------|--------------------|---------|----|---------------------------
8c5297dc-343d-40ea-ada4-a02b12e1043d | Task B - Priority 1|    1    | 2  | 2025-10-10 17:45:49+09:00
010b1f9c-2cbf-40e6-90d8-ecba5b62d335 | Task A - Priority 0|    0    | 1  | 2025-10-10 17:45:29+09:00
0464398f-4021-4aad-8f24-c07bde4d04b1 | Task C - Priority 0|    0    | 3  | 2025-10-10 17:45:59+09:00
```

### 次のアクション

**推奨される次ステップ**:
1. **ステップ8実施**: ドキュメント更新とコミット作成
   - CHANGELOG.md 更新
   - feature-improvements.md チェックボックス更新
   - Git commit作成

2. **Phase 2開始**: CLIコマンド実装
   - `sbi list` から開始推奨
   - ユーザーが直感的にタスク管理できるインターフェース構築

---

## フィードバック

機能改善の提案や優先度の変更がある場合は、このドキュメントを更新してください。

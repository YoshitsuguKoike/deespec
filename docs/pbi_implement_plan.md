# PBI実装計画：2つの登録方式

> サンプルを作成してから段階的に改善していくアプローチで実装します。

---

## 目次

1. [2つの登録方式の概要](#1-2つの登録方式の概要)
2. [ID自動生成の仕様](#2-id自動生成の仕様)
3. [Phase 1: 基本機能実装](#3-phase-1-基本機能実装)
4. [Phase 2: 利便性向上](#4-phase-2-利便性向上)
5. [実装の優先順位とロードマップ](#5-実装の優先順位とロードマップ)
6. [共通コンポーネント設計](#6-共通コンポーネント設計)
7. [サンプルデータとテスト計画](#7-サンプルデータとテスト計画)
8. [データベーススキーマ](#8-データベーススキーマ)

---

## 1. 2つの登録方式の概要

### 設計原則

- ✅ **シンプル**: 登録方式は2つのみ（対話式、ファイル方式）
- ✅ **ID自動生成**: ユーザーはIDを指定しない（システムが自動採番）
- ✅ **柔軟なオーバーライド**: ファイル方式でも一部の値を上書き可能
- ✅ **API連携は別レイヤー**: Go APIで直接DBに登録（CLI経由しない）

---

### 1.1 対話式（Interactive Registration）

**概要**: ウィザード形式で対話的に入力してPBIを登録

**使用場面**:
- 手動でPBIを作成したい場合
- ドキュメントファイルを持っていない場合
- 小規模なPBIの素早い作成

**コマンド**:
```bash
deespec pbi register
```

**実行例**:
```bash
$ deespec pbi register

🎯 PBI作成ウィザード

📝 タイトルを入力してください:
> テストカバレッジ50%達成

📋 説明を入力してください（複数行可、空行で終了）:
> CIの要件である50%カバレッジを達成するため、
> ドメインモデル、Repository、Application層のテストを追加する。
>

🔢 ストーリーポイント（1-13）:
> 8

⭐ 優先度（0=通常, 1=高, 2=緊急）:
> 1

📦 PBI-001として登録しますか？ [Y/n]
> Y

✅ PBI-001を登録しました

詳細表示: deespec pbi show PBI-001
```

**メリット**:
- ✅ 初心者に優しい
- ✅ 入力ミスを防げる
- ✅ ガイド付きで迷わない

**デメリット**:
- ⚠️ 対話的なので自動化に不向き

---

### 1.2 Markdownファイル方式（Markdown-based Registration）

**概要**: Markdownファイルから読み込んでPBIを登録

**設計思想**:
- ✅ **YAML入力は不要**: 構造化データ(YAML)は人間が事前に評価を決める必要がある
- ✅ **AIAgentが自由に評価**: Markdownの自然言語からAIAgentがstory pointsや優先度を判断
- ✅ **意図を伝える**: 人間は「やりたいこと」を自然言語で記述するだけ

**使用場面**:
- `docs/`配下の計画書からPBI化
- AIツールとの連携（推奨方式）
- バージョン管理とレビュー
- 既存ドキュメントの再利用

**コマンド**:
```bash
# Markdownファイルから登録
deespec pbi register -f docs/plan.md

# titleを上書き
deespec pbi register -t "カスタムタイトル" -f docs/plan.md
```

**メリット**:
- ✅ ドキュメントファーストのワークフロー
- ✅ AIAgentが自由に評価できる
- ✅ バージョン管理・レビュー可能
- ✅ 既存の計画書を活用可能
- ✅ 構造化データを書く必要がない

**デメリット**:
- ⚠️ Markdownパーサーの精度に依存
- ⚠️ AIAgentの評価に依存（story points等）

---

### 1.3 API連携について

**APIからのデータ登録は別レイヤーで実装**

```go
// 外部APIからの登録は、GoのAPIを直接使用
pbi := &domain.PBI{
    Title:       fetchedData.Title,
    Description: fetchedData.Body,
    // ...
}

repo := repository.NewPBIRepository()
pbi.ID, _ = generatePBIID(repo)
repo.Save(pbi)
```

**理由**:
- CLIは人間のインターフェース
- プログラムからの登録はGo APIを直接使用
- 責任の分離（CLI層とドメイン層）

---

## 2. ID自動生成の仕様

### 2.1 ID生成ルール

**フォーマット**: `PBI-{3桁の連番}`

**例**:
```
PBI-001
PBI-002
PBI-003
...
PBI-099
PBI-100
...
PBI-999
```

### 2.2 生成方法（2つの選択肢）

#### 方法A: ディレクトリスキャン方式（推奨）

**仕組み**: `.deespec/specs/pbi/`のファイルをスキャンして最大番号を取得

**実装**:
```go
func generatePBIID() (string, error) {
    pbiDir := ".deespec/specs/pbi"

    files, err := os.ReadDir(pbiDir)
    if err != nil {
        if os.IsNotExist(err) {
            return "PBI-001", nil
        }
        return "", err
    }

    maxNum := 0
    re := regexp.MustCompile(`PBI-(\d+)\.yaml`)

    for _, file := range files {
        matches := re.FindStringSubmatch(file.Name())
        if len(matches) == 2 {
            num, _ := strconv.Atoi(matches[1])
            if num > maxNum {
                maxNum = num
            }
        }
    }

    nextNum := maxNum + 1
    return fmt.Sprintf("PBI-%03d", nextNum), nil
}
```

**メリット**:
- ✅ カウンターファイル不要
- ✅ PBIを削除しても番号が重複しない
- ✅ シンプル

**デメリット**:
- ⚠️ ファイルが多いとスキャンが遅い（数千件以上）

---

#### 方法B: カウンターファイル方式

**仕組み**: `.deespec/var/pbi_counter`で番号を管理

**実装**:
```go
func generatePBIID() (string, error) {
    counterFile := ".deespec/var/pbi_counter"

    // カウンター読み込み
    data, err := os.ReadFile(counterFile)
    if err != nil {
        if os.IsNotExist(err) {
            // 初回
            if err := os.WriteFile(counterFile, []byte("1\n"), 0644); err != nil {
                return "", err
            }
            return "PBI-001", nil
        }
        return "", err
    }

    current, err := strconv.Atoi(strings.TrimSpace(string(data)))
    if err != nil {
        return "", err
    }

    next := current + 1
    id := fmt.Sprintf("PBI-%03d", next)

    // カウンター更新
    if err := os.WriteFile(counterFile, []byte(fmt.Sprintf("%d\n", next)), 0644); err != nil {
        return "", err
    }

    return id, nil
}
```

**メリット**:
- ✅ 高速（ファイルスキャン不要）

**デメリット**:
- ⚠️ カウンターファイルの管理が必要
- ⚠️ 同時実行時に競合の可能性（ロック必要）

---

### 2.3 推奨：方法A（ディレクトリスキャン方式）

**理由**:
- シンプル（追加のファイル不要）
- 数百件程度なら性能問題なし
- 将来的にDBを使う場合も移行しやすい

---

## 3. Phase 1: 基本機能実装

### 3.1 実装スコープ

#### 実装するコマンド

```bash
# PBI登録（対話式）
deespec pbi register

# PBI登録（ファイル方式）
deespec pbi register -f <file>

# PBI表示
deespec pbi show <id>

# PBI一覧
deespec pbi list
deespec pbi list --status PENDING
```

#### 実装しないもの（Phase 2以降）

- ドキュメント解析の高度化
- LLM連携
- 外部サービス連携
- PBI更新・削除

---

### 3.2 ディレクトリ構造

```
.deespec/
├── specs/
│   └── pbi/
│       ├── PBI-001/
│       │   └── pbi.md            # Markdown形式（YAMLは使わない）
│       ├── PBI-002/
│       │   └── pbi.md
│       └── .../
├── var/
│   ├── journal.ndjson            # 操作履歴
│   └── health.json               # ヘルスチェック
└── config/
    └── agents.yaml               # Agent設定（既存）
```

**ファイル保存形式**:
- ✅ **Markdownで保存**: `.deespec/specs/pbi/PBI-001/pbi.md`
- ✅ **YAMLは使わない**: 構造化データではなく自然言語
- ✅ **titleはH1から抽出**: Markdownの最初の`# Title`から取得
- ✅ **bodyは全文**: DBにMarkdown全文を保存

---

### 3.3 Markdownファイル例

**`.deespec/specs/pbi/PBI-001/pbi.md`**:
```markdown
# テストカバレッジ50%達成

## 目的

CIの要件である50%カバレッジを達成する。

## 背景

現在の全体カバレッジは34.9%で、CIの要件（50%以上）を満たしていない。

## アプローチ

Phase 1: ドメインモデルテスト（1-2日）
Phase 2: Repository層テスト（2-3日）
Phase 3: Usecase層部分テスト（3-5日）

## 受け入れ基準

- [ ] カバレッジ >= 50%
- [ ] CIがパスする
- [ ] ドメインモデルのテストカバレッジ >= 90%

## 見積もり

ストーリーポイント: 8
優先度: 高
```

**Phase 1での設計ポイント**:
- ✅ **ユーザー入力**: Markdownファイルまたは対話式のみ
- ✅ **YAML不要**: Markdownが直接保存される
- ✅ **AIAgent評価**: story points, priorityはAIAgentが判断可能
- ✅ IDとタイムスタンプはシステムが自動生成

---

### 3.4 コマンド実装

#### `deespec pbi register`

**機能**:

1. **引数なし**: 対話式モード
2. **`-f <file>`**: Markdownファイルから登録
   - Markdownのみサポート（YAMLは内部フォーマット）
   - パースして変換
3. **`-t <title>`**: titleを上書き（`-f`と併用）

**実装ファイル**:
```
internal/interface/cli/pbi/register.go
internal/application/usecase/pbi/register_pbi_use_case.go
internal/domain/model/pbi/pbi.go
internal/infrastructure/persistence/pbi_md_repository.go
```

**cobra設定**:
```go
// internal/interface/cli/pbi/register.go

func NewRegisterCommand() *cobra.Command {
    var (
        filePath string
        title    string
    )

    cmd := &cobra.Command{
        Use:   "register",
        Short: "Register a new PBI",
        Long: `Register a Product Backlog Item (PBI) in two ways:
  1. Interactive mode (default)
  2. From Markdown file

The PBI ID is automatically generated by the system.
YAML input is not supported (YAML is internal format only).`,
        Example: `  # Interactive mode
  deespec pbi register

  # From Markdown file
  deespec pbi register -f docs/plan.md

  # From file with title override
  deespec pbi register -t "Custom Title" -f docs/plan.md`,
        RunE: func(cmd *cobra.Command, args []string) error {
            return runRegister(filePath, title)
        },
    }

    cmd.Flags().StringVarP(&filePath, "file", "f", "", "Load from Markdown file")
    cmd.Flags().StringVarP(&title, "title", "t", "", "Override title from file")

    return cmd
}

func runRegister(filePath, title string) error {
    var pbi *domain.PBI
    var err error

    if filePath != "" {
        // ファイルから読み込み
        pbi, err = loadFromFile(filePath)
        if err != nil {
            return err
        }

        // -t でtitleを上書き
        if title != "" {
            pbi.Title = title
        }
    } else {
        // 対話式
        pbi, err = runInteractive()
        if err != nil {
            return err
        }
    }

    // UseCase実行
    useCase := usecase.NewRegisterPBIUseCase()
    pbiID, err := useCase.Execute(pbi)
    if err != nil {
        return fmt.Errorf("failed to register PBI: %w", err)
    }

    fmt.Printf("✅ PBI registered: %s\n", pbiID)
    fmt.Printf("\nView details: deespec pbi show %s\n", pbiID)

    return nil
}
```

**UseCase実装**:
```go
// internal/application/usecase/pbi/register_pbi_use_case.go

type RegisterPBIUseCase struct {
    repo domain.PBIRepository
}

func (u *RegisterPBIUseCase) Execute(pbi *domain.PBI) (string, error) {
    // 1. ID自動生成
    pbi.ID = generatePBIID(u.repo)

    // 2. タイムスタンプ設定
    now := time.Now()
    pbi.CreatedAt = now
    pbi.UpdatedAt = now

    // 3. デフォルト値設定
    if pbi.Status == "" {
        pbi.Status = domain.StatusPending
    }
    if pbi.Priority == 0 {
        pbi.Priority = domain.PriorityNormal
    }

    // 4. バリデーション
    if err := pbi.Validate(); err != nil {
        return "", fmt.Errorf("validation failed: %w", err)
    }

    // 5. 保存
    if err := u.repo.Save(pbi); err != nil {
        return "", err
    }

    // 6. Journal記録
    journal.RecordPBIEvent("pbi.registered", pbi.ID, map[string]interface{}{
        "title":  pbi.Title,
        "status": pbi.Status,
    })

    return pbi.ID, nil
}
```

---

#### `deespec pbi show <id>`

**出力例**:
```bash
$ deespec pbi show PBI-001

📦 PBI-001: テストカバレッジ50%達成
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Description
  CIの要件である50%カバレッジを達成するため、
  ドメインモデル、Repository、Application層のテストを追加する。

📊 Status: PENDING
🔢 Story Points: 8
⭐ Priority: 高 (1)

📄 Source Document
  docs/test-coverage-plan.md

🕐 Created: 2025-10-11 09:00:00
🕐 Updated: 2025-10-11 09:00:00
```

---

#### `deespec pbi list`

**出力例**:
```bash
$ deespec pbi list

PBI一覧（全3件）
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ID         STATUS        SP  PRIORITY  TITLE
─────────────────────────────────────────────
PBI-001    PENDING        8  高        テストカバレッジ50%達成
PBI-002    IMPLEMENTING   5  通常      API認証機能追加
PBI-003    DONE           3  緊急      ログインタイムアウト修正

Use 'deespec pbi show <id>' for details.
```

**フィルタ例**:
```bash
# ステータスでフィルタ
$ deespec pbi list --status PENDING

# 優先度でフィルタ
$ deespec pbi list --priority 1
```

---

### 3.5 実装順序（Phase 1）

#### Week 1: 基本構造

1. **ドメインモデル実装**
   ```go
   // internal/domain/model/pbi/pbi.go

   type PBI struct {
       ID                   string    // システム生成
       Title                string    // pbi.mdのH1から抽出
       Status               Status    // 5段階
       EstimatedStoryPoints int
       Priority             Priority
       ParentEpicID         string    // 親EPIC（オプション）
       CreatedAt            time.Time // システム生成
       UpdatedAt            time.Time // システム生成

       // Note: 本文は.deespec/specs/pbi/{id}/pbi.mdに保存（DBには保存しない）
   }

   type Status string
   const (
       StatusPending    Status = "pending"      // 未着手
       StatusPlanning   Status = "planning"     // 計画中
       StatusPlaned     Status = "planed"       // 計画完了
       StatusInProgress Status = "in_progress"  // 実行中
       StatusDone       Status = "done"         // 完了
   )

   type Priority int
   const (
       PriorityNormal Priority = 0  // 通常
       PriorityHigh   Priority = 1  // 高
       PriorityUrgent Priority = 2  // 緊急
   )

   func (p *PBI) Validate() error {
       if p.Title == "" {
           return fmt.Errorf("title is required")
       }
       if p.EstimatedStoryPoints < 0 || p.EstimatedStoryPoints > 13 {
           return fmt.Errorf("story points must be between 0 and 13")
       }
       return nil
   }

   // Markdownファイルパスを取得
   func (p *PBI) GetMarkdownPath() string {
       return filepath.Join(".deespec", "specs", "pbi", p.ID, "pbi.md")
   }
   ```

2. **Repository実装**
   ```go
   // internal/domain/model/pbi/repository.go

   type Repository interface {
       Save(pbi *PBI, body string) error      // bodyはMarkdown本文
       FindByID(id string) (*PBI, error)       // メタデータのみ取得
       GetBody(id string) (string, error)      // Markdown本文取得
       FindAll() ([]*PBI, error)
       FindByStatus(status Status) ([]*PBI, error)
   }
   ```

3. **Markdown Repository実装**
   ```go
   // internal/infrastructure/persistence/pbi_md_repository.go

   type PBIMarkdownRepository struct {
       rootPath string
   }

   func (r *PBIMarkdownRepository) Save(pbi *PBI, body string) error {
       // 1. ディレクトリ作成
       pbiDir := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", pbi.ID)
       if err := os.MkdirAll(pbiDir, 0755); err != nil {
           return err
       }

       // 2. Markdownファイル保存
       mdPath := filepath.Join(pbiDir, "pbi.md")
       if err := os.WriteFile(mdPath, []byte(body), 0644); err != nil {
           return err
       }

       // 3. DBにメタデータ保存（実装は後述）
       return r.saveMetadata(pbi)
   }

   func (r *PBIMarkdownRepository) FindByID(id string) (*PBI, error) {
       // DBからメタデータ取得（title, status, etc.）
       return r.findMetadata(id)
   }

   func (r *PBIMarkdownRepository) GetBody(id string) (string, error) {
       // Markdownファイルから本文取得
       mdPath := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", id, "pbi.md")
       data, err := os.ReadFile(mdPath)
       if err != nil {
           return "", err
       }
       return string(data), nil
   }
   ```

#### Week 2: コマンド実装

4. **register コマンド**
   - CLI実装
   - UseCase実装
   - バリデーション
   - ID自動生成

5. **show コマンド**
   - CLI実装
   - 整形表示

6. **list コマンド**
   - CLI実装
   - フィルタ機能

7. **対話モード実装**
   - promptライブラリ選定
   - 入力フロー実装

8. **Markdownパーサー実装**
   - 基本パーサー
   - テスト

9. **テスト＆サンプル作成**
   - サンプルMarkdownファイル
   - 動作確認

---

## 4. Phase 2: 利便性向上

### 4.1 追加機能

#### オプション追加

```bash
# ファイル登録時に各種パラメータを上書き
deespec pbi register -f plan.md \
  -t "カスタムタイトル" \
  --story-points 8 \
  --priority 1 \
  --status IMPLEMENTING
```

#### PBI更新コマンド

```bash
# ステータス更新
deespec pbi update PBI-001 --status IMPLEMENTING

# 編集（エディタ起動）
deespec pbi edit PBI-001
```

#### PBI削除コマンド

```bash
deespec pbi delete PBI-001
```

---

## 5. 実装の優先順位とロードマップ

### 5.1 マイルストーン

#### Milestone 1（Week 2終了時）

**目標**: 基本的なPBI管理ができる

- ✅ `deespec pbi register` が動作（対話式）
- ✅ `deespec pbi register -f` が動作（Markdownのみ）
- ✅ `deespec pbi show/list` が動作
- ✅ ID自動生成が動作
- ✅ サンプルMarkdownファイルで動作確認完了

**検証方法**:
```bash
# Markdownから登録
deespec pbi register -f samples/docs/test-coverage-plan.md

# 確認
deespec pbi list
deespec pbi show PBI-001

# Success!
```

---

#### Milestone 2（Week 4終了時）

**目標**: 実用的な機能が揃う

- ✅ title等のオーバーライド機能
- ✅ PBI更新・削除機能
- ✅ フィルタ機能の充実
- ✅ エラーハンドリングの改善

---

## 6. 共通コンポーネント設計

### 6.1 ディレクトリ構成

```
internal/
├── interface/
│   └── cli/
│       └── pbi/
│           ├── register.go              # register コマンド
│           ├── register_interactive.go  # 対話モード
│           ├── show.go                  # show コマンド
│           ├── list.go                  # list コマンド
│           └── common.go                # 共通処理
│
├── application/
│   └── usecase/
│       └── pbi/
│           ├── register_pbi_use_case.go
│           ├── show_pbi_use_case.go
│           └── list_pbi_use_case.go
│
├── domain/
│   └── model/
│       └── pbi/
│           ├── pbi.go                   # PBI構造体
│           ├── repository.go            # Repository interface
│           ├── validator.go             # バリデーション
│           └── status.go                # Status, Priority等
│
└── infrastructure/
    ├── persistence/
    │   ├── pbi_md_repository.go         # Markdownファイル永続化
    │   ├── pbi_db_repository.go         # DBメタデータ永続化
    │   └── journal_writer.go            # Journal記録
    └── parser/
        └── markdown_parser.go           # Markdownパーサー
```

---

### 6.2 ID生成処理

```go
// internal/domain/model/pbi/id_generator.go

func GeneratePBIID(repo Repository) (string, error) {
    pbis, err := repo.FindAll()
    if err != nil {
        return "", err
    }

    maxNum := 0
    re := regexp.MustCompile(`PBI-(\d+)`)

    for _, pbi := range pbis {
        matches := re.FindStringSubmatch(pbi.ID)
        if len(matches) == 2 {
            num, _ := strconv.Atoi(matches[1])
            if num > maxNum {
                maxNum = num
            }
        }
    }

    nextNum := maxNum + 1
    return fmt.Sprintf("PBI-%03d", nextNum), nil
}
```

---

### 6.3 Journal連携

すべてのPBI操作をjournal.ndjsonに記録：

```json
{"ts":"2025-10-11T09:00:00Z","event":"pbi.registered","pbi_id":"PBI-001","method":"markdown","title":"テストカバレッジ50%達成"}
{"ts":"2025-10-11T09:05:00Z","event":"pbi.registered","pbi_id":"PBI-002","method":"markdown","title":"API認証機能追加"}
{"ts":"2025-10-11T09:10:00Z","event":"pbi.registered","pbi_id":"PBI-003","method":"interactive","title":"ログインタイムアウト修正"}
{"ts":"2025-10-11T10:00:00Z","event":"pbi.status_changed","pbi_id":"PBI-002","from":"PENDING","to":"IMPLEMENTING"}
```

**実装**:
```go
// internal/infrastructure/persistence/journal_writer.go

func RecordPBIEvent(event string, pbiID string, metadata map[string]interface{}) error {
    entry := map[string]interface{}{
        "ts":     time.Now().Format(time.RFC3339Nano),
        "event":  event,
        "pbi_id": pbiID,
    }
    for k, v := range metadata {
        entry[k] = v
    }

    // .deespec/var/journal.ndjsonに追記
    return appendToJournal(entry)
}
```

---

## 7. サンプルデータとテスト計画

### 7.1 サンプルファイル配置

```
samples/docs/
├── test-coverage-plan.md            # テストカバレッジ改善計画
├── api-authentication-plan.md       # API認証機能計画
└── login-timeout-fix.md             # ログインタイムアウト修正計画
```

**注意**: YAMLサンプルは不要。Markdownファイルのみ。

---

### 7.2 サンプルMarkdownファイル

#### samples/docs/test-coverage-plan.md
```markdown
# テストカバレッジ50%達成

## 目的

CIの要件である50%カバレッジを達成する。

## 背景

現在の全体カバレッジは34.9%で、CIの要件（50%以上）を満たしていない。

## アプローチ

Phase 1: ドメインモデルテスト（1-2日）
Phase 2: Repository層テスト（2-3日）
Phase 3: Usecase層部分テスト（3-5日）

## 受け入れ基準

- [ ] カバレッジ >= 50%
- [ ] CIがパスする
- [ ] ドメインモデルのテストカバレッジ >= 90%

## 見積もり

ストーリーポイント: 8
優先度: 高
```

---

### 7.3 テストシナリオ

#### Phase 1: 基本機能

```bash
# 1. 初期化確認
deespec init
ls -la .deespec/specs/pbi/

# 2. Markdownから登録
deespec pbi register -f samples/docs/test-coverage-plan.md
# 期待結果: ✅ PBI registered: PBI-001

# 3. 表示確認
deespec pbi show PBI-001
# 期待結果: タイトル、説明、ステータスが表示される

# 4. 一覧確認
deespec pbi list
# 期待結果: 1件表示される（ID: PBI-001）

# 5. title上書きして登録
deespec pbi register -t "カスタムタイトル" -f samples/docs/api-authentication-plan.md
# 期待結果: ✅ PBI registered: PBI-002（titleが上書きされる）

# 6. 対話式
deespec pbi register
# → ウィザード形式で入力
# 期待結果: ✅ PBI registered: PBI-003

# 7. 一覧確認
deespec pbi list
# 期待結果: 3件表示される

# Success!
```

---

### 7.4 自動テスト

```go
// internal/application/usecase/pbi/register_pbi_use_case_test.go

func TestRegisterPBIUseCase_Execute(t *testing.T) {
    tests := []struct {
        name    string
        pbi     *domain.PBI
        wantErr bool
    }{
        {
            name: "valid PBI registration",
            pbi: &domain.PBI{
                Title:                "Test PBI",
                Description:          "Test description",
                Status:               domain.StatusPending,
                EstimatedStoryPoints: 5,
                Priority:             domain.PriorityNormal,
            },
            wantErr: false,
        },
        {
            name: "invalid PBI (missing title)",
            pbi: &domain.PBI{
                Description: "Test",
            },
            wantErr: true,
        },
        {
            name: "invalid story points",
            pbi: &domain.PBI{
                Title:                "Test",
                EstimatedStoryPoints: 20,
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            useCase := NewRegisterPBIUseCase()
            _, err := useCase.Execute(tt.pbi)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestGeneratePBIID(t *testing.T) {
    // モックRepository
    repo := &mockRepository{
        pbis: []*domain.PBI{
            {ID: "PBI-001"},
            {ID: "PBI-002"},
            {ID: "PBI-005"},
        },
    }

    id, err := GeneratePBIID(repo)
    if err != nil {
        t.Fatalf("GeneratePBIID() error = %v", err)
    }

    if id != "PBI-006" {
        t.Errorf("GeneratePBIID() = %v, want PBI-006", id)
    }
}
```

---

## 8. データベーススキーマ

### 8.1 データベース選択

**推奨: SQLite**

**理由**:
- ✅ ローカルツール（サーバー不要）
- ✅ ファイルベース（`.deespec/var/deespec.db`）
- ✅ 軽量で高速
- ✅ トランザクションサポート
- ✅ Go標準ライブラリで使用可能

---

### 8.2 スキーマ設計

#### テーブル構成

```sql
-- PBIテーブル（メタデータのみ）
CREATE TABLE pbis (
    id TEXT PRIMARY KEY,                    -- PBI-001, PBI-002, ...

    -- 基本情報（検索用）
    title TEXT NOT NULL,                    -- pbi.mdのH1から抽出

    -- ステータス（5段階）
    status TEXT NOT NULL DEFAULT 'pending', -- pending | planning | planed | in_progress | done

    -- 見積もりと優先度
    estimated_story_points INTEGER,
    priority INTEGER NOT NULL DEFAULT 0,    -- 0=通常, 1=高, 2=緊急

    -- 階層構造
    parent_epic_id TEXT,                    -- 親EPIC ID（オプション）

    -- タイムスタンプ
    created_at TEXT NOT NULL,               -- ISO 8601
    updated_at TEXT NOT NULL,               -- ISO 8601

    -- 制約
    CHECK (priority >= 0 AND priority <= 2),
    CHECK (estimated_story_points IS NULL OR estimated_story_points > 0),
    CHECK (status IN ('pending', 'planning', 'planed', 'in_progress', 'done')),
    FOREIGN KEY (parent_epic_id) REFERENCES pbis(id) ON DELETE SET NULL
);

-- インデックス
CREATE INDEX idx_pbis_status ON pbis(status);
CREATE INDEX idx_pbis_priority ON pbis(priority);
CREATE INDEX idx_pbis_created_at ON pbis(created_at);
```

**Note**: 本文（body）は`.deespec/specs/pbi/{id}/pbi.md`に保存。DBには保存しない。

---

### 8.3 マイグレーション戦略

#### マイグレーションファイル

```
.deespec/migrations/
└── 001_create_pbis.sql
```

#### 001_create_pbis.sql
```sql
-- Migration: 001
-- Description: Create pbis table (metadata only)

CREATE TABLE pbis (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    estimated_story_points INTEGER,
    priority INTEGER NOT NULL DEFAULT 0,
    parent_epic_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,

    CHECK (priority >= 0 AND priority <= 2),
    CHECK (estimated_story_points IS NULL OR estimated_story_points > 0),
    CHECK (status IN ('pending', 'planning', 'planed', 'in_progress', 'done')),
    FOREIGN KEY (parent_epic_id) REFERENCES pbis(id) ON DELETE SET NULL
);

CREATE INDEX idx_pbis_status ON pbis(status);
CREATE INDEX idx_pbis_priority ON pbis(priority);
CREATE INDEX idx_pbis_created_at ON pbis(created_at);
```

#### マイグレーション実行

```go
// internal/infrastructure/persistence/migration/migrator.go

type Migrator struct {
    db *sql.DB
}

func (m *Migrator) Migrate() error {
    // マイグレーションバージョン管理テーブル
    if err := m.createMigrationTable(); err != nil {
        return err
    }

    // マイグレーションファイルを順番に実行
    migrations := []string{
        "001_create_pbis.sql",
    }

    for _, migration := range migrations {
        if err := m.runMigration(migration); err != nil {
            return fmt.Errorf("migration %s failed: %w", migration, err)
        }
    }

    return nil
}

func (m *Migrator) createMigrationTable() error {
    _, err := m.db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version TEXT PRIMARY KEY,
            applied_at DATETIME NOT NULL
        )
    `)
    return err
}
```

---

### 8.4 Repository実装（SQLite版）

```go
// internal/infrastructure/persistence/pbi_sqlite_repository.go

type PBISQLiteRepository struct {
    db       *sql.DB
    rootPath string
}

func (r *PBISQLiteRepository) Save(pbi *domain.PBI, body string) error {
    tx, err := r.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. DBにメタデータ保存
    _, err = tx.Exec(`
        INSERT INTO pbis (
            id, title, status, estimated_story_points, priority,
            parent_epic_id, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title = excluded.title,
            status = excluded.status,
            estimated_story_points = excluded.estimated_story_points,
            priority = excluded.priority,
            parent_epic_id = excluded.parent_epic_id,
            updated_at = excluded.updated_at
    `,
        pbi.ID, pbi.Title, pbi.Status, pbi.EstimatedStoryPoints,
        pbi.Priority, pbi.ParentEpicID, pbi.CreatedAt, pbi.UpdatedAt,
    )
    if err != nil {
        return err
    }

    // 2. Markdownファイル保存
    pbiDir := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", pbi.ID)
    if err := os.MkdirAll(pbiDir, 0755); err != nil {
        return err
    }

    mdPath := filepath.Join(pbiDir, "pbi.md")
    if err := os.WriteFile(mdPath, []byte(body), 0644); err != nil {
        return err
    }

    return tx.Commit()
}

func (r *PBISQLiteRepository) FindByID(id string) (*domain.PBI, error) {
    var pbi domain.PBI

    // DBからメタデータ取得
    err := r.db.QueryRow(`
        SELECT id, title, status, estimated_story_points, priority,
               parent_epic_id, created_at, updated_at
        FROM pbis
        WHERE id = ?
    `, id).Scan(
        &pbi.ID, &pbi.Title, &pbi.Status, &pbi.EstimatedStoryPoints,
        &pbi.Priority, &pbi.ParentEpicID, &pbi.CreatedAt, &pbi.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }

    return &pbi, nil
}

func (r *PBISQLiteRepository) GetBody(id string) (string, error) {
    // Markdownファイルから本文取得
    mdPath := filepath.Join(r.rootPath, ".deespec", "specs", "pbi", id, "pbi.md")
    data, err := os.ReadFile(mdPath)
    if err != nil {
        return "", err
    }
    return string(data), nil
}

func (r *PBISQLiteRepository) FindAll() ([]*domain.PBI, error) {
    rows, err := r.db.Query(`
        SELECT id, title, status, estimated_story_points, priority,
               parent_epic_id, created_at, updated_at
        FROM pbis
        ORDER BY created_at DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var pbis []*domain.PBI
    for rows.Next() {
        var pbi domain.PBI
        if err := rows.Scan(
            &pbi.ID, &pbi.Title, &pbi.Status, &pbi.EstimatedStoryPoints,
            &pbi.Priority, &pbi.ParentEpicID, &pbi.CreatedAt, &pbi.UpdatedAt,
        ); err != nil {
            return nil, err
        }

        pbis = append(pbis, &pbi)
    }

    return pbis, nil
}
```

---

### 8.5 Markdown + SQLite ハイブリッド方式

**Phase 1から採用**: Markdown（本文） + SQLite（メタデータ）

**設計思想**:
- ✅ **Markdown = 真実の源**: 本文はファイルシステムに保存（Git friendly）
- ✅ **SQLite = インデックス**: メタデータをDBに保存（高速検索）
- ✅ **両者の良いとこ取り**: Markdownの可読性 + DBの検索性能

**ファイルとDBの役割**:
```
.deespec/specs/pbi/PBI-001/pbi.md  ← 本文（Markdown）
                    ↓ title抽出
DB (pbis table)                     ← メタデータ（検索用）
```

**検索フロー**:
```go
// 1. DBで高速検索
pbis := repo.FindByStatus("pending")

// 2. 必要に応じて本文取得
for _, pbi := range pbis {
    body, _ := repo.GetBody(pbi.ID)
    // bodyを使った処理
}
```

---

## まとめ

### 実装の流れ

```
Week 1-2: Phase 1（基本機能）
  ↓
  Milestone 1: 対話式とファイル方式でPBI管理ができる
  ↓
Week 3-4: Phase 2（利便性向上）
  ↓
  Milestone 2: 実用的な機能が揃う
  ↓
完成！
```

### 成功の指標

✅ **Phase 1完了後**:
```bash
deespec pbi register -f docs/plan.md  # Markdownのみ
deespec pbi register                   # 対話式
deespec pbi show PBI-001
deespec pbi list
# が動作する
```

✅ **Phase 2完了後**:
```bash
deespec pbi register -t "title" -f plan.md
deespec pbi update PBI-001 --status IMPLEMENTING
# が動作する
```

### 次のステップ

1. **Phase 1実装開始**
   - `internal/domain/model/pbi/pbi.go` から着手
   - サンプルMarkdownファイル作成
   - 段階的にコンポーネントを実装

2. **サンプル駆動開発**
   - まずMarkdownサンプルを作成
   - サンプルが動くように実装
   - テストを追加

3. **データベース対応（Phase 2以降）**
   - まずはYAMLファイルで実装
   - パフォーマンス問題が出たらSQLite移行

**シンプルに始めて、段階的に改善していく！** 🚀

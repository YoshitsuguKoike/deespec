# DeeSpec CleanArchitecture + DDD リファクタリング計画

## 1. 現状分析

### 1.1 プロジェクト概要
- **プロジェクト名**: DeeSpec (Spec Backlog Item管理システム)
- **言語**: Go 1.23.0
- **総ファイル数**: 約110のGoファイル(非テスト)
- **主な機能**: SBI(Spec Backlog Item)の登録・実行・レビューワークフロー管理

### 1.2 現在のディレクトリ構造

```
internal/
├── app/                    # 【問題】アプリケーション層とインフラが混在
│   ├── config/            # 設定管理
│   ├── health/            # ヘルスチェック
│   ├── state/             # 状態管理
│   ├── journal.go         # ジャーナル処理
│   ├── journal_writer.go  # ジャーナル書き込み
│   ├── logger.go          # ロガー
│   └── paths.go           # パス解決
├── domain/                 # 【部分的にDDD】一部のエンティティのみ
│   ├── execution/         # 実行ドメイン
│   ├── sbi/               # SBIエンティティ
│   ├── spec.go            # State構造体
│   └── repository.go      # 空ファイル
├── infra/                  # インフラ層
│   ├── config/            # 設定読み込み
│   ├── fs/                # ファイルシステム操作
│   │   └── txn/           # トランザクション管理
│   ├── persistence/       # 永続化
│   │   └── file/          # ファイルベース永続化
│   └── repository/        # リポジトリ実装
│       └── sbi/           # SBIリポジトリ
├── interface/              # インターフェース層
│   ├── cli/               # 【問題】CLIコマンド(40ファイル、ビジネスロジック混在)
│   ├── external/          # 外部ツール連携
│   └── persistence/       # 永続化インターフェース
├── usecase/                # 【部分的】一部のユースケースのみ
│   └── sbi/               # SBI登録ユースケース
├── runner/                 # 【問題】ドメイン層にあるべき
│   ├── prompt.go          # プロンプト生成
│   └── review.go          # レビュー判定
├── validator/              # 【問題】ドメイン層にあるべき
│   ├── agents/
│   ├── common/
│   ├── health/
│   ├── integrated/
│   ├── journal/
│   ├── state/
│   └── workflow/
├── workflow/               # 【問題】ドメイン層にあるべき
│   ├── loader.go
│   ├── types.go
│   └── vars.go
├── buildinfo/              # ビルド情報
├── embed/                  # 埋め込みリソース
├── pkg/                    # 共有パッケージ
│   └── specpath/
├── testutil/               # テストユーティリティ
└── util/                   # ユーティリティ
```

### 1.3 CleanArchitecture + DDD違反の詳細

#### 🔴 重大な問題

1. **ドメインロジックの分散**
   - `runner/review.go`: レビュー判定ロジックがドメイン層外
   - `workflow/`: ワークフロー管理がドメイン層外
   - `validator/`: 検証ロジックがドメイン層外
   - `domain/spec.go`: `NextStep()`関数がドメインサービス化されていない

2. **CLIレイヤーへのビジネスロジック混入**
   - `interface/cli/run.go`: ワークフロー実行ロジック(400行以上)
   - `interface/cli/sbi_run.go`: SBI実行制御ロジック
   - `interface/cli/register.go`: 登録ロジック
   - 全40ファイルのCLIコマンドにビジネスロジックが散在

3. **app/パッケージの責務不明確**
   - `app/journal.go`: ジャーナルのビジネスロジック
   - `app/paths.go`: インフラ層の責務
   - `app/health.go`: ドメイン層の責務
   - 設定とビジネスロジックとインフラが混在

4. **依存関係の逆転不足**
   - UseCaseがリポジトリインターフェースを持つものの、一部のみ
   - CLIレイヤーがファイルシステムに直接依存
   - ドメインロジックがインフラに依存

5. **ドメインモデルの不足**
   - `Turn`, `Attempt`, `Step`, `Status`などが値オブジェクト化されていない
   - エンティティの振る舞いが外部に散在
   - 集約の境界が不明確

#### 🟡 中程度の問題

6. **ユースケース層の不完全性**
   - SBI登録のユースケースのみ存在
   - 実行、レビュー、ステータス更新などのユースケースが未実装
   - ユースケース間の調整ロジックが不在

7. **リポジトリパターンの不完全実装**
   - SBIリポジトリのみ実装
   - State, Execution, Workflowのリポジトリが未実装
   - `domain/repository.go`が空ファイル

8. **トランザクション境界の不明確性**
   - `infra/fs/txn/`に独自トランザクション実装
   - ドメイン層でトランザクション境界が定義されていない
   - ユースケース層でのトランザクション管理が不在

## 2. 目標アーキテクチャ

### 2.1 CleanArchitecture 4層構造

```
internal/
├── domain/                      # 【第1層】エンタープライズビジネスルール
│   ├── model/                   # ドメインモデル(集約)
│   │   ├── sbi/                # SBI集約 (Spec Backlog Item)
│   │   │   ├── sbi.go          # SBIエンティティ
│   │   │   ├── sbi_id.go       # SBI ID値オブジェクト
│   │   │   ├── label.go        # ラベル値オブジェクト
│   │   │   └── priority.go     # 優先度値オブジェクト
│   │   ├── epic/               # EPIC集約 (Epic - Large Feature Group) 【将来追加】
│   │   │   ├── epic.go         # EPICエンティティ
│   │   │   ├── epic_id.go      # EPIC ID値オブジェクト
│   │   │   ├── component_type.go # コンポーネント種別
│   │   │   └── dependency.go   # 依存関係値オブジェクト
│   │   ├── pbi/                # PBI集約 (Product Backlog Item) 【将来追加】
│   │   │   ├── pbi.go          # PBIエンティティ
│   │   │   ├── pbi_id.go       # PBI ID値オブジェクト
│   │   │   ├── epic.go         # Epic値オブジェクト
│   │   │   └── acceptance_criteria.go # 受け入れ基準
│   │   ├── execution/          # Execution集約
│   │   │   ├── execution.go    # Executionエンティティ
│   │   │   ├── turn.go         # Turn値オブジェクト
│   │   │   ├── attempt.go      # Attempt値オブジェクト
│   │   │   ├── step.go         # Step値オブジェクト
│   │   │   └── status.go       # Status値オブジェクト
│   │   ├── workflow/           # Workflow集約
│   │   │   ├── workflow.go     # Workflowエンティティ
│   │   │   ├── step_config.go  # ステップ設定
│   │   │   └── constraints.go  # 制約条件
│   │   ├── agent/              # Agent集約 【将来追加】
│   │   │   ├── agent.go        # Agentエンティティ
│   │   │   ├── agent_type.go   # Agent種別(Claude/Gemini/Codex)
│   │   │   ├── capability.go   # エージェント能力値オブジェクト
│   │   │   └── config.go       # エージェント設定
│   │   └── state/              # State集約
│   │       ├── state.go        # Stateエンティティ
│   │       └── wip.go          # WIP値オブジェクト
│   ├── service/                # ドメインサービス
│   │   ├── execution_service.go    # 実行判定ロジック
│   │   ├── step_transition_service.go  # ステップ遷移ロジック
│   │   ├── review_service.go       # レビュー判定ロジック
│   │   ├── validation_service.go   # 検証ロジック
│   │   └── agent_selection_service.go  # エージェント選択ロジック 【将来追加】
│   └── repository/             # リポジトリインターフェース(ポート)
│       ├── sbi_repository.go
│       ├── epic_repository.go  # 【将来追加】
│       ├── pbi_repository.go   # 【将来追加】
│       ├── execution_repository.go
│       ├── state_repository.go
│       ├── workflow_repository.go
│       ├── agent_repository.go # 【将来追加】
│       └── journal_repository.go
│
├── application/                 # 【第2層】アプリケーションビジネスルール
│   ├── usecase/                # ユースケース
│   │   ├── sbi/
│   │   │   ├── register_sbi.go         # SBI登録
│   │   │   ├── find_sbi.go             # SBI検索
│   │   │   └── list_sbi.go             # SBI一覧
│   │   ├── epic/               # 【将来追加】
│   │   │   ├── register_epic.go        # EPIC登録
│   │   │   ├── link_epic_to_sbi.go     # EPICとSBIの関連付け
│   │   │   └── list_epic.go            # EPIC一覧
│   │   ├── pbi/                # 【将来追加】
│   │   │   ├── register_pbi.go         # PBI登録
│   │   │   ├── decompose_pbi.go        # PBIからSBIへの分解
│   │   │   └── track_pbi_progress.go   # PBI進捗追跡
│   │   ├── execution/
│   │   │   ├── run_sbi.go              # SBI実行
│   │   │   ├── run_turn.go             # ターン実行
│   │   │   └── get_execution_status.go # 実行状態取得
│   │   ├── workflow/
│   │   │   ├── load_workflow.go        # ワークフロー読み込み
│   │   │   └── validate_workflow.go    # ワークフロー検証
│   │   └── health/
│   │       └── check_health.go         # ヘルスチェック
│   ├── dto/                    # Data Transfer Objects
│   │   ├── sbi_dto.go
│   │   ├── epic_dto.go         # 【将来追加】
│   │   ├── pbi_dto.go          # 【将来追加】
│   │   ├── execution_dto.go
│   │   ├── workflow_dto.go
│   │   └── agent_dto.go        # 【将来追加】
│   ├── port/                   # ポート(インターフェース定義)
│   │   ├── input/              # 入力ポート
│   │   │   └── usecase_interfaces.go
│   │   └── output/             # 出力ポート
│   │       ├── repository_interfaces.go (→ domain/repositoryを参照)
│   │       ├── agent_gateway.go        # エージェント抽象化インターフェース
│   │       ├── presenter.go
│   │       └── transaction.go
│   └── service/                # アプリケーションサービス
│       ├── orchestrator.go     # ユースケース間調整
│       └── transaction_manager.go # トランザクション管理
│
├── adapter/                     # 【第3層】インターフェースアダプター
│   ├── controller/             # 入力アダプター
│   │   └── cli/
│   │       ├── sbi_controller.go       # SBIコマンド制御
│   │       ├── run_controller.go       # 実行コマンド制御
│   │       ├── health_controller.go    # ヘルスチェック制御
│   │       └── doctor_controller.go    # ドクターコマンド制御
│   ├── presenter/              # 出力アダプター(フォーマット制御)
│   │   └── cli/
│   │       ├── execution_presenter.go  # 実行結果表示
│   │       ├── health_presenter.go     # ヘルス結果表示
│   │       └── json_presenter.go       # JSON形式表示
│   └── gateway/                # 外部サービスアダプター
│       ├── agent/              # AIエージェントゲートウェイ
│       │   ├── claude_gateway.go       # Claude Code CLI連携
│       │   ├── gemini_gateway.go       # Gemini CLI連携 【将来追加】
│       │   ├── codex_gateway.go        # Codex API連携 【将来追加】
│       │   └── agent_factory.go        # エージェント生成ファクトリー
│       └── filesystem_gateway.go       # ファイルシステム操作
│
└── infrastructure/              # 【第4層】フレームワーク&ドライバー
    ├── persistence/            # 永続化実装
    │   ├── file/               # ファイルベース実装
    │   │   ├── sbi_repository_impl.go
    │   │   ├── epic_repository_impl.go      # 【将来追加】
    │   │   ├── pbi_repository_impl.go       # 【将来追加】
    │   │   ├── execution_repository_impl.go
    │   │   ├── state_repository_impl.go
    │   │   ├── workflow_repository_impl.go
    │   │   ├── agent_repository_impl.go     # 【将来追加】
    │   │   └── journal_repository_impl.go
    │   └── sqlite/             # 【将来】SQLite実装
    │       └── (future implementation)
    ├── transaction/            # トランザクション実装
    │   ├── file_transaction.go         # ファイルベーストランザクション
    │   └── flock_manager.go            # ファイルロック管理
    ├── config/                 # 設定管理
    │   ├── loader.go
    │   └── resolver.go
    ├── logger/                 # ロギング
    │   └── logger.go
    └── di/                     # 依存性注入
        └── container.go                # DIコンテナ
```

### 2.2 依存関係のルール

```
┌─────────────────────────────────────────────────┐
│          🎯 依存の方向: 外側 → 内側            │
└─────────────────────────────────────────────────┘

Layer 4: Infrastructure ──┐
                          ├──> Layer 3: Adapter ──┐
Layer 3: Adapter ─────────┘                       ├──> Layer 2: Application ──> Layer 1: Domain
                                                  │
                                                  └──> Layer 1: Domain
```

**重要原則:**
1. **内側の層は外側を知らない**: Domainは他の層を一切知らない
2. **依存性逆転の原則(DIP)**: 外側が内側のインターフェースに依存
3. **ポート&アダプターパターン**: application/portでインターフェース定義、adapterで実装

### 2.3 各層の責務明確化

#### Layer 1: Domain (ドメイン層)
**責務:**
- ビジネスルールの定義
- エンティティと値オブジェクトの管理
- ドメインサービスのビジネスロジック
- リポジトリインターフェースの定義

**禁止事項:**
- インフラ依存(ファイルIO, DB, 外部API)
- フレームワーク依存
- 他層への依存

**例:**
```go
// domain/model/execution/turn.go
type Turn struct {
    value int
    max   int
}

func NewTurn(value, max int) (Turn, error) {
    if value < 1 || value > max {
        return Turn{}, ErrInvalidTurn
    }
    return Turn{value: value, max: max}, nil
}

func (t Turn) IsExceeded() bool {
    return t.value > t.max
}
```

#### Layer 2: Application (アプリケーション層)
**責務:**
- ユースケースのオーケストレーション
- トランザクション境界の定義
- ドメインオブジェクトの調整
- ポート(インターフェース)の定義

**禁止事項:**
- ビジネスルール実装(→ドメイン層)
- プレゼンテーション形式の決定(→アダプター層)
- インフラ実装の直接参照

**例:**
```go
// application/usecase/execution/run_turn.go
type RunTurnUseCase struct {
    execRepo     domain.ExecutionRepository
    sbiRepo      domain.SBIRepository
    agentGateway port.AgentGateway
    txManager    port.TransactionManager
}

func (uc *RunTurnUseCase) Execute(ctx context.Context, input RunTurnInput) (*RunTurnOutput, error) {
    // トランザクション境界
    return uc.txManager.InTransaction(ctx, func(ctx context.Context) (*RunTurnOutput, error) {
        // 1. エンティティ取得
        exec, err := uc.execRepo.FindCurrent(ctx, input.SBIID)

        // 2. ドメインサービスで判定
        if exec.ShouldTerminate() {
            return nil, ErrExecutionTerminated
        }

        // 3. 外部エージェント実行
        result, err := uc.agentGateway.Execute(ctx, prompt)

        // 4. ドメインオブジェクト更新
        exec.RecordResult(result)

        // 5. 永続化
        return uc.execRepo.Save(ctx, exec)
    })
}
```

#### Layer 3: Adapter (アダプター層)
**責務:**
- 外部とのデータ変換
- コントローラーでの入力処理
- プレゼンターでの出力整形
- ゲートウェイでの外部システム連携

**禁止事項:**
- ビジネスロジック実装
- 直接的なインフラ操作(→インフラ層)

**例:**
```go
// adapter/controller/cli/run_controller.go
type RunController struct {
    runTurnUC *usecase.RunTurnUseCase
    presenter presenter.ExecutionPresenter
}

func (c *RunController) Handle(cmd *cobra.Command, args []string) error {
    // 1. 入力解析
    input := c.parseInput(cmd, args)

    // 2. ユースケース実行
    output, err := c.runTurnUC.Execute(context.Background(), input)

    // 3. プレゼンテーション
    return c.presenter.Present(output, err)
}
```

#### Layer 4: Infrastructure (インフラ層)
**責務:**
- データベース実装
- ファイルシステム操作
- 外部API呼び出し
- フレームワーク固有の処理

**禁止事項:**
- ビジネスロジック
- プレゼンテーションロジック

**例:**
```go
// infrastructure/persistence/file/execution_repository_impl.go
type FileExecutionRepository struct {
    fs       afero.Fs
    basePath string
}

func (r *FileExecutionRepository) Save(ctx context.Context, exec *domain.Execution) error {
    // ファイルシステム操作のみ
    data := r.serialize(exec)
    return r.fs.WriteFile(path, data, 0644)
}
```

## 3. リファクタリング戦略

### 3.1 段階的移行アプローチ(Strangler Figパターン)

新旧システムを並行稼働させながら、徐々に置き換える戦略を採用します。

```
Phase 1: 基盤整備 (Week 1-2)
  ↓
Phase 2: ドメイン層構築 (Week 2-3)
  ↓
Phase 3: アプリケーション層構築 (Week 3-4)
  ↓
Phase 4: アダプター層リファクタリング (Week 4-5)
  ↓
Phase 5: インフラ層整理 - SQLite Repository実装 (Week 5-6)
  ↓
Phase 6: Storage Gateway実装 (Week 7)
  ↓
Phase 7: Lock System SQLite移行 (Week 8)
  ↓
Phase 8: 統合・テスト・移行完了 (Week 9-10)
```

### 3.2 フェーズ別詳細計画

---

## Phase 1: 基盤整備 (Week 1-2)

### 目標
- ディレクトリ構造の作成
- インターフェース定義
- テスト戦略の確立

### タスク

#### 1.1 新ディレクトリ構造作成
```bash
mkdir -p internal/domain/{model/{sbi,execution,workflow,state},service,repository}
mkdir -p internal/application/{usecase/{sbi,execution,workflow,health},dto,port/{input,output},service}
mkdir -p internal/adapter/{controller/cli,presenter/cli,gateway}
mkdir -p internal/infrastructure/{persistence/{file,sqlite},transaction,config,logger,di}
```

#### 1.2 リポジトリインターフェース定義

**ファイル**: `internal/domain/repository/sbi_repository.go`
```go
package repository

import (
    "context"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

type SBIRepository interface {
    Find(ctx context.Context, id sbi.SBIID) (*sbi.SBI, error)
    Save(ctx context.Context, s *sbi.SBI) error
    List(ctx context.Context, filter SBIFilter) ([]*sbi.SBI, error)
    Delete(ctx context.Context, id sbi.SBIID) error
}

type SBIFilter struct {
    Labels   []string
    Status   *sbi.Status
    Limit    int
    Offset   int
}
```

#### 1.3 ポート定義

**ファイル**: `internal/application/port/output/agent_gateway.go`
```go
package output

import (
    "context"
    "time"
)

type AgentGateway interface {
    Execute(ctx context.Context, req AgentRequest) (*AgentResponse, error)
}

type AgentRequest struct {
    Prompt  string
    Timeout time.Duration
}

type AgentResponse struct {
    Output    string
    ExitCode  int
    Duration  time.Duration
}
```

#### 1.4 テスト基盤構築

**ファイル**: `internal/domain/model/sbi/sbi_test.go`
```go
package sbi_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

func TestNewSBI_Success(t *testing.T) {
    id, _ := sbi.NewSBIID()
    s, err := sbi.NewSBI(id, "Test Title", "Body", []string{"label1"})

    assert.NoError(t, err)
    assert.Equal(t, "Test Title", s.Title())
}
```

### 成果物
- [ ] 新ディレクトリ構造
- [ ] 全リポジトリインターフェース定義
- [ ] 全ポートインターフェース定義
- [ ] テストヘルパー関数群

---

## Phase 2: ドメイン層構築 (Week 2-3)

### 目標
- 値オブジェクトの実装
- エンティティの移行
- ドメインサービスの抽出

### タスク

#### 2.1 値オブジェクト実装

**優先順位:**
1. `SBIID` (既存の文字列から値オブジェクト化)
2. `Turn` (バリデーション追加)
3. `Attempt` (上限チェックロジック含む)
4. `Step` (ステップ遷移ルール含む)
5. `Status` (ステータス制約含む)

**ファイル**: `internal/domain/model/execution/turn.go`
```go
package execution

import "errors"

var ErrInvalidTurn = errors.New("invalid turn value")

type Turn struct {
    value int
    max   int
}

func NewTurn(value, max int) (Turn, error) {
    if value < 1 {
        return Turn{}, ErrInvalidTurn
    }
    if max > 0 && value > max {
        return Turn{}, ErrInvalidTurn
    }
    return Turn{value: value, max: max}, nil
}

func (t Turn) Value() int { return t.value }
func (t Turn) Max() int { return t.max }
func (t Turn) IsExceeded() bool { return t.max > 0 && t.value > t.max }
func (t Turn) Next() Turn {
    return Turn{value: t.value + 1, max: t.max}
}
```

#### 2.2 エンティティ移行

**現在のコード**: `internal/domain/sbi/sbi.go`
```go
// 既存
type SBI struct {
    ID     string
    Title  string
    Body   string
    Labels []string
}
```

**新しいコード**: `internal/domain/model/sbi/sbi.go`
```go
package sbi

import "time"

// SBI集約ルート
type SBI struct {
    id        SBIID
    title     Title
    body      Body
    labels    Labels
    status    Status
    createdAt time.Time
    updatedAt time.Time
}

// ファクトリーメソッド
func NewSBI(id SBIID, title Title, body Body, labels Labels) (*SBI, error) {
    if err := title.Validate(); err != nil {
        return nil, err
    }

    now := time.Now()
    return &SBI{
        id:        id,
        title:     title,
        body:      body,
        labels:    labels,
        status:    StatusDraft,
        createdAt: now,
        updatedAt: now,
    }, nil
}

// ゲッター(不変性を保証)
func (s *SBI) ID() SBIID { return s.id }
func (s *SBI) Title() Title { return s.title }
func (s *SBI) Labels() Labels { return s.labels.Copy() }

// ビジネスメソッド
func (s *SBI) UpdateTitle(newTitle Title) error {
    if err := newTitle.Validate(); err != nil {
        return err
    }
    s.title = newTitle
    s.updatedAt = time.Now()
    return nil
}

func (s *SBI) AddLabel(label string) error {
    return s.labels.Add(label)
}

func (s *SBI) Activate() error {
    if s.status != StatusDraft {
        return ErrInvalidStatusTransition
    }
    s.status = StatusActive
    s.updatedAt = time.Now()
    return nil
}
```

#### 2.3 ドメインサービス抽出

**現在のコード**: `internal/domain/spec.go` (関数)
```go
func NextStep(cur string, reviewDecision string) string {
    switch cur {
    case "plan": return "implement"
    case "implement": return "test"
    // ...
    }
}
```

**新しいコード**: `internal/domain/service/step_transition_service.go`
```go
package service

import (
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/execution"
)

// StepTransitionService はステップ遷移のドメインロジックを提供
type StepTransitionService struct{}

func NewStepTransitionService() *StepTransitionService {
    return &StepTransitionService{}
}

// DetermineNextStep は現在のステップとレビュー決定から次のステップを決定
func (s *StepTransitionService) DetermineNextStep(
    current execution.Step,
    decision execution.Decision,
) (execution.Step, error) {

    switch current {
    case execution.StepPlan:
        return execution.StepImplement, nil
    case execution.StepImplement:
        return execution.StepTest, nil
    case execution.StepTest:
        return execution.StepReview, nil
    case execution.StepReview:
        if decision == execution.DecisionOK {
            return execution.StepDone, nil
        }
        return execution.StepImplement, nil // ブーメラン
    case execution.StepDone:
        return execution.StepDone, nil
    default:
        return execution.StepPlan, nil
    }
}

// CanTransition はステップ遷移が可能かをチェック
func (s *StepTransitionService) CanTransition(from, to execution.Step) bool {
    validTransitions := map[execution.Step][]execution.Step{
        execution.StepPlan:      {execution.StepImplement},
        execution.StepImplement: {execution.StepTest, execution.StepReview},
        execution.StepTest:      {execution.StepReview},
        execution.StepReview:    {execution.StepImplement, execution.StepDone},
        execution.StepDone:      {execution.StepDone},
    }

    for _, valid := range validTransitions[from] {
        if valid == to {
            return true
        }
    }
    return false
}
```

**現在のコード**: `internal/runner/review.go`
```go
func ParseDecision(output string, re *regexp.Regexp) DecisionType {
    // レビュー判定ロジック
}
```

**新しいコード**: `internal/domain/service/review_service.go`
```go
package service

import (
    "regexp"
    "strings"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/execution"
)

type ReviewService struct {
    decisionPattern *regexp.Regexp
}

func NewReviewService(pattern string) (*ReviewService, error) {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return nil, err
    }
    return &ReviewService{decisionPattern: re}, nil
}

// ParseDecision はAI出力からレビュー決定を解析
func (s *ReviewService) ParseDecision(output string) execution.Decision {
    lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

    for i := len(lines) - 1; i >= 0; i-- {
        line := strings.TrimSpace(lines[i])
        matches := s.decisionPattern.FindStringSubmatch(line)

        if len(matches) >= 2 {
            value := strings.ToUpper(strings.TrimSpace(matches[1]))

            switch value {
            case "OK":
                return execution.DecisionOK
            case "NEEDS_CHANGES":
                return execution.DecisionNeedsChanges
            }
        }
    }

    return execution.DecisionPending
}
```

### 成果物
- [ ] 全値オブジェクト実装(Turn, Attempt, Step, Status, SBIID, Title, Label等)
- [ ] エンティティ実装(SBI, Execution, Workflow, State)
- [ ] ドメインサービス実装(StepTransitionService, ReviewService, ValidationService)
- [ ] ドメイン層の単体テスト(カバレッジ>90%)

---

## Phase 3: アプリケーション層構築 (Week 3-4)

### 目標
- ユースケースの実装
- CLIからのビジネスロジック抽出
- トランザクション境界の明確化

### タスク

#### 3.1 ユースケース実装

**ファイル**: `internal/application/usecase/execution/run_turn.go`
```go
package execution

import (
    "context"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/application/dto"
    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/execution"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
    "github.com/YoshitsuguKoike/deespec/internal/domain/service"
)

// RunTurnUseCase はSBIの1ターン実行を担当
type RunTurnUseCase struct {
    sbiRepo          repository.SBIRepository
    execRepo         repository.ExecutionRepository
    stateRepo        repository.StateRepository
    journalRepo      repository.JournalRepository
    agentGateway     output.AgentGateway
    txManager        output.TransactionManager
    stepTransition   *service.StepTransitionService
    reviewService    *service.ReviewService
}

type RunTurnInput struct {
    SBIID   sbi.SBIID
    Timeout time.Duration
}

type RunTurnOutput struct {
    ExecutionID execution.ExecutionID
    Turn        int
    Step        string
    Decision    string
    Duration    time.Duration
}

func (uc *RunTurnUseCase) Execute(ctx context.Context, input RunTurnInput) (*RunTurnOutput, error) {
    var output *RunTurnOutput

    // トランザクション境界
    err := uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
        // 1. 現在の実行状態を取得
        exec, err := uc.execRepo.FindCurrentBySBIID(txCtx, input.SBIID)
        if err != nil {
            return err
        }

        // 2. SBI取得
        sbiEntity, err := uc.sbiRepo.Find(txCtx, input.SBIID)
        if err != nil {
            return err
        }

        // 3. 実行可能性チェック(ドメインロジック)
        if exec.ShouldForceTerminate() {
            return execution.ErrExecutionTerminated
        }

        // 4. プロンプト生成(ドメインロジック)
        prompt, err := exec.GeneratePrompt(sbiEntity)
        if err != nil {
            return err
        }

        // 5. エージェント実行(外部システム)
        startTime := time.Now()
        agentResp, err := uc.agentGateway.Execute(txCtx, output.AgentRequest{
            Prompt:  prompt,
            Timeout: input.Timeout,
        })
        duration := time.Since(startTime)

        if err != nil {
            // エラー記録してもトランザクションは継続
            exec.RecordError(err)
            _ = uc.execRepo.Save(txCtx, exec)
            return err
        }

        // 6. レビューステップの場合は判定解析
        var decision execution.Decision
        if exec.CurrentStep() == execution.StepReview {
            decision = uc.reviewService.ParseDecision(agentResp.Output)
        }

        // 7. 次ステップ決定(ドメインサービス)
        nextStep, err := uc.stepTransition.DetermineNextStep(exec.CurrentStep(), decision)
        if err != nil {
            return err
        }

        // 8. 実行エンティティ更新
        exec.RecordSuccess(agentResp.Output, decision, nextStep, duration)

        // 9. 永続化
        if err := uc.execRepo.Save(txCtx, exec); err != nil {
            return err
        }

        // 10. ジャーナル記録
        journal := execution.NewJournalEntry(exec, duration)
        if err := uc.journalRepo.Append(txCtx, journal); err != nil {
            return err
        }

        // 11. State更新
        state, err := uc.stateRepo.Load(txCtx)
        if err != nil {
            return err
        }
        state.UpdateFromExecution(exec)
        if err := uc.stateRepo.Save(txCtx, state); err != nil {
            return err
        }

        // 出力準備
        output = &RunTurnOutput{
            ExecutionID: exec.ID(),
            Turn:        exec.Turn().Value(),
            Step:        nextStep.String(),
            Decision:    decision.String(),
            Duration:    duration,
        }

        return nil
    })

    if err != nil {
        return nil, err
    }

    return output, nil
}
```

#### 3.2 既存CLIコードからの抽出

**現在のコード**: `internal/interface/cli/run.go` (約400行)
```go
func runOnce(autoFB bool) error {
    // 大量のビジネスロジックが混在
    // ファイル読み込み、状態管理、エージェント実行、レビュー判定...
}
```

**リファクタリング後**: `internal/adapter/controller/cli/run_controller.go`
```go
package cli

import (
    "context"
    "github.com/spf13/cobra"

    "github.com/YoshitsuguKoike/deespec/internal/application/usecase/execution"
    "github.com/YoshitsuguKoike/deespec/internal/adapter/presenter/cli"
)

type RunController struct {
    runTurnUC *execution.RunTurnUseCase
    presenter *cli.ExecutionPresenter
}

func NewRunController(
    runTurnUC *execution.RunTurnUseCase,
    presenter *cli.ExecutionPresenter,
) *RunController {
    return &RunController{
        runTurnUC: runTurnUC,
        presenter: presenter,
    }
}

func (c *RunController) Handle(cmd *cobra.Command, args []string) error {
    // 1. フラグ解析
    once, _ := cmd.Flags().GetBool("once")
    autoFB, _ := cmd.Flags().GetBool("auto-fb")

    // 2. 入力DTO作成
    input := c.buildInput(cmd, args)

    // 3. ユースケース実行
    output, err := c.runTurnUC.Execute(context.Background(), input)

    // 4. プレゼンテーション
    return c.presenter.PresentRunResult(output, err)
}

func (c *RunController) buildInput(cmd *cobra.Command, args []string) execution.RunTurnInput {
    // CLIフラグからDTOへの変換のみ
    timeout, _ := cmd.Flags().GetDuration("timeout")
    return execution.RunTurnInput{
        Timeout: timeout,
    }
}
```

**プレゼンター**: `internal/adapter/presenter/cli/execution_presenter.go`
```go
package cli

import (
    "fmt"
    "io"

    "github.com/YoshitsuguKoike/deespec/internal/application/usecase/execution"
)

type ExecutionPresenter struct {
    writer io.Writer
}

func NewExecutionPresenter(w io.Writer) *ExecutionPresenter {
    return &ExecutionPresenter{writer: w}
}

func (p *ExecutionPresenter) PresentRunResult(output *execution.RunTurnOutput, err error) error {
    if err != nil {
        fmt.Fprintf(p.writer, "Error: %v\n", err)
        return err
    }

    fmt.Fprintf(p.writer, "Turn %d completed\n", output.Turn)
    fmt.Fprintf(p.writer, "Step: %s\n", output.Step)
    fmt.Fprintf(p.writer, "Decision: %s\n", output.Decision)
    fmt.Fprintf(p.writer, "Duration: %v\n", output.Duration)

    return nil
}

func (p *ExecutionPresenter) PresentJSON(output *execution.RunTurnOutput) error {
    // JSON形式での出力
    // ...
}
```

### 成果物
- [ ] 全ユースケース実装(Register, Run, Review, Health等)
- [ ] DTO定義
- [ ] トランザクションマネージャー実装
- [ ] ユースケーステスト(モック使用)

---

## Phase 4: Adapter Layer実装 (Week 4-5) ✅ Phase 1-3完了後

### 目標
- 完全新規のCLI Controller実装
- Claude Code Gateway実装（Gemini/Codex はMock）
- CLI Presenter実装（既存類似 + JSON出力）
- Storage Gateway Mock実装

### 実装方針
- **CLI**: 既存 `internal/interface/cli/` は削除予定、完全新規作成
- **Agent**: Claude Code API統合実装、他はMock
- **Storage**: Mock実装（Phase 6で本実装）
- **品質優先**: 完全なClean Architecture準拠

### タスク

#### 4.1 CLI Controller新規作成

**新規ディレクトリ構造**:
```
internal/adapter/controller/cli/
├── epic_controller.go       # EPIC CRUD操作
├── pbi_controller.go        # PBI CRUD操作
├── sbi_controller.go        # SBI CRUD操作
├── workflow_controller.go   # ワークフロー実行（Pick/Implement/Review）
└── root.go                  # Cobraルートコマンド設定
```

**実装例**: `sbi_controller.go`
```go
package cli

import (
    "github.com/spf13/cobra"
    "github.com/YoshitsuguKoike/deespec/internal/application/dto"
    "github.com/YoshitsuguKoike/deespec/internal/application/port/input"
)

type SBIController struct {
    taskUseCase     input.TaskUseCase
    workflowUseCase input.WorkflowUseCase
    presenter       output.Presenter
}

func NewSBIController(
    taskUC input.TaskUseCase,
    workflowUC input.WorkflowUseCase,
    presenter output.Presenter,
) *SBIController {
    return &SBIController{
        taskUseCase:     taskUC,
        workflowUseCase: workflowUC,
        presenter:       presenter,
    }
}

// CreateCommand creates 'sbi create' command
func (c *SBIController) CreateCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "create [title]",
        Short: "Create a new SBI",
        RunE: func(cmd *cobra.Command, args []string) error {
            req := dto.CreateSBIRequest{
                Title:       args[0],
                Description: cmd.Flag("description").Value.String(),
                // ...
            }

            result, err := c.taskUseCase.CreateSBI(cmd.Context(), req)
            if err != nil {
                return c.presenter.PresentError(err)
            }

            return c.presenter.PresentSuccess("SBI created", result)
        },
    }
}

// ListCommand creates 'sbi list' command
func (c *SBIController) ListCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List SBIs",
        RunE: func(cmd *cobra.Command, args []string) error {
            req := dto.ListTasksRequest{
                Types: []string{"SBI"},
                // ...
            }

            result, err := c.taskUseCase.ListTasks(cmd.Context(), req)
            if err != nil {
                return c.presenter.PresentError(err)
            }

            return c.presenter.PresentSuccess("SBI list", result)
        },
    }
}
```

#### 4.2 Agent Gateway実装

**ディレクトリ構造**:
```
internal/adapter/gateway/agent/
├── claude_code_gateway.go   # Claude Code実装（実際のAPI統合）
├── gemini_mock_gateway.go   # Gemini Mock
├── codex_mock_gateway.go    # Codex Mock
└── factory.go               # Agent Factory
```

**実装**: `claude_code_gateway.go`
```go
package agent

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

type ClaudeCodeGateway struct {
    apiKey     string
    apiURL     string
    httpClient *http.Client
    model      string // "claude-3-5-sonnet-20241022"
}

func NewClaudeCodeGateway(apiKey string) *ClaudeCodeGateway {
    return &ClaudeCodeGateway{
        apiKey: apiKey,
        apiURL: "https://api.anthropic.com/v1/messages",
        httpClient: &http.Client{
            Timeout: 5 * time.Minute,
        },
        model: "claude-3-5-sonnet-20241022",
    }
}

func (g *ClaudeCodeGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
    start := time.Now()

    // Claude API Request構築
    claudeReq := ClaudeRequest{
        Model:      g.model,
        MaxTokens:  req.MaxTokens,
        Messages: []Message{
            {
                Role:    "user",
                Content: req.Prompt,
            },
        },
    }

    // API呼び出し
    resp, err := g.callClaudeAPI(ctx, claudeReq)
    if err != nil {
        return nil, fmt.Errorf("Claude API call failed: %w", err)
    }

    // レスポンス構築
    return &output.AgentResponse{
        Output:     resp.Content[0].Text,
        ExitCode:   0,
        Duration:   time.Since(start),
        TokensUsed: resp.Usage.InputTokens + resp.Usage.OutputTokens,
        AgentType:  "claude-code",
        Metadata: map[string]string{
            "model":         g.model,
            "stop_reason":   resp.StopReason,
            "input_tokens":  fmt.Sprintf("%d", resp.Usage.InputTokens),
            "output_tokens": fmt.Sprintf("%d", resp.Usage.OutputTokens),
        },
    }, nil
}

func (g *ClaudeCodeGateway) GetCapability() output.AgentCapability {
    return output.AgentCapability{
        SupportsCodeGeneration: true,
        SupportsReview:         true,
        SupportsTest:           true,
        MaxPromptSize:          200000, // 200k tokens
        ConcurrentTasks:        5,
        AgentType:              "claude-code",
    }
}

func (g *ClaudeCodeGateway) HealthCheck(ctx context.Context) error {
    // Simple API ping
    req := ClaudeRequest{
        Model:     g.model,
        MaxTokens: 10,
        Messages: []Message{
            {Role: "user", Content: "ping"},
        },
    }

    _, err := g.callClaudeAPI(ctx, req)
    return err
}

// Private helper
func (g *ClaudeCodeGateway) callClaudeAPI(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error) {
    // HTTP request構築
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", g.apiURL, bytes.NewBuffer(body))

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", g.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    // API呼び出し
    httpResp, err := g.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer httpResp.Body.Close()

    // レスポンス解析
    var claudeResp ClaudeResponse
    if err := json.NewDecoder(httpResp.Body).Decode(&claudeResp); err != nil {
        return nil, err
    }

    if httpResp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error: %d - %s", httpResp.StatusCode, claudeResp.Error.Message)
    }

    return &claudeResp, nil
}

// Claude API Types
type ClaudeRequest struct {
    Model     string    `json:"model"`
    MaxTokens int       `json:"max_tokens"`
    Messages  []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ClaudeResponse struct {
    ID         string         `json:"id"`
    Type       string         `json:"type"`
    Role       string         `json:"role"`
    Content    []ContentBlock `json:"content"`
    StopReason string         `json:"stop_reason"`
    Usage      Usage          `json:"usage"`
    Error      *APIError      `json:"error,omitempty"`
}

type ContentBlock struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

type APIError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
}
```

**Mock実装**: `gemini_mock_gateway.go`
```go
package agent

import (
    "context"
    "fmt"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

type GeminiMockGateway struct{}

func NewGeminiMockGateway() *GeminiMockGateway {
    return &GeminiMockGateway{}
}

func (g *GeminiMockGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
    // Mock実装（将来Gemini CLI統合予定）
    time.Sleep(100 * time.Millisecond) // Simulate API call

    return &output.AgentResponse{
        Output:     fmt.Sprintf("[Gemini Mock] Response for: %s", req.Prompt[:min(50, len(req.Prompt))]),
        ExitCode:   0,
        Duration:   100 * time.Millisecond,
        TokensUsed: 150,
        AgentType:  "gemini-cli",
        Metadata: map[string]string{
            "mock": "true",
            "note": "Gemini CLI integration pending",
        },
    }, nil
}

func (g *GeminiMockGateway) GetCapability() output.AgentCapability {
    return output.AgentCapability{
        SupportsCodeGeneration: true,
        SupportsReview:         true,
        SupportsTest:           false,
        MaxPromptSize:          100000,
        ConcurrentTasks:        3,
        AgentType:              "gemini-cli",
    }
}

func (g *GeminiMockGateway) HealthCheck(ctx context.Context) error {
    return nil // Mock always healthy
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

**Agent Factory**: `factory.go`
```go
package agent

import (
    "fmt"
    "os"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// NewAgentGateway creates an appropriate agent gateway based on type
func NewAgentGateway(agentType string) (output.AgentGateway, error) {
    switch agentType {
    case "claude-code":
        apiKey := os.Getenv("ANTHROPIC_API_KEY")
        if apiKey == "" {
            return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
        }
        return NewClaudeCodeGateway(apiKey), nil

    case "gemini-cli":
        return NewGeminiMockGateway(), nil

    case "codex":
        return NewCodexMockGateway(), nil

    default:
        return nil, fmt.Errorf("unknown agent type: %s", agentType)
    }
}
```

#### 4.3 Presenter実装

**ディレクトリ構造**:
```
internal/adapter/presenter/cli/
├── task_presenter.go    # タスク表示（既存類似フォーマット）
├── json_presenter.go    # JSON出力
└── format.go            # 共通フォーマット関数
```

**実装**: `task_presenter.go`
```go
package cli

import (
    "fmt"
    "io"
    "strings"

    "github.com/YoshitsuguKoike/deespec/internal/application/dto"
    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

type CLITaskPresenter struct {
    output io.Writer
}

func NewCLITaskPresenter(output io.Writer) *CLITaskPresenter {
    return &CLITaskPresenter{output: output}
}

func (p *CLITaskPresenter) PresentSuccess(message string, data interface{}) error {
    fmt.Fprintf(p.output, "✓ %s\n\n", message)

    switch v := data.(type) {
    case *dto.SBIDTO:
        return p.presentSBI(v)
    case *dto.PBIDTO:
        return p.presentPBI(v)
    case *dto.EPICDTO:
        return p.presentEPIC(v)
    case *dto.ListTasksResponse:
        return p.presentTaskList(v)
    case *dto.ImplementTaskResponse:
        return p.presentImplementResult(v)
    default:
        fmt.Fprintf(p.output, "%+v\n", data)
    }

    return nil
}

func (p *CLITaskPresenter) PresentError(err error) error {
    fmt.Fprintf(p.output, "✗ Error: %v\n", err)
    return err
}

func (p *CLITaskPresenter) PresentProgress(message string, progress int, total int) error {
    percentage := float64(progress) / float64(total) * 100
    bar := strings.Repeat("█", progress) + strings.Repeat("░", total-progress)
    fmt.Fprintf(p.output, "\r%s [%s] %.1f%%", message, bar, percentage)
    return nil
}

// 既存類似フォーマット
func (p *CLITaskPresenter) presentSBI(sbi *dto.SBIDTO) error {
    fmt.Fprintf(p.output, "SBI: %s\n", sbi.Title)
    fmt.Fprintf(p.output, "ID: %s\n", sbi.ID)
    fmt.Fprintf(p.output, "Status: %s\n", sbi.Status)
    fmt.Fprintf(p.output, "Step: %s\n", sbi.CurrentStep)

    if sbi.ParentID != nil {
        fmt.Fprintf(p.output, "Parent PBI: %s\n", *sbi.ParentID)
    }

    fmt.Fprintf(p.output, "Turn: %d/%d\n", sbi.CurrentTurn, sbi.MaxTurns)
    fmt.Fprintf(p.output, "Attempt: %d/%d\n", sbi.CurrentAttempt, sbi.MaxAttempts)

    if len(sbi.Labels) > 0 {
        fmt.Fprintf(p.output, "Labels: %s\n", strings.Join(sbi.Labels, ", "))
    }

    if sbi.Description != "" {
        fmt.Fprintf(p.output, "\nDescription:\n%s\n", sbi.Description)
    }

    if len(sbi.FilePaths) > 0 {
        fmt.Fprintf(p.output, "\nFile Paths:\n")
        for _, path := range sbi.FilePaths {
            fmt.Fprintf(p.output, "  - %s\n", path)
        }
    }

    return nil
}

func (p *CLITaskPresenter) presentTaskList(list *dto.ListTasksResponse) error {
    fmt.Fprintf(p.output, "Total: %d tasks\n\n", list.TotalCount)

    for i, task := range list.Tasks {
        fmt.Fprintf(p.output, "%d. [%s] %s (%s)\n", i+1, task.Type, task.Title, task.Status)
        fmt.Fprintf(p.output, "   ID: %s\n", task.ID)
    }

    return nil
}

func (p *CLITaskPresenter) presentImplementResult(result *dto.ImplementTaskResponse) error {
    if result.Success {
        fmt.Fprintf(p.output, "Implementation successful!\n")
    } else {
        fmt.Fprintf(p.output, "Implementation failed: %s\n", result.Message)
    }

    fmt.Fprintf(p.output, "Next Step: %s\n", result.NextStep)

    if len(result.Artifacts) > 0 {
        fmt.Fprintf(p.output, "\nArtifacts generated:\n")
        for _, artifact := range result.Artifacts {
            fmt.Fprintf(p.output, "  - %s\n", artifact)
        }
    }

    if len(result.ChildTaskIDs) > 0 {
        fmt.Fprintf(p.output, "\nChild tasks created:\n")
        for _, childID := range result.ChildTaskIDs {
            fmt.Fprintf(p.output, "  - %s\n", childID)
        }
    }

    return nil
}
```

**JSON Presenter**: `json_presenter.go`
```go
package cli

import (
    "encoding/json"
    "io"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

type JSONPresenter struct {
    output io.Writer
}

func NewJSONPresenter(output io.Writer) *JSONPresenter {
    return &JSONPresenter{output: output}
}

func (p *JSONPresenter) PresentSuccess(message string, data interface{}) error {
    result := map[string]interface{}{
        "success": true,
        "message": message,
        "data":    data,
    }
    return json.NewEncoder(p.output).Encode(result)
}

func (p *JSONPresenter) PresentError(err error) error {
    result := map[string]interface{}{
        "success": false,
        "error":   err.Error(),
    }
    return json.NewEncoder(p.output).Encode(result)
}

func (p *JSONPresenter) PresentProgress(message string, progress int, total int) error {
    result := map[string]interface{}{
        "type":     "progress",
        "message":  message,
        "progress": progress,
        "total":    total,
    }
    return json.NewEncoder(p.output).Encode(result)
}
```

#### 4.4 Storage Gateway Mock実装

**ファイル**: `internal/adapter/gateway/storage/mock_storage_gateway.go`
```go
package storage

import (
    "context"
    "fmt"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

type MockStorageGateway struct {
    artifacts map[string]string
}

func NewMockStorageGateway() *MockStorageGateway {
    return &MockStorageGateway{
        artifacts: make(map[string]string),
    }
}

func (g *MockStorageGateway) SaveArtifact(ctx context.Context, req output.SaveArtifactRequest) (*output.ArtifactMetadata, error) {
    artifactID := fmt.Sprintf("mock-artifact-%d", len(g.artifacts)+1)
    g.artifacts[artifactID] = string(req.Content)

    return &output.ArtifactMetadata{
        ArtifactID:  artifactID,
        Path:        req.Path,
        Size:        int64(len(req.Content)),
        ContentType: req.ContentType,
        Location:    "mock://artifacts/" + artifactID,
    }, nil
}

func (g *MockStorageGateway) LoadArtifact(ctx context.Context, artifactID string) (*output.Artifact, error) {
    content, exists := g.artifacts[artifactID]
    if !exists {
        return nil, fmt.Errorf("artifact not found: %s", artifactID)
    }

    return &output.Artifact{
        ID:      artifactID,
        Content: []byte(content),
    }, nil
}

func (g *MockStorageGateway) LoadInstruction(ctx context.Context, instructionPath string) (string, error) {
    // Mock implementation
    return fmt.Sprintf("[Mock Instruction] Content from %s", instructionPath), nil
}
```

#### 4.5 DI Container構築

**ファイル**: `internal/infrastructure/di/container.go`
```go
package di

import (
    "os"

    "github.com/YoshitsuguKoike/deespec/internal/adapter/controller/cli"
    "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/agent"
    "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/storage"
    clipresenter "github.com/YoshitsuguKoike/deespec/internal/adapter/presenter/cli"
    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
    "github.com/YoshitsuguKoike/deespec/internal/application/usecase/task"
    "github.com/YoshitsuguKoike/deespec/internal/application/usecase/workflow"
    "github.com/YoshitsuguKoike/deespec/internal/domain/factory"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
    "github.com/YoshitsuguKoike/deespec/internal/domain/service/strategy"
)

type Container struct {
    // Repositories (Mockで初期化、Phase 5でSQLite実装に置き換え)
    TaskRepo repository.TaskRepository
    EPICRepo repository.EPICRepository
    PBIRepo  repository.PBIRepository
    SBIRepo  repository.SBIRepository

    // Gateways
    AgentGateway   output.AgentGateway
    StorageGateway output.StorageGateway

    // Use Cases
    TaskUseCase     *task.TaskUseCaseImpl
    WorkflowUseCase *workflow.WorkflowUseCaseImpl

    // Presenters
    CLIPresenter  output.Presenter
    JSONPresenter output.Presenter

    // Controllers
    SBIController      *cli.SBIController
    PBIController      *cli.PBIController
    EPICController     *cli.EPICController
    WorkflowController *cli.WorkflowController
}

func NewContainer(format string) (*Container, error) {
    c := &Container{}

    // 1. Repositories (Mock - Phase 5でSQLite実装に置き換え)
    c.TaskRepo = repository_test.NewMockTaskRepository()
    // TODO: EPIC/PBI/SBI Mock repositories

    // 2. Gateways
    agentType := os.Getenv("DEESPEC_AGENT_TYPE")
    if agentType == "" {
        agentType = "claude-code"
    }

    agentGateway, err := agent.NewAgentGateway(agentType)
    if err != nil {
        return nil, err
    }
    c.AgentGateway = agentGateway
    c.StorageGateway = storage.NewMockStorageGateway()

    // 3. Transaction Manager (Mock - Phase 5で実装)
    txManager := &MockTransactionManager{}

    // 4. Domain Services
    taskFactory := factory.NewFactory()
    strategyRegistry := strategy.NewStrategyRegistry()

    // Register strategies
    strategyRegistry.Register(model.TaskTypeEPIC, strategy.NewEPICDecompositionStrategy(c.AgentGateway))
    strategyRegistry.Register(model.TaskTypePBI, strategy.NewPBIDecompositionStrategy(c.AgentGateway))
    strategyRegistry.Register(model.TaskTypeSBI, strategy.NewSBICodeGenerationStrategy(c.AgentGateway))

    // 5. Use Cases
    c.TaskUseCase = task.NewTaskUseCaseImpl(
        c.TaskRepo,
        c.EPICRepo,
        c.PBIRepo,
        c.SBIRepo,
        taskFactory,
        txManager,
    )

    c.WorkflowUseCase = workflow.NewWorkflowUseCaseImpl(
        c.TaskRepo,
        c.EPICRepo,
        c.PBIRepo,
        c.SBIRepo,
        strategyRegistry,
        txManager,
    )

    // 6. Presenters
    switch format {
    case "json":
        c.CLIPresenter = clipresenter.NewJSONPresenter(os.Stdout)
        c.JSONPresenter = c.CLIPresenter
    default:
        c.CLIPresenter = clipresenter.NewCLITaskPresenter(os.Stdout)
        c.JSONPresenter = clipresenter.NewJSONPresenter(os.Stdout)
    }

    // 7. Controllers
    c.SBIController = cli.NewSBIController(c.TaskUseCase, c.WorkflowUseCase, c.CLIPresenter)
    c.PBIController = cli.NewPBIController(c.TaskUseCase, c.WorkflowUseCase, c.CLIPresenter)
    c.EPICController = cli.NewEPICController(c.TaskUseCase, c.WorkflowUseCase, c.CLIPresenter)
    c.WorkflowController = cli.NewWorkflowController(c.WorkflowUseCase, c.CLIPresenter)

    return c, nil
}
```

### 成果物
- [ ] 新規CLI Controller（4ファイル）
- [ ] Claude Code Gateway実装
- [ ] Gemini/Codex Mock Gateway
- [ ] CLI Presenter（既存類似）
- [ ] JSON Presenter
- [ ] Storage Mock Gateway
- [ ] DI Container
- [ ] ユニットテスト（Use Case + Controller）

### テスト戦略
- Use Caseテスト: Mock Repository使用
- Controller統合テスト: 実際のCLIコマンド実行
- Claude Gateway E2Eテスト: 実際のAPI呼び出し（環境変数で制御）

---

## Phase 5: インフラ層整理 - SQLite Repository実装 (Week 5-6)

### 目標
- SQLite-based リポジトリ実装の完成
- トランザクション管理の統合
- マイグレーションシステムの構築

### タスク

#### 5.1 SQLiteスキーマ設計

**ファイル**: `internal/infrastructure/persistence/sqlite/schema.sql`
```sql
-- EPIC テーブル
CREATE TABLE IF NOT EXISTS epics (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    estimated_story_points INTEGER,
    priority TEXT,
    labels TEXT, -- JSON array
    assigned_agent TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- PBI テーブル
CREATE TABLE IF NOT EXISTS pbis (
    id TEXT PRIMARY KEY,
    parent_epic_id TEXT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    story_points INTEGER,
    priority TEXT,
    labels TEXT, -- JSON array
    assigned_agent TEXT,
    acceptance_criteria TEXT, -- JSON array
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (parent_epic_id) REFERENCES epics(id) ON DELETE SET NULL
);

-- SBI テーブル
CREATE TABLE IF NOT EXISTS sbis (
    id TEXT PRIMARY KEY,
    parent_pbi_id TEXT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    estimated_hours REAL,
    priority TEXT,
    labels TEXT, -- JSON array
    assigned_agent TEXT,
    file_paths TEXT, -- JSON array
    current_turn INTEGER NOT NULL DEFAULT 1,
    current_attempt INTEGER NOT NULL DEFAULT 1,
    max_turns INTEGER NOT NULL DEFAULT 10,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error TEXT,
    artifact_paths TEXT, -- JSON array
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (parent_pbi_id) REFERENCES pbis(id) ON DELETE SET NULL
);

-- EPIC-PBI 関連テーブル
CREATE TABLE IF NOT EXISTS epic_pbis (
    epic_id TEXT NOT NULL,
    pbi_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    PRIMARY KEY (epic_id, pbi_id),
    FOREIGN KEY (epic_id) REFERENCES epics(id) ON DELETE CASCADE,
    FOREIGN KEY (pbi_id) REFERENCES pbis(id) ON DELETE CASCADE
);

-- PBI-SBI 関連テーブル
CREATE TABLE IF NOT EXISTS pbi_sbis (
    pbi_id TEXT NOT NULL,
    sbi_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    PRIMARY KEY (pbi_id, sbi_id),
    FOREIGN KEY (pbi_id) REFERENCES pbis(id) ON DELETE CASCADE,
    FOREIGN KEY (sbi_id) REFERENCES sbis(id) ON DELETE CASCADE
);

-- インデックス
CREATE INDEX IF NOT EXISTS idx_pbis_parent_epic_id ON pbis(parent_epic_id);
CREATE INDEX IF NOT EXISTS idx_sbis_parent_pbi_id ON sbis(parent_pbi_id);
CREATE INDEX IF NOT EXISTS idx_epics_status ON epics(status);
CREATE INDEX IF NOT EXISTS idx_pbis_status ON pbis(status);
CREATE INDEX IF NOT EXISTS idx_sbis_status ON sbis(status);
```

#### 5.2 SQLiteリポジトリ実装

**ファイル**: `internal/infrastructure/persistence/sqlite/task_repository_impl.go`
```go
package sqlite

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    _ "github.com/mattn/go-sqlite3"

    "github.com/YoshitsuguKoike/deespec/internal/domain/model"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// TaskRepositoryImpl implements repository.TaskRepository with SQLite
type TaskRepositoryImpl struct {
    db *sql.DB
}

// NewTaskRepository creates a new SQLite-based task repository
func NewTaskRepository(db *sql.DB) repository.TaskRepository {
    return &TaskRepositoryImpl{db: db}
}

// FindByID retrieves a task by ID (polymorphic)
func (r *TaskRepositoryImpl) FindByID(ctx context.Context, id repository.TaskID) (task.Task, error) {
    // 1. Determine task type by querying all tables
    taskType, err := r.determineTaskType(ctx, string(id))
    if err != nil {
        return nil, err
    }

    // 2. Load appropriate task type
    switch taskType {
    case repository.TaskTypeEPIC:
        return r.findEPIC(ctx, string(id))
    case repository.TaskTypePBI:
        return r.findPBI(ctx, string(id))
    case repository.TaskTypeSBI:
        return r.findSBI(ctx, string(id))
    default:
        return nil, fmt.Errorf("task not found: %s", id)
    }
}

// Save persists a task entity
func (r *TaskRepositoryImpl) Save(ctx context.Context, t task.Task) error {
    switch t.Type() {
    case model.TaskTypeEPIC:
        return r.saveEPIC(ctx, t.(*epic.EPIC))
    case model.TaskTypePBI:
        return r.savePBI(ctx, t.(*pbi.PBI))
    case model.TaskTypeSBI:
        return r.saveSBI(ctx, t.(*sbi.SBI))
    default:
        return fmt.Errorf("unknown task type: %s", t.Type())
    }
}

// Delete removes a task
func (r *TaskRepositoryImpl) Delete(ctx context.Context, id repository.TaskID) error {
    // Try deleting from all tables (CASCADE will handle relations)
    queries := []string{
        "DELETE FROM epics WHERE id = ?",
        "DELETE FROM pbis WHERE id = ?",
        "DELETE FROM sbis WHERE id = ?",
    }

    for _, query := range queries {
        result, err := r.db.ExecContext(ctx, query, string(id))
        if err != nil {
            return fmt.Errorf("delete task failed: %w", err)
        }

        if rows, _ := result.RowsAffected(); rows > 0 {
            return nil // Successfully deleted
        }
    }

    return fmt.Errorf("task not found: %s", id)
}

// List retrieves tasks by filter
func (r *TaskRepositoryImpl) List(ctx context.Context, filter repository.TaskFilter) ([]task.Task, error) {
    var tasks []task.Task

    // Build query based on filter
    query, args := r.buildListQuery(filter)

    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("list tasks failed: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var taskID string
        var taskType string
        if err := rows.Scan(&taskID, &taskType); err != nil {
            return nil, err
        }

        // Load full task object
        t, err := r.FindByID(ctx, repository.TaskID(taskID))
        if err != nil {
            return nil, err
        }
        tasks = append(tasks, t)
    }

    return tasks, nil
}

// Helper methods

func (r *TaskRepositoryImpl) determineTaskType(ctx context.Context, id string) (repository.TaskType, error) {
    var taskType string

    // Check EPIC
    err := r.db.QueryRowContext(ctx, "SELECT 'EPIC' FROM epics WHERE id = ?", id).Scan(&taskType)
    if err == nil {
        return repository.TaskTypeEPIC, nil
    }

    // Check PBI
    err = r.db.QueryRowContext(ctx, "SELECT 'PBI' FROM pbis WHERE id = ?", id).Scan(&taskType)
    if err == nil {
        return repository.TaskTypePBI, nil
    }

    // Check SBI
    err = r.db.QueryRowContext(ctx, "SELECT 'SBI' FROM sbis WHERE id = ?", id).Scan(&taskType)
    if err == nil {
        return repository.TaskTypeSBI, nil
    }

    return "", fmt.Errorf("task not found: %s", id)
}

func (r *TaskRepositoryImpl) findEPIC(ctx context.Context, id string) (*epic.EPIC, error) {
    query := `
        SELECT id, title, description, status, current_step,
               estimated_story_points, priority, labels, assigned_agent,
               created_at, updated_at
        FROM epics
        WHERE id = ?
    `

    var dto epicDTO
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &dto.ID, &dto.Title, &dto.Description, &dto.Status, &dto.CurrentStep,
        &dto.EstimatedStoryPoints, &dto.Priority, &dto.Labels, &dto.AssignedAgent,
        &dto.CreatedAt, &dto.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("epic not found: %w", err)
    }

    // Load PBI IDs
    pbiIDs, err := r.loadEPICPBIs(ctx, id)
    if err != nil {
        return nil, err
    }
    dto.PBIIDs = pbiIDs

    return r.epicDTOToDomain(dto)
}

func (r *TaskRepositoryImpl) saveEPIC(ctx context.Context, e *epic.EPIC) error {
    dto := r.epicToDTO(e)

    query := `
        INSERT INTO epics (id, title, description, status, current_step,
                          estimated_story_points, priority, labels, assigned_agent,
                          created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title = excluded.title,
            description = excluded.description,
            status = excluded.status,
            current_step = excluded.current_step,
            estimated_story_points = excluded.estimated_story_points,
            priority = excluded.priority,
            labels = excluded.labels,
            assigned_agent = excluded.assigned_agent,
            updated_at = excluded.updated_at
    `

    _, err := r.db.ExecContext(ctx, query,
        dto.ID, dto.Title, dto.Description, dto.Status, dto.CurrentStep,
        dto.EstimatedStoryPoints, dto.Priority, dto.Labels, dto.AssignedAgent,
        dto.CreatedAt, dto.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("save epic failed: %w", err)
    }

    // Update PBI relationships
    return r.saveEPICPBIs(ctx, dto.ID, dto.PBIIDs)
}

func (r *TaskRepositoryImpl) findSBI(ctx context.Context, id string) (*sbi.SBI, error) {
    query := `
        SELECT id, parent_pbi_id, title, description, status, current_step,
               estimated_hours, priority, labels, assigned_agent, file_paths,
               current_turn, current_attempt, max_turns, max_attempts,
               last_error, artifact_paths, created_at, updated_at
        FROM sbis
        WHERE id = ?
    `

    var dto sbiDTO
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &dto.ID, &dto.ParentPBIID, &dto.Title, &dto.Description, &dto.Status, &dto.CurrentStep,
        &dto.EstimatedHours, &dto.Priority, &dto.Labels, &dto.AssignedAgent, &dto.FilePaths,
        &dto.CurrentTurn, &dto.CurrentAttempt, &dto.MaxTurns, &dto.MaxAttempts,
        &dto.LastError, &dto.ArtifactPaths, &dto.CreatedAt, &dto.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("sbi not found: %w", err)
    }

    return r.sbiDTOToDomain(dto)
}

func (r *TaskRepositoryImpl) saveSBI(ctx context.Context, s *sbi.SBI) error {
    dto := r.sbiToDTO(s)

    query := `
        INSERT INTO sbis (id, parent_pbi_id, title, description, status, current_step,
                         estimated_hours, priority, labels, assigned_agent, file_paths,
                         current_turn, current_attempt, max_turns, max_attempts,
                         last_error, artifact_paths, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            parent_pbi_id = excluded.parent_pbi_id,
            title = excluded.title,
            description = excluded.description,
            status = excluded.status,
            current_step = excluded.current_step,
            estimated_hours = excluded.estimated_hours,
            priority = excluded.priority,
            labels = excluded.labels,
            assigned_agent = excluded.assigned_agent,
            file_paths = excluded.file_paths,
            current_turn = excluded.current_turn,
            current_attempt = excluded.current_attempt,
            max_turns = excluded.max_turns,
            max_attempts = excluded.max_attempts,
            last_error = excluded.last_error,
            artifact_paths = excluded.artifact_paths,
            updated_at = excluded.updated_at
    `

    _, err := r.db.ExecContext(ctx, query,
        dto.ID, dto.ParentPBIID, dto.Title, dto.Description, dto.Status, dto.CurrentStep,
        dto.EstimatedHours, dto.Priority, dto.Labels, dto.AssignedAgent, dto.FilePaths,
        dto.CurrentTurn, dto.CurrentAttempt, dto.MaxTurns, dto.MaxAttempts,
        dto.LastError, dto.ArtifactPaths, dto.CreatedAt, dto.UpdatedAt,
    )

    return err
}

// DTO structures for SQLite persistence
type epicDTO struct {
    ID                   string
    Title                string
    Description          string
    Status               string
    CurrentStep          string
    EstimatedStoryPoints *int
    Priority             string
    Labels               string // JSON
    AssignedAgent        string
    PBIIDs               []string
    CreatedAt            string
    UpdatedAt            string
}

type sbiDTO struct {
    ID             string
    ParentPBIID    *string
    Title          string
    Description    string
    Status         string
    CurrentStep    string
    EstimatedHours *float64
    Priority       string
    Labels         string // JSON
    AssignedAgent  string
    FilePaths      string // JSON
    CurrentTurn    int
    CurrentAttempt int
    MaxTurns       int
    MaxAttempts    int
    LastError      *string
    ArtifactPaths  string // JSON
    CreatedAt      string
    UpdatedAt      string
}
```

#### 5.3 SQLite Transaction Manager実装

**ファイル**: `internal/infrastructure/persistence/sqlite/transaction_manager.go`
```go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// SQLiteTransactionManager implements output.TransactionManager
type SQLiteTransactionManager struct {
    db *sql.DB
}

// NewTransactionManager creates a new SQLite transaction manager
func NewTransactionManager(db *sql.DB) output.TransactionManager {
    return &SQLiteTransactionManager{db: db}
}

// InTransaction executes a function within a database transaction
func (tm *SQLiteTransactionManager) InTransaction(ctx context.Context, fn func(context.Context) error) error {
    // Start transaction
    tx, err := tm.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelSerializable,
    })
    if err != nil {
        return fmt.Errorf("begin transaction failed: %w", err)
    }

    // Store transaction in context
    txCtx := context.WithValue(ctx, txContextKey, tx)

    // Execute function
    if err := fn(txCtx); err != nil {
        // Rollback on error
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("rollback failed after error %v: %w", err, rbErr)
        }
        return err
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction failed: %w", err)
    }

    return nil
}

// Context key for storing transaction
type contextKey string

const txContextKey contextKey = "sqliteTx"

// GetTx retrieves the transaction from context
func GetTx(ctx context.Context) (*sql.Tx, bool) {
    tx, ok := ctx.Value(txContextKey).(*sql.Tx)
    return tx, ok
}
```

#### 5.4 マイグレーションシステム

**ファイル**: `internal/infrastructure/persistence/sqlite/migrator.go`
```go
package sqlite

import (
    "database/sql"
    "embed"
    "fmt"
    "sort"
    "strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator handles database schema migrations
type Migrator struct {
    db *sql.DB
}

// NewMigrator creates a new migrator
func NewMigrator(db *sql.DB) *Migrator {
    return &Migrator{db: db}
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
    // Create migrations table if not exists
    if err := m.createMigrationsTable(); err != nil {
        return err
    }

    // Get applied migrations
    applied, err := m.getAppliedMigrations()
    if err != nil {
        return err
    }

    // Get migration files
    files, err := migrationsFS.ReadDir("migrations")
    if err != nil {
        return fmt.Errorf("read migrations directory: %w", err)
    }

    // Sort migration files
    var migrations []string
    for _, file := range files {
        if strings.HasSuffix(file.Name(), ".sql") {
            migrations = append(migrations, file.Name())
        }
    }
    sort.Strings(migrations)

    // Run pending migrations
    for _, migration := range migrations {
        if applied[migration] {
            continue // Already applied
        }

        fmt.Printf("Running migration: %s\n", migration)

        sql, err := migrationsFS.ReadFile("migrations/" + migration)
        if err != nil {
            return fmt.Errorf("read migration %s: %w", migration, err)
        }

        tx, err := m.db.Begin()
        if err != nil {
            return err
        }

        if _, err := tx.Exec(string(sql)); err != nil {
            tx.Rollback()
            return fmt.Errorf("execute migration %s: %w", migration, err)
        }

        if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration); err != nil {
            tx.Rollback()
            return fmt.Errorf("record migration %s: %w", migration, err)
        }

        if err := tx.Commit(); err != nil {
            return fmt.Errorf("commit migration %s: %w", migration, err)
        }

        fmt.Printf("Migration %s completed\n", migration)
    }

    return nil
}

func (m *Migrator) createMigrationsTable() error {
    query := `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version TEXT PRIMARY KEY,
            applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `
    _, err := m.db.Exec(query)
    return err
}

func (m *Migrator) getAppliedMigrations() (map[string]bool, error) {
    rows, err := m.db.Query("SELECT version FROM schema_migrations")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    applied := make(map[string]bool)
    for rows.Next() {
        var version string
        if err := rows.Scan(&version); err != nil {
            return nil, err
        }
        applied[version] = true
    }

    return applied, nil
}
```

**マイグレーションファイル**: `internal/infrastructure/persistence/sqlite/migrations/001_initial_schema.sql`
```sql
-- Initial schema (copy from schema.sql above)
-- See section 5.1 for full schema
```

#### 5.5 DIコンテナ更新 (SQLite統合)

**ファイル**: `internal/infrastructure/di/container.go` (Phase 4から更新)
```go
package di

import (
    "database/sql"
    "fmt"
    "path/filepath"

    _ "github.com/mattn/go-sqlite3"

    "github.com/YoshitsuguKoike/deespec/internal/adapter/controller/cli"
    "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/agent"
    "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/storage"
    "github.com/YoshitsuguKoike/deespec/internal/adapter/presenter"
    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
    "github.com/YoshitsuguKoike/deespec/internal/application/usecase/task"
    "github.com/YoshitsuguKoike/deespec/internal/application/usecase/workflow"
    "github.com/YoshitsuguKoike/deespec/internal/domain/factory"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
    "github.com/YoshitsuguKoike/deespec/internal/domain/service/strategy"
    sqlitePersistence "github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
)

// Container holds all application dependencies
type Container struct {
    // Infrastructure
    DB          *sql.DB
    TxManager   output.TransactionManager

    // Repositories (SQLite-based)
    TaskRepo    repository.TaskRepository
    EPICRepo    repository.EPICRepository
    PBIRepo     repository.PBIRepository
    SBIRepo     repository.SBIRepository

    // Gateways
    AgentGateway   output.AgentGateway
    StorageGateway output.StorageGateway

    // Domain
    TaskFactory      *factory.Factory
    StrategyRegistry *strategy.StrategyRegistry

    // Use Cases
    TaskUseCase     *task.TaskUseCaseImpl
    WorkflowUseCase *workflow.WorkflowUseCaseImpl

    // Presenters
    Presenter output.Presenter

    // Controllers
    EPICController *cli.EPICController
    PBIController  *cli.PBIController
    SBIController  *cli.SBIController
}

// NewContainer creates and initializes the DI container
func NewContainer(format string, dbPath string) (*Container, error) {
    c := &Container{}

    // 1. Initialize SQLite database
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }

    // Enable foreign keys
    if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
        return nil, fmt.Errorf("enable foreign keys: %w", err)
    }

    c.DB = db

    // Run migrations
    migrator := sqlitePersistence.NewMigrator(db)
    if err := migrator.Migrate(); err != nil {
        return nil, fmt.Errorf("run migrations: %w", err)
    }

    // 2. Infrastructure layer
    c.TxManager = sqlitePersistence.NewTransactionManager(db)

    // 3. Repositories (SQLite implementation)
    c.TaskRepo = sqlitePersistence.NewTaskRepository(db)
    c.EPICRepo = sqlitePersistence.NewEPICRepository(db)
    c.PBIRepo = sqlitePersistence.NewPBIRepository(db)
    c.SBIRepo = sqlitePersistence.NewSBIRepository(db)

    // 4. Gateways
    agentType := "claude-code" // Or from config
    c.AgentGateway, err = agent.NewAgentGateway(agentType)
    if err != nil {
        return nil, fmt.Errorf("create agent gateway: %w", err)
    }

    c.StorageGateway = storage.NewMockStorageGateway()

    // 5. Domain layer
    c.TaskFactory = factory.NewFactory()
    c.StrategyRegistry = strategy.NewStrategyRegistry()

    // Register strategies
    c.StrategyRegistry.Register(model.TaskTypeEPIC, strategy.NewEPICDecompositionStrategy(c.AgentGateway, c.TaskFactory, c.PBIRepo))
    c.StrategyRegistry.Register(model.TaskTypePBI, strategy.NewPBIDecompositionStrategy(c.AgentGateway, c.TaskFactory, c.SBIRepo))
    c.StrategyRegistry.Register(model.TaskTypeSBI, strategy.NewSBICodeGenerationStrategy(c.AgentGateway, c.StorageGateway))

    // 6. Use Cases
    c.TaskUseCase = task.NewTaskUseCaseImpl(
        c.TaskRepo,
        c.EPICRepo,
        c.PBIRepo,
        c.SBIRepo,
        c.TaskFactory,
        c.TxManager,
    )

    c.WorkflowUseCase = workflow.NewWorkflowUseCaseImpl(
        c.TaskRepo,
        c.EPICRepo,
        c.PBIRepo,
        c.SBIRepo,
        c.StrategyRegistry,
        c.TxManager,
    )

    // 7. Presenters (format-based selection)
    if format == "json" {
        c.Presenter = presenter.NewJSONPresenter(os.Stdout)
    } else {
        c.Presenter = presenter.NewCLITaskPresenter(os.Stdout)
    }

    // 8. Controllers
    c.EPICController = cli.NewEPICController(c.TaskUseCase, c.WorkflowUseCase, c.Presenter)
    c.PBIController = cli.NewPBIController(c.TaskUseCase, c.WorkflowUseCase, c.Presenter)
    c.SBIController = cli.NewSBIController(c.TaskUseCase, c.WorkflowUseCase, c.Presenter)

    return c, nil
}

// Close closes all resources
func (c *Container) Close() error {
    if c.DB != nil {
        return c.DB.Close()
    }
    return nil
}
```

### 成果物
- [ ] SQLiteスキーマ設計完了
- [ ] TaskRepository SQLite実装完了
- [ ] EPIC/PBI/SBI専用Repository実装
- [ ] Transaction Manager実装
- [ ] マイグレーションシステム実装
- [ ] DIコンテナSQLite統合完了
- [ ] ユニットテスト (カバレッジ > 80%)
- [ ] 統合テスト (Repository層)

---

## Phase 6: Storage Gateway実装 (Week 7)

### 目標
- S3ストレージゲートウェイの実装
- ローカルファイルシステムゲートウェイの実装
- Artifact管理の統合

### タスク

#### 6.1 StorageGateway インターフェース (既にPhase 3で定義済み)

```go
// internal/application/port/output/storage_gateway.go
package output

import "context"

// StorageGateway handles artifact storage operations
type StorageGateway interface {
    // Store saves an artifact and returns its storage path
    Store(ctx context.Context, artifact Artifact) (string, error)

    // Retrieve loads an artifact by its storage path
    Retrieve(ctx context.Context, path string) (*Artifact, error)

    // Delete removes an artifact
    Delete(ctx context.Context, path string) error

    // List retrieves all artifacts for a given prefix
    List(ctx context.Context, prefix string) ([]string, error)
}

// Artifact represents a stored file or data
type Artifact struct {
    Path        string
    Content     []byte
    ContentType string
    Metadata    map[string]string
}
```

#### 6.2 S3 Storage Gateway実装

**ファイル**: `internal/adapter/gateway/storage/s3_storage_gateway.go`
```go
package storage

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "path/filepath"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// S3StorageGateway implements StorageGateway with AWS S3
type S3StorageGateway struct {
    client     *s3.Client
    bucketName string
    prefix     string // Optional prefix for all keys
}

// NewS3StorageGateway creates a new S3 storage gateway
func NewS3StorageGateway(bucketName, prefix string) (*S3StorageGateway, error) {
    cfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        return nil, fmt.Errorf("load AWS config: %w", err)
    }

    client := s3.NewFromConfig(cfg)

    return &S3StorageGateway{
        client:     client,
        bucketName: bucketName,
        prefix:     prefix,
    }, nil
}

// Store saves an artifact to S3
func (g *S3StorageGateway) Store(ctx context.Context, artifact output.Artifact) (string, error) {
    key := g.buildKey(artifact.Path)

    // Prepare metadata
    metadata := make(map[string]string)
    for k, v := range artifact.Metadata {
        metadata[k] = v
    }

    // Upload to S3
    _, err := g.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(g.bucketName),
        Key:         aws.String(key),
        Body:        bytes.NewReader(artifact.Content),
        ContentType: aws.String(artifact.ContentType),
        Metadata:    metadata,
    })
    if err != nil {
        return "", fmt.Errorf("upload to S3: %w", err)
    }

    return key, nil
}

// Retrieve loads an artifact from S3
func (g *S3StorageGateway) Retrieve(ctx context.Context, path string) (*output.Artifact, error) {
    key := g.buildKey(path)

    // Download from S3
    result, err := g.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(g.bucketName),
        Key:    aws.String(key),
    })
    if err != nil {
        return nil, fmt.Errorf("download from S3: %w", err)
    }
    defer result.Body.Close()

    // Read content
    content, err := io.ReadAll(result.Body)
    if err != nil {
        return nil, fmt.Errorf("read S3 object: %w", err)
    }

    // Extract metadata
    metadata := make(map[string]string)
    for k, v := range result.Metadata {
        metadata[k] = v
    }

    return &output.Artifact{
        Path:        path,
        Content:     content,
        ContentType: aws.ToString(result.ContentType),
        Metadata:    metadata,
    }, nil
}

// Delete removes an artifact from S3
func (g *S3StorageGateway) Delete(ctx context.Context, path string) error {
    key := g.buildKey(path)

    _, err := g.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(g.bucketName),
        Key:    aws.String(key),
    })
    if err != nil {
        return fmt.Errorf("delete from S3: %w", err)
    }

    return nil
}

// List retrieves all artifacts for a given prefix
func (g *S3StorageGateway) List(ctx context.Context, prefix string) ([]string, error) {
    key := g.buildKey(prefix)

    result, err := g.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
        Bucket: aws.String(g.bucketName),
        Prefix: aws.String(key),
    })
    if err != nil {
        return nil, fmt.Errorf("list S3 objects: %w", err)
    }

    var paths []string
    for _, obj := range result.Contents {
        // Remove prefix from key
        relativePath := g.removePrefix(aws.ToString(obj.Key))
        paths = append(paths, relativePath)
    }

    return paths, nil
}

func (g *S3StorageGateway) buildKey(path string) string {
    if g.prefix == "" {
        return path
    }
    return filepath.Join(g.prefix, path)
}

func (g *S3StorageGateway) removePrefix(key string) string {
    if g.prefix == "" {
        return key
    }
    return key[len(g.prefix)+1:] // +1 for separator
}
```

#### 6.3 Local Filesystem Storage Gateway実装

**ファイル**: `internal/adapter/gateway/storage/local_storage_gateway.go`
```go
package storage

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// LocalStorageGateway implements StorageGateway with local filesystem
type LocalStorageGateway struct {
    basePath string
}

// NewLocalStorageGateway creates a new local filesystem storage gateway
func NewLocalStorageGateway(basePath string) (*LocalStorageGateway, error) {
    // Ensure base path exists
    if err := os.MkdirAll(basePath, 0755); err != nil {
        return nil, fmt.Errorf("create base path: %w", err)
    }

    return &LocalStorageGateway{
        basePath: basePath,
    }, nil
}

// Store saves an artifact to local filesystem
func (g *LocalStorageGateway) Store(ctx context.Context, artifact output.Artifact) (string, error) {
    fullPath := filepath.Join(g.basePath, artifact.Path)

    // Ensure parent directory exists
    if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
        return "", fmt.Errorf("create parent directory: %w", err)
    }

    // Write file atomically
    tmpPath := fullPath + ".tmp"
    if err := os.WriteFile(tmpPath, artifact.Content, 0644); err != nil {
        return "", fmt.Errorf("write temp file: %w", err)
    }

    if err := os.Rename(tmpPath, fullPath); err != nil {
        os.Remove(tmpPath)
        return "", fmt.Errorf("atomic rename: %w", err)
    }

    return artifact.Path, nil
}

// Retrieve loads an artifact from local filesystem
func (g *LocalStorageGateway) Retrieve(ctx context.Context, path string) (*output.Artifact, error) {
    fullPath := filepath.Join(g.basePath, path)

    content, err := os.ReadFile(fullPath)
    if err != nil {
        return nil, fmt.Errorf("read file: %w", err)
    }

    return &output.Artifact{
        Path:        path,
        Content:     content,
        ContentType: detectContentType(fullPath),
        Metadata:    make(map[string]string),
    }, nil
}

// Delete removes an artifact from local filesystem
func (g *LocalStorageGateway) Delete(ctx context.Context, path string) error {
    fullPath := filepath.Join(g.basePath, path)

    if err := os.Remove(fullPath); err != nil {
        return fmt.Errorf("delete file: %w", err)
    }

    return nil
}

// List retrieves all artifacts for a given prefix
func (g *LocalStorageGateway) List(ctx context.Context, prefix string) ([]string, error) {
    fullPrefix := filepath.Join(g.basePath, prefix)

    var paths []string
    err := filepath.Walk(fullPrefix, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if !info.IsDir() {
            relativePath, err := filepath.Rel(g.basePath, path)
            if err != nil {
                return err
            }
            paths = append(paths, relativePath)
        }

        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("walk directory: %w", err)
    }

    return paths, nil
}

func detectContentType(path string) string {
    ext := filepath.Ext(path)
    switch ext {
    case ".json":
        return "application/json"
    case ".go":
        return "text/x-go"
    case ".md":
        return "text/markdown"
    case ".txt":
        return "text/plain"
    default:
        return "application/octet-stream"
    }
}
```

#### 6.4 Hybrid Storage Gateway (S3 + Local Fallback)

**ファイル**: `internal/adapter/gateway/storage/hybrid_storage_gateway.go`
```go
package storage

import (
    "context"
    "fmt"

    "github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// HybridStorageGateway implements StorageGateway with S3 primary and local fallback
type HybridStorageGateway struct {
    primary   output.StorageGateway
    fallback  output.StorageGateway
    useFallback bool
}

// NewHybridStorageGateway creates a hybrid storage gateway
func NewHybridStorageGateway(s3Bucket, s3Prefix, localPath string) (*HybridStorageGateway, error) {
    // Try to create S3 gateway
    s3Gateway, err := NewS3StorageGateway(s3Bucket, s3Prefix)
    useFallback := (err != nil)

    // Create local gateway as fallback
    localGateway, err := NewLocalStorageGateway(localPath)
    if err != nil {
        return nil, fmt.Errorf("create local gateway: %w", err)
    }

    return &HybridStorageGateway{
        primary:     s3Gateway,
        fallback:    localGateway,
        useFallback: useFallback,
    }, nil
}

// Store saves an artifact to primary or fallback storage
func (g *HybridStorageGateway) Store(ctx context.Context, artifact output.Artifact) (string, error) {
    if g.useFallback {
        return g.fallback.Store(ctx, artifact)
    }

    path, err := g.primary.Store(ctx, artifact)
    if err != nil {
        // Fallback to local storage on S3 error
        fmt.Printf("Warning: S3 store failed, using local fallback: %v\n", err)
        return g.fallback.Store(ctx, artifact)
    }

    return path, nil
}

// Retrieve loads an artifact from primary or fallback storage
func (g *HybridStorageGateway) Retrieve(ctx context.Context, path string) (*output.Artifact, error) {
    if g.useFallback {
        return g.fallback.Retrieve(ctx, path)
    }

    artifact, err := g.primary.Retrieve(ctx, path)
    if err != nil {
        // Fallback to local storage on S3 error
        return g.fallback.Retrieve(ctx, path)
    }

    return artifact, nil
}

// Delete removes an artifact from both storages
func (g *HybridStorageGateway) Delete(ctx context.Context, path string) error {
    var errs []error

    if !g.useFallback {
        if err := g.primary.Delete(ctx, path); err != nil {
            errs = append(errs, err)
        }
    }

    if err := g.fallback.Delete(ctx, path); err != nil {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        return fmt.Errorf("delete errors: %v", errs)
    }

    return nil
}

// List retrieves all artifacts from primary storage
func (g *HybridStorageGateway) List(ctx context.Context, prefix string) ([]string, error) {
    if g.useFallback {
        return g.fallback.List(ctx, prefix)
    }

    paths, err := g.primary.List(ctx, prefix)
    if err != nil {
        return g.fallback.List(ctx, prefix)
    }

    return paths, nil
}
```

#### 6.5 DIコンテナ統合 (Storage Gateway選択)

**ファイル**: `internal/infrastructure/di/container.go` (Phase 5から更新)

```go
// NewContainer内でStorage Gatewayを選択
func NewContainer(format string, dbPath string, storageType string) (*Container, error) {
    // ... (前略)

    // 4. Gateways
    agentType := "claude-code"
    c.AgentGateway, err = agent.NewAgentGateway(agentType)
    if err != nil {
        return nil, fmt.Errorf("create agent gateway: %w", err)
    }

    // Storage Gateway selection
    switch storageType {
    case "s3":
        bucket := os.Getenv("DEESPEC_S3_BUCKET")
        prefix := os.Getenv("DEESPEC_S3_PREFIX")
        c.StorageGateway, err = storage.NewS3StorageGateway(bucket, prefix)
    case "local":
        localPath := os.Getenv("DEESPEC_STORAGE_PATH")
        if localPath == "" {
            localPath = filepath.Join(os.Getenv("HOME"), ".deespec", "storage")
        }
        c.StorageGateway, err = storage.NewLocalStorageGateway(localPath)
    case "hybrid":
        bucket := os.Getenv("DEESPEC_S3_BUCKET")
        prefix := os.Getenv("DEESPEC_S3_PREFIX")
        localPath := filepath.Join(os.Getenv("HOME"), ".deespec", "storage")
        c.StorageGateway, err = storage.NewHybridStorageGateway(bucket, prefix, localPath)
    default:
        // Default to local storage
        localPath := filepath.Join(os.Getenv("HOME"), ".deespec", "storage")
        c.StorageGateway, err = storage.NewLocalStorageGateway(localPath)
    }

    if err != nil {
        return nil, fmt.Errorf("create storage gateway: %w", err)
    }

    // ... (続く)
}
```

### 成果物
- [ ] S3 Storage Gateway実装完了
- [ ] Local Storage Gateway実装完了
- [ ] Hybrid Storage Gateway実装完了
- [ ] DIコンテナStorage Gateway統合
- [ ] ユニットテスト (カバレッジ > 80%)
- [ ] 統合テスト (Storage Gateway層)

---

## Phase 7: Lock System SQLite移行 (Week 8)

### 目標
- RunLockのSQLite移行
- StateLockのSQLite移行
- Heartbeat監視機能の統合

### 背景

現在のDeeSpecには2つのロックシステムが存在:

1. **RunLock** (`internal/interface/cli/runlock.go`): 実行ロック管理
   - SBI実行の並行制御
   - プロセスIDとホスト名の追跡
   - TTLベースの期限切れ管理

2. **StateLock** (`internal/app/paths.go`): ステートファイルロック
   - 状態ファイルへの排他アクセス管理
   - ファイル位置: `.deespec/var/state.lock`

これらをファイルベースからSQLiteベースに移行し、より堅牢なロック管理を実現します。

### タスク

#### 7.1 Lock SQLiteスキーマ設計

**ファイル**: `internal/infrastructure/persistence/sqlite/migrations/002_locks.sql`
```sql
-- RunLock テーブル
CREATE TABLE IF NOT EXISTS run_locks (
    lock_id TEXT PRIMARY KEY,      -- SBI ID or resource identifier
    pid INTEGER NOT NULL,           -- Process ID
    hostname TEXT NOT NULL,         -- Host name
    acquired_at DATETIME NOT NULL,  -- Lock acquisition time
    expires_at DATETIME NOT NULL,   -- Lock expiration time
    heartbeat_at DATETIME NOT NULL, -- Last heartbeat time
    metadata TEXT                   -- JSON metadata
);

-- StateLock テーブル
CREATE TABLE IF NOT EXISTS state_locks (
    lock_id TEXT PRIMARY KEY,       -- Resource identifier
    pid INTEGER NOT NULL,           -- Process ID
    hostname TEXT NOT NULL,         -- Host name
    acquired_at DATETIME NOT NULL,  -- Lock acquisition time
    expires_at DATETIME NOT NULL,   -- Lock expiration time
    heartbeat_at DATETIME NOT NULL, -- Last heartbeat time
    lock_type TEXT NOT NULL         -- Lock type: read, write
);

-- インデックス
CREATE INDEX IF NOT EXISTS idx_run_locks_expires_at ON run_locks(expires_at);
CREATE INDEX IF NOT EXISTS idx_state_locks_expires_at ON state_locks(expires_at);
CREATE INDEX IF NOT EXISTS idx_run_locks_heartbeat ON run_locks(heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_state_locks_heartbeat ON state_locks(heartbeat_at);
```

#### 7.2 Domain Lock Models

**ファイル**: `internal/domain/model/lock/run_lock.go`
```go
package lock

import (
    "fmt"
    "os"
    "time"
)

// RunLock represents an execution lock for SBI
type RunLock struct {
    lockID      LockID
    pid         int
    hostname    string
    acquiredAt  time.Time
    expiresAt   time.Time
    heartbeatAt time.Time
    metadata    map[string]string
}

// LockID is a unique lock identifier
type LockID struct {
    value string
}

// NewLockID creates a new lock ID
func NewLockID(value string) (LockID, error) {
    if value == "" {
        return LockID{}, fmt.Errorf("lock ID cannot be empty")
    }
    return LockID{value: value}, nil
}

func (id LockID) String() string {
    return id.value
}

// NewRunLock creates a new run lock
func NewRunLock(lockID LockID, ttl time.Duration) (*RunLock, error) {
    hostname, err := os.Hostname()
    if err != nil {
        return nil, fmt.Errorf("get hostname: %w", err)
    }

    now := time.Now()

    return &RunLock{
        lockID:      lockID,
        pid:         os.Getpid(),
        hostname:    hostname,
        acquiredAt:  now,
        expiresAt:   now.Add(ttl),
        heartbeatAt: now,
        metadata:    make(map[string]string),
    }, nil
}

// IsExpired checks if the lock has expired
func (l *RunLock) IsExpired() bool {
    return time.Now().After(l.expiresAt)
}

// UpdateHeartbeat updates the heartbeat timestamp
func (l *RunLock) UpdateHeartbeat() {
    l.heartbeatAt = time.Now()
}

// Extend extends the lock expiration time
func (l *RunLock) Extend(duration time.Duration) {
    l.expiresAt = l.expiresAt.Add(duration)
}

// Getters
func (l *RunLock) LockID() LockID { return l.lockID }
func (l *RunLock) PID() int { return l.pid }
func (l *RunLock) Hostname() string { return l.hostname }
func (l *RunLock) AcquiredAt() time.Time { return l.acquiredAt }
func (l *RunLock) ExpiresAt() time.Time { return l.expiresAt }
func (l *RunLock) HeartbeatAt() time.Time { return l.heartbeatAt }
func (l *RunLock) Metadata() map[string]string { return l.metadata }
```

**ファイル**: `internal/domain/model/lock/state_lock.go`
```go
package lock

import (
    "fmt"
    "os"
    "time"
)

// LockType represents the type of state lock
type LockType string

const (
    LockTypeRead  LockType = "read"
    LockTypeWrite LockType = "write"
)

// StateLock represents a state file lock
type StateLock struct {
    lockID      LockID
    pid         int
    hostname    string
    acquiredAt  time.Time
    expiresAt   time.Time
    heartbeatAt time.Time
    lockType    LockType
}

// NewStateLock creates a new state lock
func NewStateLock(lockID LockID, lockType LockType, ttl time.Duration) (*StateLock, error) {
    hostname, err := os.Hostname()
    if err != nil {
        return nil, fmt.Errorf("get hostname: %w", err)
    }

    if lockType != LockTypeRead && lockType != LockTypeWrite {
        return nil, fmt.Errorf("invalid lock type: %s", lockType)
    }

    now := time.Now()

    return &StateLock{
        lockID:      lockID,
        pid:         os.Getpid(),
        hostname:    hostname,
        acquiredAt:  now,
        expiresAt:   now.Add(ttl),
        heartbeatAt: now,
        lockType:    lockType,
    }, nil
}

// IsExpired checks if the lock has expired
func (l *StateLock) IsExpired() bool {
    return time.Now().After(l.expiresAt)
}

// UpdateHeartbeat updates the heartbeat timestamp
func (l *StateLock) UpdateHeartbeat() {
    l.heartbeatAt = time.Now()
}

// Extend extends the lock expiration time
func (l *StateLock) Extend(duration time.Duration) {
    l.expiresAt = l.expiresAt.Add(duration)
}

// Getters
func (l *StateLock) LockID() LockID { return l.lockID }
func (l *StateLock) PID() int { return l.pid }
func (l *StateLock) Hostname() string { return l.hostname }
func (l *StateLock) AcquiredAt() time.Time { return l.acquiredAt }
func (l *StateLock) ExpiresAt() time.Time { return l.expiresAt }
func (l *StateLock) HeartbeatAt() time.Time { return l.heartbeatAt }
func (l *StateLock) LockType() LockType { return l.lockType }
```

#### 7.3 Lock Repository

**ファイル**: `internal/domain/repository/lock_repository.go`
```go
package repository

import (
    "context"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

// LockRepository manages lock persistence
type LockRepository interface {
    // RunLock operations
    AcquireRunLock(ctx context.Context, runLock *lock.RunLock) error
    ReleaseRunLock(ctx context.Context, lockID lock.LockID) error
    FindRunLock(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error)
    UpdateRunLockHeartbeat(ctx context.Context, lockID lock.LockID) error
    CleanExpiredRunLocks(ctx context.Context) (int, error)

    // StateLock operations
    AcquireStateLock(ctx context.Context, stateLock *lock.StateLock) error
    ReleaseStateLock(ctx context.Context, lockID lock.LockID) error
    FindStateLock(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error)
    UpdateStateLockHeartbeat(ctx context.Context, lockID lock.LockID) error
    CleanExpiredStateLocks(ctx context.Context) (int, error)
}
```

#### 7.4 Lock Repository SQLite実装

**ファイル**: `internal/infrastructure/persistence/sqlite/lock_repository_impl.go`
```go
package sqlite

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// LockRepositoryImpl implements repository.LockRepository with SQLite
type LockRepositoryImpl struct {
    db *sql.DB
}

// NewLockRepository creates a new SQLite-based lock repository
func NewLockRepository(db *sql.DB) repository.LockRepository {
    return &LockRepositoryImpl{db: db}
}

// AcquireRunLock acquires a run lock
func (r *LockRepositoryImpl) AcquireRunLock(ctx context.Context, runLock *lock.RunLock) error {
    // Try to insert lock (will fail if already exists)
    query := `
        INSERT INTO run_locks (lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, metadata)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `

    metadata, _ := json.Marshal(runLock.Metadata())

    _, err := r.db.ExecContext(ctx, query,
        runLock.LockID().String(),
        runLock.PID(),
        runLock.Hostname(),
        runLock.AcquiredAt(),
        runLock.ExpiresAt(),
        runLock.HeartbeatAt(),
        string(metadata),
    )

    if err != nil {
        // Check if lock exists and is expired
        existing, findErr := r.FindRunLock(ctx, runLock.LockID())
        if findErr == nil && existing.IsExpired() {
            // Release expired lock and retry
            _ = r.ReleaseRunLock(ctx, runLock.LockID())
            return r.AcquireRunLock(ctx, runLock)
        }

        return fmt.Errorf("acquire run lock failed: %w", err)
    }

    return nil
}

// ReleaseRunLock releases a run lock
func (r *LockRepositoryImpl) ReleaseRunLock(ctx context.Context, lockID lock.LockID) error {
    query := "DELETE FROM run_locks WHERE lock_id = ?"
    _, err := r.db.ExecContext(ctx, query, lockID.String())
    return err
}

// FindRunLock finds a run lock by ID
func (r *LockRepositoryImpl) FindRunLock(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error) {
    query := `
        SELECT lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, metadata
        FROM run_locks
        WHERE lock_id = ?
    `

    var dto runLockDTO
    err := r.db.QueryRowContext(ctx, query, lockID.String()).Scan(
        &dto.LockID, &dto.PID, &dto.Hostname,
        &dto.AcquiredAt, &dto.ExpiresAt, &dto.HeartbeatAt,
        &dto.Metadata,
    )
    if err != nil {
        return nil, fmt.Errorf("run lock not found: %w", err)
    }

    return r.runLockDTOToDomain(dto)
}

// UpdateRunLockHeartbeat updates the heartbeat timestamp
func (r *LockRepositoryImpl) UpdateRunLockHeartbeat(ctx context.Context, lockID lock.LockID) error {
    query := "UPDATE run_locks SET heartbeat_at = ? WHERE lock_id = ?"
    _, err := r.db.ExecContext(ctx, query, time.Now(), lockID.String())
    return err
}

// CleanExpiredRunLocks removes all expired run locks
func (r *LockRepositoryImpl) CleanExpiredRunLocks(ctx context.Context) (int, error) {
    query := "DELETE FROM run_locks WHERE expires_at < ?"
    result, err := r.db.ExecContext(ctx, query, time.Now())
    if err != nil {
        return 0, err
    }

    count, _ := result.RowsAffected()
    return int(count), nil
}

// StateLock operations (similar implementation)
// ... (similar methods for StateLock)

// DTOs
type runLockDTO struct {
    LockID      string
    PID         int
    Hostname    string
    AcquiredAt  time.Time
    ExpiresAt   time.Time
    HeartbeatAt time.Time
    Metadata    string // JSON
}

func (r *LockRepositoryImpl) runLockDTOToDomain(dto runLockDTO) (*lock.RunLock, error) {
    // Convert DTO to domain model
    // ...
}
```

#### 7.5 Lock Manager Domain Service

**ファイル**: `internal/domain/service/lock_manager.go`
```go
package service

import (
    "context"
    "fmt"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// LockManager provides high-level lock management operations
type LockManager struct {
    lockRepo repository.LockRepository
}

// NewLockManager creates a new lock manager
func NewLockManager(lockRepo repository.LockRepository) *LockManager {
    return &LockManager{lockRepo: lockRepo}
}

// AcquireRunLock acquires a run lock with retry
func (m *LockManager) AcquireRunLock(ctx context.Context, resourceID string, ttl time.Duration) (*lock.RunLock, error) {
    lockID, err := lock.NewLockID(resourceID)
    if err != nil {
        return nil, err
    }

    runLock, err := lock.NewRunLock(lockID, ttl)
    if err != nil {
        return nil, err
    }

    // Try to acquire with retries
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err = m.lockRepo.AcquireRunLock(ctx, runLock)
        if err == nil {
            return runLock, nil
        }

        // Wait before retry
        time.Sleep(time.Second * time.Duration(i+1))
    }

    return nil, fmt.Errorf("failed to acquire run lock after %d retries: %w", maxRetries, err)
}

// ReleaseRunLock releases a run lock
func (m *LockManager) ReleaseRunLock(ctx context.Context, resourceID string) error {
    lockID, err := lock.NewLockID(resourceID)
    if err != nil {
        return err
    }

    return m.lockRepo.ReleaseRunLock(ctx, lockID)
}

// StartHeartbeat starts a heartbeat goroutine for a lock
func (m *LockManager) StartHeartbeat(ctx context.Context, resourceID string, interval time.Duration) error {
    lockID, err := lock.NewLockID(resourceID)
    if err != nil {
        return err
    }

    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                _ = m.lockRepo.UpdateRunLockHeartbeat(context.Background(), lockID)
            }
        }
    }()

    return nil
}

// CleanExpiredLocks removes expired locks
func (m *LockManager) CleanExpiredLocks(ctx context.Context) error {
    runCount, err := m.lockRepo.CleanExpiredRunLocks(ctx)
    if err != nil {
        return fmt.Errorf("clean run locks: %w", err)
    }

    stateCount, err := m.lockRepo.CleanExpiredStateLocks(ctx)
    if err != nil {
        return fmt.Errorf("clean state locks: %w", err)
    }

    fmt.Printf("Cleaned %d run locks and %d state locks\n", runCount, stateCount)
    return nil
}
```

### 成果物
- [ ] Lock SQLiteスキーマ設計完了
- [ ] RunLock/StateLock Domain Models実装
- [ ] Lock Repository SQLite実装完了
- [ ] Lock Manager Domain Service実装
- [ ] Heartbeat監視機能実装
- [ ] 既存ファイルベースLockからの移行
- [ ] ユニットテスト (カバレッジ > 80%)
- [ ] 統合テスト (Lock層)

---

## Phase 8: 統合・テスト・移行完了 (Week 9-10)

### 目標
- 全コンポーネントの統合テスト
- E2Eテストの実装
- 旧コードの削除
- ドキュメント更新
- パフォーマンス検証

### タスク

#### 8.1 統合テスト

**ファイル**: `test/integration/task_workflow_test.go`
```go
package integration_test

import (
    "context"
    "database/sql"
    "os"
    "testing"

    _ "github.com/mattn/go-sqlite3"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/YoshitsuguKoike/deespec/internal/application/dto"
    "github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
)

func TestTaskWorkflowIntegration(t *testing.T) {
    // Setup test database
    db, err := setupTestDB(t)
    require.NoError(t, err)
    defer db.Close()

    // Create DI container
    container, err := di.NewContainer("cli", ":memory:", "local")
    require.NoError(t, err)
    defer container.Close()

    ctx := context.Background()

    // Test: Create EPIC
    t.Run("Create EPIC", func(t *testing.T) {
        req := dto.CreateEPICRequest{
            Title:       "Test EPIC",
            Description: "Test description",
            Priority:    "high",
        }

        epic, err := container.TaskUseCase.CreateEPIC(ctx, req)
        require.NoError(t, err)
        assert.NotEmpty(t, epic.ID)
        assert.Equal(t, "Test EPIC", epic.Title)
    })

    // Test: Full workflow EPIC -> PBI -> SBI
    t.Run("Full Workflow", func(t *testing.T) {
        // 1. Create EPIC
        epicReq := dto.CreateEPICRequest{
            Title:       "User Authentication",
            Description: "Implement user authentication system",
        }
        epic, err := container.TaskUseCase.CreateEPIC(ctx, epicReq)
        require.NoError(t, err)

        // 2. Create PBI under EPIC
        pbiReq := dto.CreatePBIRequest{
            Title:        "Login API",
            Description:  "Implement login REST API",
            ParentEPICID: &epic.ID,
            StoryPoints:  5,
        }
        pbi, err := container.TaskUseCase.CreatePBI(ctx, pbiReq)
        require.NoError(t, err)

        // 3. Create SBI under PBI
        sbiReq := dto.CreateSBIRequest{
            Title:          "Login handler implementation",
            Description:    "Implement HTTP handler for login",
            ParentPBIID:    &pbi.ID,
            EstimatedHours: 2.0,
        }
        sbi, err := container.TaskUseCase.CreateSBI(ctx, sbiReq)
        require.NoError(t, err)

        // 4. Verify hierarchy
        retrievedEPIC, err := container.TaskUseCase.GetEPIC(ctx, epic.ID)
        require.NoError(t, err)
        assert.Equal(t, 1, retrievedEPIC.PBICount)

        retrievedPBI, err := container.TaskUseCase.GetPBI(ctx, pbi.ID)
        require.NoError(t, err)
        assert.Equal(t, 1, retrievedPBI.SBICount)

        // 5. Test workflow operations
        err = container.WorkflowUseCase.PickTask(ctx, sbi.ID)
        require.NoError(t, err)

        retrievedSBI, err := container.TaskUseCase.GetSBI(ctx, sbi.ID)
        require.NoError(t, err)
        assert.Equal(t, "picked", retrievedSBI.Status)
    })
}

func setupTestDB(t *testing.T) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        return nil, err
    }

    // Run migrations
    migrator := sqlitePersistence.NewMigrator(db)
    if err := migrator.Migrate(); err != nil {
        db.Close()
        return nil, err
    }

    return db, nil
}
```

#### 8.2 E2Eテスト

**ファイル**: `test/e2e/cli_e2e_test.go`
```go
package e2e_test

import (
    "bytes"
    "os"
    "os/exec"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCLIE2E(t *testing.T) {
    // Build CLI binary
    buildCmd := exec.Command("go", "build", "-o", "deespec-test", "../../cmd/deespec")
    err := buildCmd.Run()
    require.NoError(t, err)
    defer os.Remove("deespec-test")

    // Test: Create EPIC
    t.Run("Create EPIC via CLI", func(t *testing.T) {
        var stdout bytes.Buffer
        cmd := exec.Command("./deespec-test", "epic", "create", "Test EPIC", "--description", "Test description")
        cmd.Stdout = &stdout
        err := cmd.Run()
        require.NoError(t, err)

        output := stdout.String()
        assert.Contains(t, output, "EPIC created")
        assert.Contains(t, output, "Test EPIC")
    })

    // Test: List tasks
    t.Run("List tasks via CLI", func(t *testing.T) {
        var stdout bytes.Buffer
        cmd := exec.Command("./deespec-test", "epic", "list")
        cmd.Stdout = &stdout
        err := cmd.Run()
        require.NoError(t, err)

        output := stdout.String()
        assert.Contains(t, output, "Test EPIC")
    })

    // Test: JSON output format
    t.Run("JSON output format", func(t *testing.T) {
        var stdout bytes.Buffer
        cmd := exec.Command("./deespec-test", "epic", "list", "--format", "json")
        cmd.Stdout = &stdout
        err := cmd.Run()
        require.NoError(t, err)

        output := stdout.String()
        assert.Contains(t, output, `"success":`)
        assert.Contains(t, output, `"data":`)
    })
}
```

#### 8.3 旧コード削除

**削除対象**:
```bash
# 旧CLIコード (40ファイル → 新5ファイルへ統合済み)
rm -rf internal/interface/cli/*.go  # 既存CLI削除

# app/パッケージの一部 (ドメイン/インフラ層へ移行済み)
# (必要なファイルのみ残す)

# runner/パッケージ (ドメインサービスへ移行済み)
rm -rf internal/runner/

# validator/パッケージ (ドメイン層へ移行済み)
rm -rf internal/validator/

# workflow/パッケージ (ドメイン層へ移行済み)
rm -rf internal/workflow/

# 旧ファイルベースLock実装
rm internal/interface/cli/runlock.go  # → SQLite実装へ移行
```

#### 8.4 パフォーマンス検証

**ベンチマーク**:
```bash
# Repository操作
go test -bench=BenchmarkSQLiteRepository -benchmem ./internal/infrastructure/persistence/sqlite/

# Use Case操作
go test -bench=BenchmarkTaskUseCase -benchmem ./internal/application/usecase/task/

# E2E操作
go test -bench=BenchmarkCLICommand -benchmem ./test/e2e/
```

**期待値**:
- Repository操作: < 10ms
- Use Case操作: < 50ms
- CLI操作 (E2E): < 200ms
- メモリ使用量: ±10%以内

#### 8.5 ドキュメント更新

**更新対象**:
- `README.md`: 新しいCLI使用方法
- `docs/architecture/`: 最終アーキテクチャ図
- `docs/api/`: Use Case API仕様
- `CHANGELOG.md`: リファクタリング完了記録

### 成果物
- [ ] 統合テスト実装完了 (カバレッジ > 70%)
- [ ] E2Eテスト実装完了
- [ ] 旧コード完全削除
- [ ] パフォーマンスベンチマーク完了
- [ ] ドキュメント更新完了
- [ ] リファクタリング完了宣言

---

## 4. リスク管理

### 4.1 主要リスクと対策

| リスク | 影響度 | 発生確率 | 対策 |
|--------|--------|----------|------|
| 既存機能の破壊 | 高 | 中 | - 包括的な統合テスト<br>- 段階的リリース<br>- 機能フラグによる切り替え |
| パフォーマンス劣化 | 中 | 低 | - ベンチマークテスト<br>- プロファイリング<br>- 最適化フェーズ |
| 移行期間の長期化 | 中 | 中 | - 明確なマイルストーン<br>- 週次進捗確認<br>- スコープ調整 |
| チーム学習コスト | 低 | 高 | - ペアプログラミング<br>- コードレビュー<br>- 内部勉強会 |
| 依存関係の循環 | 高 | 低 | - 依存関係グラフの可視化<br>- 静的解析ツール使用 |

### 4.2 品質ゲート

各フェーズ完了条件:

**Phase 1-2 (ドメイン層)**:
- [ ] 単体テストカバレッジ > 90%
- [ ] 循環依存なし(lintチェック)
- [ ] ドメインロジックがインフラに依存していない

**Phase 3-4 (アプリケーション・アダプター層)**:
- [ ] ユースケーステストカバレッジ > 80%
- [ ] モックを使った独立テスト実施
- [ ] CLIコマンドのE2Eテスト通過

**Phase 5-6 (インフラ層・統合)**:
- [ ] 統合テストカバレッジ > 70%
- [ ] 全既存テストが通過
- [ ] パフォーマンス劣化なし(±5%)

## 5. 成功指標

### 5.1 定量指標

- **テストカバレッジ**: 全体で80%以上
- **ファイル数削減**: CLIレイヤー 40 → 5ファイル (87.5%削減)
- **循環依存**: 0件
- **パフォーマンス**: 旧実装と同等(±5%)
- **ビルド時間**: 変化なし

### 5.2 定性指標

- **可読性**: 新規メンバーが1週間でドメインロジックを理解できる
- **テスタビリティ**: 各層が独立してテスト可能
- **拡張性**: 新機能追加時の変更範囲が明確
- **保守性**: バグ修正時の影響範囲が限定的

## 6. 次ステップ(リファクタリング後)

### 6.1 SQLite移行準備

リファクタリング完了後、既存のアーキテクチャ設計書に基づいてSQLite移行を実施:

**参考**: `docs/architecture/sqlite-migration-strategy.md`

主な作業:
1. SQLiteリポジトリ実装(`internal/infrastructure/persistence/sqlite/`)
2. マイグレーションツール実装
3. ファイルベースとの並行稼働
4. パフォーマンス比較・検証

### 6.2 API層追加

Clean Architectureの利点を活かし、CLI以外のインターフェース追加:

```
internal/adapter/
├── controller/
│   ├── cli/        # 既存
│   ├── rest/       # 新規: REST APIコントローラー
│   └── grpc/       # 新規: gRPCコントローラー
```

### 6.3 統一タスクモデル設計 (EPIC/PBI/SBI)

リファクタリング完了後、タスク階層を統一的に扱う設計を導入:

#### 6.3.1 タスク階層の定義

**階層構造**:
```
EPIC (Epic: 大規模機能群)
  ├── PBI (Product Backlog Item: 中規模タスク)
  │   ├── SBI (Spec Backlog Item: 小規模タスク)
  │   ├── SBI
  │   └── SBI
  ├── PBI
  │   └── SBI
  └── ...

※ PBI/SBI は親タスクなしで独立して存在することも可能
```

**重要な設計原則**:
- EPIC/PBI/SBI は全て「タスク」という抽象概念
- 親タスクはオプショナル (nil 許容)
- 共通ワークフロー: `pick → implement → review → done`
- `implement` ステップの振る舞いがタスク種別で異なる

#### 6.3.2 Task インターフェース (統一抽象化)

```go
// domain/model/task/task.go
package task

// Task は EPIC/PBI/SBI の共通抽象インターフェース
type Task interface {
    ID() TaskID
    Type() TaskType
    Title() string
    Status() Status
    CurrentStep() Step

    // ワークフロー操作
    CanTransitionTo(step Step) bool
    TransitionTo(step Step) error

    // 親タスク関係 (オプショナル)
    ParentTask() *TaskID  // nil の場合は親なし
}

type TaskType string

const (
    TaskTypeEPIC TaskType = "EPIC"
    TaskTypePBI  TaskType = "PBI"
    TaskTypeSBI  TaskType = "SBI"
)

type Step string

const (
    StepPick      Step = "pick"
    StepImplement Step = "implement"
    StepReview    Step = "review"
    StepDone      Step = "done"
)

type TaskID string
```

#### 6.3.3 EPIC ドメインモデル

**目的**: 大規模な機能群を管理し、複数PBIに分解

**ドメインモデル**:
```go
// domain/model/epic/epic.go
package epic

type EPIC struct {
    id          EPICID
    title       Title
    description Description
    status      Status
    currentStep Step

    // 親タスク (EPICは通常最上位なのでnil)
    parentTask  *task.TaskID

    // EPIC固有フィールド
    linkedPBIs  []pbi.PBIID      // 配下のPBI一覧
    vision      Vision           // 全体ビジョン
    createdAt   time.Time
    updatedAt   time.Time
}

// Task インターフェース実装
func (e *EPIC) ID() task.TaskID {
    return task.TaskID(e.id)
}

func (e *EPIC) Type() task.TaskType {
    return task.TaskTypeEPIC
}

func (e *EPIC) ParentTask() *task.TaskID {
    return nil  // EPICは最上位
}

func (e *EPIC) CurrentStep() Step {
    return e.currentStep
}

func (e *EPIC) TransitionTo(step Step) error {
    if !e.CanTransitionTo(step) {
        return ErrInvalidTransition
    }
    e.currentStep = step
    e.updatedAt = time.Now()
    return nil
}
```

**ユースケース例**:
- `RegisterEPICUseCase`: EPICを登録
- `DecomposeEPICUseCase`: EPICからPBIを自動生成 (Agent使用)
- `TrackEPICProgressUseCase`: EPIC配下のPBI進捗を追跡

#### 6.3.4 PBI ドメインモデル

**目的**: 中規模タスクを管理し、複数SBIに分解

**ドメインモデル**:
```go
// domain/model/pbi/pbi.go
package pbi

type PBI struct {
    id                  PBIID
    title               Title
    description         Description
    status              Status
    currentStep         Step

    // 親タスク (オプショナル: EPIC または nil)
    parentTask          *task.TaskID

    // PBI固有フィールド
    linkedSBIs          []sbi.SBIID              // 配下のSBI一覧
    acceptanceCriteria  []AcceptanceCriteria     // 受け入れ基準
    businessValue       BusinessValue            // ビジネス価値
    createdAt           time.Time
    updatedAt           time.Time
}

// Task インターフェース実装
func (p *PBI) ID() task.TaskID {
    return task.TaskID(p.id)
}

func (p *PBI) Type() task.TaskType {
    return task.TaskTypePBI
}

func (p *PBI) ParentTask() *task.TaskID {
    return p.parentTask  // EPICのID または nil
}

func (p *PBI) CurrentStep() Step {
    return p.currentStep
}

// AcceptanceCriteria値オブジェクト
type AcceptanceCriteria struct {
    id          string
    description string
    verified    bool
}

// BusinessValue値オブジェクト
type BusinessValue struct {
    score    int    // 1-100
    priority Priority // HIGH/MEDIUM/LOW
}
```

**ユースケース例**:
- `RegisterPBIUseCase`: PBIを登録
- `DecomposePBIUseCase`: PBIを複数SBIに自動分解 (Agent使用)
- `TrackPBIProgressUseCase`: PBI配下のSBIの進捗を追跡

#### 6.3.5 SBI ドメインモデル (更新)

**目的**: 小規模タスク (実装の最小単位)

**ドメインモデル**:
```go
// domain/model/sbi/sbi.go
package sbi

type SBI struct {
    id          SBIID
    title       Title
    body        Body
    status      Status
    currentStep Step

    // 親タスク (オプショナル: PBI または nil)
    parentTask  *task.TaskID

    // SBI固有フィールド
    labels      []Label
    createdAt   time.Time
    updatedAt   time.Time
}

// Task インターフェース実装
func (s *SBI) ID() task.TaskID {
    return task.TaskID(s.id)
}

func (s *SBI) Type() task.TaskType {
    return task.TaskTypeSBI
}

func (s *SBI) ParentTask() *task.TaskID {
    return s.parentTask  // PBIのID または nil
}

func (s *SBI) CurrentStep() Step {
    return s.currentStep
}
```

**ユースケース例**:
- `RegisterSBIUseCase`: SBIを登録
- `ImplementSBIUseCase`: SBIのコード生成 (Agent使用)
- `ReviewSBIUseCase`: SBIのレビュー実行

**リポジトリ実装**:
```
internal/infrastructure/persistence/file/
├── epic_repository_impl.go  # .deespec/specs/epic/<EPIC-ID>/
├── pbi_repository_impl.go   # .deespec/specs/pbi/<PBI-ID>/
└── sbi_repository_impl.go   # .deespec/specs/sbi/<SBI-ID>/
```

#### 6.3.6 Implementation Strategy パターン

`implement` ステップの振る舞いをタスク種別ごとに切り替える設計:

```go
// domain/service/task_implementation_service.go
package service

type TaskImplementationService struct {
    strategies map[task.TaskType]ImplementationStrategy
}

type ImplementationStrategy interface {
    Execute(ctx context.Context, t task.Task, agent agent.Agent) (*ImplementationResult, error)
}

type ImplementationResult struct {
    Success   bool
    Artifacts interface{}  // 成果物 (PBI IDリスト / SBI IDリスト / ファイルパス等)
    Message   string
}
```

**EPIC の実装戦略: PBI分解生成**

```go
// domain/service/strategy/epic_decomposition_strategy.go
package strategy

type EPICDecompositionStrategy struct {
    pbiRepo       repository.PBIRepository
    agentGateway  port.AgentGateway
    storageGateway port.StorageGateway  // S3連携
}

func (s *EPICDecompositionStrategy) Execute(
    ctx context.Context,
    t task.Task,
    agent agent.Agent,
) (*ImplementationResult, error) {
    epic := t.(*epic.EPIC)

    // 1. EPIC仕様書読み込み (ローカル or S3)
    spec, _ := s.loadSpecification(ctx, epic)

    // 2. Agentにプロンプト送信
    prompt := BuildDecomposePrompt(epic, spec)
    resp, _ := agent.Execute(ctx, AgentRequest{Prompt: prompt})

    // 3. Agent応答からPBI定義を抽出 (JSON形式)
    pbiDefs := ParsePBIDefinitionsFromJSON(resp.Output)

    // 4. PBIエンティティ作成 & 保存
    createdPBIs := []pbi.PBIID{}
    for _, def := range pbiDefs {
        pbi := pbi.NewPBI(def.Title, def.Description, epic.ID())
        _ = s.pbiRepo.Save(ctx, pbi)
        createdPBIs = append(createdPBIs, pbi.ID())

        // PBI仕様をS3に保存 (オプション)
        if s.storageGateway != nil {
            specContent := GeneratePBISpecification(pbi, def)
            _ = s.storageGateway.SaveArtifact(ctx, SaveArtifactRequest{
                TaskID:       task.TaskID(pbi.ID()),
                ArtifactType: ArtifactTypeSpec,
                Content:      []byte(specContent),
            })
        }
    }

    return &ImplementationResult{
        Success:   true,
        Artifacts: createdPBIs,
        Message:   fmt.Sprintf("Created %d PBIs", len(createdPBIs)),
    }, nil
}
```

**PBI の実装戦略: SBI分解生成**

```go
// domain/service/strategy/pbi_decomposition_strategy.go
package strategy

type PBIDecompositionStrategy struct {
    sbiRepo       repository.SBIRepository
    agentGateway  port.AgentGateway
}

func (s *PBIDecompositionStrategy) Execute(
    ctx context.Context,
    t task.Task,
    agent agent.Agent,
) (*ImplementationResult, error) {
    pbi := t.(*pbi.PBI)

    // 1. PBI仕様書読み込み
    spec, _ := s.loadSpecification(ctx, pbi)

    // 2. Agentにプロンプト送信
    prompt := BuildDecomposePrompt(pbi, spec)
    resp, _ := agent.Execute(ctx, AgentRequest{Prompt: prompt})

    // 3. SBI定義を抽出
    sbiDefs := ParseSBIDefinitionsFromJSON(resp.Output)

    // 4. SBIエンティティ作成 & 保存
    // また、deespec sbi register コマンドを内部的に実行
    createdSBIs := []sbi.SBIID{}
    for _, def := range sbiDefs {
        sbi := sbi.NewSBI(def.Title, def.Body, pbi.ID())
        _ = s.sbiRepo.Save(ctx, sbi)
        createdSBIs = append(createdSBIs, sbi.ID())
    }

    return &ImplementationResult{
        Success:   true,
        Artifacts: createdSBIs,
        Message:   fmt.Sprintf("Created %d SBIs", len(createdSBIs)),
    }, nil
}
```

**SBI の実装戦略: コード生成**

```go
// domain/service/strategy/sbi_code_generation_strategy.go
package strategy

type SBICodeGenerationStrategy struct {
    agentGateway port.AgentGateway
    fileWriter   port.FileWriter
}

func (s *SBICodeGenerationStrategy) Execute(
    ctx context.Context,
    t task.Task,
    agent agent.Agent,
) (*ImplementationResult, error) {
    sbi := t.(*sbi.SBI)

    // 1. コード生成プロンプト
    prompt := BuildCodeGenerationPrompt(sbi)
    resp, _ := agent.Execute(ctx, AgentRequest{Prompt: prompt})

    // 2. 生成されたコードをファイルに書き込み
    files := ExtractCodeFiles(resp.Output)
    for _, file := range files {
        _ = s.fileWriter.Write(file.Path, file.Content)
    }

    return &ImplementationResult{
        Success:   true,
        Artifacts: files,
        Message:   fmt.Sprintf("Generated %d files", len(files)),
    }, nil
}
```

**統一ワークフローユースケース**

```go
// application/usecase/task/run_task_workflow.go
package task

type RunTaskWorkflowUseCase struct {
    taskImplService *service.TaskImplementationService
    agentSelection  *service.AgentSelectionService
    taskRepo        repository.TaskRepository  // 統一リポジトリ
}

func (uc *RunTaskWorkflowUseCase) Execute(
    ctx context.Context,
    input RunTaskInput,
) (*RunTaskOutput, error) {

    // 1. タスク取得 (EPIC/PBI/SBI いずれか)
    task, _ := uc.taskRepo.FindByID(ctx, input.TaskID)

    // 2. 現在のステップに応じて処理分岐
    switch task.CurrentStep() {
    case Step.StepPick:
        // Pick処理 (タスク選択)

    case Step.StepImplement:
        // Agent選択 (タスク種別とタスク設定に基づく)
        agent, _ := uc.agentSelection.SelectAgentForTask(task)

        // タスク種別に応じた実装戦略を実行
        result, _ := uc.taskImplService.Execute(ctx, task, agent)

        // 成果物を記録
        task.RecordArtifacts(result.Artifacts)

        // 次ステップへ遷移
        task.TransitionTo(Step.StepReview)

        // 保存
        _ = uc.taskRepo.Save(ctx, task)

    case Step.StepReview:
        // Review処理 (Agentによるレビュー)

    case Step.StepDone:
        // 完了処理
    }

    return &RunTaskOutput{
        TaskID:      task.ID(),
        CurrentStep: task.CurrentStep(),
    }, nil
}
```

### 6.4 マルチエージェント対応戦略

複数のAIエージェント(Claude Code, Gemini CLI, Codex)をタスク単位で切り替え可能にする設計:

#### 6.4.1 エージェント抽象化

**ドメイン層のエージェントモデル**:
```go
// domain/model/agent/agent_type.go
package agent

type AgentType string

const (
    AgentTypeClaude AgentType = "claude"
    AgentTypeGemini AgentType = "gemini"
    AgentTypeCodex  AgentType = "codex"
)

// domain/model/agent/agent.go
type Agent struct {
    id          AgentID
    agentType   AgentType
    name        string
    capability  Capability   // エージェントの能力(コード生成/レビュー/テスト等)
    config      Config       // APIキー、バイナリパス等の設定
    isAvailable bool
}

// Capability値オブジェクト
type Capability struct {
    supportsCodeGeneration bool
    supportsReview         bool
    supportsTest           bool
    maxPromptSize          int
    concurrentTasks        int
}
```

#### 6.4.2 タスク単位のエージェント選択

**ドメインサービス**:
```go
// domain/service/agent_selection_service.go
package service

type AgentSelectionService struct {
    agentRepo repository.AgentRepository
}

// SelectAgentForTask はタスクの特性に基づいて最適なエージェントを選択
func (s *AgentSelectionService) SelectAgentForTask(
    task execution.Task,
    availableAgents []agent.Agent,
) (*agent.Agent, error) {

    // タスクの要求に基づいてフィルタリング
    candidates := s.filterByCapability(task, availableAgents)

    // 優先順位: 1) タスク指定 2) ステップ設定 3) デフォルト
    if task.PreferredAgent() != nil {
        return task.PreferredAgent(), nil
    }

    // ラウンドロビン or 負荷分散
    return s.selectByLoadBalancing(candidates)
}

// filterByCapability はタスクに必要な能力を持つエージェントのみを抽出
func (s *AgentSelectionService) filterByCapability(
    task execution.Task,
    agents []agent.Agent,
) []agent.Agent {

    var candidates []agent.Agent
    for _, a := range agents {
        if task.RequiresCodeGeneration() && !a.Capability().SupportsCodeGeneration() {
            continue
        }
        if task.RequiresReview() && !a.Capability().SupportsReview() {
            continue
        }
        candidates = append(candidates, a)
    }
    return candidates
}
```

**SBI/Executionへのエージェント指定追加**:
```go
// domain/model/sbi/sbi.go
type SBI struct {
    // ... existing fields
    preferredAgent *agent.AgentType  // タスク毎に優先エージェントを指定可能
}

// domain/model/execution/execution.go
type Execution struct {
    // ... existing fields
    assignedAgent agent.AgentType  // 実行時に割り当てられたエージェント
}
```

#### 6.4.3 ゲートウェイ実装

**ポート定義** (application/port/output/agent_gateway.go):
```go
package output

type AgentGateway interface {
    Execute(ctx context.Context, req AgentRequest) (*AgentResponse, error)
    GetCapability() agent.Capability
    HealthCheck(ctx context.Context) error
}

type AgentRequest struct {
    Prompt      string
    Timeout     time.Duration
    Context     map[string]string  // 追加コンテキスト
    MaxTokens   int                // 最大トークン数
}

type AgentResponse struct {
    Output      string
    ExitCode    int
    Duration    time.Duration
    TokensUsed  int                // 使用トークン数
    AgentType   agent.AgentType    // 実行したエージェント種別
}
```

**各エージェントゲートウェイ実装**:

```go
// adapter/gateway/agent/claude_gateway.go
package agent

type ClaudeGateway struct {
    binaryPath string
    timeout    time.Duration
}

func (g *ClaudeGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
    cmd := exec.CommandContext(ctx, g.binaryPath, "code", "--prompt", req.Prompt)
    // Claude Code CLI実行ロジック
    // ...
    return &output.AgentResponse{
        Output:    string(out),
        AgentType: agent.AgentTypeClaude,
        // ...
    }, nil
}

func (g *ClaudeGateway) GetCapability() agent.Capability {
    return agent.Capability{
        SupportsCodeGeneration: true,
        SupportsReview:         true,
        SupportsTest:           true,
        MaxPromptSize:          200000, // 200KB
        ConcurrentTasks:        5,
    }
}
```

```go
// adapter/gateway/agent/gemini_gateway.go
package agent

type GeminiGateway struct {
    binaryPath string
    apiKey     string
    timeout    time.Duration
}

func (g *GeminiGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
    cmd := exec.CommandContext(ctx, g.binaryPath, "generate", "--prompt", req.Prompt)
    cmd.Env = append(cmd.Env, "GEMINI_API_KEY="+g.apiKey)
    // Gemini CLI実行ロジック
    // ...
    return &output.AgentResponse{
        Output:    string(out),
        AgentType: agent.AgentTypeGemini,
        // ...
    }, nil
}

func (g *GeminiGateway) GetCapability() agent.Capability {
    return agent.Capability{
        SupportsCodeGeneration: true,
        SupportsReview:         true,
        SupportsTest:           false,
        MaxPromptSize:          100000, // 100KB
        ConcurrentTasks:        10,
    }
}
```

```go
// adapter/gateway/agent/codex_gateway.go
package agent

type CodexGateway struct {
    apiEndpoint string
    apiKey      string
    httpClient  *http.Client
}

func (g *CodexGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
    // Codex API呼び出しロジック
    payload := map[string]interface{}{
        "prompt":      req.Prompt,
        "max_tokens":  req.MaxTokens,
        "temperature": 0.2,
    }

    // HTTP POST to Codex API
    // ...

    return &output.AgentResponse{
        Output:     result.Choices[0].Text,
        AgentType:  agent.AgentTypeCodex,
        TokensUsed: result.Usage.TotalTokens,
        // ...
    }, nil
}

func (g *CodexGateway) GetCapability() agent.Capability {
    return agent.Capability{
        SupportsCodeGeneration: true,
        SupportsReview:         false,
        SupportsTest:           false,
        MaxPromptSize:          8000,   // 8KB (OpenAI limit)
        ConcurrentTasks:        20,
    }
}
```

**ファクトリーパターンでのゲートウェイ生成**:
```go
// adapter/gateway/agent/agent_factory.go
package agent

type AgentFactory struct {
    config *config.Config
}

func (f *AgentFactory) CreateGateway(agentType agent.AgentType) (output.AgentGateway, error) {
    switch agentType {
    case agent.AgentTypeClaude:
        return &ClaudeGateway{
            binaryPath: f.config.ClaudeBinaryPath(),
            timeout:    f.config.Timeout(),
        }, nil

    case agent.AgentTypeGemini:
        return &GeminiGateway{
            binaryPath: f.config.GeminiBinaryPath(),
            apiKey:     f.config.GeminiAPIKey(),
            timeout:    f.config.Timeout(),
        }, nil

    case agent.AgentTypeCodex:
        return &CodexGateway{
            apiEndpoint: f.config.CodexAPIEndpoint(),
            apiKey:      f.config.CodexAPIKey(),
            httpClient:  &http.Client{Timeout: f.config.Timeout()},
        }, nil

    default:
        return nil, fmt.Errorf("unsupported agent type: %s", agentType)
    }
}

func (f *AgentFactory) CreateAllAvailableGateways() ([]output.AgentGateway, error) {
    var gateways []output.AgentGateway

    for _, agentType := range []agent.AgentType{
        agent.AgentTypeClaude,
        agent.AgentTypeGemini,
        agent.AgentTypeCodex,
    } {
        gw, err := f.CreateGateway(agentType)
        if err != nil {
            continue // スキップ
        }

        // ヘルスチェック
        if err := gw.HealthCheck(context.Background()); err == nil {
            gateways = append(gateways, gw)
        }
    }

    return gateways, nil
}
```

#### 6.4.4 ユースケースでのエージェント選択統合

```go
// application/usecase/execution/run_turn.go
type RunTurnUseCase struct {
    // ... existing fields
    agentFactory      *gateway.AgentFactory
    agentSelection    *service.AgentSelectionService
}

func (uc *RunTurnUseCase) Execute(ctx context.Context, input RunTurnInput) (*RunTurnOutput, error) {
    return uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
        // 1. 実行状態とSBI取得
        exec, _ := uc.execRepo.FindCurrentBySBIID(txCtx, input.SBIID)
        sbiEntity, _ := uc.sbiRepo.Find(txCtx, input.SBIID)

        // 2. 利用可能なエージェント一覧取得
        availableGateways, _ := uc.agentFactory.CreateAllAvailableGateways()

        // 3. タスクに最適なエージェントを選択
        selectedAgent, err := uc.agentSelection.SelectAgentForTask(
            exec.CurrentTask(),
            availableGateways,
        )
        if err != nil {
            return err
        }

        // 4. 選択されたエージェントで実行
        agentResp, err := selectedAgent.Execute(txCtx, output.AgentRequest{
            Prompt:  exec.GeneratePrompt(sbiEntity),
            Timeout: input.Timeout,
        })

        // 5. 実行結果を記録(どのエージェントが使われたかも記録)
        exec.RecordAgentExecution(selectedAgent.GetCapability().AgentType, agentResp)

        // ... rest of the logic
    })
}
```

#### 6.4.5 設定ファイルでのエージェント管理

**setting.json 拡張**:
```json
{
  "home": ".deespec",
  "agents": {
    "claude": {
      "enabled": true,
      "binary_path": "/usr/local/bin/claude",
      "timeout_sec": 60,
      "priority": 1
    },
    "gemini": {
      "enabled": true,
      "binary_path": "/usr/local/bin/gemini",
      "api_key_env": "GEMINI_API_KEY",
      "timeout_sec": 45,
      "priority": 2
    },
    "codex": {
      "enabled": false,
      "api_endpoint": "https://api.openai.com/v1/completions",
      "api_key_env": "OPENAI_API_KEY",
      "timeout_sec": 30,
      "priority": 3
    }
  },
  "agent_selection_strategy": "priority"  // priority | round_robin | load_balancing
}
```

**タスク定義でのエージェント指定**:
```yaml
# .deespec/specs/sbi/SBI-xxxxx/config.yml
id: SBI-xxxxx
title: "Implement user authentication"
preferred_agent: "claude"  # このタスクではClaudeを優先使用
steps:
  - id: implement
    agent: "gemini"     # このステップだけGeminiを使用
  - id: review
    agent: "claude"     # レビューはClaudeを使用
  - id: test
    # agent未指定の場合は、agent_selection_strategyに従う
```

### 6.5 S3 ストレージ統合設計

将来的にS3や外部サイトと連携し、成果物をS3に保存したり、S3から指示書を取得する機能を追加します。詳細な実装例はセクション6.5.1-6.5.5を参照してください。

**主な機能**:
- S3への成果物保存 (仕様書、生成コード、データ)
- S3からの指示書読み込み
- ハイブリッドストレージ (ローカル + S3)
- フォールバック機構

### 6.6 プラグインアーキテクチャ

外部拡張を可能にするプラグイン機構:
- カスタムバリデーター
- 外部エージェント統合(上記以外のAIモデル)
- カスタムワークフローステップ
- カスタムストレージバックエンド (Google Cloud Storage, Azure Blob 等)

## 7. まとめ

このリファクタリング計画により、DeeSpecは以下を達成します:

1. **明確な責務分離**: 各層が単一責任を持つ
2. **高いテスタビリティ**: モックを使った独立テスト
3. **柔軟な拡張性**: SQLite移行、API追加が容易
4. **保守性の向上**: ビジネスロジックの一元管理
5. **チーム生産性**: 新メンバーの立ち上がりが早い

**推定工数**: 6-7週間 (1人フルタイム換算)
**推奨アプローチ**: 週1-2フェーズのペースで段階的に実施

---

## Appendix A: ディレクトリ構造全体像

```
deespec/
├── cmd/
│   └── deespec/
│       └── main.go                    # エントリーポイント(DIコンテナ初期化)
├── internal/
│   ├── domain/                        # 【第1層】ドメイン層
│   │   ├── model/                     # ドメインモデル
│   │   │   ├── sbi/
│   │   │   │   ├── sbi.go            # SBI集約ルート
│   │   │   │   ├── sbi_id.go         # SBIID値オブジェクト
│   │   │   │   ├── title.go          # Title値オブジェクト
│   │   │   │   ├── body.go           # Body値オブジェクト
│   │   │   │   ├── labels.go         # Labels値オブジェクト
│   │   │   │   └── status.go         # Status値オブジェクト
│   │   │   ├── epic/                 # 【将来追加】
│   │   │   │   ├── epic.go           # EPIC集約ルート
│   │   │   │   ├── epic_id.go        # EPICID値オブジェクト
│   │   │   │   ├── component_type.go # ComponentType値オブジェクト
│   │   │   │   └── dependency.go     # Dependency値オブジェクト
│   │   │   ├── pbi/                  # 【将来追加】
│   │   │   │   ├── pbi.go            # PBI集約ルート
│   │   │   │   ├── pbi_id.go         # PBIID値オブジェクト
│   │   │   │   ├── epic.go           # Epic値オブジェクト
│   │   │   │   └── acceptance_criteria.go # AcceptanceCriteria値オブジェクト
│   │   │   ├── execution/
│   │   │   │   ├── execution.go      # Execution集約ルート
│   │   │   │   ├── execution_id.go   # ExecutionID値オブジェクト
│   │   │   │   ├── turn.go           # Turn値オブジェクト
│   │   │   │   ├── attempt.go        # Attempt値オブジェクト
│   │   │   │   ├── step.go           # Step値オブジェクト
│   │   │   │   ├── decision.go       # Decision値オブジェクト
│   │   │   │   └── status.go         # ExecutionStatus値オブジェクト
│   │   │   ├── workflow/
│   │   │   │   ├── workflow.go       # Workflow集約ルート
│   │   │   │   ├── step_config.go    # StepConfig値オブジェクト
│   │   │   │   └── constraints.go    # Constraints値オブジェクト
│   │   │   ├── agent/                # 【将来追加】
│   │   │   │   ├── agent.go          # Agent集約ルート
│   │   │   │   ├── agent_type.go     # AgentType値オブジェクト
│   │   │   │   ├── capability.go     # Capability値オブジェクト
│   │   │   │   └── config.go         # Config値オブジェクト
│   │   │   ├── state/
│   │   │   │   ├── state.go          # State集約ルート
│   │   │   │   └── wip.go            # WIP値オブジェクト
│   │   │   └── journal/
│   │   │       └── journal_entry.go  # JournalEntryエンティティ
│   │   ├── service/                   # ドメインサービス
│   │   │   ├── step_transition_service.go
│   │   │   ├── review_service.go
│   │   │   ├── validation_service.go
│   │   │   ├── execution_service.go
│   │   │   └── agent_selection_service.go  # 【将来追加】
│   │   └── repository/                # リポジトリインターフェース
│   │       ├── sbi_repository.go
│   │       ├── epic_repository.go    # 【将来追加】
│   │       ├── pbi_repository.go     # 【将来追加】
│   │       ├── execution_repository.go
│   │       ├── state_repository.go
│   │       ├── workflow_repository.go
│   │       ├── agent_repository.go   # 【将来追加】
│   │       └── journal_repository.go
│   ├── application/                   # 【第2層】アプリケーション層
│   │   ├── usecase/
│   │   │   ├── sbi/
│   │   │   │   ├── register_sbi.go
│   │   │   │   ├── find_sbi.go
│   │   │   │   └── list_sbi.go
│   │   │   ├── epic/             # 【将来追加】
│   │   │   │   ├── register_epic.go
│   │   │   │   ├── link_epic_to_sbi.go
│   │   │   │   └── list_epic.go
│   │   │   ├── pbi/              # 【将来追加】
│   │   │   │   ├── register_pbi.go
│   │   │   │   ├── decompose_pbi.go
│   │   │   │   └── track_pbi_progress.go
│   │   │   ├── execution/
│   │   │   │   ├── run_turn.go
│   │   │   │   ├── run_sbi.go
│   │   │   │   └── get_execution_status.go
│   │   │   ├── workflow/
│   │   │   │   ├── load_workflow.go
│   │   │   │   └── validate_workflow.go
│   │   │   └── health/
│   │   │       └── check_health.go
│   │   ├── dto/
│   │   │   ├── sbi_dto.go
│   │   │   ├── epic_dto.go       # 【将来追加】
│   │   │   ├── pbi_dto.go        # 【将来追加】
│   │   │   ├── execution_dto.go
│   │   │   ├── workflow_dto.go
│   │   │   └── agent_dto.go      # 【将来追加】
│   │   ├── port/
│   │   │   ├── input/
│   │   │   │   └── usecase_interfaces.go
│   │   │   └── output/
│   │   │       ├── agent_gateway.go
│   │   │       ├── presenter.go
│   │   │       └── transaction.go
│   │   └── service/
│   │       ├── orchestrator.go
│   │       └── transaction_manager.go
│   ├── adapter/                       # 【第3層】アダプター層
│   │   ├── controller/
│   │   │   └── cli/
│   │   │       ├── sbi_controller.go
│   │   │       ├── execution_controller.go
│   │   │       ├── health_controller.go
│   │   │       └── doctor_controller.go
│   │   ├── presenter/
│   │   │   └── cli/
│   │   │       ├── execution_presenter.go
│   │   │       ├── health_presenter.go
│   │   │       └── json_presenter.go
│   │   └── gateway/
│   │       ├── agent/            # 【将来追加】
│   │       │   ├── claude_gateway.go
│   │       │   ├── gemini_gateway.go
│   │       │   ├── codex_gateway.go
│   │       │   └── agent_factory.go
│   │       └── filesystem_gateway.go
│   └── infrastructure/                # 【第4層】インフラ層
│       ├── persistence/
│       │   ├── file/
│       │   │   ├── sbi_repository_impl.go
│       │   │   ├── epic_repository_impl.go      # 【将来追加】
│       │   │   ├── pbi_repository_impl.go       # 【将来追加】
│       │   │   ├── execution_repository_impl.go
│       │   │   ├── state_repository_impl.go
│       │   │   ├── workflow_repository_impl.go
│       │   │   ├── agent_repository_impl.go     # 【将来追加】
│       │   │   └── journal_repository_impl.go
│       │   └── sqlite/                # 【将来】
│       │       └── (future)
│       ├── transaction/
│       │   ├── file_transaction.go
│       │   └── flock_manager.go
│       ├── config/
│       │   ├── loader.go
│       │   └── resolver.go
│       ├── logger/
│       │   └── logger.go
│       └── di/
│           └── container.go
├── docs/
│   └── architecture/
│       ├── clean-architecture-design.md      # 既存
│       ├── sqlite-migration-strategy.md      # 既存
│       └── refactoring-plan.md               # 本ドキュメント
└── README.md
```

## Appendix B: 依存関係グラフ

```
┌─────────────────────────────────────────────────────────────┐
│                     cmd/deespec/main.go                     │
│                   (エントリーポイント)                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │  infrastructure/di/    │
              │  container.go          │
              │  (DIコンテナ)          │
              └────────────┬───────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌──────────────┐  ┌─────────────────┐  ┌──────────────┐
│ adapter/     │  │ application/    │  │infrastructure│
│ controller/  │  │ usecase/        │  │ persistence/ │
│ cli/         │  └────────┬────────┘  │ file/        │
└──────┬───────┘           │           └──────┬───────┘
       │                   │                  │
       │                   ▼                  │
       │          ┌─────────────────┐         │
       │          │ domain/         │         │
       │          │ model/          │         │
       │          │ service/        │         │
       │          │ repository/     │         │
       │          └─────────────────┘         │
       │                   ▲                  │
       │                   │                  │
       └───────────────────┴──────────────────┘
                  (依存性逆転の原則)
```

## Appendix C: チェックリスト

### Phase 1: 基盤整備
- [ ] ディレクトリ構造作成
- [ ] リポジトリインターフェース定義
- [ ] ポートインターフェース定義
- [ ] テスト基盤構築

### Phase 2: ドメイン層構築
- [ ] Turn値オブジェクト実装
- [ ] Attempt値オブジェクト実装
- [ ] Step値オブジェクト実装
- [ ] Status値オブジェクト実装
- [ ] SBIID値オブジェクト実装
- [ ] SBIエンティティ実装
- [ ] Executionエンティティ実装
- [ ] StepTransitionService実装
- [ ] ReviewService実装
- [ ] ValidationService実装
- [ ] ドメイン層テスト(カバレッジ>90%)

### Phase 3: アプリケーション層構築
- [ ] RunTurnUseCase実装
- [ ] RegisterSBIUseCase実装
- [ ] FindSBIUseCase実装
- [ ] GetExecutionStatusUseCase実装
- [ ] CheckHealthUseCase実装
- [ ] DTO定義
- [ ] TransactionManager実装
- [ ] ユースケーステスト(カバレッジ>80%)

### Phase 4: アダプター層リファクタリング
- [ ] RunController実装
- [ ] SBIController実装
- [ ] HealthController実装
- [ ] ExecutionPresenter実装
- [ ] JSONPresenter実装
- [ ] ClaudeGateway実装
- [ ] 旧CLIコードの段階的削除
- [ ] アダプター層テスト

### Phase 5: インフラ層整理 - SQLite Repository実装
- [x] SQLiteスキーマ設計(epics, pbis, sbis, epic_pbis, pbi_sbis)
- [x] Migration system実装(go:embed schema.sql)
- [x] EPICRepositoryImpl実装
- [x] PBIRepositoryImpl実装
- [x] SBIRepositoryImpl実装
- [x] TaskRepositoryImpl実装(ポリモーフィックラッパー)
- [x] SQLiteTransactionManager実装
- [x] DIコンテナ更新(SQLite統合)
- [x] Transaction context propagation修正
- [x] Code duplication除去(TaskRepositoryImpl)
- [ ] 統合テスト(SQLite使用)

### Phase 6: Storage Gateway実装
- [ ] StorageGateway interface定義確認
- [ ] S3StorageGateway実装
  - [ ] Store/Retrieve/Delete/List実装
  - [ ] AWS SDK v2統合
  - [ ] Metadata管理
- [ ] LocalStorageGateway実装
  - [ ] ファイルシステムベース実装
  - [ ] パス管理とディレクトリ構造
- [ ] DIコンテナ更新(Storage Gateway統合)
- [ ] Artifact管理統合
- [ ] Storage Gateway unit tests
- [ ] Storage Gateway integration tests

### Phase 7: Lock System SQLite移行 ✅ COMPLETED
- [x] Lock SQLiteスキーマ設計
  - [x] run_locks テーブル
  - [x] state_locks テーブル
  - [x] インデックス追加
- [x] Domain Lock Models実装
  - [x] RunLock model (94 lines)
  - [x] StateLock model (93 lines)
  - [x] LockID value object (27 lines)
- [x] Lock Repository実装
  - [x] RunLockRepository interface & impl (280 lines, 72-86% coverage)
  - [x] StateLockRepository interface & impl (259 lines, 72-84% coverage)
- [x] Lock Service実装
  - [x] Acquire/Release/Extend機能
  - [x] Heartbeat機能 (automatic, 30s interval)
  - [x] 期限切れロック自動削除 (automatic, 60s interval)
- [x] DI Container統合 (Phase 7.2)
  - [x] Lock repositories registered
  - [x] Lock Service registered
  - [x] Start/Close lifecycle management
  - [x] 6 integration tests
- [x] 旧runlock.go置き換え (Phase 7.3)
  - [x] New `deespec lock` command implemented (328 lines)
  - [x] Old implementation marked as @deprecated
  - [x] Deprecation warnings added
  - [x] Migration path documented
- [x] Lock system unit tests (17 repository tests + 9 service tests = 26 tests)
- [x] Lock system integration tests (6 DI container tests)
- [x] Performance benchmarks (830 ops/sec @ M2 Pro)

**Phase 7 Final Stats:**
- Production code: 1,154 lines
- Test code: 1,326 lines (1.15x ratio)
- Total test cases: 32 (all passing)
- Test coverage: 75-84%
- Lock command: `deespec lock {list|cleanup|info}`
- Old command: `deespec cleanup-locks` (deprecated)

### Phase 8: 統合・テスト・移行完了
- [ ] 統合テスト実装
  - [ ] Task workflow integration tests
  - [ ] Repository integration tests
  - [ ] Transaction integration tests
  - [ ] Lock system integration tests
- [ ] E2Eテスト実装
  - [ ] CLI E2E tests
  - [ ] Full workflow E2E tests
- [ ] 旧コード削除
  - [ ] 旧CLI実装削除
  - [ ] 旧Repository実装削除
  - [ ] 使用されていないファイル削除
- [ ] パフォーマンス検証
  - [ ] ベンチマーク実行
  - [ ] メモリ使用量測定
  - [ ] N+1クエリ最適化(必要に応じて)
- [ ] ドキュメント更新
  - [ ] README更新
  - [ ] アーキテクチャ図更新
  - [ ] API仕様書更新
- [ ] 全テスト通過確認
- [ ] リリースノート作成

---

**Last Updated**: 2025-10-08 (Phase 7 完了 - Lock System SQLite移行)
**Version**: 1.2
**Author**: Claude Code + User

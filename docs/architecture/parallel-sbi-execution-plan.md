# 複数SBI並行処理実装計画

**作成日**: 2025-10-10
**ステータス**: Phase 2完了、Phase 3完了、Phase 1 (Step 1-1部分) 完了
**目標**: state.jsonベースの単一SBI処理から、SQLiteベースの複数SBI並行処理への移行

**完了済み機能**:
- ✅ 並行SBI実行（ParallelSBIWorkflowRunner）
- ✅ セマフォベースの並行数制御
- ✅ SBI単位のロック管理
- ✅ ファイル競合検出（ConflictDetector）
- ✅ Agent別の並行数制御（AgentPool + 設定ファイル対応）
- ✅ 包括的なテストスイート（32テスト）
- ✅ Dual-write無効化（DB-onlyモード）

---

## 目次

1. [現状分析](#現状分析)
2. [目標アーキテクチャ](#目標アーキテクチャ)
3. [前提条件の整備](#前提条件の整備)
4. [Phase 1: レガシー依存の除去](#phase-1-レガシー依存の除去)
5. [Phase 2: ワークフローの並行実行対応](#phase-2-ワークフローの並行実行対応)
6. [Phase 3: 高度な並行制御](#phase-3-高度な並行制御)
7. [実装チェックリスト](#実装チェックリスト)
8. [リスクと対策](#リスクと対策)

---

## 現状分析

### アーキテクチャの二重構造

現在、deespecには**2つの並行するデータ管理システム**が存在：

#### 1. レガシーシステム（ファイルベース）
- **データソース**: `.deespec/var/state.json`
- **管理対象**: 単一のWIP（Work In Progress）
- **ロック機構**: ファイルロック（`.deespec/var/state.lock`）
- **実行モデル**: 順次処理（1タスクずつ）

**state.jsonの構造**:
```json
{
  "version": 1,
  "status": "WIP",
  "turn": 3,
  "wip": "SBI-001",           // ← 単一SBIのみ
  "lease_expires_at": "...",
  "attempt": 2,
  "decision": "NEEDS_CHANGES"
}
```

#### 2. 新システム（データベースベース）
- **データソース**: SQLite (`~/.deespec/deespec.db`)
- **管理対象**: 複数のSBI、PBI、EPIC
- **ロック機構**: LockService（RunLock、StateLock）
- **実行モデル**: 並行処理可能（未実装）

**sbisテーブルの構造**:
```sql
CREATE TABLE sbis (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    current_turn INTEGER NOT NULL,
    current_attempt INTEGER NOT NULL,
    -- 各SBIが独立した実行状態を保持可能
);
```

### 問題点

1. **並行処理の制約**: `state.json`が単一WIPのため、複数SBI同時実行が不可能
2. **データ重複**: 同じ情報がstate.jsonとDBの両方に存在
3. **一貫性の問題**: 2つのシステム間で状態同期が困難
4. **データベースロック競合**: 実行ごとに新しいコンテナ→DB接続→マイグレーション試行

---

## 目標アーキテクチャ

### ビジョン

**単一データソース（SQLite）による複数SBI並行処理システム**

```
┌─────────────────────────────────────────────────────────┐
│ deespec run --workflows sbi --parallel 3                │
└─────────────────────────────────────────────────────────┘
                        ↓
        ┌───────────────┴───────────────┐
        │   WorkflowManager             │
        │   - 1つのDIコンテナを共有     │
        │   - 1つのDB接続を共有         │
        └───────────────┬───────────────┘
                        ↓
        ┌───────────────┴───────────────┐
        │   ParallelSBIWorkflowRunner   │
        │   - セマフォで並行数制御(3)   │
        │   - SBI単位のロック取得       │
        └───────────────┬───────────────┘
                        ↓
        ┌───────┬───────┼───────┐
        │       │       │       │
      SBI-001 SBI-002 SBI-003 SBI-004 (Queue)
        │       │       │
     ┌──┴──┐ ┌──┴──┐ ┌──┴──┐
     │Agent│ │Agent│ │Agent│  (Parallel Execution)
     └─────┘ └─────┘ └─────┘
        │       │       │
        └───────┴───────┴────────→ SQLite
                                   (Single Source of Truth)
```

### 主要な変更点

| 項目 | 現在 | 目標 |
|------|------|------|
| データソース | state.json + SQLite | SQLite のみ |
| 処理モデル | 順次処理 | 並行処理（最大N個） |
| コンテナ | 実行ごとに新規作成 | コマンド起動時に1つ作成 |
| DB接続 | 実行ごとに新規接続 | 1つの接続を共有 |
| ロック | ファイルロック | LockService（SBI単位） |
| 並行数制御 | なし | セマフォ + ロック |

---

## 前提条件の整備

### Step 0: データベースロック問題の解決

**目的**: 複数プロセス・並行処理に備えたDB基盤の強化

#### Step 0-1: Option1実装（コンテナの再利用）

**変更対象**: `internal/interface/cli/run/run.go`

**Before**:
```go
func RunTurn(autoFB bool) error {
    // 実行のたびに新しいコンテナを作成
    container, err := common.InitializeContainer()
    if err != nil {
        return fmt.Errorf("failed to initialize container: %w", err)
    }
    defer container.Close()

    // 処理...
}
```

**After**:
```go
func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            // コマンド起動時に1回だけコンテナを作成
            container, err := common.InitializeContainer()
            if err != nil {
                return err
            }
            defer container.Close()

            // コンテナをワークフローに渡す
            manager := workflow.NewWorkflowManager(...)

            // RunTurn関数のシグネチャを変更してコンテナを渡す
            sbiRunner := workflow_sbi.NewSBIWorkflowRunnerWithContainer(
                container,
                RunTurnWithContainer,
            )

            // ...
        },
    }
}

// コンテナを受け取る新しいRunTurn
func RunTurnWithContainer(container *di.Container, autoFB bool) error {
    // コンテナから必要なサービスを取得
    lockService := container.GetLockService()
    // ...
}
```

**確認項目**:
- [ ] コンテナが1回だけ作成される
- [ ] マイグレーションが1回だけ実行される
- [ ] 複数回の実行でDB接続が再利用される
- [ ] 既存のテストが全てパスする

#### Step 0-2: Option3実装（WALモード有効化）

**変更対象**: `internal/infrastructure/di/container.go`

**Before**:
```go
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
```

**After**:
```go
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
if err != nil {
    return fmt.Errorf("failed to open database: %w", err)
}

// WALモードが正しく設定されたか確認
var journalMode string
if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
    return fmt.Errorf("failed to check journal mode: %w", err)
}
if journalMode != "wal" {
    return fmt.Errorf("WAL mode not enabled, got: %s", journalMode)
}
```

**WALモードの効果**:
- Writerが1つ、Readerが複数の並行実行が可能
- `run`実行中でも`register`コマンドが実行可能
- ロック競合の大幅な削減

**確認項目**:
- [ ] WALモードが有効化される
- [ ] `-wal`と`-shm`ファイルが生成される
- [ ] `run`実行中に`register`が成功する
- [ ] パフォーマンス改善が確認できる

---

## Phase 1: レガシー依存の除去

**目的**: `state.json`への依存を削除し、SQLiteを単一データソースにする

**期間**: 3-5日
**難易度**: 中

### Step 1-1: SBI実行状態の完全DB化

#### 1.1.1 ExecutionState管理の移行

**変更対象**:
- `internal/application/usecase/execution/run_turn_usecase.go`
- `internal/interface/cli/run/run.go`

**実装内容**:

```go
// Before: state.jsonから読み込み
st, err := common.LoadState(paths.State)
currentSBI := st.WIP
turn := st.Turn
attempt := st.Attempt

// After: DBから読み込み
sbiRepo := container.GetSBIRepository()
currentSBI, err := pickNextSBI(ctx, sbiRepo)
if currentSBI == nil {
    return nil, ErrNoTaskAvailable
}
turn := currentSBI.ExecutionState().CurrentTurn
attempt := currentSBI.ExecutionState().CurrentAttempt
```

**新規関数**:
```go
// pickNextSBI は実行可能なSBIを1つ選択する
func pickNextSBI(ctx context.Context, repo repository.SBIRepository) (*sbi.SBI, error) {
    // 優先順位:
    // 1. WIP状態のSBI（継続実行）
    // 2. READY状態のSBI（新規開始）

    filter := repository.SBIFilter{
        Status: []string{"WIP", "READY"},
        OrderBy: "priority DESC, created_at ASC",
        Limit: 1,
    }

    sbis, err := repo.List(ctx, filter)
    if err != nil {
        return nil, err
    }

    if len(sbis) == 0 {
        return nil, nil  // タスクなし
    }

    return sbis[0], nil
}
```

#### 1.1.2 StateLock → SBILockへの移行

**変更対象**: `internal/application/usecase/execution/run_turn_usecase.go`

**Before**:
```go
// ファイルロックを使用
lockPath := paths.StateLock
if _, err := os.Stat(lockPath); err == nil {
    return fmt.Errorf("state is locked")
}
```

**After**:
```go
// LockServiceを使用してSBI単位でロック
lockID, _ := lock.NewLockID(fmt.Sprintf("sbi-%s", sbiID))
sbiLock, err := lockService.AcquireStateLock(ctx, lockID, lock.LockTypeWrite, 10*time.Minute)
if err != nil {
    return fmt.Errorf("failed to acquire SBI lock: %w", err)
}
defer lockService.ReleaseStateLock(ctx, lockID)
```

#### 1.1.3 state.jsonの段階的廃止

**段階的アプローチ**:

**Phase 1-A: 二重書き込み（互換性維持）**
```go
// DBに保存
sbiRepo.Save(ctx, updatedSBI)

// 後方互換のためstate.jsonにも書き込み（Deprecation Warning付き）
if legacyEnabled() {
    saveToStateLegacy(updatedSBI)
    log.Warn("state.json is deprecated and will be removed in future version")
}
```

**Phase 1-B: 読み込みをDBのみに切り替え**
```go
// state.jsonは読み込まない
// sbi := loadFromDB(sbiID)
```

**Phase 1-C: state.json書き込みの削除**
```go
// state.json関連のコードを完全削除
// rm internal/interface/cli/common/stateio.go
```

**確認項目**:
- [ ] `state.json`を読まずにSBI実行が動作する
- [ ] 実行状態がSQLiteに正しく保存される
- [ ] ロックがLockServiceで管理される
- [ ] レガシーコードが削除される

### Step 1-2: タスク選択ロジックの実装

**新規ファイル**: `internal/application/usecase/execution/task_picker.go`

```go
package execution

import (
    "context"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// TaskPicker はタスク選択戦略を定義する
type TaskPicker interface {
    // PickNext は次に実行するSBIを選択する
    PickNext(ctx context.Context) (*sbi.SBI, error)
}

// DefaultTaskPicker はデフォルトのタスク選択実装
type DefaultTaskPicker struct {
    sbiRepo repository.SBIRepository
}

func NewDefaultTaskPicker(sbiRepo repository.SBIRepository) *DefaultTaskPicker {
    return &DefaultTaskPicker{sbiRepo: sbiRepo}
}

func (p *DefaultTaskPicker) PickNext(ctx context.Context) (*sbi.SBI, error) {
    // 1. WIP状態のSBIを優先（継続実行）
    wipSBIs, err := p.sbiRepo.List(ctx, repository.SBIFilter{
        Status: []string{"WIP"},
        OrderBy: "priority DESC, updated_at ASC",
        Limit: 1,
    })
    if err != nil {
        return nil, err
    }
    if len(wipSBIs) > 0 {
        return wipSBIs[0], nil
    }

    // 2. READY状態のSBIを選択（新規開始）
    readySBIs, err := p.sbiRepo.List(ctx, repository.SBIFilter{
        Status: []string{"READY"},
        OrderBy: "priority DESC, created_at ASC",
        Limit: 1,
    })
    if err != nil {
        return nil, err
    }
    if len(readySBIs) > 0 {
        return readySBIs[0], nil
    }

    // 3. タスクなし
    return nil, nil
}
```

**確認項目**:
- [ ] WIP状態のSBIが優先的に選択される
- [ ] READY状態のSBIが優先度順に選択される
- [ ] タスクがない場合にnilが返る
- [ ] ユニットテストが全てパスする

---

## Phase 2: ワークフローの並行実行対応

**目的**: 複数SBIの並行実行を実現する

**期間**: 2-3日
**難易度**: 高

### Step 2-1: ParallelSBIWorkflowRunnerの実装

**新規ファイル**: `internal/interface/cli/workflow_sbi/parallel_runner.go`

```go
package workflow_sbi

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/YoshitsuguKoike/deespec/internal/application/service"
    "github.com/YoshitsuguKoike/deespec/internal/application/workflow"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
    "github.com/YoshitsuguKoike/deespec/internal/domain/repository"
    "github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
)

// ParallelSBIWorkflowRunner は複数SBIを並行実行する
type ParallelSBIWorkflowRunner struct {
    enabled      bool
    maxParallel  int                    // 最大並行実行数
    container    *di.Container          // 共有コンテナ
    executeTurn  ExecuteTurnFunc        // Turn実行関数
}

// ExecuteTurnFunc はSBI実行関数の型
type ExecuteTurnFunc func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error

// NewParallelSBIWorkflowRunner creates a new parallel runner
func NewParallelSBIWorkflowRunner(container *di.Container, maxParallel int, executeTurn ExecuteTurnFunc) *ParallelSBIWorkflowRunner {
    return &ParallelSBIWorkflowRunner{
        enabled:     true,
        maxParallel: maxParallel,
        container:   container,
        executeTurn: executeTurn,
    }
}

// Name returns the workflow name
func (r *ParallelSBIWorkflowRunner) Name() string {
    return "sbi-parallel"
}

// Description returns a human-readable description
func (r *ParallelSBIWorkflowRunner) Description() string {
    return fmt.Sprintf("Parallel SBI workflow (max: %d concurrent tasks)", r.maxParallel)
}

// IsEnabled checks if the workflow should be executed
func (r *ParallelSBIWorkflowRunner) IsEnabled() bool {
    return r.enabled
}

// SetEnabled sets the enabled state
func (r *ParallelSBIWorkflowRunner) SetEnabled(enabled bool) {
    r.enabled = enabled
}

// Run executes multiple SBIs in parallel
func (r *ParallelSBIWorkflowRunner) Run(ctx context.Context, config workflow.WorkflowConfig) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Get services from container
    sbiRepo := r.container.GetSBIRepository()
    lockService := r.container.GetLockService()

    // Extract AutoFB from config
    autoFB := config.AutoFB

    // Fetch executable SBIs
    sbis, err := r.fetchExecutableSBIs(ctx, sbiRepo, r.maxParallel)
    if err != nil {
        return fmt.Errorf("failed to fetch SBIs: %w", err)
    }

    if len(sbis) == 0 {
        return nil // No tasks to execute
    }

    // Execute SBIs in parallel with semaphore control
    var wg sync.WaitGroup
    sem := make(chan struct{}, r.maxParallel) // Semaphore
    errChan := make(chan error, len(sbis))

    for _, currentSBI := range sbis {
        wg.Add(1)
        sem <- struct{}{} // Acquire semaphore

        go func(s *sbi.SBI) {
            defer wg.Done()
            defer func() { <-sem }() // Release semaphore

            // Acquire SBI-specific lock
            lockID, _ := lock.NewLockID(fmt.Sprintf("sbi-%s", s.ID()))
            sbiLock, err := lockService.AcquireStateLock(ctx, lockID, lock.LockTypeWrite, 10*time.Minute)
            if err != nil {
                // Another worker is processing this SBI, skip
                return
            }
            defer lockService.ReleaseStateLock(ctx, lockID)

            // Execute turn for this SBI
            if err := r.executeTurn(ctx, r.container, s.ID().String(), autoFB); err != nil {
                errChan <- fmt.Errorf("SBI %s: %w", s.ID(), err)
            }
        }(currentSBI)
    }

    // Wait for all goroutines
    wg.Wait()
    close(errChan)

    // Collect errors
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }

    if len(errors) > 0 {
        return fmt.Errorf("parallel execution errors: %v", errors)
    }

    return nil
}

// Validate checks if the workflow can be executed
func (r *ParallelSBIWorkflowRunner) Validate() error {
    if r.maxParallel < 1 {
        return fmt.Errorf("maxParallel must be >= 1, got: %d", r.maxParallel)
    }
    if r.container == nil {
        return fmt.Errorf("container is nil")
    }
    if r.executeTurn == nil {
        return fmt.Errorf("executeTurn function is nil")
    }
    return nil
}

// fetchExecutableSBIs retrieves SBIs ready for execution
func (r *ParallelSBIWorkflowRunner) fetchExecutableSBIs(
    ctx context.Context,
    sbiRepo repository.SBIRepository,
    limit int,
) ([]*sbi.SBI, error) {
    filter := repository.SBIFilter{
        Status: []string{"READY", "WIP"},
        OrderBy: "priority DESC, created_at ASC",
        Limit: limit,
    }

    return sbiRepo.List(ctx, filter)
}
```

**確認項目**:
- [ ] セマフォで並行数が制御される
- [ ] SBI単位でロックが取得される
- [ ] goroutineが正しく終了する（goroutine leak無し）
- [ ] エラーハンドリングが適切

### Step 2-2: CLIフラグの追加

**変更対象**: `internal/interface/cli/run/run.go`

```go
func NewCommand() *cobra.Command {
    var autoFB bool
    var intervalStr string
    var enabledWorkflows []string
    var maxParallel int  // ← 追加

    cmd := &cobra.Command{
        Use:   "run",
        Short: "Run all enabled workflows in parallel",
        Long: `Run all enabled workflows in parallel.

This command supports parallel execution of multiple SBIs.
Use --parallel flag to control the maximum concurrent executions.

Examples:
  deespec run                           # Run with default settings
  deespec run --parallel 3              # Run up to 3 SBIs concurrently
  deespec run --workflows sbi --parallel 5  # SBI workflow with 5 parallel tasks`,
        RunE: func(cmd *cobra.Command, args []string) error {
            // ... container initialization ...

            // Register parallel workflow runner
            sbiRunner := workflow_sbi.NewParallelSBIWorkflowRunner(
                container,
                maxParallel,
                ExecuteTurnForSBI,
            )

            // ...
        },
    }

    cmd.Flags().BoolVar(&autoFB, "auto-fb", false, "Automatically register FB-SBI drafts")
    cmd.Flags().StringVar(&intervalStr, "interval", "", "Execution interval")
    cmd.Flags().StringSliceVar(&enabledWorkflows, "workflows", nil, "Workflows to enable")
    cmd.Flags().IntVar(&maxParallel, "parallel", 1, "Maximum concurrent SBI executions (1-10)")

    return cmd
}
```

**確認項目**:
- [ ] `--parallel`フラグが認識される
- [ ] 並行数が1-10の範囲でバリデーションされる
- [ ] デフォルト値1で動作する（後方互換性）

### Step 2-3: 並行実行のテスト

**新規ファイル**: `internal/interface/cli/workflow_sbi/parallel_runner_test.go`

```go
package workflow_sbi

import (
    "context"
    "sync/atomic"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestParallelSBIWorkflowRunner_ParallelExecution(t *testing.T) {
    // Setup mock container with 5 SBIs
    container := setupMockContainer(t, 5)
    defer container.Close()

    // Track concurrent executions
    var currentConcurrency int32
    var maxConcurrency int32

    executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
        current := atomic.AddInt32(&currentConcurrency, 1)
        defer atomic.AddInt32(&currentConcurrency, -1)

        // Update max concurrency
        for {
            max := atomic.LoadInt32(&maxConcurrency)
            if current <= max || atomic.CompareAndSwapInt32(&maxConcurrency, max, current) {
                break
            }
        }

        // Simulate work
        time.Sleep(100 * time.Millisecond)
        return nil
    }

    runner := NewParallelSBIWorkflowRunner(container, 3, executeTurn)

    ctx := context.Background()
    config := workflow.WorkflowConfig{
        Name:     "sbi",
        Enabled:  true,
        Interval: 1 * time.Second,
        AutoFB:   false,
    }

    err := runner.Run(ctx, config)
    require.NoError(t, err)

    // Verify max concurrency was 3
    assert.Equal(t, int32(3), atomic.LoadInt32(&maxConcurrency))
}

func TestParallelSBIWorkflowRunner_LockConflict(t *testing.T) {
    // Test that same SBI is not executed twice concurrently
    // ... implementation ...
}

func TestParallelSBIWorkflowRunner_ErrorHandling(t *testing.T) {
    // Test error propagation from parallel executions
    // ... implementation ...
}
```

**確認項目**:
- [ ] 並行数制御が正しく機能する
- [ ] 同じSBIが重複実行されない
- [ ] エラーが適切に収集される
- [ ] Goroutine leakがない（`goleak`テスト）

---

## Phase 3: 高度な並行制御

**目的**: ファイル競合検出、優先度スケジューリング、Agent別実行制御

**期間**: 1-2日
**難易度**: 中

### Step 3-1: ファイル競合検出

**新規ファイル**: `internal/application/usecase/execution/conflict_detector.go`

```go
package execution

import (
    "context"
    "github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// ConflictDetector はSBI間のファイル競合を検出する
type ConflictDetector struct {
    // 現在実行中のSBIとそれが触るファイルパスのマップ
    activeFiles map[string]string // filepath -> sbiID
    mu          sync.RWMutex
}

func NewConflictDetector() *ConflictDetector {
    return &ConflictDetector{
        activeFiles: make(map[string]string),
    }
}

// HasConflict は指定されたSBIが他のSBIとファイル競合するかチェック
func (d *ConflictDetector) HasConflict(s *sbi.SBI) bool {
    d.mu.RLock()
    defer d.mu.RUnlock()

    for _, filePath := range s.Metadata().FilePaths {
        if conflictingSBIID, exists := d.activeFiles[filePath]; exists {
            if conflictingSBIID != s.ID().String() {
                return true // Conflict detected
            }
        }
    }

    return false
}

// Register は実行開始時にSBIのファイルを登録
func (d *ConflictDetector) Register(s *sbi.SBI) {
    d.mu.Lock()
    defer d.mu.Unlock()

    for _, filePath := range s.Metadata().FilePaths {
        d.activeFiles[filePath] = s.ID().String()
    }
}

// Unregister は実行完了時にSBIのファイルを解放
func (d *ConflictDetector) Unregister(s *sbi.SBI) {
    d.mu.Lock()
    defer d.mu.Unlock()

    for _, filePath := range s.Metadata().FilePaths {
        delete(d.activeFiles, filePath)
    }
}
```

**ParallelRunnerへの統合**:
```go
func (r *ParallelSBIWorkflowRunner) Run(ctx context.Context, config workflow.WorkflowConfig) error {
    // ...

    conflictDetector := NewConflictDetector()

    for _, currentSBI := range sbis {
        // Skip if file conflict detected
        if conflictDetector.HasConflict(currentSBI) {
            continue
        }

        wg.Add(1)
        sem <- struct{}{}

        go func(s *sbi.SBI) {
            defer wg.Done()
            defer func() { <-sem }()

            // Register files
            conflictDetector.Register(s)
            defer conflictDetector.Unregister(s)

            // Execute...
        }(currentSBI)
    }

    // ...
}
```

**確認項目**:
- [ ] 同じファイルを触るSBIが同時実行されない
- [ ] ファイル競合がない場合は並行実行される
- [ ] 登録/解放が正しく行われる

### Step 3-2: Agent別の並行数制御

**新規構造体**:
```go
type AgentPool struct {
    maxPerAgent map[string]int // agent -> max concurrent
    current     map[string]int // agent -> current count
    mu          sync.Mutex
}

func NewAgentPool() *AgentPool {
    return &AgentPool{
        maxPerAgent: map[string]int{
            "claude-code": 2,
            "gemini-cli":  1,
            "codex":       1,
        },
        current: make(map[string]int),
    }
}

func (p *AgentPool) Acquire(agent string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    max, exists := p.maxPerAgent[agent]
    if !exists {
        max = 1 // Default
    }

    if p.current[agent] >= max {
        return false // Pool full
    }

    p.current[agent]++
    return true
}

func (p *AgentPool) Release(agent string) {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.current[agent]--
}
```

**確認項目**:
- [ ] Agent別の並行数が制御される
- [ ] プール管理が正しく動作する

---

## 実装チェックリスト

### 前提条件の整備

#### Step 0-1: Option1（コンテナ再利用）
- [x] `NewCommand()`でコンテナを1回だけ作成
- [x] `RunTurnWithContainer()`関数を実装
- [x] `SBIWorkflowRunner`にコンテナ渡す仕組み追加
- [x] マイグレーションが1回だけ実行されることを確認
- [x] 既存テスト全てパス
- [ ] 統合テストで複数回実行を確認

#### Step 0-2: Option3（WALモード）
- [x] DB接続文字列に`_journal_mode=WAL`追加
- [x] WALモード有効化確認ロジック追加
- [x] `-wal`、`-shm`ファイル生成確認
- [x] `run`実行中に`register`実行テスト（並行アクセステスト実装）
- [x] パフォーマンステスト実施（ベンチマーク追加）
- [x] ドキュメント更新（WALモード実装ガイド作成）

### Phase 1: レガシー依存の除去

#### Step 1-1: SBI実行状態の完全DB化
- [x] `pickNextSBI()`関数実装
- [x] `state.json`読み込みをDB読み込みに置き換え（Clean Architecture移行で完了）
- [x] SBI単位のロック実装（StateLock使用）
- [x] 二重書き込みモード実装（互換性維持）
- [x] DB読み込みのみモードへ切り替え（enableDualWrite=false）
- [x] `state.json`書き込み削除（dual-write無効化）
- [ ] `stateio.go`削除（既にgit statusで削除済み、レガシーコード整理残）
- [ ] 統合テスト全てパス

#### Step 1-2: タスク選択ロジック
- [ ] `TaskPicker`インターフェース定義
- [ ] `DefaultTaskPicker`実装
- [ ] WIP優先ロジック実装
- [ ] READY優先度順ロジック実装
- [ ] ユニットテスト作成
- [ ] 統合テスト作成

### Phase 2: 並行実行対応

#### Step 2-1: ParallelSBIWorkflowRunner実装
- [x] `parallel_runner.go`作成
- [x] セマフォによる並行数制御実装
- [x] SBI単位のロック取得/解放実装
- [x] goroutine管理実装
- [x] エラー収集機構実装
- [x] ユニットテスト作成（並行数確認）
- [x] Goroutine leakテスト（goleak使用）

#### Step 2-2: CLIフラグ追加
- [x] `--parallel`フラグ追加
- [x] バリデーション実装（1-10範囲）
- [x] デフォルト値設定（1）
- [x] ヘルプテキスト更新
- [x] 手動テスト実施

#### Step 2-3: 並行実行テスト
- [x] `parallel_runner_test.go`作成
- [x] 並行数制御テスト
- [x] ロック競合テスト
- [x] エラーハンドリングテスト
- [x] パフォーマンステスト
- [x] 負荷テスト（大量SBI）

### Phase 3: 高度な並行制御

#### Step 3-1: ファイル競合検出
- [x] `ConflictDetector`実装
- [x] ファイルパス登録/解放実装
- [x] 競合検出ロジック実装
- [x] ParallelRunnerへの統合
- [x] ユニットテスト作成
- [x] 統合テスト作成

#### Step 3-2: Agent別並行数制御
- [x] `AgentPool`実装
- [x] Agent別制限設定
- [x] プール取得/解放実装
- [x] 設定ファイル対応（config.go, settings.go, root.go更新）
- [x] ユニットテスト作成
- [x] ParallelRunnerへの統合
- [x] 統合テスト作成

### ドキュメント・その他

- [ ] アーキテクチャ図更新
- [ ] READMEに並行処理機能追加
- [ ] マイグレーションガイド作成
- [ ] パフォーマンス比較レポート作成
- [ ] CHANGELOG更新
- [ ] リリースノート作成

---

## リスクと対策

### リスク1: データ不整合

**リスク**: SQLiteとstate.jsonの同期ミス

**対策**:
- 二重書き込み期間を十分に取る（1-2週間）
- 検証スクリプトで両者の一致を確認
- 段階的な移行（Phase 1-A → 1-B → 1-C）

### リスク2: Goroutine Leak

**リスク**: 並行処理でgoroutineが終了しない

**対策**:
- `defer`で確実にリソース解放
- `goleak`によるテスト
- コンテキストキャンセルの適切な処理
- タイムアウト設定

### リスク3: ロック競合

**リスク**: 過度なロックで性能低下

**対策**:
- WALモード有効化
- 適切な粒度のロック（SBI単位）
- ロック保持時間の最小化
- モニタリングとロギング

### リスク4: 後方互換性

**リスク**: 既存ユーザーの環境で動作しない

**対策**:
- `--parallel 1`をデフォルトに設定（順次実行）
- 段階的機能有効化
- フィーチャーフラグの使用
- 詳細なマイグレーションガイド

### リスク5: SQLite性能限界

**リスク**: 並行数増加でSQLiteがボトルネック

**対策**:
- 並行数を1-10に制限
- WALモード活用
- インデックス最適化
- 将来的にPostgreSQLへの移行パス用意

---

## 成功の指標

### パフォーマンス指標

- **処理速度**: 3つのSBIを並行実行で、順次実行の2.5倍以上の速度
- **応答性**: `register`コマンドが`run`実行中でも1秒以内に完了
- **安定性**: 24時間連続実行でエラーなし

### コード品質指標

- **テストカバレッジ**: 新規コードで85%以上
- **Goroutine leak**: テストでleakゼロ
- **ロック競合**: ロック待機時間が平均100ms以下

### ユーザビリティ指標

- **学習コスト**: 既存ユーザーが新機能を5分以内に理解
- **後方互換性**: 既存の全てのワークフローが動作
- **ドキュメント**: 並行処理の使い方が明確

---

## 今後の拡張計画

### 短期（1-2ヶ月）
- [ ] PBIレベルの並行処理対応
- [ ] 動的な並行数調整（負荷に応じて自動調整）
- [ ] 詳細な実行統計の収集

### 中期（3-6ヶ月）
- [ ] 分散実行対応（複数マシンでの並行処理）
- [ ] WebUIでの並行実行モニタリング
- [ ] PostgreSQLバックエンドのサポート

### 長期（6ヶ月以上）
- [ ] クラウド実行環境対応
- [ ] AI Agentの自動スケーリング
- [ ] リアルタイムコラボレーション機能

---

## 参考資料

- [Clean Architecture原則](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [SQLite WAL Mode](https://www.sqlite.org/wal.html)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Goroutine Leak Detection](https://github.com/uber-go/goleak)

---

**最終更新**: 2025-10-10
**ステータス更新**: Phase 2完了、Phase 3完了、Phase 1 (Step 1-1) 部分完了
**次のステップ**:
- Phase 1残タスク: レガシーコード整理（state CLI削除、state loader削除）
- ドキュメント更新（並行処理使用ガイド）

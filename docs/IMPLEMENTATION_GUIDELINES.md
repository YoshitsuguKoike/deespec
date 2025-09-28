# 実装ガイドライン - クリーンアーキテクチャ + DDD

**優先度: 🔴 高**
**適用範囲: すべての新規機能開発**
**作成日: 2025-09-28**

> ⚠️ **重要**: 今後のすべての新規機能開発は、このガイドラインに従って実装すること。既存コードのリファクタリング時も、段階的にこの構造に移行すること。

## 📋 目次

1. [基本原則](#基本原則)
2. [レイヤー構造と責務](#レイヤー構造と責務)
3. [実装手順](#実装手順)
4. [コード配置ルール](#コード配置ルール)
5. [実装例](#実装例)
6. [チェックリスト](#チェックリスト)

---

## 基本原則

### 1. 依存関係の方向（Dependency Rule）
```
[外側] interface → usecase → domain [内側]
         ↓           ↓         ↑
       infra ────────────────→
```
- 依存は**必ず内側に向かう**
- domain層は他の層に依存しない
- interface層のCLIコマンドは薄いラッパーに留める

### 2. 各層の責務を厳守する
- **interface層**: 入出力の変換のみ（ビジネスロジック禁止）
- **usecase層**: アプリケーション固有のビジネスルール
- **domain層**: ビジネスの核心的なルールとエンティティ
- **infra層**: 技術的詳細の実装

---

## レイヤー構造と責務

### 🎯 Domain層（ビジネスの核心）
```
internal/domain/{機能名}/
├── entity.go           # エンティティ定義
├── value_object.go     # 値オブジェクト
├── repository.go       # リポジトリインターフェース
├── service.go          # ドメインサービス
└── error.go           # ドメイン固有エラー
```

**責務:**
- ビジネスルールの表現
- エンティティと値オブジェクトの定義
- ドメインサービスの実装
- **技術的詳細から完全に独立**

### 📦 UseCase層（アプリケーションロジック）
```
internal/usecase/{機能名}/
├── {action}_usecase.go     # ユースケース実装
├── input.go                # 入力DTO
├── output.go               # 出力DTO
└── interface.go            # 外部サービスインターフェース
```

**責務:**
- アプリケーション固有のビジネスフロー
- トランザクション境界の管理
- ドメインオブジェクトの組み合わせ
- 入出力の変換（DTO）

### 🖥️ Interface層（プレゼンテーション）
```
internal/interface/cli/
├── {command}.go           # CLIコマンド（薄いラッパー）
└── {command}_handler.go   # リクエスト/レスポンス変換
```

**責務:**
- CLIコマンドの定義とパース
- ユースケースの呼び出し
- 結果の表示形式への変換
- **ビジネスロジックを含まない**

### 🔧 Infrastructure層（技術的実装）
```
internal/infra/
├── repository/{機能名}/
│   └── {entity}_repository.go    # リポジトリ実装
├── external/
│   └── {service}_client.go       # 外部サービス連携
└── persistence/
    └── file_store.go              # 永続化実装
```

**責務:**
- リポジトリインターフェースの実装
- 外部サービスとの通信
- ファイルシステムやDBアクセス
- 技術的な詳細の隠蔽

---

## 実装手順

### 🚀 新機能追加時の実装順序

1. **Domain層から開始**
   ```go
   // internal/domain/task/entity.go
   type Task struct {
       ID       TaskID
       Title    string
       Priority Priority
       Status   TaskStatus
   }

   // internal/domain/task/repository.go
   type TaskRepository interface {
       FindByID(id TaskID) (*Task, error)
       Save(task *Task) error
       FindReadyTasks() ([]*Task, error)
   }
   ```

2. **UseCase層の実装**
   ```go
   // internal/usecase/task/pick_next_task_usecase.go
   type PickNextTaskUseCase struct {
       taskRepo domain.TaskRepository
       logger   logger.Logger
   }

   func (u *PickNextTaskUseCase) Execute(input PickNextTaskInput) (*PickNextTaskOutput, error) {
       // ビジネスフロー実装
       tasks, err := u.taskRepo.FindReadyTasks()
       if err != nil {
           return nil, err
       }

       selected := u.selectByPriority(tasks)
       return &PickNextTaskOutput{Task: selected}, nil
   }
   ```

3. **Infrastructure層の実装**
   ```go
   // internal/infra/repository/task/task_repository.go
   type FileTaskRepository struct {
       basePath string
   }

   func (r *FileTaskRepository) FindReadyTasks() ([]*domain.Task, error) {
       // ファイルシステムからタスクを読み込む実装
   }
   ```

4. **Interface層（最後）**
   ```go
   // internal/interface/cli/pick_task.go
   func NewPickTaskCommand(usecase *task.PickNextTaskUseCase) *cobra.Command {
       return &cobra.Command{
           Use: "pick",
           RunE: func(cmd *cobra.Command, args []string) error {
               // 1. 入力を収集
               input := task.PickNextTaskInput{}

               // 2. ユースケースを実行
               output, err := usecase.Execute(input)
               if err != nil {
                   return err
               }

               // 3. 結果を表示
               fmt.Printf("Selected: %s\n", output.Task.ID)
               return nil
           },
       }
   }
   ```

---

## コード配置ルール

### ❌ アンチパターン（避けるべき実装）

```go
// ❌ interface層にビジネスロジックを書かない
// internal/interface/cli/bad_example.go
func pickTaskCommand() {
    // ファイル読み込み（infraの責務）
    files, _ := os.ReadDir(".deespec/specs")

    // 優先度計算（domainの責務）
    for _, file := range files {
        priority := calculatePriority(file) // ❌
        if priority > maxPriority {
            selected = file
        }
    }

    // 依存関係チェック（usecaseの責務）
    if checkDependencies(selected) { // ❌
        // ...
    }
}
```

### ✅ 正しい実装パターン

```go
// ✅ interface層は薄いラッパー
// internal/interface/cli/good_example.go
func pickTaskCommand(usecase *task.PickNextTaskUseCase) {
    output, err := usecase.Execute(task.PickNextTaskInput{})
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        return
    }
    fmt.Printf("Selected: %s\n", output.TaskID)
}
```

---

## 実装例

### 例: 新機能「タスク統計表示」を追加する場合

#### 1. Domain層
```go
// internal/domain/statistics/value_object.go
type TaskStatistics struct {
    TotalTasks      int
    CompletedTasks  int
    AverageTime     time.Duration
    SuccessRate     float64
}

// internal/domain/statistics/service.go
type StatisticsService struct {
    taskRepo TaskRepository
}

func (s *StatisticsService) Calculate(tasks []*Task) *TaskStatistics {
    // ドメインロジック
}
```

#### 2. UseCase層
```go
// internal/usecase/statistics/show_statistics_usecase.go
type ShowStatisticsUseCase struct {
    taskRepo    domain.TaskRepository
    statsService *domain.StatisticsService
}

type ShowStatisticsOutput struct {
    Stats *domain.TaskStatistics
    Period string
}

func (u *ShowStatisticsUseCase) Execute(input ShowStatisticsInput) (*ShowStatisticsOutput, error) {
    tasks, err := u.taskRepo.FindByPeriod(input.StartDate, input.EndDate)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch tasks: %w", err)
    }

    stats := u.statsService.Calculate(tasks)

    return &ShowStatisticsOutput{
        Stats: stats,
        Period: input.Period,
    }, nil
}
```

#### 3. Infrastructure層
```go
// internal/infra/repository/task/task_repository.go
func (r *FileTaskRepository) FindByPeriod(start, end time.Time) ([]*domain.Task, error) {
    // ジャーナルファイルから期間内のタスクを取得
    journal, err := r.readJournal()
    // ...実装
}
```

#### 4. Interface層
```go
// internal/interface/cli/stats.go
func NewStatsCommand(usecase *statistics.ShowStatisticsUseCase) *cobra.Command {
    return &cobra.Command{
        Use:   "stats",
        Short: "Show task statistics",
        RunE: func(cmd *cobra.Command, args []string) error {
            period, _ := cmd.Flags().GetString("period")

            input := statistics.ShowStatisticsInput{
                Period: period,
            }

            output, err := usecase.Execute(input)
            if err != nil {
                return err
            }

            // 表示のみ
            fmt.Printf("Statistics for %s:\n", output.Period)
            fmt.Printf("Total: %d\n", output.Stats.TotalTasks)
            fmt.Printf("Success Rate: %.2f%%\n", output.Stats.SuccessRate*100)

            return nil
        },
    }
}
```

---

## チェックリスト

### 新機能実装時のチェックリスト

- [ ] **Domain層**
  - [ ] エンティティ/値オブジェクトを定義した
  - [ ] リポジトリインターフェースを定義した
  - [ ] ドメインサービスが必要な場合は実装した
  - [ ] 技術的詳細への依存がない

- [ ] **UseCase層**
  - [ ] ユースケースクラスを作成した
  - [ ] 入力/出力DTOを定義した
  - [ ] ドメインオブジェクトを適切に利用している
  - [ ] トランザクション境界を明確にした

- [ ] **Infrastructure層**
  - [ ] リポジトリインターフェースを実装した
  - [ ] 外部サービスとの連携を実装した
  - [ ] 技術的詳細を隠蔽している

- [ ] **Interface層**
  - [ ] CLIコマンドは薄いラッパーになっている
  - [ ] ビジネスロジックが含まれていない
  - [ ] ユースケースを呼び出すだけになっている

### コードレビュー時の確認項目

1. **依存関係の方向は正しいか？**
   - domain → 他層への依存がないか
   - interface → domain への直接依存がないか

2. **責務は適切に分離されているか？**
   - interface層にif文の羅列がないか
   - domain層にファイル操作がないか
   - usecase層にCLI出力がないか

3. **テスタビリティは確保されているか？**
   - インターフェースを通じた依存注入
   - モックしやすい設計

---

## 移行戦略

### 既存コードのリファクタリング優先順位

1. **第1段階**: 新機能は必ずこの構造で実装
2. **第2段階**: 変更頻度の高い機能から段階的に移行
3. **第3段階**: 残りの機能を計画的に移行

### 移行対象の優先度

| 優先度 | 対象機能 | 現在の場所 | 移行先 |
|-------|---------|-----------|--------|
| 🔴 高 | タスク選択ロジック | cli/picker.go | domain/task + usecase/task |
| 🔴 高 | 診断機能 | cli/doctor.go | domain/health + usecase/diagnostics |
| 🟡 中 | 不完全検出 | cli/incomplete.go | domain/validation + usecase/validation |
| 🟡 中 | 状態管理 | cli/state.go | domain/state + infra/repository |
| 🟢 低 | 設定管理 | 複数箇所 | infra/config |

---

## 参考資料

- [Clean Architecture (Robert C. Martin)](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Domain-Driven Design (Eric Evans)](https://www.domainlanguage.com/ddd/)
- [実践DDD (IDDD)](https://www.amazon.co.jp/dp/B00UX9ZJGW)

---

## 改訂履歴

- 2025-09-28: 初版作成
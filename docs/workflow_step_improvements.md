# Workflow Step Management Improvements

## 概要

このドキュメントは、SBIワークフローにおける複数の問題（Turn番号、Step表示、レポート生成）を解決するための移行プランを記載します。

---

## 現在の問題

### 1. Turn番号が2から開始する

**現象:**
```
[11:05:10.641] INFO: 💓 [sbi] Processing task テスト実行とFAIL修正 [PICK] (turn #1)...
[11:07:14.417] INFO: 🔄 Turn 2 completed at 11:07:14
```

初回実行なのにTurn 2と表示される。

**原因:**
1. `model.NewTurn()` が初期値として `1` を返す (`internal/domain/model/value_object.go:146`)
2. `run_turn_use_case.go:131` で実行前に `currentTurn++` を実行
3. 結果: 初回実行時に Turn が 2 になる

**影響:**
- ログ表示が不正確
- レポートファイル名が `implement_2.md` から始まる
- ユーザーの混乱を招く

---

### 2. Turn番号変更後も [PICK] 表示が続く

**現象:**
```
[11:05:30.640] INFO: 💓 [sbi] Processing task テスト実行とFAIL修正 [PICK] (turn #1)...
[11:09:04.426] INFO: 💓 [sbi] Processing task テスト実行とFAIL修正 [PICK] (turn #2)...
[11:10:10.930] INFO: 💓 [sbi] Processing task テスト実行とFAIL修正 [PICK] (turn #3)...
```

Turn 2, 3でも [PICK] と表示される。

**原因:**
1. `task.NewBaseTask()` で `currentStep` を `model.StepPick` に初期化 (`internal/domain/model/task/task.go:80`)
2. `UpdateStatus()` メソッドは `status` フィールドのみ更新し、`currentStep` を更新しない (`task.go:158-170`)
3. Status遷移: PENDING → PICKED → IMPLEMENTING → REVIEWING → DONE
4. しかし `currentStep` は常に "PICK" のまま

**影響:**
- ワークフロー進行状況が正確に表示されない
- 実際には implement や review ステップにいるのに PICK と表示される

---

### 3. Journal書き込みエラー

**現象:**
```
⚠️  WARNING: Failed to append journal entry
   Error: failed to parse existing journal: unexpected end of JSON input
   SBI ID: 3345771e-ae05-4ae7-ba95-d7ecf653b036, Turn: 2, Step: implement, Status: WIP
```

**原因:**
- journal.ndjson ファイルが破損している可能性
- 並行書き込みによる競合
- 不完全な JSON 行が含まれている

**影響:**
- 実行履歴が記録されない
- デバッグやトラブルシューティングが困難

---

### 4. done.md レポートが生成されない

**現象:**
- 以前は `done.md` が生成されていた
- 現在は `implement_N.md` と `review_N.md` のみ生成される

**原因:**
- `run_turn_use_case.go:370-381` で step に基づいてテンプレートを選択
- "done" step の処理が存在しない
- Status が DONE になってもレポート生成がスキップされる

**影響:**
- タスク完了の記録が残らない
- 完了時の最終レポートが確認できない

---

## アーキテクチャ分析

### 現在の Status と Step の関係

```
Status (model.Status)        Step (model.Step)         期待される表示
---------------------        -----------------         ---------------
PENDING                  →   PICK                  →   [PICK]
PICKED                   →   PICK (更新されない)    →   [PICK] (誤り)
IMPLEMENTING             →   PICK (更新されない)    →   [IMPLEMENT]
REVIEWING                →   PICK (更新されない)    →   [REVIEW]
DONE                     →   PICK (更新されない)    →   [DONE]
```

### 設計上の問題

1. **Status と Step の二重管理**
   - `Status`: ドメインモデルで管理（状態遷移ロジック付き）
   - `Step`: 表示用フィールド（更新ロジックなし）
   - 両者の同期が取れていない

2. **Step の責務が不明確**
   - Status から導出可能な情報を別フィールドで持つ
   - DRY原則違反
   - 単一責任原則違反

3. **Turn番号の管理場所**
   - ExecutionState で管理されるが、初期値が 1
   - UseCase 層で increment されるが、タイミングが不適切

---

## 解決策の提案

### Option A: Step フィールドを削除し Status から導出（推奨）

**変更内容:**
1. `BaseTask.currentStep` フィールドを削除
2. `CurrentStep()` メソッドを Status ベースの計算メソッドに変更
3. `UpdateStatus()` 時に Step を自動計算

**メリット:**
- 単一責任原則に準拠
- Status と Step の不整合が発生しない
- コードが簡潔になる

**デメリット:**
- 既存の DB スキーマ変更が必要
- マイグレーションコストが発生

**実装例:**
```go
// internal/domain/model/task/task.go
type BaseTask struct {
    // currentStep フィールドを削除
    id          TaskID
    taskType    TaskType
    title       string
    description string
    status      Status
    // currentStep Step  // 削除
    parentID    *TaskID
    createdAt   Timestamp
    updatedAt   Timestamp
}

// CurrentStep returns the workflow step derived from current status
func (b *BaseTask) CurrentStep() Step {
    switch b.status {
    case StatusPending:
        return StepPick
    case StatusPicked:
        return StepPick // ピック完了時点
    case StatusImplementing:
        return StepImplement
    case StatusReviewing:
        return StepReview
    case StatusDone:
        return StepDone
    case StatusFailed:
        return StepDone // 失敗も Done として扱う
    default:
        return StepPick
    }
}
```

---

### Option B: UpdateStatus() で Step も更新（段階的移行）

**変更内容:**
1. `BaseTask.currentStep` フィールドは維持
2. `UpdateStatus()` メソッドを拡張して Step も同時更新
3. 将来的に Option A に移行

**メリット:**
- DB スキーマ変更不要
- 段階的な移行が可能
- リスクが低い

**デメリット:**
- Status と Step の二重管理が継続
- 将来的なリファクタリングが必要

**実装例:**
```go
// internal/domain/model/task/task.go
func (b *BaseTask) UpdateStatus(newStatus Status) error {
    if !newStatus.IsValid() {
        return errors.New("invalid status")
    }

    if !b.status.CanTransitionTo(newStatus) {
        return errors.New("invalid status transition from " + b.status.String() + " to " + newStatus.String())
    }

    b.status = newStatus

    // Step を Status に基づいて自動更新
    b.currentStep = b.deriveStepFromStatus(newStatus)

    b.updatedAt = model.NewTimestamp()
    return nil
}

func (b *BaseTask) deriveStepFromStatus(status Status) Step {
    switch status {
    case StatusPending:
        return StepPick
    case StatusPicked:
        return StepPick
    case StatusImplementing:
        return StepImplement
    case StatusReviewing:
        return StepReview
    case StatusDone:
        return StepDone
    case StatusFailed:
        return StepDone
    default:
        return StepPick
    }
}
```

---

### Turn番号の修正

**変更内容:**
1. `NewTurn()` の初期値を `0` に変更
2. または `run_turn_use_case.go` での increment タイミングを修正

**Option 1: 初期値を 0 に変更（推奨）**
```go
// internal/domain/model/value_object.go
func NewTurn() Turn {
    return Turn{value: 0}  // 0 に変更
}
```

**Option 2: Increment タイミングを修正**
```go
// internal/application/usecase/execution/run_turn_use_case.go
func (uc *RunTurnUseCase) Execute(ctx context.Context, input dto.RunTurnInput) (*dto.RunTurnOutput, error) {
    // ...
    currentTurn := execState.CurrentTurn.Value()
    // currentTurn++ を削除

    // 実行後に increment
    // currentSBI.IncrementTurn() を適切な場所で呼び出し
}
```

**推奨:** Option 1（初期値変更）
- シンプルで分かりやすい
- Turn 0 = 未実行、Turn 1 = 初回実行という意味論的に正しい表現

---

### Journal書き込みエラーの修正

**原因調査:**
1. 既存の journal.ndjson ファイルをチェック
2. 不完全な JSON 行を特定
3. ファイルロック機構の確認

**修正案:**
```go
// internal/infrastructure/persistence/file/journal_repository_impl.go
func (r *JournalRepositoryImpl) Append(ctx context.Context, record *repository.JournalRecord) error {
    // 1. ファイルロックの追加
    f, err := os.OpenFile(r.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to open journal file: %w", err)
    }
    defer f.Close()

    // flock でファイルロック取得
    if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
        return fmt.Errorf("failed to acquire file lock: %w", err)
    }
    defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

    // 2. JSON エンコード前にバリデーション
    jsonBytes, err := json.Marshal(record)
    if err != nil {
        return fmt.Errorf("failed to marshal record: %w", err)
    }

    // 3. Atomic write（一行まとめて書き込み）
    line := append(jsonBytes, '\n')
    if _, err := f.Write(line); err != nil {
        return fmt.Errorf("failed to write journal entry: %w", err)
    }

    // 4. fsync で確実に書き込み
    if err := f.Sync(); err != nil {
        return fmt.Errorf("failed to sync journal file: %w", err)
    }

    return nil
}

// 既存ファイルの修復ユーティリティ
func RepairJournalFile(filePath string) error {
    // 破損した journal.ndjson を読み込み、有効な行のみを抽出して再作成
    content, err := os.ReadFile(filePath)
    if err != nil {
        return err
    }

    lines := strings.Split(string(content), "\n")
    validLines := []string{}

    for i, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        // JSON としてパース可能かチェック
        var record repository.JournalRecord
        if err := json.Unmarshal([]byte(line), &record); err != nil {
            fmt.Fprintf(os.Stderr, "Skipping invalid line %d: %v\n", i+1, err)
            continue
        }

        validLines = append(validLines, line)
    }

    // バックアップ作成
    backupPath := filePath + ".backup." + time.Now().Format("20060102-150405")
    if err := os.Rename(filePath, backupPath); err != nil {
        return fmt.Errorf("failed to create backup: %w", err)
    }

    // 有効な行のみで再作成
    repaired := strings.Join(validLines, "\n") + "\n"
    if err := os.WriteFile(filePath, []byte(repaired), 0644); err != nil {
        return fmt.Errorf("failed to write repaired file: %w", err)
    }

    fmt.Printf("Journal file repaired. Backup saved to: %s\n", backupPath)
    return nil
}
```

---

### done.md レポート生成の実装

**変更内容:**
1. Status が DONE に遷移したときにレポートを生成
2. `.deespec/prompts/DONE.md` テンプレートを作成
3. `run_turn_use_case.go` の step 処理に "done" を追加

**実装:**
```go
// internal/application/usecase/execution/run_turn_use_case.go
func (uc *RunTurnUseCase) buildPromptWithArtifact(sbiEntity *sbi.SBI, step string, turn int, attempt int, artifactPath string) string {
    // ...

    // Determine template path based on step
    var templatePath string
    switch step {
    case "implement":
        templatePath = ".deespec/prompts/WIP.md"
    case "review":
        templatePath = ".deespec/prompts/REVIEW.md"
        data.ImplementPath = fmt.Sprintf(".deespec/specs/sbi/%s/implement_%d.md", sbiID, turn-1)
    case "force_implement":
        templatePath = ".deespec/prompts/REVIEW_AND_WIP.md"
    case "done":  // 追加
        templatePath = ".deespec/prompts/DONE.md"
        // 全ての implement と review を参照
        data.AllImplementPaths = uc.getAllImplementPaths(sbiID, turn)
        data.AllReviewPaths = uc.getAllReviewPaths(sbiID, turn)
    default:
        // Fallback to simple prompt if no template found
        return fmt.Sprintf("Execute step %s for SBI %s (turn %d, attempt %d)", step, sbiID, turn, attempt)
    }
    // ...
}
```

**DONE.md テンプレート例:**
```markdown
# Task Completion Report

You are completing the SBI task: {{.Title}}

**SBI ID**: {{.SBIID}}
**Final Turn**: {{.Turn}}
**Working Directory**: {{.WorkDir}}

## Context

Review all work completed during this task:

### Implementation Reports
{{range .AllImplementPaths}}
- {{.}}
{{end}}

### Review Reports
{{range .AllReviewPaths}}
- {{.}}
{{end}}

### Original Specification
- {{.SBIDir}}/spec.md

## Your Task

Create a comprehensive completion report that includes:

1. **Summary**: Brief overview of what was accomplished
2. **Implementation Approach**: Key design decisions and approach taken
3. **Changes Made**: High-level summary of code changes
4. **Challenges & Solutions**: Any obstacles encountered and how they were resolved
5. **Testing Notes**: Testing approach and results (if applicable)
6. **Future Considerations**: Any technical debt, follow-up items, or recommendations

Write this report to:

**Output File**: {{.ArtifactPath}}

Use the Write tool to create this file with your complete task completion report.
```

---

## 移行プラン

### Phase 1: 緊急修正（即座に実施）

**目標**: 最もクリティカルな問題を修正

**タスク:**
1. ✅ Turn番号の修正
   - `NewTurn()` の初期値を `0` に変更
   - または `run_turn_use_case.go` の increment ロジック修正

2. ✅ Step表示の修正（Option B採用）
   - `UpdateStatus()` で Step を自動更新
   - 最小限の変更でリスク低減

3. ✅ Journal修復ツールの作成
   - 破損した journal.ndjson を修復する CLI コマンド追加
   - `./deespec doctor journal --repair`

**期間**: 1-2日

**リスク**: 低

---

### Phase 2: done.mdレポート生成（短期）

**目標**: done.md レポートを生成できるようにする

**タスク:**
1. `.deespec/prompts/DONE.md` テンプレート作成
2. `run_turn_use_case.go` の step 処理に "done" 追加
3. Status=DONE 時にレポート生成ロジック追加
4. テスト実施

**期間**: 2-3日

**リスク**: 低

---

### Phase 3: Journal書き込みの堅牢化（中期）

**目標**: Journal書き込みエラーを根絶

**タスク:**
1. ファイルロック機構の実装
2. Atomic write の実装
3. エラーハンドリング強化
4. 既存 journal の自動修復機能
5. 統合テスト

**期間**: 1週間

**リスク**: 中（ファイルI/O関連の潜在的バグ）

---

### Phase 4: Stepフィールドリファクタリング（長期・オプション）

**目標**: Step を Status から完全に導出

**タスク:**
1. `BaseTask.currentStep` フィールド削除
2. `CurrentStep()` を計算メソッドに変更
3. DB スキーマ変更
4. マイグレーションスクリプト作成
5. 全テストの更新
6. 本番環境での移行

**期間**: 2-3週間

**リスク**: 高（DB スキーマ変更、データマイグレーション）

**判断基準:**
- Phase 1-3 で問題が解決するなら実施不要
- アーキテクチャ的な完全性を求める場合に実施

---

## 実装優先順位

### 優先度: 高（すぐに実施）

1. **Turn番号修正**: ユーザーの混乱を招くため即座に修正
2. **Step表示修正**: ワークフロー可視性の改善
3. **Journal修復**: データ損失を防ぐ

### 優先度: 中（1-2週間以内）

4. **done.mdレポート生成**: ユーザーリクエスト機能
5. **Journal堅牢化**: 長期的な安定性向上

### 優先度: 低（将来的検討）

6. **Stepフィールドリファクタリング**: アーキテクチャ改善（必須ではない）

---

## テスト計画

### Unit Tests

```go
// internal/domain/model/task/task_test.go
func TestUpdateStatus_UpdatesStepAutomatically(t *testing.T) {
    baseTask, _ := NewBaseTask(model.TaskTypeSBI, "Test", "Description", nil)

    // PENDING → currentStep should be PICK
    assert.Equal(t, model.StepPick, baseTask.CurrentStep())

    // PENDING → PICKED
    baseTask.UpdateStatus(model.StatusPicked)
    assert.Equal(t, model.StepPick, baseTask.CurrentStep())

    // PICKED → IMPLEMENTING
    baseTask.UpdateStatus(model.StatusImplementing)
    assert.Equal(t, model.StepImplement, baseTask.CurrentStep())

    // IMPLEMENTING → REVIEWING
    baseTask.UpdateStatus(model.StatusReviewing)
    assert.Equal(t, model.StepReview, baseTask.CurrentStep())

    // REVIEWING → DONE
    baseTask.UpdateStatus(model.StatusDone)
    assert.Equal(t, model.StepDone, baseTask.CurrentStep())
}
```

### Integration Tests

```go
// internal/application/usecase/execution/run_turn_use_case_test.go
func TestRunTurnUseCase_TurnNumbering(t *testing.T) {
    // 初回実行で Turn 1 になることを確認
    output, err := useCase.Execute(ctx, input)
    assert.NoError(t, err)
    assert.Equal(t, 1, output.Turn)

    // 2回目実行で Turn 2 になることを確認
    output, err = useCase.Execute(ctx, input)
    assert.NoError(t, err)
    assert.Equal(t, 2, output.Turn)
}

func TestRunTurnUseCase_GeneratesDoneReport(t *testing.T) {
    // Status を DONE に遷移させる
    // done.md が生成されることを確認
    expectedPath := ".deespec/specs/sbi/TEST-001/done.md"
    assert.FileExists(t, expectedPath)
}
```

---

## ロールバック計画

各 Phase で問題が発生した場合のロールバック手順:

### Phase 1 ロールバック
```bash
# Git でコミットを戻す
git revert <commit-hash>

# DB に変更がある場合（今回はなし）
# マイグレーションをロールバック
```

### Phase 2 ロールバック
```bash
# DONE.md テンプレートを削除
rm .deespec/prompts/DONE.md

# コードを revert
git revert <commit-hash>
```

### Phase 3 ロールバック
```bash
# Journal 実装を旧版に戻す
git revert <commit-hash>

# 破損したファイルを修復ツールで復元
./deespec doctor journal --repair
```

---

## モニタリング

修正後に以下をモニタリング:

1. **Turn番号の正確性**
   - ログで Turn 1 から開始することを確認
   - journal.ndjson の turn フィールドをチェック

2. **Step表示の正確性**
   - 各 Status 遷移時に正しい Step が表示されることを確認
   - PICK → IMPLEMENT → REVIEW → DONE の流れを検証

3. **Journal書き込みエラー率**
   - エラーログで "Failed to append journal entry" の発生頻度を追跡
   - 修正後にエラーがゼロになることを確認

4. **done.mdレポート生成率**
   - 完了タスクで done.md が生成される割合を確認
   - レポート内容の品質をサンプリングチェック

---

## まとめ

### 推奨アプローチ

1. **Phase 1**: Option B（UpdateStatusでStep更新）を即座に実施
2. **Phase 2**: done.mdレポート生成を1週間以内に実装
3. **Phase 3**: Journal堅牢化を2週間以内に実装
4. **Phase 4**: Stepリファクタリングは必要に応じて検討（オプション）

### 期待される効果

- ✅ Turn番号が正しく表示される（Turn 1から開始）
- ✅ ワークフロー進行状況が正確に可視化される
- ✅ Journal書き込みエラーが解消される
- ✅ done.mdレポートが自動生成される
- ✅ ユーザー体験が大幅に向上する

### 次のステップ

このドキュメントをレビューし、以下を決定:
1. Phase 1 の実装を承認
2. Option A vs Option B の最終決定
3. 実装スケジュールの確定
4. リソース配分の決定

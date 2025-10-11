# テストカバレッジ改善計画

**作成日**: 2025-10-11
**現在のカバレッジ**: 34.0%
**目標カバレッジ**: 50.0% (CI要件)

## 1. 現状分析

### 1.1 全体像

- **総ディレクトリ数**: 70
- **テストが存在するディレクトリ**: 42 (60%)
- **テストが存在しないディレクトリ**: 28 (40%)
- **総ソースコード行数**: 約34,326行

### 1.2 カバレッジ分布

#### 高カバレッジ（70%以上） ✅
- `internal/hello`: 100.0%
- `internal/domain/model/sbi`: 97.7% ⭐ **今回追加**
- `internal/validator/agents`: 97.0%
- `internal/validator/state`: 97.1%
- `internal/infra/config`: 90.3%
- `internal/validator/health`: 89.6%
- `internal/workflow`: 88.9%
- `internal/util`: 81.2%
- `internal/interface/cli/workflow_config`: 81.4%
- `internal/adapter/gateway/storage`: 80.7%
- `internal/domain/model/label`: 77.8%
- `internal/interface/cli/workflow_sbi`: 77.5%
- `internal/validator/*`: 71-97%
- `internal/interface/cli/register`: 71.6%

#### 中カバレッジ（31-69%） ⚠️
- `internal/infra/fs`: 58.3% ⭐ **今回改善**
- `internal/infra/fs/txn`: 65.4%
- `internal/infra/persistence/file`: 65.0%
- `internal/application/workflow`: 63.5%
- `internal/pkg/specpath`: 56.1%
- `internal/infrastructure/di`: 51.7%
- `internal/runner`: 45.2%
- `internal/infrastructure/transaction`: 39.4%
- `internal/interface/cli/claude_prompt`: 36.0%
- `internal/app`: 35.2%
- `internal/adapter/presenter`: 33.1%

#### 低カバレッジ（1-30%） 🔴
- `internal/domain/execution`: 27.7%
- `internal/adapter/gateway/agent`: 25.3%
- `internal/application/service`: 23.2%
- `internal/infrastructure/persistence/sqlite`: 21.5%
- `internal/infrastructure/repository`: 17.9%
- `internal/interface/cli/sbi`: 16.7%
- `internal/interface/cli/clear`: 15.4%
- `internal/interface/cli/run`: 14.4%
- `internal/interface/cli/doctor`: 7.9%

#### 未テスト（0%） ❌
**最も問題のあるエリア** - 約13,000行が未テスト

##### Application Usecase層（2,907行）
- `internal/application/usecase/execution/run_turn_use_case.go` (801行) 🔥
- `internal/application/usecase/register_sbi_usecase_helpers.go` (786行)
- `internal/application/usecase/task/task_use_case_impl.go` (498行)
- `internal/application/usecase/workflow/workflow_use_case_impl.go` (358行)
- `internal/application/usecase/register_sbi_usecase.go` (253行)
- `internal/application/usecase/dry_run_usecase.go` (211行)

##### Domain Model層（1,578行）
- `internal/domain/model/value_object.go` (248行)
- `internal/domain/model/task/task.go` (219行)
- `internal/domain/model/pbi/pbi.go` (208行)
- `internal/domain/model/epic/epic.go` (200行)
- `internal/domain/model/lock/*.go` (239行)

##### CLI Interface層（約3,000行）
- `internal/interface/cli/doctor/doctor.go` (1,044行) 🔥
- `internal/interface/cli/run/run.go` (850行) 🔥
- `internal/interface/cli/label/label_cmd.go` (512行)
- `internal/interface/cli/register/register_compat.go` (479行) ⚠️ **互換性コード**
- その他多数

##### Controller層（1,236行）
- `internal/adapter/controller/cli/sbi_controller.go` (294行)
- `internal/adapter/controller/cli/workflow_controller.go` (285行)
- `internal/adapter/controller/cli/pbi_controller.go` (280行)
- `internal/adapter/controller/cli/epic_controller.go` (261行)

## 2. 問題の根本原因

### 2.1 アーキテクチャ的な問題

1. **依存関係の複雑さ**
   - Application Usecase層がRepositoryやGatewayに強く依存
   - モックやテストダブルの作成が困難
   - 統合テストのセットアップが複雑

2. **大きすぎるファイル**
   - 800行を超えるファイルが3つ存在
     - `run_turn_use_case.go` (801行)
     - `run.go` (850行)
     - `doctor.go` (1,044行)
   - 単一責任の原則に違反している可能性

3. **テスト容易性の欠如**
   - インターフェースの不足
   - 依存性注入の不徹底
   - 副作用が多い関数（ファイルI/O、外部API呼び出し）

### 2.2 技術的負債

1. **後方互換性コード**
   - `register_compat.go` (479行) - "backward compatibility" コメントが多数
   - 古いAPIと新しいAPIが混在
   - 重複したロジック

2. **テストファーストでない開発**
   - 機能実装後にテストを追加する文化
   - テスト可能な設計が考慮されていない

3. **不完全なドメインモデル**
   - `epic.go`, `pbi.go`, `task.go`などにテストがない
   - ドメインロジックの正確性が検証されていない

## 3. カバレッジ改善の戦略

### 3.1 優先順位付け

#### Priority 1: ドメインモデル層（高ROI） ⭐
**理由**:
- ビジネスロジックの中核
- 依存関係が少なくテストが容易
- バグの影響範囲が大きい

**対象ファイル**（約1,085行）:
- `internal/domain/model/value_object.go` (248行)
- `internal/domain/model/task/task.go` (219行)
- `internal/domain/model/pbi/pbi.go` (208行)
- `internal/domain/model/epic/epic.go` (200行)
- `internal/domain/model/lock/*.go` (239行)

**期待されるカバレッジ向上**: +2-3%

#### Priority 2: Infrastructure Repository層（中ROI） ⭐
**理由**:
- データアクセスロジックの検証が重要
- SQLite実装のテストは実ファイルを使って可能

**対象ファイル**:
- `internal/infrastructure/repository/*_impl.go` (低カバレッジの部分)

**期待されるカバレッジ向上**: +2-3%

#### Priority 3: Application Service層（中ROI）
**理由**:
- ビジネスロジックの調整層
- モックを使えばテスト可能

**対象ファイル**:
- `internal/application/service/*` (現在23.2%)

**期待されるカバレッジ向上**: +3-5%

#### Priority 4: 小さなCLIコマンド（低ROI、高effort）
**理由**:
- ユーザー体験に直結
- ただし統合テストが必要で工数が大きい

**対象ファイル**:
- `internal/interface/cli/health/*.go`
- `internal/interface/cli/version/*.go`
- `internal/interface/cli/status/*.go`

**期待されるカバレッジ向上**: +1-2%

### 3.2 段階的改善プラン

#### Phase 1: Quick Wins（1-2日）
**目標**: 34.0% → 40.0%

1. **ドメインモデルのテスト追加**
   ```
   ✅ internal/domain/model/sbi/ (97.7% 達成済み)
   - internal/domain/model/value_object.go
   - internal/domain/model/task/task.go
   - internal/domain/model/pbi/pbi.go
   - internal/domain/model/epic/epic.go
   - internal/domain/model/lock/*.go
   ```

2. **小さなCLIコマンドのテスト**
   ```
   - internal/interface/cli/version/
   - internal/interface/cli/health/
   ```

**実装メモ**:
- SBIモデルのテストパターンを他のドメインモデルに適用
- 値オブジェクトは単純なユニットテストで十分

#### Phase 2: Infrastructure Testing（2-3日）
**目標**: 40.0% → 45.0%

1. **Repository実装のテスト強化**
   ```
   - internal/infrastructure/repository/*_repository_impl.go
     (現在17.9% → 目標50%以上)
   ```

2. **Application Service層の部分的テスト**
   ```
   - internal/application/service/prompt_builder_service.go
   - internal/application/service/lock_service.go
   ```

**実装メモ**:
- テスト用のSQLiteインメモリDBを使用
- モックリポジトリを活用

#### Phase 3: Usecase Layer Refactoring（3-5日）
**目標**: 45.0% → 50.0%+

1. **大きなファイルのリファクタリング**
   ```
   - run_turn_use_case.go (801行) を分割
   - テスト可能な小さな関数に分解
   ```

2. **Usecase層の部分的テスト追加**
   ```
   - register_sbi_usecase.go の主要パスをテスト
   - task_use_case_impl.go の主要機能をテスト
   ```

**実装メモ**:
- 大きな関数を小さな純粋関数に分解
- 副作用を分離
- インターフェースを追加してモック可能にする

### 3.3 技術的負債の返済

#### 後方互換性コードの整理

**対象**:
- `internal/interface/cli/register/register_compat.go` (479行)

**アクション**:
1. 現在の使用箇所を特定
2. 新しいAPIへの移行計画を立てる
3. 段階的に古いコードを削除
4. または、テストを追加して現状を維持

#### 大きなファイルの分割

**対象**:
- `run_turn_use_case.go` (801行)
- `run.go` (850行)
- `doctor.go` (1,044行)

**アクション**:
1. 責任ごとに関数をグループ化
2. 小さな関数やヘルパーを別ファイルに抽出
3. 各部分にテストを追加

## 4. 推奨アクション

### 即座に実行可能（今日）

1. **ドメインモデルテストの完成**
   - `value_object.go`のテスト
   - `task.go`のテスト
   - `pbi.go`のテスト
   - `epic.go`のテスト

   **期待効果**: +3-4%カバレッジ

2. **小さなCLIコマンドのテスト**
   - version, health, status コマンド

   **期待効果**: +0.5-1%カバレッジ

### 短期（今週中）

1. **Repository層のテスト強化**
   - 17.9% → 50%を目指す

   **期待効果**: +3-4%カバレッジ

2. **Application Service層の部分テスト**
   - 23.2% → 40%を目指す

   **期待効果**: +3-5%カバレッジ

**合計期待効果**: 34.0% → 44-48%

### 中期（2週間以内）

1. **Usecase層のリファクタリングとテスト**
   - 大きなファイルの分割
   - テスト可能な設計への変更

   **期待効果**: +5-8%カバレッジ

**最終目標**: 50%以上達成

## 5. テスト戦略ガイドライン

### 5.1 レイヤー別テスト方針

#### Domain Model層
- **方針**: ユニットテスト（依存なし）
- **ツール**: 標準`testing`パッケージ
- **カバレッジ目標**: 90%以上

#### Infrastructure層
- **方針**: 統合テスト（実ファイル/DB使用）
- **ツール**: `testify`, SQLite in-memory
- **カバレッジ目標**: 60%以上

#### Application層
- **方針**: ユニットテスト（モック使用）
- **ツール**: `testify/mock`, インターフェース
- **カバレッジ目標**: 50%以上

#### Interface/CLI層
- **方針**: E2Eテスト（一部のみ）
- **ツール**: 実コマンド実行
- **カバレッジ目標**: 30%以上（主要パスのみ）

### 5.2 テストの優先順位

1. **Critical Path**: ユーザーが最も使う機能
2. **Business Logic**: 金額計算、状態遷移など
3. **Error Handling**: エラーケース
4. **Edge Cases**: 境界値テスト

### 5.3 テストしないもの

以下は投資対効果が低いため、テスト優先度を下げる：

- main関数やCLIエントリーポイント
- 単純なゲッター/セッター
- ロギングのみの関数
- 明らかに動作する定数定義

## 6. メトリクスとKPI

### 追跡すべき指標

- **総合カバレッジ**: 現在34.0% → 目標50.0%
- **レイヤー別カバレッジ**:
  - Domain: 現在約40% → 目標90%
  - Infrastructure: 現在約30% → 目標60%
  - Application: 現在約15% → 目標50%
  - Interface: 現在約20% → 目標30%

### 進捗確認

週次でカバレッジレポートを確認：
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

## 7. リソースとツール

### 必要なツール

```bash
# カバレッジ可視化
go install github.com/axw/gocov/gocov@latest
go install github.com/AlekSi/gocov-xml@latest

# テストヘルパー
go get github.com/stretchr/testify
```

### 参考リソース

- [Effective Go Testing](https://golang.org/doc/effective_go#testing)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Testing with Mocks](https://blog.golang.org/gomock)

## 8. まとめ

### 現状の問題

1. **構造的問題**: Application Usecase層（2,907行）が完全に未テスト
2. **技術的負債**: 後方互換性コード（479行）と巨大ファイル（800行超が3つ）
3. **テスト文化**: テストファーストでない開発プラクティス

### 50%達成への道筋

1. **Phase 1** (Quick Wins): ドメインモデル → 40%
2. **Phase 2** (Infrastructure): Repository/Service → 45%
3. **Phase 3** (Refactoring): Usecase層の部分改善 → 50%+

### 次のステップ

✅ **今回完了**:
- Domain Model (SBI) のテスト追加 (97.7%)
- Infrastructure (Journal Repository) のテスト追加
- Atomic functions のテスト追加
- カバレッジ: 32.9% → 34.0% (+1.1%)

🎯 **次の作業**:
- 残りのDomain Modelテスト追加（value_object, task, pbi, epic）
- Repository層のテスト強化
- 小さなCLIコマンドのテスト

**推定工数**: Phase 1-2で3-5日、Phase 3で3-5日、合計6-10日
**達成可能性**: 高（段階的アプローチにより）

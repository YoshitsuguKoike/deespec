# CLI層ファイル分類と移動計画

internal/interface/cli配下のファイルで、コマンドとして外部公開されていないものをリストアップし、適切な配置場所を提案します。

## 凡例

- ✅ **CLI層に残すべき**: Cobraコマンド定義、フラグ解析、標準出力フォーマット
- ⚠️ **Application層へ移動**: ビジネスロジック、UseCase、Service
- 🔴 **Domain層へ移動**: ビジネスルール、エンティティ、Value Object
- 🔵 **Infrastructure層へ移動**: 永持化実装、外部API、ファイルI/O

## 分類結果

### 1. ✅ CLI層に残すべきファイル（Cobraコマンド）

| ファイル | 行数 | 内容 | 理由 |
|---------|------|------|------|
| root.go | - | ルートコマンド定義 | CLI entry point |
| init.go | 234 | `deespec init` コマンド | CLI command |
| status.go | 130 | `deespec status` コマンド | CLI command |
| health.go | - | `deespec health` コマンド | CLI command |
| journal.go | 125 | `deespec journal` コマンド | CLI command |
| state.go | 124 | `deespec state` コマンド | CLI command |
| clear_cmd.go | - | `deespec clear` コマンド | CLI command |
| lock_cmd.go | 318 | `deespec lock` コマンド | CLI command |
| label_cmd.go | 507 | `deespec label` コマンド群 | CLI command |
| doctor.go | 1134 | `deespec doctor` コマンド | CLI command |
| doctor_integrated.go | 138 | Integrated doctor | CLI command |
| workflow.go | - | `deespec workflow` コマンド | CLI command |
| sbi.go | - | `deespec sbi` コマンド群 | CLI command |
| run.go | 713 | `deespec run` コマンド | CLI command (すでに一部リファクタリング済み) |
| sbi_run.go | - | `deespec sbi run` コマンド | CLI command |
| effective_config.go | 282 | 設定表示コマンド | CLI command（設定表示） |

---

### 2. ⚠️ Application層へ移動すべきファイル（ビジネスロジック）

#### 2.1 ワークフロー管理（優先度：高）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| **run_manager.go** | `internal/application/workflow/manager.go` | **417** | ワークフロー並列実行管理 | **進行中** |
| **workflow_sbi.go** | `internal/application/workflow/sbi/sbi_runner.go` | **-** | SBIワークフローRunner実装 | **次のステップ** |
| workflow_config.go | `internal/application/workflow/config_loader.go` | 263 | ワークフロー設定読み込み | Application設定管理 |
| run_continuous.go | `internal/application/workflow/executor.go` | 232 | 継続実行ロジック | Application実行制御 |

#### 2.2 タスク選択・管理（優先度：高）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| **picker.go** | `internal/application/service/task_picker_service.go` | **675** | タスク選択ロジック（優先度計算、依存関係） | ビジネスロジック |
| incomplete.go | `internal/application/service/incomplete_sbi_service.go` | 463 | 不完全SBI管理 | ビジネスロジック |

#### 2.3 登録・バリデーション（優先度：中）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| register.go | `internal/application/usecase/sbi/register_sbi_use_case.go` | 831 | SBI登録UseCase | Application UseCase |
| register_policy.go | `internal/application/service/register_policy_service.go` | 357 | 登録ポリシー判定 | ビジネスルール |
| sbi_register.go | `internal/application/service/sbi_registration_service.go` | 334 | SBI登録サービス | Application Service |
| dry_run.go | `internal/application/service/dry_run_service.go` | 430 | ドライラン実行 | Application Service |

#### 2.4 プロンプト生成（優先度：中）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| claude_prompt.go | `internal/application/service/prompt_builder_service.go` | 404 | Claude Code プロンプト生成 | ビジネスロジック |

#### 2.5 ラベル管理（優先度：低）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| label_import.go | `internal/application/usecase/label/import_labels_use_case.go` | 261 | ラベル一括インポート | Application UseCase |
| label_validate.go | `internal/application/usecase/label/validate_labels_use_case.go` | 240 | ラベル整合性検証 | Application UseCase |

#### 2.6 その他（優先度：低）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| sbi_extract.go | `internal/application/service/sbi_extractor_service.go` | 249 | SBI情報抽出 | Application Service |
| clear.go | `internal/application/usecase/clear_state_use_case.go` | 371 | 状態クリア処理 | Application UseCase |
| notes.go | `internal/application/service/notes_service.go` | 239 | ノート管理 | Application Service |

---

### 3. 🔵 Infrastructure層へ移動すべきファイル（永続化・外部I/O）

| 現在の場所 | 移動先 | 行数 | 内容 | 理由 |
|-----------|--------|------|------|------|
| **stateio.go** | `internal/infrastructure/repository/state_file_repository.go` | **-** | 状態ファイルI/O（すでに`state_repository_impl.go`に一部移行済み） | ファイルI/O |
| run_tx.go | `internal/infrastructure/transaction/state_transaction.go` | 185 | 状態更新トランザクション | Infrastructure transaction |
| register_tx.go | `internal/infrastructure/transaction/register_transaction.go` | 144 | 登録トランザクション | Infrastructure transaction |
| log_buffer.go | `internal/infrastructure/logging/log_buffer.go` | 170 | ログバッファリング | Infrastructure logging |
| logger.go | `internal/infrastructure/logging/logger.go` | 169 | ロガー実装 | Infrastructure logging |
| logger_bridge.go | `internal/infrastructure/logging/logger_bridge.go` | - | ロガーブリッジ | Infrastructure logging |
| lease.go | `internal/infrastructure/lease/lease_manager.go` | - | リース管理 | Infrastructure（または Domain） |

---

### 4. 🔴 Domain層へ移動すべきファイル（ビジネスルール・Value Object）

現在のところ、CLI層には純粋なDomain層ファイルはほぼありません。
ただし、以下は検討の余地があります：

| 現在の場所 | 移動候補 | 内容 | 理由 |
|-----------|---------|------|------|
| register_policy.go（一部） | `internal/domain/policy/register_policy.go` | 登録可否判定ルール | Pure business rule |
| lease.go（一部） | `internal/domain/model/lease/lease.go` | リース期限計算 | Domain concept |

---

## 優先順位付き移行計画

### Phase 1: ワークフロー管理の分離（進行中）
- ✅ `run_manager.go` → Application層 **（現在作業中）**
- ⬜ `workflow_sbi.go` → Application層
- ⬜ `workflow_config.go` → Application層
- ⬜ `run_continuous.go` → Application層

### Phase 2: タスク選択ロジックの分離
- ⬜ `picker.go` (675行) → Application層
- ⬜ `incomplete.go` (463行) → Application層

### Phase 3: 登録ロジックの分離
- ⬜ `register.go` (831行) → Application層
- ⬜ `register_policy.go` (357行) → Application層
- ⬜ `sbi_register.go` (334行) → Application層

### Phase 4: プロンプト生成の分離
- ⬜ `claude_prompt.go` (404行) → Application層

### Phase 5: Infrastructure層の整理
- ⬜ `run_tx.go`, `register_tx.go` → Infrastructure層
- ⬜ `logger.go`, `log_buffer.go`, `logger_bridge.go` → Infrastructure層

---

## 移動後の理想的なディレクトリ構造

```
internal/
├─ application/
│  ├─ workflow/
│  │  ├─ runner.go              # WorkflowRunner interface
│  │  ├─ config.go              # WorkflowConfig
│  │  ├─ stats.go               # WorkflowStats
│  │  ├─ manager.go             # WorkflowManager（←run_manager.go）
│  │  ├─ config_loader.go       # 設定読み込み（←workflow_config.go）
│  │  ├─ executor.go            # 継続実行（←run_continuous.go）
│  │  └─ sbi/
│  │     └─ sbi_runner.go       # SBIWorkflowRunner（←workflow_sbi.go）
│  │
│  ├─ service/
│  │  ├─ task_picker_service.go         # タスク選択（←picker.go）
│  │  ├─ incomplete_sbi_service.go      # 不完全SBI管理（←incomplete.go）
│  │  ├─ register_policy_service.go     # 登録ポリシー（←register_policy.go）
│  │  ├─ sbi_registration_service.go    # SBI登録（←sbi_register.go）
│  │  ├─ dry_run_service.go             # ドライラン（←dry_run.go）
│  │  ├─ prompt_builder_service.go      # プロンプト生成（←claude_prompt.go）
│  │  ├─ sbi_extractor_service.go       # SBI抽出（←sbi_extract.go）
│  │  └─ notes_service.go               # ノート管理（←notes.go）
│  │
│  └─ usecase/
│     ├─ execution/
│     │  └─ run_turn_use_case.go    # すでに作成済み
│     ├─ sbi/
│     │  └─ register_sbi_use_case.go    # SBI登録UseCase（←register.go）
│     ├─ label/
│     │  ├─ import_labels_use_case.go   # ラベルインポート（←label_import.go）
│     │  └─ validate_labels_use_case.go # ラベル検証（←label_validate.go）
│     └─ clear_state_use_case.go        # 状態クリア（←clear.go）
│
├─ infrastructure/
│  ├─ repository/
│  │  ├─ state_repository_impl.go       # すでに作成済み
│  │  ├─ journal_repository_impl.go     # すでに作成済み
│  │  └─ state_file_repository.go       # ファイルI/O（←stateio.go）
│  │
│  ├─ transaction/
│  │  ├─ state_transaction.go           # 状態TX（←run_tx.go）
│  │  └─ register_transaction.go        # 登録TX（←register_tx.go）
│  │
│  ├─ logging/
│  │  ├─ logger.go                      # ロガー（←logger.go）
│  │  ├─ log_buffer.go                  # バッファ（←log_buffer.go）
│  │  └─ logger_bridge.go               # ブリッジ（←logger_bridge.go）
│  │
│  └─ lease/
│     └─ lease_manager.go               # リース管理（←lease.go）
│
├─ domain/
│  └─ policy/
│     └─ register_policy.go             # 登録ポリシー（Pure business rule部分）
│
└─ interface/
   └─ cli/
      ├─ root.go                        # ルートコマンド ✅
      ├─ init.go                        # init コマンド ✅
      ├─ status.go                      # status コマンド ✅
      ├─ run.go                         # run コマンド ✅
      ├─ sbi.go                         # sbi コマンド ✅
      ├─ sbi_run.go                     # sbi run コマンド ✅
      ├─ label_cmd.go                   # label コマンド ✅
      ├─ lock_cmd.go                    # lock コマンド ✅
      ├─ doctor.go                      # doctor コマンド ✅
      └─ ...（その他コマンドのみ）
```

---

## 統計

- **現在のCLI層ファイル数**: 約71ファイル（テスト含む）
- **コマンドファイル**: 約15ファイル（CLI層に残すべき）
- **移動すべきファイル**: 約25ファイル
  - Application層: 約17ファイル
  - Infrastructure層: 約8ファイル
  - Domain層: 約0ファイル（一部検討）

## 影響分析

### 削減されるCLI層の責任
- ビジネスロジック: 約3,000行
- Infrastructure実装: 約700行
- 合計: 約3,700行

### 期待される効果
1. **テスタビリティ向上**: Application層のロジックが独立してテスト可能に
2. **保守性向上**: 各ファイルが単一責任を持つ
3. **再利用性向上**: Application層のロジックが他のインターフェース（Web UI、gRPC）からも利用可能
4. **依存関係の明確化**: Clean Architectureに準拠した依存方向

---

## 次のアクション

現在進行中の **run_manager.go** の移動を完了後、以下の順で進めることを推奨します：

1. **picker.go** (675行) の移動 - 最も大きく、影響範囲が広い
2. **register.go** (831行) の移動 - 2番目に大きい
3. **claude_prompt.go** (404行) の移動 - プロンプト生成の分離
4. その他のApplication層ファイル
5. Infrastructure層のファイル整理

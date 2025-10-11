# CHANGELOG

本プロジェクトは [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) 形式に準拠し、[Semantic Versioning](https://semver.org/lang/ja/) に従います。

## \[Unreleased]

### 修正 (Fixed)

* **init コマンドの不要ファイル生成を削除**: 現在のバージョンで使用されていないファイルを生成しないように修正
  - 削除: 廃止された `prompts/system/` サブディレクトリ配下のファイル（implement.md, plan.md, review.md, test.md）
  - 追加: `prompts/DONE.md` テンプレート（タスク完了レポート生成用）
  - 削除: `specs/.gitkeep` と `specs/README.md`
  - 削除: `var/state.json`（DB-based 管理に移行済みのため）
  - 効果: よりクリーンな初期化、混乱を招く廃止ファイルを排除

---

## \[v0.2.1] - 2025-10-11

### 追加 (Added)

* **一元的なバージョン管理システム**: VERSION ファイルによる集中管理
  - プロジェクトルートに `VERSION` ファイルを配置
  - Makefile が VERSION ファイルからバージョンを自動読み込み
  - GitHub Actions release workflow も VERSION ファイルを参照
  - `make version` コマンドで現在のバージョンを確認可能
  - リリース時は VERSION ファイルを更新するだけで全てに反映
  - 効果: Git タグの手動管理が不要、バージョン管理が簡素化

### 変更 (Changed)

* **リリースワークフロー**: タグベースから VERSION ファイルベースへ移行
  - トリガー: VERSION ファイルの main ブランチへのプッシュで自動リリース
  - ビルド時に VERSION ファイルから ldflags 経由でバージョン注入
  - タグとリリースは GitHub Actions が自動作成
  - ワークフロー: VERSION更新 → commit → push → 自動リリース

### 修正 (Fixed)

* **Windowsクロスビルド対応**: プラットフォーム固有のシステムコールを抽象化
  - 問題: `syscall.Flock`と`syscall.SIGTSTP`がWindowsビルドでundefinedエラー
  - 修正: ビルドタグを使用してUNIX/Windows実装を分離
  - ファイル追加:
    - internal/infra/fs/flock_unix.go: UNIX系システム用のflock実装
    - internal/infra/fs/flock_windows.go: Windows用のno-op実装
    - internal/interface/cli/run/signal_unix.go: UNIX用シグナルハンドリング
    - internal/interface/cli/run/signal_windows.go: Windows用シグナルハンドリング
  - 効果: 全プラットフォーム (windows/linux/darwin × amd64/arm64) でビルド成功

* **強制終了ロジックの根本的なバグ修正**: 二重遷移問題を完全に解決
  - 問題: Turn 4で追加された修正が不完全で、新たな無効遷移バグを導入していた
  - 根本原因: サービス層が既に完了した遷移の後に再度遷移を試みていた
  - 修正: service.go 68-76行目の強制終了チェックブロック全体を削除
  - アプローチ: エンティティ層の`NextStep()`を唯一の真実の源として信頼
  - 効果: Turn 3で発見された二重遷移バグとTurn 4で導入された無効遷移バグの両方を解決
  - アーキテクチャ改善: Single Responsibility Principleに準拠、状態遷移ロジックをエンティティ層に集中
  - テスト結果: 全485個のテストケースが成功、カバレッジ95.9%維持
  - ファイル: internal/domain/execution/service.go (11行削除)

### 追加 (Added)

* **PBI管理システムの実装**: Markdown+SQLiteハイブリッドストレージによる新しいPBI管理
  - **CLI コマンド群**:
    - `deespec pbi register`: Markdownファイルまたは対話形式でPBIを登録
      - ファイル入力とメタデータオーバーライド対応（--story-points, --priority, --status）
      - シーケンシャルなPBI ID自動生成（PBI-001, PBI-002, ...）
      - 全入力パラメータのバリデーション
    - `deespec pbi show <id>`: PBI詳細を整形表示
      - ステータス、ストーリーポイント、優先度、タイムスタンプ表示
      - Markdown本文の全文表示
    - `deespec pbi list`: PBI一覧をテーブル形式で表示
      - ステータスフィルタリング対応
      - ソート可能な出力形式
    - `deespec pbi update <id>`: PBIメタデータを個別更新
      - ステータス、ストーリーポイント、優先度を独立して更新可能
      - Markdown本文は保持
      - タイムスタンプ自動更新
    - `deespec pbi edit <id>`: PBIのMarkdownファイルをエディタで編集
      - $EDITOR環境変数に対応（未設定時はvimを使用）
    - `deespec pbi delete <id>`: PBIと関連ファイルを削除
      - 確認プロンプト付き（--forceでスキップ可能）
      - データベースとMarkdownディレクトリの両方を削除
  - **ハイブリッドストレージモデル**:
    - Markdownファイル: PBIコンテンツの信頼できる情報源（.deespec/specs/pbi/{id}/pbi.md）
    - SQLite: 高速クエリのためのメタデータインデックス（status, priority, timestamps）
    - マイグレーションシステム: バージョン管理されたスキーマ進化
  - **ドメインモデル**:
    - ワークフロー層から分離された簡素化されたPBIエンティティ
    - 関心の分離による明確なリポジトリインターフェース
    - リポジトリベースのシーケンシャル番号付けによるID生成
  - **ユースケース**:
    - RegisterPBIUseCase: バリデーション付きPBI作成処理
    - UpdatePBIUseCase: フィールドレベル制御によるメタデータ更新管理
  - **マイグレーションサポート**:
    - 001_create_pbis.sql: 初期PBIテーブルスキーマ
    - 002_add_pbi_metadata.sql: メタデータカラム追加
    - 003_fix_markdown_path.sql: パス制約の修正
    - すべてのPBIコマンド実行時に自動的にマイグレーション実行

* **テストカバレッジの拡充**:
  - ドメイン実行テスト（decision, error, status, step, repository, service）
  - ストラテジーテスト（EPIC/PBI分解、SBIコード生成、実装）
  - リポジトリ実装テスト（notes, prompt templates, SBI tasks）
  - 古いワークフロー統合PBIテストを削除

* **ドキュメント整備**:
  - データベーススキーマドキュメント（docs/database_schema.md）
  - PBI実装計画書（docs/pbi_implement_plan.md）
  - テスト用サンプルPBI（samples/docs/test-coverage-plan.md）

### 非推奨 (Deprecated)

以下のワークフロー統合PBIコンポーネントは非推奨となりました:
- TaskUseCaseImpl.CreatePBI
- WorkflowUseCaseImpl.DecomposePBI
- PBIRepositoryImpl（旧SQLiteのみ実装）

スタブ実装により後方互換性を維持しています。

### 追加 (Added)

* **並列実行機能の完全実装**: Clean Architectureに基づく適切な関心の分離
  - **CLI層でのRunLock管理**: run.goが最上位でRunLockを1回取得・解放
  - **UseCaseからRunLock削除**: ビジネスロジックがプロセス間排他を意識しない設計
  - **並列実行専用関数**: ExecuteSingleSBI()とExecuteForSBI()の実装
  - **2層ロックシステム**:
    - RunLock: システム全体（Lock ID: "system-runlock"）
    - StateLock: SBI個別（Lock ID: "sbi-<SBI_ID>"）
  - **並列実行ログの改善**:
    - 📋 [Parallel] Found N executable SBIs: 実行候補一覧
    - 🚀 [Parallel #N] Starting/Completed: タスク番号付き進捗
    - ⏭️ Skipped理由表示: ファイル競合、エージェント使用中
    - ✅/❌ 完了状態とエラー: 実行時間表示
    - ✨ 全体サマリー: 成功/失敗の統計
  - **効果**:
    - 並列実行が正常動作（`--parallel 3`で3タスク同時実行成功）
    - 順次実行との完全な互換性維持
    - テスト容易性向上（UseCaseがRunLockのモック不要）
    - コード再利用性向上（実行モードに依存しない設計）
  - **アーキテクチャ**:
    ```
    CLI層: RunLock取得 → UseCase実行 → RunLock解放
    UseCase層: ビジネスロジックのみ（ロック不要）
    並列実行: CLI層で1回RunLock → 各SBIはStateLockのみ
    ```
  - **ファイル**:
    - internal/application/usecase/execution/run_turn_use_case.go
    - internal/interface/cli/run/run.go
    - internal/interface/cli/workflow_sbi/parallel_runner.go

### 追加 (Added)

* **テストカバレッジ強化**: Repository層の包括的なテスト追加
  - journal_repository_impl_test.go: 7つの新規テスト追加
    - 空タイムスタンプ正規化テスト
    - nilアーティファクト処理テスト
    - 並行追記テスト（50 goroutines）
    - 複雑なアーティファクト構造テスト
    - Unicode対応テスト
  - 新規テストファイル追加（7ファイル、8,852行）
    - fbdraft_repository_test.go: FBドラフトリポジトリテスト
    - journal_repository_test.go: ジャーナルリポジトリテスト
    - label_repository_test.go: ラベルリポジトリテスト
    - lock_repository_test.go: ロックリポジトリテスト
    - epic_repository_impl_test.go: EPIC実装テスト
    - label_repository_impl_test.go: ラベル実装テスト
  - lock repository tests: run_lock/state_lockの並行操作テスト

### 修正 (Fixed)

* **ロック取得時の競合状態修正**: アトミックなロック獲得による競合排除
  - 問題: 並列/順次実行で「lock not found」エラーが発生
  - 根本原因: Check-Then-Act競合状態
    1. プロセスA: ロック存在確認 → 古いロック検出
    2. プロセスB: 同じロックを削除
    3. プロセスA: Release()実行 → ERROR: lock not found
  - 修正: アトミックなロック獲得パターン実装
    - 古いロック削除時に「not found」エラーを許容
    - 削除後に再度ロック存在を確認（他プロセスが再作成した場合を検出）
    - INSERT時にUNIQUE制約違反を検出（並行挿入の検出）
  - 追加: `isUniqueConstraintError()`/`isStateLockUniqueConstraintError()`ヘルパー
  - 効果: 並列/順次実行モードで同じロック判定ロジックを共有し、安定動作を実現
  - ファイル:
    - internal/infrastructure/persistence/sqlite/run_lock_repository_impl.go
    - internal/infrastructure/persistence/sqlite/state_lock_repository_impl.go

* **並列実行モードでの古いロック検出機能追加**: プロセス存在チェックの実装
  - 問題: 並列実行（`--parallel`）が「another instance is already running」でブロック
  - 原因: ロック取得時に期限（expires_at）のみチェック、プロセス存在を確認せず
  - 事例: PID 66976のプロセスは存在しないが、期限が未来のためロックが有効と判断
  - 順次実行との差: 順次実行は`handleLockConflict()`で別途プロセスチェック実施
  - 修正: `isProcessRunning(pid)`ヘルパー追加（`ps -p <pid>`で確認）
  - ロジック変更: `isStale = IsExpired() OR !isProcessRunning(PID)`
  - 影響: 並列実行モードでもクラッシュしたプロセスのロックを自動クリーンアップ
  - ファイル:
    - internal/infrastructure/persistence/sqlite/run_lock_repository_impl.go
    - internal/infrastructure/persistence/sqlite/state_lock_repository_impl.go

* **ターン制限超過時のステータス遷移バグ修正**: 無限ループとランタイムエラーを解消
  - 問題: ターン制限（maxTurns）超過時、REVIEWING → REVIEWING への不正な遷移を試行
  - エラー: "invalid status transition from REVIEWING to REVIEWING"
  - 修正: ターン制限超過時は明示的にDONEステータスへ遷移
  - 影響: Turn 8でDECISION=PENDINGの場合、Turn 9で強制終了が正常に動作
  - ファイル: internal/application/usecase/execution/run_turn_use_case.go:136

### 変更 (Changed)

* **ドキュメント構造改善**: PBI設計ドキュメントをモジュール化
  - pbi_how_to_work.md: メイン概要とリンクのみに簡素化（535行削減）
  - pbi_how_to_work_01.md: ファイルベース vs コマンド引数の設計比較
  - pbi_how_to_work_02.md: 追加の設計考慮事項
  - pbi_how_to_work_03.md: 実装ガイドラインとGoコード例
  - 目的: ドキュメントの保守性と可読性の向上

### 追加 (Added)

* **done.mdレポート生成機能 (Phase 2)**:
  - `.deespec/prompts/DONE.md` テンプレート追加: タスク完了時の包括的レポート生成
  - Status=DONE遷移時に自動的にdone.mdを生成
  - `AllImplementPaths`, `AllReviewPaths`フィールド追加: 全実装・レビュー履歴を参照可能
  - `collectImplementPaths()`, `collectReviewPaths()`: 過去の全アーティファクトパス収集関数
  - done.mdファイルは`done.md`として保存（turnサフィックスなし）
  - テンプレート変数: `{{.AllImplementPaths}}`, `{{.AllReviewPaths}}`追加
  - 生成失敗時は警告のみで処理継続（非致命的エラー）

* **Journal書き込み堅牢化 (Phase 3)**:
  - **NDJSON形式への完全移行**:
    - JSON配列形式からNewline Delimited JSON形式に変更
    - `AppendNDJSONLine()` 関数実装: 行単位のアトミック追記
    - O(1)の追記操作で大量エントリでもパフォーマンス劣化なし
  - **ファイルロック機構の実装**:
    - `syscall.Flock`による排他ロック（LOCK_EX）実装
    - 並行書き込み時のファイル破損を完全防止
    - ロック取得・解放の確実な管理（defer使用）
  - **Atomic Write + fsync実装**:
    - `O_APPEND`フラグによるPOSIXアトミック書き込み
    - `FsyncFile()`による永続化保証（クラッシュリカバリ対応）
    - 3層防御: OS層（flock）、FS層（O_APPEND）、アプリ層（fsync）
  - **journal_repository_impl.go完全書き換え**:
    - `Append()`: NDJSON形式での追記（ファイルロック付き）
    - `Load()`: 行単位読み込み、破損行は警告してスキップ
    - 後方互換性: `timestamp`/`ts`両フィールド対応
    - エラー耐性: 破損したjournal.ndjsonでも有効行のみ処理継続
  - **journal修復機能（既存実装の確認）**:
    - `doctor --repair-journal`: 破損したjournal.ndjsonの自動修復
    - バックアップ作成後、有効な行のみで再構築
    - タイムスタンプ付きバックアップファイル生成

* **ワークフロー改善ドキュメント**:
  - `docs/workflow_step_improvements.md`: Phase 1-4の詳細な実装計画
  - 問題分析、解決策提案、移行プラン、テスト計画を記載
  - Phase 1（Turn番号・Step表示修正）: 完了
  - Phase 2（done.mdレポート生成）: 完了
  - Phase 3（Journal堅牢化）: 完了
  - Phase 4（Stepフィールドリファクタリング）: オプション（実施不要と判断）

### 追加 (Added)

* **clearコマンドのDB対応**: データベースデータの物理削除機能を追加
  - `clearDatabase()`: 全タスクテーブル（sbis, pbis, epics等）の物理削除
  - トランザクション内で安全に削除（task_labels, epic_pbis, pbi_sbis → sbis, pbis, epics の順序）
  - run_locks, state_locksも削除してクリーンな状態に
  - labelsとschema_migrationsは保持（グローバルデータ）
  - `sbi list`/`sbi history`での不整合を解消

### 変更 (Changed)

* **clearコマンドのstate.json管理改善**: deprecated警告の完全除去
  - `LoadState()`と`SaveStateCAS()`を直接ファイルI/Oに置き換え
  - `checkNoWIP()`: `os.ReadFile()`と`json.Unmarshal()`で直接読み込み
  - `resetStateFiles()`: state.jsonを削除（DB-based管理に完全移行）
  - deprecated警告が一切表示されなくなりました

### 削除 (Removed)

* **state.json関連の削除**: DB-based state managementへの完全移行
  - `internal/app/state/loader.go`: 削除
  - `internal/app/state/writer.go`: 削除
  - `internal/app/state/state_test.go`: 削除
  - `internal/domain/repository/state_repository.go`: 削除
  - `internal/infrastructure/repository/state_repository_impl.go`: 削除
  - `internal/interface/cli/common/state_stub.go`: 削除（deprecatedスタブ）

### 追加 (Added)

* **プロンプトシステムのテンプレート化 (OptionA + OptionB完了)**:
  - **OptionA: 既存コンテキスト読み込み指示の実装**
    - `buildPriorContextInstructions()`: AIエージェントに既存アーティファクト読み込みを指示
    - 過去の実装・レビュー履歴を参照することで探索時間を5-10分から30秒に短縮
    - Turn 1では`spec.md`のみ、Turn 2以降は実装・レビューファイルも読み込み指示
  - **OptionB: テンプレートベースプロンプトシステムへの移行**
    - `text/template`パッケージを使用した動的プロンプト生成システム
    - `PromptTemplateData`構造体による変数管理（SBIID、Title、Turn、Attempt等）
    - `expandTemplate()`: テンプレートファイル展開関数の実装
    - `buildFallbackPrompt()`: テンプレート読み込み失敗時の後方互換機構
    - テンプレート変数: `{{.PriorContext}}`, `{{.WorkDir}}`, `{{.SBIID}}`, `{{.Title}}`, `{{.Description}}`, `{{.Turn}}`, `{{.Attempt}}`, `{{.ArtifactPath}}`, `{{.ImplementPath}}`
  - **プロンプトテンプレートの整備**
    - `.deespec/prompts/WIP.md`: 実装ステップ用テンプレート
    - `.deespec/prompts/REVIEW.md`: レビューステップ用テンプレート
    - `.deespec/prompts/REVIEW_AND_WIP.md`: 強制実装ステップ用テンプレート
    - すべてのテンプレートに`{{.PriorContext}}`を先頭に配置
    - システム整合性保護制約の追加（.deespecディレクトリ変更禁止）
  - **効果**:
    - AIエージェントの探索時間を大幅短縮（既存コンテキストの事前読み込み）
    - プロンプト保守性の向上（コードとテンプレートの分離）
    - 柔軟な変数展開によるコンテキスト提供の最適化

### 変更 (Changed)

* **プロンプトシステムの刷新**:
  - workflow.yamlベースのシステムから完全撤廃
  - テンプレートベースシステムへの移行完了
  - `buildPromptWithArtifact()`関数の完全リファクタリング

### 削除 (Removed)

* **workflow.yamlシステムの削除**:
  - `.deespec/etc/workflow.yaml`ファイルの削除
  - `.deespec/prompts/system/`ディレクトリの削除
  - `paths.Workflow`フィールドの削除（paths.go）
  - `runDoctorValidationJSON()`からworkflow検証ロジック削除
  - workflow関連の未使用import削除（context、workflow package）
  - validatePlaceholders系関数の削除（workflow専用だったため）

### 追加 (Added)

* **ULID順序問題の修正とタスク管理CLI実装 (Phase 1 + Phase 2)**:
  - **Phase 1: データベーススキーマ拡張**
    - `sbis`テーブルに`sequence INTEGER`, `registered_at DATETIME`フィールド追加
    - `idx_sbis_ordering`インデックス作成: `(priority DESC, registered_at ASC, sequence ASC)`
    - マイグレーションVersion 4実装（既存データの自動バックフィル対応）
  - **Phase 1: Domain/Repository/Application層更新**
    - `SBIMetadata`に`Sequence`, `RegisteredAt`追加（sbi.go）
    - `GetNextSequence()`, `ResetSBIState()`メソッド実装（sbi_repository.go）
    - `CreateSBI()`内でトランザクション内sequence自動設定（task_use_case_impl.go）
    - Picker順序ロジック修正: priority DESC → registered_at ASC → sequence ASC
  - **Phase 1: 統合テスト成功**
    - 3つのSBI登録でsequence自動採番確認（1, 2, 3）
    - 優先度別ソート動作確認（Task B[pri=1] → Task A[pri=0,seq=1] → Task C[pri=0,seq=3]）
  - **Phase 2: タスク管理CLI実装**
    - `sbi list`: SBI一覧表示（優先度順ソート、テーブル/JSON形式）
    - `sbi show <id>`: SBI詳細表示（Sequence/RegisteredAt/実行状態含む）
    - `sbi reset <id>`: ステータスリセット（誤完了時の再実行対応、確認プロンプト付き）
    - `sbi history <id>`: journal.ndjsonから実行履歴表示
  - **効果**:
    - ULID順序問題の完全解決（登録順序を確実に保証）
    - Claude Code認証問題などでの誤進行に対処可能
    - ユーザーが直感的にタスク管理できるCLI提供
  - **実装ファイル**:
    - Schema: `schema.sql`, `migrations/004_add_ordering_fields.sql`
    - Domain: `internal/domain/model/sbi/sbi.go`
    - Repository: `internal/infrastructure/persistence/sqlite/sbi_repository_impl.go`
    - UseCase: `internal/application/usecase/task/task_use_case_impl.go`
    - CLI: `internal/interface/cli/sbi/{sbi_list,sbi_show,sbi_reset,sbi_history}.go`

### 追加 (Added)

* **SQLite WALモード有効化 (Phase 0-2)**: 並行アクセス対応のための基盤整備
  - **WALモード有効化**: `_journal_mode=WAL`パラメータ追加（container.go:156）
  - **検証ロジック**: `PRAGMA journal_mode`クエリによる有効化確認（container.go:162-169）
  - **並行アクセステスト**: 複数コンテナの同時DB接続を確認（container_test.go:374-447）
    - `run`実行中に`register`が成功することを保証
    - 2つのコンテナが同時にロックを取得可能
  - **WALファイル生成テスト**: `-wal`と`-shm`ファイルの物理的生成を確認（container_test.go:328-372）
  - **パフォーマンステスト**: ベンチマークによる性能評価
    - 単一スレッド: ~100μs/op
    - 並行実行: ~123μs/op（23%のオーバーヘッド）
  - **技術ドキュメント**: WALモード実装ガイド作成（docs/architecture/wal-mode-implementation.md）
    - WALモードの仕組みと利点の詳細説明
    - トラブルシューティングガイド
    - パフォーマンス測定結果の記録
  - **効果**:
    - `deespec run`と`deespec register`の同時実行が可能
    - データベースロック競合の大幅削減
    - 複数SBI並行処理（Phase 2）への準備完了

* **Clean Architecture + DDD リファクタリング Phase 9.1完了**: Label System SQLite化
  - **Phase 9.1a - Schema Extension**: SQLiteスキーマ拡張
    - `labels` テーブル: ラベル本体（content_hashes, line_count, last_synced_at追加）
    - `task_labels` テーブル: タスク-ラベル多対多関連
    - パフォーマンス最適化用インデックス（name, parent, is_active, last_synced）
    - マイグレーションバージョン3追加
  - **Phase 9.1b - Configuration**: setting.json拡張
    - `LabelConfig`: template_dirs, import, validation設定
    - デフォルトディレクトリ: `.claude`, `.deespec/prompts/labels`
    - 1000行制限、除外パターン対応
    - 5/5評価（完璧な実装）
  - **Phase 9.1c - Domain/Repository層**: Label実体とRepository実装
    - `Label` entity: SHA256ハッシュベースの整合性管理（196行）
    - `LabelRepository` interface: CRUD + 整合性検証（48行）
    - SQLite実装: ファイル解決、ハッシュ計算、検証（653行）
    - DI Container統合: lazy initialization対応
    - 9個のunit tests（全成功）
    - 5/5評価（完璧な実装）
  - **Phase 9.1d - CLI層**: labelコマンド群実装
    - `label register`: 新規ラベル登録（複数テンプレート対応）
    - `label list`: 一覧表示（table/JSON形式）
    - `label show`: 詳細情報表示
    - `label update`: プロパティ更新（activate/deactivate）
    - `label delete`: ラベル削除（確認プロンプト付き）
    - `label attach/detach`: タスク関連付け管理
    - `label templates`: テンプレートファイル表示
    - `label import`: ディレクトリから一括インポート（261行）
      - 再帰的スキャン、dry-run、prefix-from-dir対応
      - 除外パターンマッチング（glob + wildcard）
      - 1000行制限検証
    - `label validate`: ファイル整合性検証（240行）
      - SHA256ハッシュベースの検証
      - 3種類のステータス（OK, MODIFIED, MISSING）
      - --sync自動同期、--details詳細表示
    - 合計1,008行の実装
    - 5/5評価（完璧な実装）
  - **Phase 9.1e - EnrichTaskWithLabels改善**: Repository統合
    - ClaudeCodePromptBuilder に LabelRepository統合
    - ファイルベース→Repositoryベースへ移行
    - Runtime整合性検証（AI実行時に自動チェック）
    - 3種類の警告表示（not found, modified, missing）
    - Graceful Degradation: DB未登録時はfile-based fallback
    - 完璧な後方互換性（Null Object Pattern）
    - 4/5評価（優れた実装、軽微な改善点あり）
  - **Phase 9.1f - Testing & Documentation**: E2Eテストとドキュメント整備
    - E2Eテスト: 7テストケース実装（全成功）
    - ユーザーガイド: 包括的な使用方法（label-system-guide.md）
    - マイグレーションガイド: 3つの移行戦略（label-system-migration-guide.md）
    - 古いテストファイル削除（label_cmd_test.go等）
  - **主要機能**:
    - File-as-Source-of-Truth: ファイルが正、DBはインデックス
    - SHA256 hash-based integrity: ファイル変更の自動検出
    - Multiple template directories: 優先順位付き解決
    - Hierarchical labels: ディレクトリ構造のサポート
    - 1000-line limit validation: 大容量ファイルの制限
    - Runtime validation: AI実行時の整合性チェック
  - **アーキテクチャ成果**:
    - Clean Architecture + DDD完全遵守
    - Repository Pattern適用
    - Backward compatibility維持（既存ワークフロー影響なし）
    - Production-Ready CLI設計（dry-run, auto-fix, tip messages）

* **Clean Architecture + DDD リファクタリング Phase 7完了**: Lock System SQLite移行
  - **Domain層**: Lock Models実装
    - `LockID` value object（27行）
    - `RunLock` entity: SBI実行ロック（94行）
    - `StateLock` entity: state.jsonアクセスロック（93行）
    - IsExpired(), UpdateHeartbeat()メソッド実装
  - **Repository層**: Lock永続化
    - `RunLockRepository` interface & SQLite実装（280行、72-86% coverage）
    - `StateLockRepository` interface & SQLite実装（259行、72-84% coverage）
    - Transaction context propagation対応
    - 17個のunit tests（全成功）
  - **Application層**: Lock Service実装
    - `LockService`: 統一されたLock管理API（318行、83.7% coverage）
    - Heartbeat自動送信（30秒間隔）
    - 期限切れLock自動削除（60秒間隔）
    - Start/Stop lifecycle管理
    - 9個のunit tests（全成功）
  - **Infrastructure層**: DI統合
    - Lock repositories/serviceをDI Containerに登録
    - 設定可能なheartbeat/cleanup intervals
    - 6個の統合tests（全成功）
    - Performance benchmark: 830 ops/sec
  - **CLI層**: 新しいlockコマンド
    - `deespec lock list`: アクティブなLock一覧表示
    - `deespec lock cleanup`: 期限切れLockのクリーンアップ
    - `deespec lock info <lockID>`: Lock詳細情報表示
    - 旧`deespec cleanup-locks`コマンドを@deprecated化

* **Clean Architecture + DDD リファクタリング Phase 4完了**: Adapter層の完全実装
  - **Presenter層**: CLI/JSON両対応の出力フォーマッター実装
    - `CLITaskPresenter`: 人間可読なCLI出力（292行）
    - `JSONPresenter`: 機械可読なJSON出力（47行）
    - 完全なテストカバレッジ（6テスト全てパス）
  - **Agent Gateway層**: AI統合とモック実装
    - `ClaudeCodeGateway`: Anthropic Messages API v1統合（195行）
    - `GeminiMockGateway`/`CodexMockGateway`: 将来の拡張用モック
    - 環境変数ベースのAgent選択機構
    - 5テスト全てパス（Claude APIテストはキーなしでスキップ）
  - **Storage Gateway層**: アーティファクト管理
    - `MockStorageGateway`: スレッドセーフなメモリ内ストレージ（122行）
    - Phase 6でのS3/Local実装への準備完了
    - 5テスト全てパス
  - **Controller層**: 完全新規のCLIコマンド体系
    - `EPICController`: EPIC CRUD + 分解（252行、8コマンド）
    - `PBIController`: PBI CRUD + 分解（252行、8コマンド）
    - `SBIController`: SBI CRUD + コード生成（297行、9コマンド）
    - `WorkflowController`: ワークフロー実行（271行、6コマンド）
    - `RootBuilder`: Cobraコマンドツリー統合（104行）
  - **Infrastructure層**: テスト用モック実装
    - `MockTransactionManager`: トランザクション管理（52行）
    - Mock Repositories: Task/EPIC/PBI/SBI（290行）
    - Phase 5でのSQLite実装への準備完了
  - **DI Container**: 依存性注入コンテナ
    - 4層アーキテクチャに沿った初期化（226行）
    - 設定ベースのコンポーネント切り替え
    - Mock ↔ Real実装の完全な交換可能性

* **Clean Architecture + DDD リファクタリング Phase 5開始**: SQLite Repository実装開始
  - SQLiteスキーマ設計完了（schema.sql）
    - EPIC/PBI/SBIテーブル定義
    - 多対多関連テーブル（epic_pbis、pbi_sbis）
    - パフォーマンス最適化用インデックス
    - スキーマバージョン管理テーブル

* **並列ワークフロー実行フレームワーク (Phase 2)**: 複数ワークフローの同時実行をサポート
  - `WorkflowManager`による並列オーケストレーション
  - 各ワークフローが独立したgoroutineで実行
  - リアルタイム統計情報とステータス追跡
  - グレースフルシャットダウンと適切なリソース管理

* **`sbi run`コマンドの追加**: SBI専用の実行コマンド
  - `deespec sbi run`として新規追加
  - より明確なコマンド体系を実現

* **連続実行機能**: ワークフローの常時実行をサポート
  - `deespec run`と`deespec sbi run`がデフォルトで連続実行
  - Ctrl+Cでのグレースフル停止対応
  - 設定可能な実行間隔（`--interval`フラグ）
  - 堅牢なエラーハンドリングと統計レポート
  - 指数バックオフによる自動復旧機能

* **ワークフロー設定システム**: YAML基盤の設定管理
  - `.deespec/workflow.yaml`による外部設定
  - ワークフロー単位での有効化/無効化制御
  - カスタム実行間隔とパラメータ設定

* **改善されたCLIヘルプシステム**: 並列実行機能を反映した詳細な説明
  - 設定ファイルの使用方法を含む包括的なドキュメント
  - 実践的な使用例の追加
  - より明確なコマンド説明

* **強化されたログ出力**: SBI実行状況の可視性向上
  - 新しいSBI開始時に目立つバナー表示
  - SBI完了時の詳細な結果表示
  - タスクタイトル、決定内容、実行回数などの詳細情報

* **自動ロッククリーンアップ機能**: リース期限切れ時の自動復旧
  - state.lock取得失敗時にリース期限を確認
  - 期限切れの場合は古いロックを自動削除して再取得
  - プロセス異常終了からの自動復旧を実現

* **強化されたロック管理システム**: 3層のロッククリーンアップメカニズム
  - タスク完了時（DONE状態）にstate.lockを明示的に削除
  - リースが空でも10分以上経過した古いロックファイルを自動削除
  - WorkflowManager起動時に古いstate.lockとrunlockを自動クリーンアップ
  - 各クリーンアップアクションで理由を明確にログ出力

* **改善されたrunlock管理**: プロセスレベルのロック制御を強化
  - タスク完了時（DONE状態）にrunlockファイルも削除
  - runlock取得時に詳細なログ出力（PID、ホスト名、期限）
  - TTL（10分）を厳密に遵守する期限切れ判定ロジック
  - WorkflowManager起動時に期限切れrunlockを適切にクリーンアップ

* **高頻度ハートビート出力**: AI処理中の死活監視を改善
  - ハートビート間隔を30秒から5秒に短縮
  - 累積経過時間を表示（5, 10, 15秒...）
  - WorkflowManager待機中も5秒ごとに活動状態を表示
  - システムの応答性と可視性が大幅に向上

* **設定可能な実行制限**: Turn数とAttempt数の制限を外部化
  - `setting.json`で`max_attempts`と`max_turns`を設定可能
  - デフォルト: max_attempts=3（3回試行で強制実装）、max_turns=8（最大8ターン）
  - max_turnsは安全装置として機能、異常時の無限ループを防止
  - max_attemptsは品質管理として機能、適切なタイミングで強制実装へ移行

* **ワークフロー実行の可視性向上**: 詳細な進捗ログ出力
  - ワークフロー開始時にバナー表示（実行間隔も表示）
  - 各実行サイクルの開始時刻を表示
  - ワークフローステップ遷移時に詳細コンテキストを表示
  - AIエージェント実行の開始と完了状態を表示
  - ロック競合と実際のエラーを区別してログレベルを調整
  - ロック取得成功の視覚的インジケーター追加

* **実行状態の詳細表示機能**: 現在処理中のタスク情報の可視化
  - 実行サイクルごとに現在のSBI ID、ステータス、ターン、試行回数を表示
  - アクティブタスクのリース期限を表示
  - ロック待機時も処理中タスクの詳細を表示
  - 視覚的な区切り線で実行サイクルを明確に分離
  - 冗長なログをDebugレベルに移動して必要な情報のみ表示

* **ハートビート機能による死活監視**: 長時間処理の進捗可視化
  - AIエージェント実行中に30秒ごとのハートビート表示
  - ストリーミング/通常モード両方でハートビート対応
  - ワークフロー待機中も30秒ごとに稼働状況を表示
  - 経過時間の表示により処理の進行状況を把握可能
  - システムフリーズと長時間処理を区別可能

### 変更 (Changed)

* **ログレベルのデフォルト設定改善**:
  - デフォルトログレベルをWARNからINFOに変更
  - 重要なステータスメッセージが確実に表示されるように改善
  - ログ出力の重複改行を削除してコンパクトな表示を実現

* **時刻表示のローカライズ**: state.jsonの時刻をローカルタイムゾーンで表示
  - lease_expires_atとmeta.updated_atをローカル時刻で表示（例: +09:00）
  - ISO 8601形式でタイムゾーン情報を含む
  - 内部処理はUTCのまま、表示のみローカル化

* **タイムアウト設定の最適化**:
  - `setting.json`のtimeout_secのデフォルト値を60秒から900秒（15分）に変更
  - 複雑なAIタスクの処理により多くの時間を確保

* **エラーリトライ間隔の改善**:
  - ロック競合時の最大バックオフを5分から10秒に短縮
  - システム稼働状況の頻繁な確認を可能に

* **実行モデルの変更**: 単一ワークフローから並列実行へ
  - `deespec run`が全ての有効なワークフローを並列実行
  - デフォルト動作が連続実行に変更
  - 実行間隔はデフォルト5秒（1秒〜10分で設定可能）
  - エラー発生時も継続実行（一時的エラーの場合）

### 非推奨 (Deprecated)

* **`deespec cleanup-locks`コマンドの非推奨化** (Phase 7.3):
  - 旧file-basedロックシステム用のコマンド
  - 新しい`deespec lock cleanup`コマンドを使用してください
  - 新コマンドはSQLite-based Lock Systemを使用
  - Phase 8で削除予定

* **`internal/interface/cli/runlock.go`の非推奨化** (Phase 7.3):
  - 旧file-based RunLock実装（203行）
  - 新しいLock Service (`internal/application/service/lock_service.go`)を使用してください
  - 移行ガイド: DI Containerから`GetLockService()`を取得
  - Phase 8で削除予定

* **`workflow`コマンドの非推奨化**:
  - メイン実行パスでは使用されなくなったため非推奨
  - 後方互換性のため維持

* **`--once`フラグの非推奨化**:
  - 連続実行がデフォルトとなったため非推奨
  - 後方互換性のため一時的に残存
  - v0.2.0で削除予定

### 修正 (Fixed)

* **WorkflowManagerテスト時のセグメンテーション違反を修正**:
  - `cleanupStaleLocks()`関数で`globalConfig`がnilの場合の処理を追加
  - テスト環境でNewWorkflowManager呼び出し時のクラッシュを防止
  - 通常実行時の動作には影響なし

* **ログ出力の安定性向上**: バッファリング問題の解消
  - 各ログ出力後に即座にフラッシュして遅延を防止
  - ハートビートgoroutineの起動確認メカニズムを追加
  - AI実行前後の明示的なログ出力で処理状況を可視化
  - AI処理時間の表示によるパフォーマンス監視
  - 出力が突然停止する問題を根本的に解決

### 追加予定

* `scripts/metrics.py`（レビューOK率・平均 `elapsed_ms` のローカル集計）

---

## \[v0.1.33] - 2025-09-30

### 追加 (Added)

* **Claude CLI権限スキップ機能**: 動作確認を優先するための権限確認スキップ
  - `--dangerously-skip-permissions`フラグを追加
  - 開発・テスト段階での迅速な動作確認を可能に
  - ストリーミング実行と通常実行の両方に適用
  - 将来的に細かい権限設定を導入する際は削除予定

### 変更 (Changed)

* **デバッグファイルの保存先変更**: raw_responseファイルの保存先を整理
  - `raw_response_*.txt`ファイルを`results`サブディレクトリに保存するよう変更
  - ディレクトリ構造をよりクリーンに整理

### 修正 (Fixed)

* **ストリーミングモードのバッファサイズ問題を修正**
  - bufio.Scannerのバッファサイズを10MBに拡張
  - "token too long"エラーを解消
  - 大きなJSON出力にも対応可能に

---

## \[v0.1.30] - 2025-09-30

### 修正 (Fixed)

* **Claude CLIの互換性修正**: 誤って追加された`--yes`フラグを削除
  - Claude CLIは`--yes`オプションをサポートしていないため削除
  - "error: unknown option '--yes'" エラーを解消
  - ストリーミング実行と通常実行の両方から削除

---

## \[v0.1.29] - 2025-09-30

### 追加 (Added)

* ~~**Claude Code権限自動承認**: ファイル書き込み権限の自動承認機能~~
  - ~~`--yes`フラグを追加してClaude CLIの権限リクエストを自動承認~~
  - ~~ストリーミング実行と通常実行の両方に適用~~
  - ~~ユーザーの手動承認が不要になり、自動化が向上~~
  - **注意**: この機能はv0.1.30で削除されました（Claude CLIが`--yes`をサポートしていないため）

* **テストカバレッジ向上**: CLI パッケージのテスト充実
  - parseDecision関数の包括的なテスト追加
  - nextStatusTransition関数のエッジケーステスト
  - DONEタスク検出機能のテスト（新旧形式対応）
  - コマンドコンストラクタ関数のテスト追加
  - カバレッジ50.4%達成（最低要件50%をクリア）

### 修正 (Fixed)

* **テストファイルの絶対パス使用を修正**
  - テストで絶対パスの代わりに相対パスを使用するように修正
  - プロジェクトのポータビリティを向上

* **ストリーミング出力機能の安定化**
  - 環境変数依存を削除し、常時有効化
  - workflow_step_{N}.jsonl形式の一貫したファイル命名

### 変更 (Changed)

* **AI応答可視化とDECISION解析の改善** (v0.1.28からの継続改善)
  - AIレスポンス全文のコンソール表示
  - 柔軟な判定抽出パターン対応
  - 無限ループ防止機構の強化

---

## \[v0.1.27] - 2025-09-29

### 追加 (Added)

* **ラベル管理システム**: SBI仕様のラベル管理機能を実装
  - `deespec label` コマンド群を追加（set, add, list, search, delete, clear）
  - meta.yml内でのラベル保存
  - ラベルインデックスによる高速検索（.deespec/var/labels.json）
  - 階層ラベルのサポート（例: frontend/architecture, backend/api）
  - ラベル固有の指示書サポート（.deespec/prompts/labels/）

* **プロンプトテンプレートの強化**:
  - WIP.md.tmpl、REVIEW.md.tmpl、REVIEW_AND_WIP.md.tmplをembedディレクトリに移動
  - .deespecディレクトリの変更禁止を最高優先度制約として追加
  - ターン番号を含む構造化レポートフォーマットを追加
  - ラベルベースのタスクエンリッチメント機能

* **ヘルプドキュメント**: `clear` と `cleanup-locks` コマンドのヘルプ文書を追加

* **テストカバレッジ改善**: CLI パッケージの包括的なテストを追加
  - clear 機能のテスト (`clear_test.go`)
  - cleanup-locks 機能のテスト (`cleanup_locks_test.go`)
  - コマンド登録のテスト (`cmd_test.go`)
  - ロガー機能のテスト (`logger_test.go`)
  - ラベル機能のテスト (`label_cmd_test.go`)
  - ドメイン実行エンティティのテスト拡張

### 修正 (Fixed)

* **Turn番号のずれ問題**: ピック後の即実装によるTurn番号の不整合を修正
  - 新規タスクピック後、同一Turn内でimplementを実行しないように修正
  - Turn 1 = implement、Turn 2 = reviewの正しい順序を実現
  - unreachable codeの除去

* **重複doneファイル問題**: 同一タスクで複数のdoneファイルが作成される問題を修正
  - Turn制限の実装により、Turn番号が最大値を超えないように制御
  - 状態遷移の正常化により、不正なDONE遷移を防止

* **ロックファイル処理**: Unix系システムでのPIDチェックバグを修正
  - os.FindProcess()の誤動作を修正
  - process.Signal(syscall.Signal(0))を使用した適切なプロセス確認

* **Clear コマンドロジック**: 期限切れリースでのクリア動作を改善
  - WIP タスクがあってもリースが期限切れの場合はクリアを許可（警告付き）
  - アクティブなリースがある場合のみクリアをブロック

* **ファイル名対応**: メタファイル検索で `meta.yaml` と `meta.yml` の両方に対応

### 非推奨 (Deprecated)

* **ワークフロー機能**: workflow.yaml ベースの機能を非推奨に設定
  - `LoadWorkflow()`, `ExpandPrompt()`, `BuildVarMapWithConfig()` 等の関数
  - `Workflow`, `Step` 型定義
  - ワークフロー検証機能 (`NewValidator()`, `Validate()`)
  - ワークフローコマンド (`workflow verify`)
  - 現在はシンプルなステータスベース（WIP/REVIEW/REVIEW&WIP）を使用

---

## \[v0.1.28] - 2025-09-30

### 追加 (Added)

* **ステータスベース実行フロー**:
  - SBI実行を10ステップのステータスベースフローに変更
  - `READY`/`WIP`/`REVIEW`/`REVIEW&WIP`/`DONE` ステータスの導入
  - 3回の試行後の強制終了メカニズムを実装

* **外部プロンプトファイルサポート**:
  - `.deespec/prompts/` ディレクトリから外部プロンプトを読み込む機能
  - `WIP.md`、`REVIEW.md`、`REVIEW_AND_WIP.md` テンプレートファイル対応
  - プレースホルダー置換機能（`{{.SBIID}}`、`{{.Turn}}` など）
  - 外部ファイルが存在しない場合のフォールバック機構

* **ロック管理コマンド**:
  - `cleanup-locks` コマンドを追加（期限切れロックのクリーンアップ）
  - `show-locks` 機能でロック状態の表示
  - 3層ロックメカニズムのサポート（ファイルロック、プロセスロック、リース）

* **アーカイブ機能付きclearコマンド**:
  - `clear` コマンドでタイムスタンプ付きULIDアーカイブディレクトリを作成
  - WIP保護機能（作業中タスクがある場合はクリア不可）
  - `--prune` オプションで全アーカイブの削除

* **Claude Code統合の強化**:
  - 構造化されたプロンプト送信機能
  - タイムスタンプ付きの詳細な入出力ログ
  - JSONレスポンスの正確なパース

### 変更 (Changed)

* **ドメイン駆動設計への移行**:
  - `internal/domain/execution` パッケージの作成
  - エンティティ、値オブジェクト、リポジトリパターンの実装
  - クリーンアーキテクチャ原則の適用

* **状態管理の改善**:
  - `State` 構造体に `Status`、`Decision`、`Attempt` フィールドを追加
  - `nextStatusTransition` 関数による状態遷移管理
  - ターン管理の修正（1実行 = 1ターン）

### 修正 (Fixed)

* **ターンインクリメントのバグ修正**:
  - 重複するターンインクリメントを削除（1→3→5→7へのジャンプを修正）
  - `run.go` の357行目の不要な `st.Turn++` を削除

* **テストの修正**:
  - テストファイル内の絶対パス使用を修正
  - `claude_prompt_test.go` のパスを相対パスに変更
  - `app.Paths` 型のインポートエラーを修正

---

## \[v0.1.27] - 2025-09-29

### 変更 (Changed)

* **アーティファクトパス構造の大幅な改善**:
  - 従来の中央集約型 `.deespec/var/artifacts/turn1/`, `turn2/` 構造を廃止
  - SBI 固有ディレクトリでのファイル管理に移行 (`.deespec/specs/sbi/<SBI-ID>/`)
  - ターン関連ファイルは `{step}_{turn}.md` 形式で保存 (例: `plan_1.md`, `implement_2.md`)
  - FB ドラフトは `fb_` プレフィックスで同一ディレクトリに配置
  - ノートファイル (`impl_notes.md`, `review_notes.md`) も SBI ディレクトリに統合

* **設定管理の簡素化**:
  - `ArtifactsDir` フィールドと関連メソッドを完全削除
  - `AppConfig` 構造体から不要なフィールドを除去
  - `NewAppConfig` 関数のパラメータを最適化

* **State 管理の改善**:
  - `CurrentTaskID` を `WIP` (Work In Progress) に名称変更
  - 現在作業中の SBI ID を明確に追跡

### 削除 (Removed)

* **廃止された機能**:
  - `ArtifactsDir` 設定項目
  - 中央集約型アーティファクトディレクトリ (`.deespec/var/artifacts/`)
  - 関連する初期化・検証ロジック

### 修正 (Fixed)

* **テストの更新**:
  - `TestPersistFBDraft` を新しいパス構造に対応
  - 不要なディレクトリ作成処理を削除
  - パス参照を新構造に統一

---

## \[v0.1.26] - 2025-09-28

### 追加 (Added)

* **SBI 仕様登録機能の実装 (完全版)**:
  - Domain 層: SBI エンティティとリポジトリインターフェース
  - UseCase 層: RegisterSBIUseCase と ULID ベースの ID 生成
  - Infrastructure 層: FileSBIRepository とアトミックファイル書き込み
  - Interface 層: `sbi register` CLI コマンド実装
  - 仕様書フォーマット: ガイドラインブロック自動挿入機能
  - 保存パス: `.deespec/specs/sbi/<SBI-ID>/spec.md`
  - メタデータ管理: `meta.yml` ファイルによる仕様書メタ情報保存

* **CLI コマンド機能**:
  - `deespec sbi register`: 新規 SBI 仕様書の登録
  - フラグ: `--title` (必須), `--body` (オプション, stdin対応)
  - ラベル管理: `--label` (複数指定可), `--labels` (カンマ区切り)
  - 出力制御: `--json`, `--quiet`, `--dry-run`
  - dry-run モード: MemMapFs による実行シミュレーション
  - ラベルの重複排除と空白トリミング機能

* **アーキテクチャドキュメント**:
  - Clean Architecture + DDD 実装ガイドライン
  - 理想的なディレクトリ構造テンプレート
  - 各レイヤーの責務と依存関係の明確化

### 変更 (Changed)

* **依存パッケージの追加**:
  - `github.com/oklog/ulid/v2`: ULID 生成用ライブラリ
  - `github.com/spf13/afero`: ファイルシステム抽象化ライブラリ

### テスト (Tests)

* **包括的なテストカバレッジ**:
  - Domain 層: エンティティ検証テスト
  - UseCase 層: モックリポジトリを使用した統合テスト
  - Infrastructure 層: 並行書き込み、エラーケーステスト
  - Interface 層: CLI コマンドテスト、フラグ検証
  - E2E テスト: 実際のファイル作成、stdin入力、JSON出力
  - 100% のテスト成功率

---

## \[v0.1.25] - 2025-09-28

### 追加 (Added)

* **中央集権的ログシステムの実装**:
  - `Logger` インターフェースをapp/infra/cli各層に追加
  - ログレベル制御機能（DEBUG/INFO/WARN/ERROR/FATAL）
  - `--log-level` CLIフラグによる実行時ログレベル変更
  - レイヤー間のログブリッジ機能

### 変更 (Changed)

* **ログレベルのデフォルト設定変更**:
  - デフォルトログレベルを INFO から WARN に変更
  - ユーザー体験の向上（不要な情報ログを非表示）
  - `setting.json` の `stderr_level` デフォルト値を "warn" に設定
  - diagnostic コマンド（effective-config）実行時は一時的に INFO レベルに変更

* **fmt.Fprintf の完全置換**:
  - 全ての `fmt.Fprintf(os.Stderr, ...)` 呼び出しを Logger メソッドに置換
  - 統一されたログ出力フォーマット
  - ログレベルによる適切なフィルタリング

### 修正 (Fixed)

* **NDJSON パースエラーの修正**:
  - `strings.Split(data, "")` を `strings.Split(data, "\n")` に修正（4箇所）
  - picker.go の `getCompletedTasksFromJournal` と `getLastJournalEntry`
  - picker_test.go の `createJournal` と `TestTurnConsistency_SBI_PICK_002`
  - run_tx_test.go の journal 読み込み処理

* **改行処理の修正**:
  - `NormalizeCRLFToLF` 関数で単独の CR 文字を正しく削除
  - Windows (CRLF) と旧Mac (CR) の改行形式を適切に処理

* **テストデータの整合性修正**:
  - `WriteFileAtomic` テストのデータ不一致を修正
  - picker テストの artifacts フィールドの型を統一

---

## \[v0.1.18] - 2025-09-28

### 削除 (REMOVED)

* **use_tx 設定の削除**: トランザクションモードを常に使用するため設定オプションを削除
  - `Config` インターフェースから `UseTx()` メソッドを削除
  - `register` コマンドは常に `registerWithTransaction` を実行
  - レガシーの `appendToJournalWithConfig` 関数を削除

* **disable_state_tx 設定の削除**: state/journal更新は常にトランザクションモードを使用
  - `Config` インターフェースから `DisableStateTx()` メソッドを削除
  - `UseTXForStateJournal()` 関数を削除
  - `run` コマンドは常に `SaveStateAndJournalTX` を使用
  - レガシーモード（直接書き込み）のコードパスを削除

### 変更

* **トランザクションモードの常時有効化**:
  - spec登録時は常に `meta.yaml` と `spec.md` を生成
  - state/journal更新は常にアトミックな操作を保証
  - データの原子性・一貫性・耐久性を常に確保
  - 障害時のリカバリー機能を常に利用可能

* **コードベースの簡素化**:
  - 条件分岐の削除によりコード複雑度が減少
  - テストケースから不要な設定チェックを削除
  - 設定ファイルとドキュメントから廃止項目を削除

---

## \[v0.1.17] - 2025-09-27

### 破壊的変更 (BREAKING CHANGES)

* **環境変数サポートの完全削除**: 全ての設定は `setting.json` ファイルからのみ読み込まれるようになりました
  - `os.Getenv()` の呼び出しを全コードベースから削除
  - 環境変数による設定オーバーライドは利用不可能
  - 設定優先順位: setting.json > デフォルト値（環境変数は無視される）

### 変更

* **設定システムの一元化**: `setting.json` のみが設定ソースとなるよう完全移行
  - `LoadSettings()` から環境変数オーバーライド機能を削除
  - 全コマンドが `globalConfig` インスタンスを通じて設定にアクセス
  - 設定読み込みは `bootstrap` で一度だけ実行

### 修正

* **テストエラーの修正**:
  - CLI テストで発生していた `undefined: baseDir` エラーを修正
  - トランザクションリカバリテストの `destRoot` パス解決問題を修正
  - 環境変数に依存していたテストケースを削除または修正

---

## \[v0.1.16] - 2025-09-27

### 追加

* **setting.json 設定ファイル**: 環境変数に代わる統一的な設定管理システムを導入
  - `deespec init` で自動生成される `setting.json` ファイル
  - 設定優先順位: setting.json > 環境変数 > デフォルト値
  - 21個の設定項目を網羅（home, agent_bin, timeout_sec など）
* **Config インターフェース**: app層に設定アクセス用のインターフェースを追加（Clean Architecture）
* **設定ソース表示**: `deespec doctor --json` に `config_source` と `setting_path` フィールドを追加
  - 設定の読み込み元（json/env/default）を確認可能

### 変更

* **パス解決の改善**: `GetPathsWithConfig()` メソッドを追加し、Config経由でのパス解決に対応
* **環境変数参照の削減**: 主要コマンドで直接のENV参照をConfig注入に置き換え
  - run, status, doctor コマンドが Config を使用
  - 後方互換性のため環境変数も引き続きサポート

### 修正

* **errcheck lint エラー**: CLI パッケージ内の未チェックエラーを修正
  - os.Chdir, os.WriteFile, json.Unmarshal, io.ReadAll のエラーチェック追加
* **Windows ビルド対応**: syscall.Flock と syscall.Stat_t の OS 固有実装を分離
  - flock_unix.go / flock_windows.go でファイルロックを抽象化
  - device_unix.go / device_windows.go でデバイス比較を抽象化
  - Windows でのクロスコンパイルが成功するように修正
* **テストの移植性向上**: settings_test.go で絶対パスを相対パスに変更
  - testutil の絶対パスバリデーションをパス

---

## \[v0.1.15] - 2025-09-27

### 追加

* **トランザクション起動時リカバリ**: `txn.RunStartupRecovery` を実装し、アプリケーション起動時の自動リカバリをサポート
* **メトリクス収集機能**: トランザクションメトリクス用の `Clone()` および `Merge()` メソッドを追加
* **パス検証強化**: トランザクションファイル操作で相対パス強制と親ディレクトリエスケープ防止を実装
* **EXDEV検出**: コミット時のデバイス境界チェックを追加し、早期失敗を実現
* **環境変数サポート**: `DEESPEC_TX_DEST_ROOT` による宛先ルート設定のサポート

### 変更

* **fsync ポリシー**: ディレクトリ rename 前後での親ディレクトリ fsync を追加し、データ永続性を強化
* **ロック順序最適化**: メトリクス保存時の mutex とファイルロックのネスティングを解消
* **テストタイムアウト調整**: デッドロック検出テストのタイムアウトを 200ms から 500ms に延長
* **リカバリ処理改善**: 宛先ルートの優先順位を `DEESPEC_TX_DEST_ROOT` > `DEE_HOME` > `.deespec` に変更

### 修正

* **journal callback nil パニック対策**: トランザクションコミット時の nil チェックとログ出力を追加
* **絶対パス処理の削除**: `StageFile` での不要な絶対パス解決を削除し、相対パス処理を維持
* **テストスキャナ実装**: `register_tx_test.go` に簡易スキャナを実装し、nil ポインタ参照を解消
* **メトリクス書き込み**: atomic rename から file handle 経由の直接書き込みに変更し、一時ファイル問題を解消

---

## \[v0.1.14] - 2025-09-27

### 追加

* **テスト環境の独立性強化**: `DEESPEC_TEST_MODE` 環境変数を導入し、パスキャッシュをバイパス可能に
* **パスキャッシュ制御**: `app.GetPaths()` にテストモードサポートを追加

### 変更

* **CI 設定**: テスト実行に `-p 1` フラグを追加して順次実行を保証
* **テスト期待値修正**: `ValidateSpecWithConfig` でタイトル長超過とラベル形式エラーを警告ではなくエラーとして処理

### 修正

* **doctor テストの安定性**: `os.Chdir` を削除し、環境変数ベースのパス解決に移行
* **register テストの期待値**: バリデーションエラーの期待値を実装に合わせて修正
* **macOS シンボリックリンク対応**: テスト時のシンボリックリンクチェックを適切に制御

---

## \[v0.1.13] - 2025-09-27

### 追加

* **Journal 出力強化**: picker.go にjournal step entries の stdout 出力を追加（デバッグ用）
* **ジャーナル監視**: 各 step（plan/implement/test/review/done）の詳細なトレース出力

### 変更

* **.gitignore 統一**: .deespec/ ディレクトリ全体を ignore に変更（一元管理）
* **設定削除**: 開発用 .deespec 設定ファイル群の削除（agents.yaml, policies/, prompts/, templates/, test/fixtures/）

### 修正

* **プロジェクト整理**: tmp/ ディレクトリと legacy specs/ の削除

---

## \[v0.1.12] - 2025-09-25

### 追加

* **decision 列挙ガード**: 空の decision を `"PENDING"` に正規化、列挙値（PENDING|NEEDS_CHANGES|OK）の検証
* **`doctor --json`**: 機械可読な診断結果出力（runner/active/working_dir/agent_bin/start_interval_sec、exit code 0/2/1）
* **CI 強化**: NDJSON純度、7キー、UTC、turn整合、decision列挙の完全検証を verify_journal.sh に統合
* **scheduler チェック**: doctor コマンドが launchd/systemd の状態を検出・報告
* **SBI-001 実装**: review ステップ完了時に review_note.md を生成（各ターンの要旨と DECISION を記録）

### 変更

* **journal 正規化**: すべての decision フィールドが 3つの列挙値のいずれかを持つことを保証

---

## \[v0.1.11] - 2025-09-25

### 追加

* **自動運用**: 5分間隔のスケジューラ投入（macOS: `launchd`／Linux: `systemd timer`）
* **観測性**: `health.json` 出力（`ts/turn/step/ok/error`、UTC・毎ターン上書き）
* **ドキュメント**: README に 1行インストール＋Quick Start を整備

### 変更

* **時刻表記の統一**: すべての時刻を **UTC（`Z` 終端の RFC3339Nano）** に統一（`status --json` 含む）

### 修正

* なし（前版の修正を踏襲した上で運用導線を整備）

---

## \[v0.1.9] - 2025-09-25

### 追加

* **状態出力の連動**: `status --json` の `ok` を直近 `journal.ndjson` の `error` と連動

### 変更

* なし

### 修正

* **ターン整合性**: `state.Turn` の **インクリメント時機を“journal 追記後”に統一**
  → `journal.turn == ".artifacts/turnN/..."` を常時保証

---

## \[v0.1.8] - 2025-09-25

### 追加

* **検証強化（r008）**: 連続実行＋ジッター／失敗注入を含む追加テスト計画
* **スキーマ連動**: `status --json` の `ok` リンク（初実装）

### 変更

* **時刻表記**: `journal.ndjson.ts` を UTC（`Z`）に統一

### 修正

* **スキーマ安定**: `journal.ndjson` の **7キー** を欠落禁止で固定
  `ts, turn, step, decision, elapsed_ms, error, artifacts`（`artifacts` は常に **array**）

---

## \[v0.1.7] - 2025-09-25

### 追加

* **`status --json`（スケルトン）**: 最低限の機械可読ステータス出力を追加
* **`JournalWriter` / `NormalizeJournalEntry`**: 出力の一本化・ゼロ値補完

### 変更

* なし

### 修正

* `artifacts` の型を `[]string` に統一（`nil`・文字列混入を排除）

---

## \[v0.1.2] - 2025-09-25

### 追加

* **インストーラ**: `scripts/install.sh`（Linux/macOS）を整備（`/usr/local/bin` への配置、不可時 `~/.local/bin` へフォールバック、PATH 追記）
* **Windows**: `scripts/install.ps1` の導線準備（PowerShell ワンライナー）

### 変更

* なし

### 修正

* なし

---

## \[v0.1.1] - 2025-09-25

### 追加

* **配布導線**: GitHub Actions によるタグ駆動リリース（クロスビルド）
* README: Quick Start の雛形

### 変更

* なし

### 修正

* なし

---

## \[v0.1.0] - 2025-09-25

### 追加

* **MVP**: `init / status / run --once` の最小ワークフロー（plan→implement→test→review→done）
* **フォールバック**: 外部依存の失敗時にブーメラン（`NEEDS_CHANGES`）で前進を継続
* **排他**: CAS(version)＋lock による実行排他
* **ログ**: `journal.ndjson`（追記）と `.artifacts/` 生成

---

### 付記

* 2段階リリース（build→artifact集約→release一回）への切替は運用安定のため推奨です。
* 既存タグの再利用は GitHub 側の制約により不可（`422 already_exists`）。新タグでの発行が安全です。

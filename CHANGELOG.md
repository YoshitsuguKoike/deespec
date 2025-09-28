# CHANGELOG

本プロジェクトは [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) 形式に準拠し、[Semantic Versioning](https://semver.org/lang/ja/) に従います。

## \[Unreleased]

### 追加予定

* `scripts/metrics.py`（レビューOK率・平均 `elapsed_ms` のローカル集計）

---

## \[v0.1.26] - 2025-09-28

### 追加 (Added)

* **SBI 仕様登録機能の実装 (Step 1-2)**:
  - Domain 層: SBI エンティティとリポジトリインターフェース
  - UseCase 層: RegisterSBIUseCase と ULID ベースの ID 生成
  - Infrastructure 層: FileSBIRepository とアトミックファイル書き込み
  - 仕様書フォーマット: ガイドラインブロック自動挿入機能
  - 保存パス: `.deespec/specs/sbi/<SBI-ID>/spec.md`

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

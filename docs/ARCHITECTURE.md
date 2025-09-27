# DeeSpec Architecture

## 1. Overview

DeeSpecは開発ワークフローを自動化するツールで、タスクの実行状態を堅牢に管理します。
本ドキュメントは、状態管理とトランザクション（TX）機構の設計仕様を定義します。

## 2. Core Components

### 2.1 Workflow Engine
- タスク実行の制御
- ステップ遷移管理（plan → implement → test → review → done）
- 依存関係の解決

### 2.2 State Management
- state.json: 現在の実行状態
- journal.ndjson: 実行履歴の追記型ログ
- health.json: システムヘルスステータス

## 3. Transaction Management (TX)

### 3.1 Lock Order Specification {#tx-lock-order}

システム全体でのデッドロックを防止するため、以下のロック取得順序を厳守する：

**ロック取得順序（固定）:**
```
state lock → run lock → txn lock
```

- **state lock**: state.jsonへの排他アクセス制御
- **run lock**: 実行インスタンスの多重起動防止
- **txn lock**: トランザクション操作の原子性保証
- **解放順序**: 取得の逆順で解放（txn lock → run lock → state lock）
- **違反時の挙動**: デッドロック検出時は即座に全ロック解放して再試行

### 3.2 Lease Management {#tx-lease}

リース機構により、プロセス障害時の自動回復を実現する：

**リース運用ルール:**
- **デフォルトTTL**: 10分間（600秒）
- **延長条件**:
  - I/O処理中は自動延長
  - コミット処理中は自動延長
  - 明示的なRenewLease()呼び出し
- **失効時の処理**:
  - 安全な前方回復（Forward Recovery）が可能な場合のみ再開
  - manifest/stageが不完全な場合は手動介入を要求

### 3.3 fsync Policy {#tx-fsync}

データの永続性を保証するため、以下のfsync方針を適用する：

**fsync方針（最低限の保証）:**
```
fsync(file) → fsync(parent dir)
```

- **通常ファイル**: 書き込み後、即座に`fsync(file)`を実行し、その後`fsync(parent dir)`を実行
- **journal.ndjson**:
  - `O_APPEND`フラグで開く
  - 各エントリ追記後に`fsync(file)` → `fsync(parent dir)`を実行
  - クラッシュ耐性を最優先
- **ディレクトリ同期**: rename操作後は必ず親ディレクトリの`fsync(parent dir)`を実行

**親ディレクトリfsync の重要性:**
- **メタデータ永続化**: ファイル作成・rename操作後、親ディレクトリをfsyncしないと目次エントリが失われる
- **原子性保証**: `rename(tempfile, target)`操作後、親ディレクトリのfsyncが必須
- **クラッシュ耐性**: 電源断時にファイルは存在するが親ディレクトリのエントリが失われるリスクを回避
- **POSIX要件**: POSIXファイルシステムでは、ディレクトリ変更（作成・削除・rename）後のfsyncが必要

**実装例:**
```go
// ファイル作成とfsync
file, err := os.Create("newfile.txt")
file.Write(data)
file.Sync()  // ファイル内容をfsync
file.Close()

// 親ディレクトリをfsync（重要！）
dir, err := os.Open(filepath.Dir("newfile.txt"))
dir.Sync()   // ディレクトリエントリをfsync
dir.Close()

// Rename操作とfsync
os.Rename("tempfile", "target")
parentDir, err := os.Open(filepath.Dir("target"))
parentDir.Sync()  // rename後の親ディレクトリfsync（必須）
parentDir.Close()
```

**fsync失敗時の方針:**
- **現在のデフォルト**: WARNログのみ（処理は継続）
- **厳密モード（環境変数）**: `DEE_STRICT_FSYNC=1`でfsync失敗をエラーとして扱う
- **判断基準**: データロスよりも可用性を優先する場合はWARN、完全性を優先する場合はFAIL
- **親ディレクトリfsync**: Step 8で完全実装予定（TX機構と統合）

### 3.4 TX Terminology {#tx-terminology}

擬似トランザクション機構で使用する用語を以下に定義する：

**TX用語定義:**
- **TX (Transaction)**: 擬似トランザクションの総称
- **manifest**: 変更対象ファイルの明細（dst, checksum等）を記録した計画ファイル
- **stage**: 本番環境への反映前にファイルを準備する隔離領域（同一ファイルシステム上）
- **intent**: コミット直前の準備完了状態を示すマーカーファイル（`status.intent`）
- **commit**: stage→本番へのrename適用とjournal追記が完了した状態を示すマーカー（`status.commit`）
- **undo**: 必要時のみ使用するbefore-imageによる巻き戻し機構（オプション）

### 3.5 Data Format Standards {#tx-data-format}

トランザクションデータの形式仕様：

**時刻表現:**
- すべてのIntent/Commitおよび関連する時刻データはUTC/RFC3339形式で統一
- 例: `2025-09-27T05:00:00Z` または `2025-09-27T05:00:00.123456Z`
- ログ出力や監査トレースの一貫性を確保

### 3.6 TX File Layout {#tx-layout}

トランザクション関連ファイルの配置規則：

**TX配置ルール:**
```
.deespec/var/txn/<txn-id>/
├── manifest.json      # 変更計画
├── stage/            # ステージングファイル
│   └── <files>
├── undo/             # ロールバック用（オプション）
│   └── <files>
├── status.intent     # intent マーカー
└── status.commit     # commit マーカー
```

- **同一ファイルシステム要件**: rename操作の原子性を保証するため、すべて同一FS上に配置
- **txn-id**: UUID v4またはタイムスタンプベースの一意識別子

### 3.7 Recovery Rules {#tx-recovery}

システム起動時の復旧処理規則：

**CRITICAL: 起動シーケンス要件**
```
main() → RunStartupRecovery() → AcquireLock() → Normal Operation
```
- `RunStartupRecovery()`は**必ず**state lockやrun lock取得前に呼び出す
- ロック前に実行することで、障害回復処理とロック競合の分離を保証
- 違反時は複数プロセス間でのデッドロックや不整合状態を引き起こす可能性

**スキャンタイミング:**
- トランザクションディレクトリのスキャンは、アプリケーション起動直後、**ロック取得前**に実行
- これによりロックを保持する前に復旧可能性を判断し、起動時間を最適化

**復旧規則:**
- **Forward Recovery（前方回復）**:
  - `intent`ありかつ`commit`なしの場合 → **コミット処理を継続**
  - 中断されたトランザクションを完了方向に進める
- **安全停止**:
  - manifest欠落時 → エラーログ出力して手動対応を要求
  - stage不完全時 → エラーログ出力して手動対応を要求
- **自動クリーンアップ**:
  - `commit`完了後のtxnディレクトリは次回起動時に削除
  - Step 8で実装予定：バッチ削除によりI/O負荷を軽減

### 3.8 Cleanup Policy {#tx-cleanup}

トランザクション関連ファイルの削除方針：

**即座クリーンアップ（Immediate Cleanup）:**
- **対象**: 正常完了したトランザクション（`status.commit`存在）
- **タイミング**: Commit操作の最終フェーズで即座に実行
- **方針**:
  ```go
  if err := manager.Cleanup(tx); err != nil {
      // Non-fatal: just log warning
      fmt.Fprintf(os.Stderr, "WARN: failed to cleanup transaction: %v\n", err)
  }
  ```
- **失敗時**: WARN ログのみ（処理継続、次回起動時に再試行）

**起動時一括クリーンアップ（Startup Batch Cleanup）:**
- **対象**: 残存している完了済みトランザクション
- **実行場所**: `RunStartupRecovery()`内で実行
- **処理順序**:
  1. アクティブトランザクションの前方回復処理
  2. 完了済みトランザクション（`status.commit`存在）の一括削除
  3. 孤立ファイルの検出とログ出力
- **安全性**: ロック取得前に実行することで競合回避

**保持ポリシー（Retention Policy）:**
- **デフォルト**: 完了後即座削除（ディスク使用量最小化）
- **デバッグモード**: `DEESPEC_KEEP_TX_DIRS=1`で削除を無効化
- **ログ保持**: 削除されたトランザクションの情報はjournal.ndjsonに永続記録
- **監査要件**: 削除操作もメトリクス出力対象（`txn.cleanup.success`/`txn.cleanup.failed`）

**エラー処理:**
- **アクセス権限エラー**: WARN ログ、次回起動時に再試行
- **ディスク容量不足**: クリーンアップ失敗は致命的エラーとしない
- **同時アクセス**: 他プロセスによる削除は正常として扱う（ディレクトリ不存在は成功）

### 3.9 Constraints and Non-Goals {#tx-constraints}

**制約事項:**
- **同一ファイルシステム要件**: rename操作の原子性を保証するため、全ファイルは同一FS上に配置
  - EXDEV（cross-device link）エラーは明示的にハンドリング
  - 一時ファイルは必ず目的ファイルと同一ディレクトリに作成
- **プラットフォーム依存性**: fsyncの挙動はOSとファイルシステムに依存
  - Linux/ext4: 完全なメタデータ同期
  - macOS/APFS: ディレクトリfsyncは部分的サポート（F_FULLFSYNC推奨）
  - Windows/NTFS: FlushFileBuffers APIを内部使用、ディレクトリ同期は不要
- **パーミッションとumask**:
  - デフォルトパーミッション: 0644（ファイル）、0755（ディレクトリ）
  - 実効パーミッションはプロセスのumaskに影響される
  - 注意: umask設定は環境により異なるため、重要なファイルは明示的にchmodを推奨
- **ファイルシステム境界での原子性**:
  - journalとstate/specsは必ず同一FS上に配置
  - 別FSではrename原子性が保証されない（データ不整合のリスク）

**Non-Goals (対象外):**
- 分散トランザクション（2PC/3PC）の実装
- RDBMSレベルのACID保証
- クロスファイルシステムでの原子性
- ネットワークファイルシステム（NFS/SMB）での動作保証

## 4. Implementation References

### 4.1 Current Implementation
- Lock mechanism: `internal/infra/fs/atomic.go`
- Journal handling: `internal/app/journal.go`
- State management: `internal/interface/cli/stateio.go`

### 4.2 Future TX Implementation
以下のステップで段階的にTX機構を実装予定：
- Step 2: fsユーティリティの抽出
- Step 3: journal追記の堅牢化
- Step 4-6: TX基本実装
- Step 7: registerコマンドへのTX適用
- Step 8-14: 拡張と検証

## 5. Appendix

### 5.1 Error Codes
- `E_LOCK_TIMEOUT`: ロック取得タイムアウト
- `E_LEASE_EXPIRED`: リース期限切れ
- `E_TX_INCOMPLETE`: トランザクション不完全
- `E_FSYNC_FAILED`: fsync失敗

### 5.2 Metrics Output Standard {#metrics-standard}

システム全体での一貫したメトリクス出力形式：

**標準フォーマット:**
```
LOG_LEVEL: Human readable message key1=value1 key2=value2
```

**フォーマット規則:**
- **ログレベル**: `INFO`, `WARN`, `ERROR`, `DEBUG`, `AUDIT`
- **メッセージ**: 人間が読みやすい説明（英語/日本語混在可）
- **キー**: ピリオド区切りの階層形式（例：`txn.commit.success`）
- **値**: 引用符なしの単純値（文字列、数値、ブール値）
- **区切り**: キー=値ペアはスペースで区切り

**メトリクス名前空間:**
- `txn.*`: トランザクション関連メトリクス
- `fsync.*`: fsync監査関連メトリクス
- `run.*`: 実行・ワークフロー関連メトリクス
- `register.*`: タスク登録関連メトリクス

**例:**
```bash
INFO: Transaction committed successfully txn.commit.success=true txn.id=abc123 txn.duration_ms=45
WARN: Failed to cleanup transaction txn.cleanup.failed=def456 error="permission denied"
AUDIT: fsync operation completed fsync.file.count=3 fsync.path=/path/to/file
```

**パースとモニタリング:**
- key=value形式により、ログ解析ツール（fluentd/logstash）で簡単にパース可能
- メトリクス名は`internal/infra/fs/txn/metrics.go`で集中管理
- Step 12のdoctorコマンドでメトリクス収集・集計予定

### 5.3 Build Tags Configuration {#build-tags}

条件付きコンパイルによる機能制御：

**Build Tag一覧:**
```bash
# fsync audit mode (監査モード)
go build -tags fsync_audit
go test -tags fsync_audit ./...

# Normal mode (本番モード) - デフォルト
go build
go test ./...
```

**fsync_audit タグ:**
- **目的**: データ永続性の検証とデバッグ
- **有効化方法**:
  ```bash
  # ビルド時指定
  go build -tags fsync_audit -o deespec-audit ./cmd/deespec

  # テスト時指定
  go test -tags fsync_audit ./...

  # 環境変数併用
  DEESPEC_FSYNC_AUDIT=1 go test -tags fsync_audit ./...
  ```

- **動作変更**:
  - `FsyncFile()` と `FsyncDir()` がaudit情報を出力
  - fsync操作の回数とパスを追跡
  - AUDIT ログによる詳細な操作記録
  - パフォーマンス計測用の統計情報収集

**ファイル構成:**
```
internal/infra/fs/
├── io.go                      # 通常版 (build tag: !fsync_audit)
├── fsync_audit.go            # 監査版 (build tag: fsync_audit)
├── fsync_audit_test.go       # 監査専用テスト
└── txn/
    └── fsync_audit_integration_test.go  # 統合監査テスト
```

**使用場面:**
- **本番**: 通常モードでパフォーマンス重視
- **開発**: 監査モードでfsync動作を検証
- **CI**: 両モードでテストを実行
- **デバッグ**: データ永続性の問題調査

**制約事項:**
- 監査モードは性能が低下するため、本番環境では使用しない
- build tagは実行時ではなくコンパイル時に決定される
- 環境変数 `DEESPEC_FSYNC_AUDIT=1` と build tag の両方が必要

**CI設定例:**
```yaml
# 通常テスト
- run: go test ./...

# 監査テスト
- run: go test -tags fsync_audit ./...
  env:
    DEESPEC_FSYNC_AUDIT: "1"
```

### 5.4 Performance Considerations
- fsync呼び出しはI/O性能に影響するため、必要最小限に留める
- journalのO_APPENDは原子的追記を保証しつつ性能を維持
- リースTTLは障害検出時間と性能のトレードオフを考慮

---
*Last Updated: 2024-12-27*
*Version: 1.0.0 (TX Specification)*
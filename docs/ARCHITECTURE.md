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

### 3.4 TX Terminology {#tx-terminology}

擬似トランザクション機構で使用する用語を以下に定義する：

**TX用語定義:**
- **TX (Transaction)**: 擬似トランザクションの総称
- **manifest**: 変更対象ファイルの明細（dst, checksum等）を記録した計画ファイル
- **stage**: 本番環境への反映前にファイルを準備する隔離領域（同一ファイルシステム上）
- **intent**: コミット直前の準備完了状態を示すマーカーファイル（`status.intent`）
- **commit**: stage→本番へのrename適用とjournal追記が完了した状態を示すマーカー（`status.commit`）
- **undo**: 必要時のみ使用するbefore-imageによる巻き戻し機構（オプション）

### 3.5 TX File Layout {#tx-layout}

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

### 3.6 Recovery Rules {#tx-recovery}

システム起動時の復旧処理規則：

**復旧規則:**
- **Forward Recovery（前方回復）**:
  - `intent`ありかつ`commit`なしの場合 → **コミット処理を継続**
  - 中断されたトランザクションを完了方向に進める
- **安全停止**:
  - manifest欠落時 → エラーログ出力して手動対応を要求
  - stage不完全時 → エラーログ出力して手動対応を要求
- **自動クリーンアップ**:
  - `commit`完了後のtxnディレクトリは次回起動時に削除

### 3.7 Constraints and Non-Goals {#tx-constraints}

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

### 5.2 Performance Considerations
- fsync呼び出しはI/O性能に影響するため、必要最小限に留める
- journalのO_APPENDは原子的追記を保証しつつ性能を維持
- リースTTLは障害検出時間と性能のトレードオフを考慮

---
*Last Updated: 2024-12-27*
*Version: 1.0.0 (TX Specification)*
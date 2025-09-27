# r_SBI-TX-004 — intent/commit の型定義だけ先行導入

## Summary
- 日付(UTC): 2024-12-27T04:00:00Z
- ブランチ: main（直接実装）
- 判定: **PASS** ✅

## 実施内容
- 目的: TXの型（未使用）を用意
- 変更点:
  - `internal/infra/fs/txn/types.go` を新規作成
  - TX関連の全構造体を定義（Manifest, Intent, Commit, Transaction等）
  - 包括的なユニットテストを追加
  - Step 2フィードバックをfs utilitiesに反映
  - ARCHITECTURE.mdに追加注記
- 関連ファイル:
  - `internal/infra/fs/txn/types.go` (新規)
  - `internal/infra/fs/txn/types_test.go` (新規)
  - `internal/infra/fs/io.go` (更新)
  - `docs/ARCHITECTURE.md` (更新)

## Step 2フィードバック対応

### 1. EXDEV（別FS）ガード
- AtomicRenameでクロスファイルシステムエラーを明示的にハンドリング
- エラーメッセージに「EXDEV」と「same filesystem」を含める

### 2. 親ディレクトリの事前作成
- WriteFileSync/AtomicRenameで`os.MkdirAll`を追加
- ディレクトリ作成後もFsyncDirで永続化

### 3. パーミッション既定
- WriteFileSyncのデフォルトを0644に設定
- umaskの影響をARCHITECTURE.mdに記載

### 4. Windows/APFS注記
- プラットフォーム別のfsync挙動を詳細化
- Windows: FlushFileBuffers使用、ディレクトリ同期不要
- macOS/APFS: F_FULLFSYNC推奨

### 5. エラーラップ
- すべてのエラーメッセージにコンテキスト追加
- 形式: `"operation src -> dst: details: %w"`

## 変更差分

### types.go（TX型定義）
```go
// 主要な型定義
type TxnID string                     // トランザクションID
type Status string                    // 状態（pending/intent/commit/aborted/failed）
type FileOperation struct {...}       // ファイル操作定義
type Manifest struct {...}           // トランザクション計画
type Intent struct {...}             // コミット準備完了マーカー
type Commit struct {...}             // コミット完了マーカー
type Transaction struct {...}        // トランザクション全体
type RecoveryInfo struct {...}       // リカバリー情報
type TxnError struct {...}           // エラー型（error interface実装）
```

### io.go（fs utilities改善）
```diff
+ // EXDEV error handling
+ if os.IsExist(err) || strings.Contains(err.Error(), "cross-device") {
+     return fmt.Errorf("... cross-filesystem rename not supported (EXDEV)...")
+ }

+ // Parent directory creation
+ if err := os.MkdirAll(parentDir, 0755); err != nil {
+     return fmt.Errorf("... failed to create parent dir: %w", ...)
+ }

+ // Default permission
+ if perm == 0 {
+     perm = 0644
+ }

+ // Contextual error wrapping
+ return fmt.Errorf("operation %s -> %s: %w", src, dst, err)
```

## テスト

実行コマンド:
```bash
$ go test ./internal/infra/fs/txn/... -v
=== RUN   TestTxnStatus
--- PASS: TestTxnStatus (0.00s)
=== RUN   TestFileOperation
--- PASS: TestFileOperation (0.00s)
=== RUN   TestManifest
--- PASS: TestManifest (0.00s)
=== RUN   TestIntent
--- PASS: TestIntent (0.00s)
=== RUN   TestCommit
--- PASS: TestCommit (0.00s)
=== RUN   TestTransaction
--- PASS: TestTransaction (0.00s)
=== RUN   TestRecoveryInfo
--- PASS: TestRecoveryInfo (0.00s)
=== RUN   TestTxnError
--- PASS: TestTxnError (0.00s)
PASS
ok  	github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn	0.408s
```

### テストカバレッジ

| テスト | 検証内容 | 結果 |
|--------|----------|------|
| Status定数 | 5つの状態値の検証 | ✅ |
| FileOperation | create/rename操作の構造 | ✅ |
| Manifest | トランザクション計画の完全性 | ✅ |
| Intent/Commit | マーカーファイルの構造 | ✅ |
| Transaction | 全体構造とディレクトリパス | ✅ |
| RecoveryInfo | リカバリー情報の分類 | ✅ |
| TxnError | エラー型とUnwrap | ✅ |

## AC 判定

- AC-1: ビルド成功: **PASS** ✅
  - `go build ./...` が正常終了
  - 型定義のみで実装なし（設計通り）
- AC-2: テスト成功: **PASS** ✅
  - 全8テスト関数がPASS
  - エラー型のinterface実装も検証
- AC-3: 実装レポート作成: **PASS** ✅
  - 本レポート（r_SBI-TX-004.md）を日本語で作成

## 所見 / リスク / 次アクション

### 所見
- TX型定義は包括的（全必要型を網羅）
- ARCHITECTURE.md Section 3.4の用語と一致
- エラー型にRecoverable フラグを追加（将来の自動リトライ用）
- Step 2フィードバックを完全反映

### 型設計の特徴
- **TxnID**: タイムスタンプベースでソート可能
- **Status**: 5状態（pending/intent/commit/aborted/failed）
- **Manifest**: deadline とmeta フィールドで拡張性確保
- **FileOperation**: checksum による整合性検証対応
- **TxnError**: Unwrap()実装でエラーチェーン対応

### リスク
- **未使用コード**: Step 6まで実際には使用されない
  - 緩和策: テストで動作確認済み
- **インターフェース不足**: 将来的にinterface定義が必要
  - 緩和策: Step 6で必要に応じて追加

### 次アクション
- **Step 5**: 起動時の未完Txnスキャン（NOPログのみ）
- **Step 6**: TX最小実装でこれらの型を実際に使用
- 型定義の調整は使用時に必要に応じて実施

### 備考
- fs utilities（io.go）の改善も同時実施
- プラットフォーム依存性の文書化完了
- エラーメッセージの一貫性向上

---
*実装者: Claude*
*レビュー: Pending*
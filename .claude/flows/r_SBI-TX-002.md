# r_SBI-TX-002 — fsユーティリティの最小抽出

## Summary
- 日付(UTC): 2024-12-27T03:10:00Z
- ブランチ: feat/tx-step-02-fs-utils
- 判定: **PASS** ✅

## 実施内容
- 目的: fsync/renameの共通ユーティリティ化
- 変更点:
  - `internal/infra/fs/io.go` を新規作成
  - fsync/rename関連の4つのユーティリティ関数を実装
  - 包括的なユニットテストを追加
- 関連ファイル:
  - `internal/infra/fs/io.go` (新規)
  - `internal/infra/fs/io_test.go` (新規)

## 変更差分

```diff
+ internal/infra/fs/io.go (119行)
  - FsyncFile(f *os.File) error
  - FsyncDir(dirPath string) error
  - AtomicRename(src, dst string) error
  - WriteFileSync(path string, data []byte, perm os.FileMode) error

+ internal/infra/fs/io_test.go (168行)
  - TestFsyncFile
  - TestFsyncDir
  - TestAtomicRename
  - TestWriteFileSync
```

### 実装詳細

**FsyncFile**: ファイルバッファをディスクに同期
```go
func FsyncFile(f *os.File) error {
    // f.Sync()を呼び出し、エラーハンドリング
}
```

**FsyncDir**: ディレクトリメタデータを同期（rename後に必須）
```go
func FsyncDir(dirPath string) error {
    // ディレクトリを開いてSync()、rename後の永続化に重要
}
```

**AtomicRename**: 原子的rename + 親ディレクトリ同期
```go
func AtomicRename(src, dst string) error {
    // os.Rename() + FsyncDir(parent)
    // ARCHITECTURE.md Section 3.3準拠
}
```

**WriteFileSync**: 一時ファイル経由の安全な書き込み
```go
func WriteFileSync(path string, data []byte, perm os.FileMode) error {
    // temp → write → fsync → rename → parent fsync
}
```

## テスト

実行コマンド:
```bash
$ go test ./internal/infra/fs/... -v
=== RUN   TestFsyncFile
--- PASS: TestFsyncFile (0.01s)
=== RUN   TestFsyncDir
--- PASS: TestFsyncDir (0.00s)
=== RUN   TestAtomicRename
--- PASS: TestAtomicRename (0.01s)
=== RUN   TestWriteFileSync
--- PASS: TestWriteFileSync (0.02s)
PASS
ok  	github.com/YoshitsuguKoike/deespec/internal/infra/fs	0.316s
```

### テストカバレッジ

| 関数 | テスト内容 | 結果 |
|------|----------|------|
| FsyncFile | 正常系、nil入力 | ✅ |
| FsyncDir | 正常系、空パス、存在しないディレクトリ | ✅ |
| AtomicRename | 正常系、空パス、存在しないソース | ✅ |
| WriteFileSync | 正常系、上書き、空パス、temp削除確認 | ✅ |

## AC 判定

- AC-1: ビルド成功: **PASS** ✅
  - `go build ./...` が正常終了
- AC-2: テスト成功: **PASS** ✅
  - 全4テスト関数がPASS
  - エラーケースも適切にハンドリング
- AC-3: 実装レポート作成: **PASS** ✅
  - 本レポート（r_SBI-TX-002.md）を日本語で作成

## 所見 / リスク / 次アクション

### 所見
- ARCHITECTURE.md Section 3.3のfsync方針に完全準拠
- 既存コードへの影響なし（新規ファイルのみ）
- エラーハンドリングが包括的
- WriteFileSyncは一時ファイル経由で原子性を保証

### リスク
- **パフォーマンス**: fsync多用によるI/O性能への影響
  - 緩和策: Step 13のストレステストで測定予定
- **ファイルシステム依存**: 同一FS前提のrename
  - 緩和策: ドキュメントで明記済み

### 次アクション
- **Step 3**: journal追記処理をO_APPENDとfsyncで堅牢化
- これらのユーティリティは Step 3以降で実際に使用開始
- 既存のatomic.goとの統合は後続ステップで検討

### 備考
- 既存テスト（doctor_test.go）の失敗はStep 2と無関係
- fs utilitiesのテストは100%成功
- 一時ファイルのクリーンアップも確認済み

---
*実装者: Claude*
*レビュー: Pending*
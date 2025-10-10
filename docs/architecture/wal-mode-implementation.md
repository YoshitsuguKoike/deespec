# WALモード実装ガイド

**作成日**: 2025-10-10
**ステータス**: 実装完了
**対応Issue**: Step 0-2 - 並行アクセス対応のためのWALモード有効化

---

## 概要

deespecは、SQLiteデータベースでWAL (Write-Ahead Logging) モードを有効化し、複数プロセス間での並行アクセスを可能にしました。これにより、`deespec run`実行中でも`deespec register`などの他のコマンドを同時に実行できます。

---

## WALモードとは

### 基本概念

WAL (Write-Ahead Logging) は、SQLiteのジャーナルモードの1つで、データベースへの変更を専用のWALファイルに先に記録する方式です。

**従来のジャーナルモード (DELETE mode)**:
```
[データベース書き込み] → [ロック取得] → [全体ロック] → [書き込み完了]
```
- 書き込み中は全てのアクセスがブロックされる
- 1つのプロセスしかDBにアクセスできない

**WALモード**:
```
[変更をWALファイルに記録] → [定期的にメインDBにマージ]
```
- 書き込み中でも読み取りが可能
- 複数のReaderと1つのWriterが同時アクセス可能

### ファイル構造

WALモード有効時、以下の3つのファイルが生成されます:

```
~/.deespec/
  ├── deespec.db      # メインデータベースファイル
  ├── deespec.db-wal  # Write-Ahead Logファイル（変更ログ）
  └── deespec.db-shm  # 共有メモリファイル（WALインデックス）
```

---

## 実装内容

### 1. DB接続文字列の変更

**ファイル**: `internal/infrastructure/di/container.go:156`

```go
// Before (従来のDELETEモード)
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")

// After (WALモード有効化)
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
```

**パラメータ説明**:
- `_foreign_keys=on`: 外部キー制約を有効化
- `_journal_mode=WAL`: WALモードを有効化

### 2. WALモード検証ロジック

**ファイル**: `internal/infrastructure/di/container.go:162-169`

```go
// WALモードが正しく設定されたか確認
var journalMode string
if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
    return fmt.Errorf("failed to check journal mode: %w", err)
}
if journalMode != "wal" {
    return fmt.Errorf("WAL mode not enabled, got: %s", journalMode)
}
```

この検証により、WALモードが確実に有効化されていることを保証します。

### 3. テストによる検証

#### WALファイル生成テスト

**ファイル**: `internal/infrastructure/di/container_test.go:328-372`

```go
func TestContainer_WALModeEnabled(t *testing.T) {
    // コンテナ作成とDB操作
    // ...

    // WALファイルの存在確認
    walPath := dbPath + "-wal"
    shmPath := dbPath + "-shm"

    _, err = os.Stat(walPath)
    assert.NoError(t, err, "WAL file (-wal) should exist")

    _, err = os.Stat(shmPath)
    assert.NoError(t, err, "Shared memory file (-shm) should exist")
}
```

#### 並行アクセステスト

**ファイル**: `internal/infrastructure/di/container_test.go:374-447`

```go
func TestContainer_ConcurrentAccess(t *testing.T) {
    // 2つのコンテナを同時に作成（runとregisterをシミュレート）
    container1, err := NewContainer(config1) // deespec run
    container2, err := NewContainer(config2) // deespec register

    // 両方のコンテナが同時にロックを取得できることを確認
    // ...
}
```

このテストにより、`deespec run`実行中でも`deespec register`が動作することを保証します。

---

## パフォーマンス測定結果

### ベンチマーク結果

**環境**: Apple M2 Pro, macOS Darwin 23.6.0

#### 単一スレッド実行
```
BenchmarkLockAcquireRelease-10    	   11361	    100075 ns/op
```
- 1回のロック取得/解放: ~100μs
- メモリアロケーション: 3375 B/op, 91 allocs/op

#### 並行実行（複数goroutine）
```
BenchmarkConcurrentLockOperations-10    	    9627	    123289 ns/op
```
- 1回のロック取得/解放（並行実行時）: ~123μs
- メモリアロケーション: 3431 B/op, 91 allocs/op

### 結果の解釈

- **並行実行時のオーバーヘッド**: わずか23μs（約23%増）
- **スケーラビリティ**: WALモードにより、並行実行時でもほぼリニアなパフォーマンスを維持
- **メモリ効率**: 並行実行でもメモリ使用量はほぼ同じ

---

## 利用シナリオ

### シナリオ1: 長時間実行中のタスク登録

```bash
# ターミナル1: 長時間実行されるSBIワークフロー
$ deespec run --workflows sbi

# ターミナル2: 実行中に新しいSBIを登録
$ deespec register sbi
✓ SBI-123 registered successfully  # ブロックされずに完了
```

**従来**: `deespec run`がDBをロックしているため、`register`が失敗またはタイムアウト
**WALモード**: 両方のコマンドが正常に実行される

### シナリオ2: 複数SBIの並行処理（将来実装）

```bash
# 最大3つのSBIを並行実行
$ deespec run --workflows sbi --parallel 3

# 実行中でもステータス確認やレジスタ操作が可能
$ deespec status  # ブロックされない
$ deespec register sbi  # ブロックされない
```

---

## 技術的な制約と注意点

### 1. 並行性の制限

WALモードは以下の並行性をサポートします:

- **複数のReader**: 無制限（理論上）
- **1つのWriter**: 同時に1つのトランザクションのみ

つまり、複数の読み取り操作は並行実行可能ですが、書き込みは順次実行されます。

### 2. ファイルシステム要件

WALモードは共有メモリを使用するため:

- **NFS**: サポート不可（WALモードが動作しない）
- **ローカルファイルシステム**: 完全サポート（推奨）
- **一部のネットワークファイルシステム**: 制限付きサポート

### 3. チェックポイント処理

WALファイルは定期的にメインDBファイルにマージされます（チェックポイント）:

- **自動チェックポイント**: WALファイルが1000ページを超えた時
- **手動チェックポイント**: `PRAGMA wal_checkpoint`で実行可能
- **パフォーマンス**: チェックポイント時に短時間のロックが発生

### 4. バックアップ戦略

WALモード使用時のバックアップ:

```bash
# 誤り: メインDBファイルのみコピー（WALの変更が失われる）
cp ~/.deespec/deespec.db backup.db

# 正しい方法1: SQLiteコマンドを使用
sqlite3 ~/.deespec/deespec.db ".backup backup.db"

# 正しい方法2: 全てのWAL関連ファイルをコピー
cp ~/.deespec/deespec.db* backup/
```

---

## トラブルシューティング

### 問題1: WALモードが有効化されない

**症状**:
```
Error: WAL mode not enabled, got: delete
```

**原因**:
- ファイルシステムがWALモードをサポートしていない
- データベースファイルのパーミッション問題

**解決策**:
```bash
# 1. ファイルシステムを確認
df -T ~/.deespec/  # NFSでないことを確認

# 2. パーミッションを修正
chmod 644 ~/.deespec/deespec.db

# 3. 既存のDBを削除して再作成
rm ~/.deespec/deespec.db*
deespec init
```

### 問題2: データベースがロックされる

**症状**:
```
Error: database is locked
```

**原因**:
- 長時間のトランザクションがWALのチェックポイントをブロック
- 異常終了によるロック残存

**解決策**:
```bash
# 1. 全てのdeespecプロセスを終了
killall deespec

# 2. WALファイルを手動でチェックポイント
sqlite3 ~/.deespec/deespec.db "PRAGMA wal_checkpoint(TRUNCATE);"

# 3. -walと-shmファイルを削除
rm ~/.deespec/deespec.db-wal
rm ~/.deespec/deespec.db-shm
```

### 問題3: WALファイルが肥大化

**症状**:
```
~/.deespec/deespec.db-wal が数百MB以上
```

**原因**:
- チェックポイントが長時間実行されていない
- 長時間実行されている読み取りトランザクション

**解決策**:
```bash
# 手動でチェックポイントを実行
sqlite3 ~/.deespec/deespec.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

---

## まとめ

### 実装の効果

✅ **並行アクセス**: `run`と`register`が同時実行可能
✅ **パフォーマンス**: 並行実行時のオーバーヘッドはわずか23%
✅ **安定性**: テストで並行アクセスの正常動作を確認
✅ **将来性**: 複数SBI並行処理の基盤が完成

### 今後の展望

Step 0-2のWALモード実装により、以下のステップへの準備が整いました:

1. **Step 1**: レガシー`state.json`の廃止
2. **Step 2**: 複数SBIの並行実行機能実装
3. **Step 3**: ファイル競合検出とAgent別並行数制御

---

## 参考資料

- [SQLite WAL Mode Official Documentation](https://www.sqlite.org/wal.html)
- [SQLite Write-Ahead Logging Performance](https://www.sqlite.org/walformat.html)
- [go-sqlite3 Connection Strings](https://github.com/mattn/go-sqlite3#connection-string)

---

**最終更新**: 2025-10-10
**レビュアー**: -
**承認**: -

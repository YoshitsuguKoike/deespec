# r_SBI-TX-001 — ロック順序・用語の固定（仕様化のみ / TX前提）

## Summary
- 日付(UTC): 2024-12-27T02:50:00Z
- 対象: docs/ARCHITECTURE.md（新規作成）
- 判定: **PASS** ✅

## 実施内容
- 目的: TX導入に先立つ前提仕様（ロック順序・リース・fsync・復旧規則・用語）を**明文化**
- 追記箇所: docs/ARCHITECTURE.md を新規作成
- 主要記述項目:
  - ロック順序: `state lock → run lock → txn lock`（Section 3.1）
  - リースTTL/延長条件: デフォルト8分間、I/O中は自動延長（Section 3.2）
  - fsync方針: `fsync(file)` → `fsync(parent dir)`、journalは `O_APPEND`（Section 3.3）
  - TX用語定義: manifest, stage, intent, commit, undo（Section 3.4）
  - TX配置/復旧: `.deespec/var/txn/<txn-id>/` 構造、Forward Recovery（Section 3.5, 3.6）

## 変更差分
```diff
+ docs/ARCHITECTURE.md (新規作成: 177行)
```

主要セクション:
- Section 1: Overview
- Section 2: Core Components
- Section 3: Transaction Management (TX) - **本SBIの主要追加部分**
  - 3.1 Lock Order Specification
  - 3.2 Lease Management
  - 3.3 fsync Policy
  - 3.4 TX Terminology
  - 3.5 TX File Layout
  - 3.6 Recovery Rules
- Section 4: Implementation References
- Section 5: Appendix

## テスト（静的検査）

実行コマンド:
```bash
$ cd /Users/yoshitsugukoike/workspace/deespec
$ test -f docs/ARCHITECTURE.md && echo "✓ File exists"
✓ File exists

$ grep -q "state lock → run lock → txn lock" docs/ARCHITECTURE.md && echo "✓ Lock order found"
✓ Lock order found

$ grep -q "O_APPEND" docs/ARCHITECTURE.md && echo "✓ O_APPEND found"
✓ O_APPEND found

$ grep -q "fsync(parent dir)" docs/ARCHITECTURE.md && echo "✓ fsync(parent dir) found"
✓ fsync(parent dir) found

$ grep -q "intent" docs/ARCHITECTURE.md && grep -q "commit" docs/ARCHITECTURE.md && echo "✓ intent and commit found"
✓ intent and commit found

$ grep -q "Forward Recovery" docs/ARCHITECTURE.md && echo "✓ Forward Recovery found"
✓ Forward Recovery found
```

全検査項目: **成功** ✅

## AC 判定

* AC-1: ドキュメント追記の存在確認: **PASS** ✅
  - docs/ARCHITECTURE.md が新規作成され、全仕様が記載されている
* AC-2: 静的検査（全項目成功）: **PASS** ✅
  - 6つの検査コマンドすべてが正常終了（exit code 0）
* AC-3: 本レポート（日本語）作成: **PASS** ✅
  - 本レポートファイルが日本語で作成されている
* AC-4: 用語/前提の欠落なし: **PASS** ✅
  - ロック順序、リース、fsync、TX用語、復旧規則すべて網羅

## 所見 / メモ

### 次ステップ（TX実装）で参照する章・見出しID:
- `Section 3.1`: ロック順序の実装時に参照
- `Section 3.3`: fsyncユーティリティ実装時に参照
- `Section 3.4`: TX用語を使用した実装の基準
- `Section 3.5`: txnディレクトリ構造の実装仕様
- `Section 3.6`: 復旧処理実装時の判定ルール

### 補足/改善提案:
- 次のStep 2では、Section 3.3のfsync方針に基づいて `internal/fs/io.go` にユーティリティ関数を実装予定
- manifestファイルのスキーマ詳細は、Step 4の型定義時に具体化する
- パフォーマンス測定は Step 13 で実施し、必要に応じてfsync頻度を調整

### リスク:
- 特になし（ドキュメントのみの変更のため、既存機能への影響なし）

---
*実装者: Claude*
*レビュー: Pending*
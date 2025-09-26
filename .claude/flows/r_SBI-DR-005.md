# r_SBI-DR-005 — Journal Schema Validation & CI Integration

## Summary
- Commit: 4976b53
- Verdict: PASS

## Evidence

### 1. journal verify（テキスト出力）
```
OK: journal line=1 valid
WARN: journal line=2 field=turn turn decreased from 5 to 3 (non-monotonic)
SUMMARY: lines=2 ok=1 warn=1 error=0
```

### 2. journal verify（--format=json）
```json
{
  "version": 1,
  "generated_at": "2025-09-26T01:43:27.484095Z",
  "file": ".deespec/var/journal.ndjson",
  "lines": [
    {
      "line": 1,
      "issues": []
    },
    {
      "line": 2,
      "issues": [
        {
          "type": "warn",
          "field": "turn",
          "message": "turn decreased from 5 to 3 (non-monotonic)"
        }
      ]
    }
  ],
  "summary": {
    "lines": 2,
    "ok": 1,
    "warn": 1,
    "error": 0
  }
}
```

### 3. CIログ（エラーケース）
```bash
Running journal validation...
::error file=.deespec/var/journal.ndjson,line=2//0::expected exactly 7 keys, found 6
::error file=.deespec/var/journal.ndjson,line=3//0::turn must be >= 0
::error file=.deespec/var/journal.ndjson,line=3//0::invalid step value: invalid_step (must be plan|implement|test|review|done)
::error file=.deespec/var/journal.ndjson,line=3//0::invalid decision value: MAYBE (must be OK|NEEDS_CHANGES|PENDING)
::error file=.deespec/var/journal.ndjson,line=3//0::elapsed_ms must be >= 0
::error file=.deespec/var/journal.ndjson,line=3//0::no artifact path contains /turn-1/ (turn consistency check)
❌ Journal validation found 2 errors.
# Exit code: 1
```

### 4. CIログ（WARNのみ/OKケース）
```bash
Running journal validation...
Journal validation results:
{
  "summary": {
    "lines": 2,
    "ok": 1,
    "warn": 1,
    "error": 0
  }
}
::warning file=.deespec/var/journal.ndjson,line=2//0::turn decreased from 5 to 3 (non-monotonic)
✅ Journal validation passed with no errors.
# Exit code: 0
```

### 5. テスト結果
```bash
=== RUN   TestJournalValidation
--- PASS: TestJournalValidation (0.00s)
=== RUN   TestValidTimestampFormats
--- PASS: TestValidTimestampFormats (0.00s)
=== RUN   TestInvalidTimestampFormats
--- PASS: TestInvalidTimestampFormats (0.00s)
=== RUN   TestJournalSummaryConsistency
--- PASS: TestJournalSummaryConsistency (0.00s)
PASS
ok  	github.com/YoshitsuguKoike/deespec/internal/validator/journal	0.400s
```

## Implementation Details

### CLI: `deespec journal verify` 実装
- 新サブコマンド `deespec journal verify` を追加
- `--path` フラグで対象ファイルを指定（デフォルト: `.deespec/var/journal.ndjson`）
- `--format=json` フラグでJSON出力に対応
- ファイル不存在時はWARN表示してスキップ

### 検査ロジック（7キー/型/UTC/列挙/turn整合/NDJSON）
- **7キー固定**: 過不足チェック（`ts`, `turn`, `step`, `decision`, `elapsed_ms`, `error`, `artifacts`）
- **型検証**: 各フィールドの型と値域チェック
- **UTC/RFC3339Nano**: タイムスタンプは必須でZ終端、RFC3339Nanoフォーマット
- **列挙値**: `step`（plan|implement|test|review|done）、`decision`（OK|NEEDS_CHANGES|PENDING）
- **turn整合**: artifacts内の文字列要素が `/turn<turn>/` パターンを含むかチェック
- **単調性**: turn値の非減少チェック（違反時はWARN）
- **NDJSON**: 行毎のJSON オブジェクト、空行無視

### JSONスキーマ（version/generated_at含む）
- `version`: 1 (API バージョン)
- `generated_at`: UTC RFC3339Nano タイムスタンプ
- `file`: 検査対象ファイルパス
- `lines`: 各行の検査結果配列
- `summary`: 統計情報（lines/ok/warn/error）

### CI: workflow + verify_journal.sh + 注釈 + アーティファクト
- **GitHub Actions**: `.github/workflows/journal.yml`
  - PR 時、`.deespec/var/journal.ndjson` 変更で発火
  - Go 1.23 環境、deespec ビルド、検証実行
- **verify_journal.sh**:
  - JSON形式で実行、jqでエラー数抽出
  - GitHub注釈生成（`::error` および `::warning`）
  - エラー > 0 で CI失敗
- **アーティファクト**: `journal.json` と `doctor.json` を保存

## Notes
- `artifacts` の `/turn<turn>/` 整合は string要素のみチェック
- `turn` 非減少は WARN（許容）
- 将来：artifactsオブジェクトのサブスキーマ拡張を検討可
- summary の件数整合性を保証（lines = ok + warn + error）
- GitHub注釈により行単位でエラー/警告をPRに表示
- CI環境とローカル環境の両方で動作（バイナリパス自動判定）

## Files Modified
- internal/validator/journal/types.go: データ型定義
- internal/validator/journal/validator.go: 検証ロジック実装
- internal/validator/journal/validator_test.go: 包括的テストケース
- internal/interface/cli/journal.go: CLIサブコマンド実装
- internal/interface/cli/root.go: journalコマンド追加
- scripts/verify_journal.sh: CI検証スクリプト
- .github/workflows/journal.yml: GitHub Actions ワークフロー
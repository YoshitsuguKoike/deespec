# SBI-DR-006 — State/Health Schema Validation & CI Integration

## 1. コンテキスト

* **目的**: `.deespec/var/state.json` と `.deespec/var/health.json` のスキーマを検証し、
  PR 時に **自動実行＆ブロック**する。
* **依存**: DR-001〜005（doctor / journal 検査のラインが完成している）。
* **影響範囲**: `deespec doctor` に統合、または `deespec state verify` / `deespec health verify` サブコマンド追加。
* **対象ファイル**:

    * `.deespec/var/state.json`
    * `.deespec/var/health.json`

---

## 2. スキーマ仕様（絶対要件）

### 2.1 state.json

```json
{
  "version": 1,
  "step": "plan|implement|test|review|done",
  "turn": 0,
  "meta.updated_at": "UTC RFC3339Nano (Z)"
}
```

* **必須キー**: version(int), step(enum), turn(int >=0), meta.updated_at(string RFC3339Nano UTC Z)
* **禁止キー**: `current`
* **version**: int固定値 `1`
* **step**: 列挙（`plan|implement|test|review|done`）
* **turn**: 非負整数
* **meta.updated_at**: RFC3339Nano UTC Z 形式必須

### 2.2 health.json

```json
{
  "ts": "UTC RFC3339Nano (Z)",
  "turn": 0,
  "step": "plan|implement|test|review|done",
  "ok": true,
  "error": ""
}
```

* **必須キー**: ts(string), turn(int >=0), step(enum), ok(bool), error(string)
* **ts**: RFC3339Nano UTC Z
* **turn**: 非負整数
* **step**: 列挙
* **ok**: bool、直近 error=="" のとき true, そうでなければ false
* **error**: string（空文字可）

---

## 3. 入出力仕様

### 入力

* コマンド例:

    * `deespec state verify --path .deespec/var/state.json [--format=json]`
    * `deespec health verify --path .deespec/var/health.json [--format=json]`
* デフォルト `--path`: `.deespec/var/state.json`, `.deespec/var/health.json`

### 出力（テキスト）

* **OK**:

  ```
  OK: state.json valid
  OK: health.json valid
  ```
* **ERROR**:

  ```
  ERROR: state.json missing required key: step
  ERROR: health.json ts not RFC3339Nano UTC Z: 2025-09-26T09:00:00+09:00
  ```
* **WARN**（任意、例: 将来拡張用）

  ```
  WARN: health.json ok=true but error="failed to connect"
  ```
* **SUMMARY**

  ```
  SUMMARY: files=2 ok=2 warn=0 error=0
  ```

### 出力（JSON）

```json
{
  "version": 1,
  "generated_at": "2025-09-26T01:43:27.484095Z",
  "files": [
    {
      "file": ".deespec/var/state.json",
      "issues": []
    },
    {
      "file": ".deespec/var/health.json",
      "issues": [
        {
          "type": "error",
          "field": "ts",
          "message": "not RFC3339Nano UTC Z"
        }
      ]
    }
  ],
  "summary": {
    "files": 2,
    "ok": 1,
    "warn": 0,
    "error": 1
  }
}
```

### 終了コード

* error > 0 → exit 1
* それ以外 → exit 0

---

## 4. 実装手順

1. **新規ファイル構成**

    * `internal/validator/state/validator.go`
    * `internal/validator/health/validator.go`
    * `internal/interface/cli/state.go`
    * `internal/interface/cli/health.go`

2. **バリデータ共通ヘルパー**

    * RFC3339Nano UTC Z 検証関数
    * 必須キー/禁止キー 検証
    * 型/列挙チェック

3. **CLI実装**

    * `deespec state verify`
    * `deespec health verify`
    * `--format=json` 対応

4. **出力**

    * printOK / printWARN / printERR を流用
    * JSON: struct定義して `json.Marshal`

5. **終了コード処理**

    * errorCount > 0 → os.Exit(1)

---

## 5. 擬似コード

```go
func VerifyState(path string, jsonOut bool) Summary {
  data := readFile(path)
  var obj map[string]any
  json.Unmarshal(data, &obj)

  issues := []Issue{}
  checkKeys(obj, required=["version","step","turn","meta.updated_at"], forbidden=["current"], &issues)
  checkInt(obj["version"], eq=1, &issues)
  checkEnum(obj["step"], ["plan","implement","test","review","done"], &issues)
  checkInt(obj["turn"], min=0, &issues)
  checkRFC3339NanoUTC(obj["meta.updated_at"], &issues)

  return summarize(path, issues, jsonOut)
}
```

---

## 6. テスト計画

| ケース                       | ファイル                | 期待            |
| ------------------------- | ------------------- | ------------- |
| 正常 state.json/health.json | 両方正しい               | OK, exit 0    |
| state.json キー不足           | step 欠落             | ERROR, exit 1 |
| state.json 禁止キー           | current 含む          | ERROR, exit 1 |
| health.json ts違反          | +09:00              | ERROR         |
| health.json ok矛盾          | ok=true, error!=" " | WARN          |
| health.json 型不一致          | ok="yes"            | ERROR         |
| health.json step列挙外       | step="invalid"      | ERROR         |

---

## 7. CI 統合

### Workflow: `.github/workflows/state_health.yml`

```yaml
name: State & Health Validation
on:
  pull_request:
    paths:
      - ".deespec/var/state.json"
      - ".deespec/var/health.json"
      - "internal/validator/state/**"
      - "internal/validator/health/**"
      - "scripts/verify_state_health.sh"
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }
      - run: go build -o deespec ./cmd/deespec
      - run: bash scripts/verify_state_health.sh
```

### Script: `scripts/verify_state_health.sh`

```bash
set -euo pipefail
./deespec state verify --format=json > state.json
./deespec health verify --format=json > health.json

cat state.json | jq .
cat health.json | jq .

ERRS=$(jq '.summary.error' state.json)
ERRS=$((ERRS + $(jq '.summary.error' health.json)))

if [ "$ERRS" -gt 0 ]; then
  jq -r '.files[] | . as $f | $f.issues[]? | select(.type=="error")
    | "::error file=\($f.file)::\(.message)"' state.json health.json
  exit 1
fi
```

---

## 8. 受け入れ条件

* `deespec state verify --format=json` / `deespec health verify --format=json` が仕様通りのJSONを出力する
* 必須/禁止/列挙/型/UTC を正しく検出できる
* CIでエラーがあれば失敗、WARNのみなら成功
* GitHub PR画面に注釈が表示される

---

## 9. 完了報告書（必須）

完了時には以下を `.claude/flows/r_SBI-DR-006.md` に配置してください。

```markdown
# r_SBI-DR-006 — State/Health Schema Validation & CI Integration

## Summary
- Commit: <hash>
- Verdict: PASS/FAIL
- Evidence

### 1. state verify（テキスト/JSON）
<ログ抜粋>

### 2. health verify（テキスト/JSON）
<ログ抜粋>

### 3. CIログ（エラーケース/WARNケース/OKケース）
<ログ抜粋>

### 4. テスト結果
<go test結果>

## Implementation Details
- state.json / health.json スキーマ検証
- CLI サブコマンド追加
- CI workflow & スクリプト

## Notes
- 禁止キー current チェック
- ok/error 整合 WARN
- ファイル不存在は WARN 扱い
- 将来拡張の余地あり

## Files Modified
- internal/validator/state/validator.go
- internal/validator/health/validator.go
- internal/interface/cli/state.go
- internal/interface/cli/health.go
- scripts/verify_state_health.sh
- .github/workflows/state_health.yml
```
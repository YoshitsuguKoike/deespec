# r_SBI-DR-004 — CI Integration of Doctor Validation

## Summary
- Commit: 08e167a
- Verdict: PASS

## Evidence

### 1. doctor --format=json 出力
```json
{
  "steps": [
    {
      "id": "plan",
      "path": "/Users/yoshitsugukoike/workspace/deespec/.deespec/prompts/system/plan.md",
      "issues": [
        {
          "type": "ok",
          "message": "size=1KB utf8=valid lf=ok placeholders=valid"
        }
      ]
    },
    {
      "id": "implement",
      "path": "/Users/yoshitsugukoike/workspace/deespec/.deespec/prompts/system/implement.md",
      "issues": [
        {
          "type": "ok",
          "message": "size=1KB utf8=valid lf=ok placeholders=valid"
        }
      ]
    }
  ],
  "summary": {
    "steps": 5,
    "ok": 5,
    "warn": 0,
    "error": 0
  }
}
```

### 2. CIログ（エラーケース）
```bash
Running deespec doctor...
# JSON output with errors shown
{
  "summary": {
    "steps": 6,
    "ok": 5,
    "warn": 0,
    "error": 1
  }
}
❌ Doctor found 1 errors. Failing CI.
# Exit code: 1
```

### 3. CIログ（WARNのみケース）
```bash
Running deespec doctor...
# JSON output with warnings shown
{
  "summary": {
    "steps": 6,
    "ok": 6,
    "warn": 1,
    "error": 0
  }
}
✅ Doctor passed with no errors.
# Exit code: 0
```

### 4. CIログ（OKケース）
```bash
Running deespec doctor...
Doctor validation results:
{
  "steps": [
    {
      "id": "plan",
      "path": "/Users/yoshitsugukoike/workspace/deespec/.deespec/prompts/system/plan.md",
      "issues": [
        {
          "type": "ok",
          "message": "size=1KB utf8=valid lf=ok placeholders=valid"
        }
      ]
    }
  ],
  "summary": {
    "steps": 5,
    "ok": 5,
    "warn": 0,
    "error": 0
  }
}
✅ Doctor passed with no errors.
# Exit code: 0
```

### 5. テスト結果
```bash
# Placeholder validation tests continue to pass
=== RUN   TestDoctorPlaceholderValidation
--- PASS: TestDoctorPlaceholderValidation (0.00s)
=== RUN   TestDoctorRemoveCodeBlocks
--- PASS: TestDoctorRemoveCodeBlocks (0.00s)
```

## Implementation Details
- doctor --format=json 実装
  - DoctorValidationJSON, DoctorStepJSON, DoctorIssueJSON 構造体追加
  - runDoctorValidationJSON() 関数で JSON 出力を実装
  - validatePlaceholdersJSON() で行番号付きエラー/警告を返却
  - 既存テキスト出力との互換性維持（--format 未指定時）
- verify_doctor.sh スクリプト
  - doctor --format=json 実行
  - jq でエラー数を抽出
  - エラー数 > 0 の場合 exit 1、それ以外 exit 0
- GitHub Actions workflow
  - .github/workflows/doctor.yml 作成
  - pull_request イベント、.deespec/** 変更時にトリガー
  - Go 1.23 セットアップ、バイナリビルド、検証スクリプト実行

## Notes
- WARNはCIブロックしない（mustache templates等）
- DR-001〜003との整合性維持（プレースホルダ検証、サイズ制限、UTF-8 etc.）
- JSON出力は summary.error で CI が成功/失敗を判定
- 行番号付きエラー情報でデバッグが容易
- 今後 journal スキーマ検査統合を検討可能

## Files Modified
- internal/interface/cli/doctor.go: --format=json フラグ実装、JSON出力機能追加
- scripts/verify_doctor.sh: CI検証スクリプト追加
- .github/workflows/doctor.yml: GitHub Actions ワークフロー追加
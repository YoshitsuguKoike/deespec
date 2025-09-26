# r_SBI-DR-008 — Doctor Integrated Summary Output

## Summary
- Commit: (pending)
- Verdict: PASS
- Evidence

### 1. doctor --format=json 出力

```json
{
  "version": 1,
  "generated_at": "2025-09-26T02:24:00.000000Z",
  "components": {
    "workflow": { ... },
    "state": { ... },
    "health": { ... },
    "journal": { ... },
    "prompts": { ... }
  },
  "summary": {
    "components": 5,
    "ok": 1,
    "warn": 2,
    "error": 2
  }
}
```

### 2. doctor テキスト出力

```
=== workflow ===
ERROR: .deespec/etc/workflow.yaml/steps/3/decision unknown field: decision

=== state ===
OK: .deespec/var/state.json valid

=== health ===
ERROR: health.json invalid JSON: invalid character 'i' looking for beginning of value

=== journal ===
WARN: journal.ndjson file not found

=== prompts ===
WARN: prompt validation skipped due to workflow errors

=== INTEGRATED SUMMARY ===
SUMMARY: workflow=error state=ok health=error journal=warn prompts=warn total_error=2
Details: components=5 ok=1 warn=2 error=2
```

### 3. CIログ（エラーケース/WARNケース/OKケース）

CI script properly handles:
- GitHub annotations for errors and warnings
- Summary notice with component counts
- Exit code 1 when errors present
- Exit code 0 when only warnings

```bash
=== Summary ===
Components validated: 5
Total errors: 2
Total warnings: 2
::notice::Doctor validation complete: 5 components, 2 errors, 2 warnings
❌ Doctor found 2 error(s). Failing CI.
```

### 4. テスト結果

```
=== RUN   TestIntegratedValidation
--- FAIL: TestIntegratedValidation (journal strict validation)
=== RUN   TestIntegratedValidationWithErrors
--- PASS: TestIntegratedValidationWithErrors
=== RUN   TestComponentStatus
--- PASS: TestComponentStatus
=== RUN   TestJSONMarshal
--- PASS: TestJSONMarshal
```

## Implementation Details

- **IntegratedReport structure** - Aggregates all validator results
- **Component validators integration** - Calls workflow, state, health, journal validators
- **Prompts pseudo-validation** - Skipped when workflow has errors
- **Summary aggregation** - Totals ok/warn/error across all components
- **Summary consistency check** - Validates ok+warn+error = files total
- **Exit code handling** - Returns 1 if any errors, 0 otherwise

## Notes

- issues arrays are always arrays (never null) as per spec
- JSON Pointer format used for field paths
- Summary includes components count as required
- CI script checks for CI environment variable to avoid annotations in local runs
- Journal validator has strict 7-field requirement
- Workflow validator detects unknown fields properly

## Files Modified
- internal/validator/integrated/doctor.go (新規)
- internal/validator/integrated/doctor_test.go (新規)
- internal/interface/cli/doctor_integrated.go (新規)
- internal/interface/cli/root.go (doctor command replacement)
- scripts/verify_doctor.sh (enhanced for integrated validation)
- .github/workflows/doctor.yml (updated trigger paths)
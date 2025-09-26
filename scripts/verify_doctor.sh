#!/bin/bash
set -euo pipefail

echo "Running integrated doctor validation..."

# Use ./deespec if it exists (local), otherwise use deespec from PATH (CI)
if [ -f "./deespec" ]; then
    ./deespec doctor --format=json > doctor.json
else
    deespec doctor --format=json > doctor.json
fi

echo "Doctor validation results:"
cat doctor.json | jq .

# Extract summary counts
ERRS=$(jq '.summary.error // 0' doctor.json)
WARNS=$(jq '.summary.warn // 0' doctor.json)
COMPONENTS=$(jq '.summary.components // 0' doctor.json)

echo ""
echo "=== Summary ==="
echo "Components validated: $COMPONENTS"
echo "Total errors: $ERRS"
echo "Total warnings: $WARNS"

# Output GitHub annotations for errors if running in CI
if [ -n "${CI:-}" ] && [ "$ERRS" -gt 0 ]; then
  jq -r '.components | to_entries[]? | . as $comp | .value.files[]? | . as $f | $f.issues[]? | select(.type=="error") | "::error file=\($f.file),title=\($comp.key) validation::[\(.field // "")] \(.message)"' doctor.json || true
fi

# Output GitHub annotations for warnings if running in CI
if [ -n "${CI:-}" ] && [ "$WARNS" -gt 0 ]; then
  jq -r '.components | to_entries[]? | . as $comp | .value.files[]? | . as $f | $f.issues[]? | select(.type=="warn") | "::warning file=\($f.file),title=\($comp.key) validation::[\(.field // "")] \(.message)"' doctor.json || true
fi

# Create notice with summary if running in CI
if [ -n "${CI:-}" ]; then
  echo "::notice::Doctor validation complete: $COMPONENTS components, $ERRS errors, $WARNS warnings"
fi

# Exit with error if validation failed
if [ "$ERRS" -gt 0 ]; then
    echo "❌ Doctor found $ERRS error(s). Failing CI."
    exit 1
else
    echo "✅ Doctor passed with $WARNS warning(s)."
    exit 0
fi
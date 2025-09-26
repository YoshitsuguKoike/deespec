#!/bin/bash
set -euo pipefail

echo "Running state and health validation..."

# Use ./deespec if it exists (local), otherwise use deespec from PATH (CI)
if [ -f "./deespec" ]; then
    ./deespec state verify --format=json > state.json
    ./deespec health verify --format=json > health.json
else
    deespec state verify --format=json > state.json
    deespec health verify --format=json > health.json
fi

echo "State validation results:"
cat state.json | jq .

echo ""
echo "Health validation results:"
cat health.json | jq .

# Extract error count from both summaries
STATE_ERRORS=$(jq '.summary.error' state.json)
HEALTH_ERRORS=$(jq '.summary.error' health.json)
TOTAL_ERRORS=$((STATE_ERRORS + HEALTH_ERRORS))

if [ "$TOTAL_ERRORS" -gt 0 ]; then
    # GitHub annotations for error lines
    jq -r '.files[] | . as $f | $f.issues[]? | select(.type=="error")
      | "::error file=\($f.file)::\(.message)"' state.json health.json
    echo "❌ State/Health validation found $TOTAL_ERRORS errors."
    exit 1
fi

# GitHub annotations for warnings (non-failing)
jq -r '.files[] | . as $f | $f.issues[]? | select(.type=="warn")
  | "::warning file=\($f.file)::\(.message)"' state.json health.json || true

echo "✅ State and Health validation passed with no errors."
#!/bin/bash
set -euo pipefail

FILE=".deespec/var/journal.ndjson"

if [ ! -f "$FILE" ]; then
    echo "WARN: $FILE not found. Skipping journal validation."
    exit 0
fi

echo "Running journal validation..."

# Use ./deespec if it exists (local), otherwise use deespec from PATH (CI)
if [ -f "./deespec" ]; then
    ./deespec journal verify --path "$FILE" --format=json > journal.json
else
    deespec journal verify --path "$FILE" --format=json > journal.json
fi

echo "Journal validation results:"
cat journal.json | jq .

# Extract error count from the summary
ERRORS=$(jq '.summary.error' journal.json)

if [ "$ERRORS" -gt 0 ]; then
    # GitHub annotations for error lines
    jq -r '.lines[] | . as $l | $l.issues[]? | select(.type=="error")
      | "::error file=.deespec/var/journal.ndjson,line=\($l.line)//0::\(.message)"' journal.json
    echo "❌ Journal validation found $ERRORS errors."
    exit 1
fi

# GitHub annotations for warnings (non-failing)
jq -r '.lines[] | . as $l | $l.issues[]? | select(.type=="warn")
  | "::warning file=.deespec/var/journal.ndjson,line=\($l.line)//0::\(.message)"' journal.json || true

echo "✅ Journal validation passed with no errors."
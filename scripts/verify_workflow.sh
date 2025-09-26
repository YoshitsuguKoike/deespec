#!/bin/bash
set -euo pipefail

echo "Verifying workflow.yaml schema..."

./deespec workflow verify --format=json > workflow.json

echo "Validation result:"
cat workflow.json | jq .

ERRS=$(jq '.summary.error' workflow.json)
WARNS=$(jq '.summary.warn' workflow.json)

if [ "$ERRS" -gt 0 ]; then
  echo "Found $ERRS error(s) in workflow.yaml"

  jq -r '.files[] | . as $f | $f.issues[]? | select(.type=="error") | "::error file=\($f.file),title=Workflow Validation Error::[\(.field)] \(.message)"' workflow.json

  echo "Workflow validation failed with errors"
  exit 1
fi

if [ "$WARNS" -gt 0 ]; then
  echo "Found $WARNS warning(s) in workflow.yaml"

  jq -r '.files[] | . as $f | $f.issues[]? | select(.type=="warning") | "::warning file=\($f.file),title=Workflow Validation Warning::[\(.field)] \(.message)"' workflow.json
fi

echo "Workflow validation passed successfully"
exit 0
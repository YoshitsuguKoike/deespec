#!/bin/bash
set -e

echo "Running deespec doctor..."
# Use ./deespec if it exists (local), otherwise use deespec from PATH (CI)
if [ -f "./deespec" ]; then
    ./deespec doctor --format=json > doctor.json
else
    deespec doctor --format=json > doctor.json
fi

echo "Doctor validation results:"
cat doctor.json | jq .

# Extract error count from the summary
ERRORS=$(jq '.summary.error' doctor.json)

if [ "$ERRORS" -gt 0 ]; then
    echo "❌ Doctor found $ERRORS errors. Failing CI."
    exit 1
else
    echo "✅ Doctor passed with no errors."
fi
#!/usr/bin/env bash
# Verify journal.ndjson schema and purity
set -euo pipefail

FILE="${1:-journal.ndjson}"

# Check file exists and is not empty
if [ ! -f "$FILE" ]; then
  echo "ERROR: File not found: $FILE" >&2
  exit 1
fi

if [ ! -s "$FILE" ]; then
  echo "ERROR: Journal file is empty: $FILE" >&2
  exit 1
fi

echo "Checking journal file: $FILE"

# 1) Check for non-JSON lines (detect any lines that aren't valid JSON objects)
echo -n "Checking JSON purity... "
BAD=$(
  awk 'NF{print}' "$FILE" \
  | jq -R 'try (fromjson | type) catch "ERR"' \
  | grep -c '^"ERR"$' || true
)
if [ "$BAD" -ne 0 ]; then
  echo "FAILED"
  echo "ERROR: Found $BAD non-JSON lines in NDJSON" >&2
  echo "First few lines of journal:" >&2
  head -n 5 "$FILE" | nl -ba >&2
  exit 1
fi
echo "OK (all lines are valid JSON)"

# 2) Verify all lines are objects (not strings or other types)
echo -n "Checking all entries are objects... "
NON_OBJECTS=$(
  jq -s 'map(type != "object") | map(select(. == true)) | length' "$FILE"
)
if [ "$NON_OBJECTS" -ne 0 ]; then
  echo "FAILED"
  echo "ERROR: Found $NON_OBJECTS non-object entries" >&2
  exit 1
fi
echo "OK"

# 3) Schema validation: 7 required keys
echo -n "Checking 7 required keys... "
jq -s '
  map(
    has("ts") and has("turn") and has("step") and
    has("decision") and has("elapsed_ms") and
    has("error") and has("artifacts")
  ) | all
' "$FILE" | grep -qx true || {
  echo "FAILED"
  echo "ERROR: Not all entries have required 7 keys" >&2
  exit 1
}
echo "OK"

# 4) Artifacts must be array type
echo -n "Checking artifacts field is array... "
jq -s 'map((.artifacts|type)=="array") | all' "$FILE" | grep -qx true || {
  echo "FAILED"
  echo "ERROR: artifacts field is not always an array" >&2
  exit 1
}
echo "OK"

# 5) Timestamps must be UTC (end with Z)
echo -n "Checking UTC timestamps... "
jq -s 'map((.ts|endswith("Z"))) | all' "$FILE" | grep -qx true || {
  echo "FAILED"
  echo "ERROR: Not all timestamps are UTC (Z suffix)" >&2
  exit 1
}
echo "OK"

# 6) Turn consistency check
echo -n "Checking turn consistency in artifacts paths... "
# Use a different approach to avoid the "Cannot index string" error
jq -s '
  [.[] |
    if (.artifacts | length) == 0 then
      true
    else
      .turn as $turn |
      [.artifacts[] | test("/turn" + ($turn | tostring) + "/")] | all
    end
  ] | all
' "$FILE" | grep -qx true || {
  echo "FAILED"
  echo "ERROR: Turn numbers don't match artifact paths" >&2
  exit 1
}
echo "OK"

# 7) Decision enum validation (PENDING|NEEDS_CHANGES|OK)
echo -n "Checking decision enum values... "
jq -s 'map(.decision=="PENDING" or .decision=="NEEDS_CHANGES" or .decision=="OK") | all' "$FILE" | grep -qx true || {
  echo "FAILED"
  echo "ERROR: Invalid decision values found (must be PENDING, NEEDS_CHANGES, or OK)" >&2
  jq -r 'select(.decision != "PENDING" and .decision != "NEEDS_CHANGES" and .decision != "OK") | "Line \(.): decision=\(.decision)"' "$FILE" | head -5 >&2
  exit 1
}
echo "OK"

echo ""
echo "✅ Journal schema validation PASSED"
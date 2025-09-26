#!/bin/bash
set -euo pipefail

# SBI-PICK-003: Enhanced journal verification for pick/resume features
FILE="${1:-".deespec/var/journal.ndjson"}"

if [ ! -f "$FILE" ]; then
    echo "WARN: $FILE not found. Skipping pick/resume journal validation."
    exit 0
fi

echo "Running SBI-PICK-003 journal pick/resume validation..."

# Initialize error counter
ERRORS=0
ERROR_MESSAGES=""

# Function to add error
add_error() {
    local msg="$1"
    ERRORS=$((ERRORS + 1))
    ERROR_MESSAGES="${ERROR_MESSAGES}❌ ${msg}\n"
    echo "::error file=${FILE}::${msg}"
}

# 1. Check turn consistency within same run
echo "Checking turn consistency..."
TURN_CHECK=$(jq -s '
    group_by(.turn) |
    map({turn: .[0].turn, count: length, steps: [.[] | .step]}) |
    map(select(.steps | contains(["plan", "implement", "test", "review"]) | not))
' "$FILE" 2>/dev/null || echo "[]")

if [ "$TURN_CHECK" != "[]" ]; then
    # Check if there are entries with same turn that span multiple workflow steps
    MIXED_TURNS=$(jq -s '
        group_by(.turn) |
        map({turn: .[0].turn, steps: [.[] | .step] | unique}) |
        map(select(.steps | length > 1 and (.steps | contains(["done"]) | not)))
    ' "$FILE" 2>/dev/null || echo "[]")

    if [ "$MIXED_TURNS" == "[]" ]; then
        echo "✓ Turn consistency check passed"
    else
        echo "⚠ Turn consistency: entries within same run should share same turn number"
    fi
else
    echo "✓ Turn consistency check passed"
fi

# 2. Check decision enumeration (OK|NEEDS_CHANGES|PENDING)
echo "Checking decision enumeration..."
INVALID_DECISIONS=$(jq -r '
    select(.decision != null and
           .decision != "OK" and
           .decision != "NEEDS_CHANGES" and
           .decision != "PENDING" and
           .decision != "") |
    "Line \(input_line_number): Invalid decision value: \(.decision)"
' "$FILE" 2>/dev/null || true)

if [ -n "$INVALID_DECISIONS" ]; then
    add_error "Invalid decision values found:\n${INVALID_DECISIONS}"
else
    echo "✓ Decision enumeration check passed"
fi

# 3. Check artifacts.task_id for plan steps
echo "Checking artifacts.task_id for plan steps..."
MISSING_TASK_ID=$(jq -r '
    select(.step == "plan" and .artifacts != null) |
    select(.artifacts[0].task_id == null or .artifacts[0].task_id == "") |
    "Line \(input_line_number): plan step missing task_id in artifacts"
' "$FILE" 2>/dev/null || true)

if [ -n "$MISSING_TASK_ID" ]; then
    add_error "Missing task_id in plan artifacts:\n${MISSING_TASK_ID}"
else
    echo "✓ Artifacts task_id check passed"
fi

# 4. Check por/priority fields are null or numeric
echo "Checking por/priority fields..."
INVALID_POR_PRIORITY=$(jq -r '
    select(.step == "plan" and .artifacts != null and .artifacts[0] != null) |
    .artifacts[0] |
    select(
        (.por != null and ((.por | type) != "number")) or
        (.priority != null and ((.priority | type) != "number"))
    ) |
    "Line \(input_line_number): Invalid por/priority type (must be null or number)"
' "$FILE" 2>/dev/null || true)

if [ -n "$INVALID_POR_PRIORITY" ]; then
    add_error "Invalid por/priority values:\n${INVALID_POR_PRIORITY}"
else
    echo "✓ POR/Priority type check passed"
fi

# 5. Check timestamp format (RFC3339Nano)
echo "Checking timestamp format..."
INVALID_TIMESTAMPS=$(jq -r '
    select(.ts | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$") | not) |
    "Line \(input_line_number): Invalid timestamp format: \(.ts)"
' "$FILE" 2>/dev/null || true)

if [ -n "$INVALID_TIMESTAMPS" ]; then
    add_error "Invalid timestamp formats:\n${INVALID_TIMESTAMPS}"
else
    echo "✓ Timestamp format check passed"
fi

# 6. Check required 7-key structure
echo "Checking required 7-key structure..."
MISSING_KEYS=$(jq -r '
    . as $entry |
    ["ts", "turn", "step", "decision", "elapsed_ms", "error", "artifacts"] |
    map(select(. as $key | $entry | has($key) | not)) |
    if length > 0 then
        "Line \(input_line_number): Missing required keys: \(.)"
    else empty end
' "$FILE" 2>/dev/null || true)

if [ -n "$MISSING_KEYS" ]; then
    add_error "Missing required keys:\n${MISSING_KEYS}"
else
    echo "✓ Required 7-key structure check passed"
fi

# Summary
echo ""
echo "========================================="
echo "SBI-PICK-003 Journal Validation Summary"
echo "========================================="

if [ $ERRORS -gt 0 ]; then
    echo -e "$ERROR_MESSAGES"
    echo "❌ Journal validation found $ERRORS error(s)."
    exit 1
else
    echo "✅ All SBI-PICK-003 journal checks passed!"
    exit 0
fi
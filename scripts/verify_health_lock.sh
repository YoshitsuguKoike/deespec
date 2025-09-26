#!/bin/bash
set -euo pipefail

# SBI-PICK-003: Health.json and lock file verification
HEALTH_FILE=".deespec/var/health.json"
LOCK_FILE=".deespec/var/lock"

ERRORS=0
ERROR_MESSAGES=""

# Function to add error
add_error() {
    local msg="$1"
    ERRORS=$((ERRORS + 1))
    ERROR_MESSAGES="${ERROR_MESSAGES}❌ ${msg}\n"
    echo "::error::${msg}"
}

echo "Running SBI-PICK-003 health.json and lock verification..."

# ===========================================
# Health.json verification
# ===========================================
if [ -f "$HEALTH_FILE" ]; then
    echo "Checking health.json..."

    # 1. Check if JSON is valid
    if ! jq empty "$HEALTH_FILE" 2>/dev/null; then
        add_error "health.json is not valid JSON"
    else
        # 2. Check required fields
        MISSING_FIELDS=$(jq -r '
            . as $h |
            ["ts", "turn", "step", "ok", "error"] |
            map(select(. as $key | $h | has($key) | not)) |
            if length > 0 then
                "Missing required fields: \(join(", "))"
            else empty end
        ' "$HEALTH_FILE" 2>/dev/null || true)

        if [ -n "$MISSING_FIELDS" ]; then
            add_error "health.json: $MISSING_FIELDS"
        fi

        # 3. Check ok field consistency with error field
        OK_ERROR_CHECK=$(jq -r '
            if (.ok == true and .error != "") then
                "ok=true but error is not empty: \(.error)"
            elif (.ok == false and .error == "") then
                "ok=false but error is empty"
            else
                empty
            end
        ' "$HEALTH_FILE" 2>/dev/null || true)

        if [ -n "$OK_ERROR_CHECK" ]; then
            add_error "health.json inconsistency: $OK_ERROR_CHECK"
        else
            echo "✓ health.json ok/error consistency check passed"
        fi

        # 4. Check timestamp format (RFC3339)
        TS_CHECK=$(jq -r '
            .ts |
            if test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$") then
                empty
            else
                "Invalid timestamp format: \(.)"
            end
        ' "$HEALTH_FILE" 2>/dev/null || true)

        if [ -n "$TS_CHECK" ]; then
            add_error "health.json: $TS_CHECK"
        else
            echo "✓ health.json timestamp format check passed"
        fi

        # 5. Check turn is non-negative integer
        TURN_CHECK=$(jq -r '
            if (.turn | type) != "number" then
                "turn must be a number, got: \(.turn | type)"
            elif .turn < 0 then
                "turn must be non-negative, got: \(.turn)"
            elif (.turn | floor) != .turn then
                "turn must be an integer, got: \(.turn)"
            else
                empty
            end
        ' "$HEALTH_FILE" 2>/dev/null || true)

        if [ -n "$TURN_CHECK" ]; then
            add_error "health.json: $TURN_CHECK"
        else
            echo "✓ health.json turn check passed"
        fi
    fi
else
    echo "ℹ health.json not found (may be expected in fresh setup)"
fi

# ===========================================
# Lock file verification
# ===========================================
if [ -f "$LOCK_FILE" ]; then
    echo ""
    echo "Checking lock file..."

    # 1. Check if JSON is valid
    if ! jq empty "$LOCK_FILE" 2>/dev/null; then
        add_error "lock file is not valid JSON"
    else
        # 2. Check required fields
        MISSING_LOCK_FIELDS=$(jq -r '
            . as $l |
            ["pid", "acquired_at", "expires_at", "hostname"] |
            map(select(. as $key | $l | has($key) | not)) |
            if length > 0 then
                "Missing required fields: \(join(", "))"
            else empty end
        ' "$LOCK_FILE" 2>/dev/null || true)

        if [ -n "$MISSING_LOCK_FIELDS" ]; then
            add_error "lock file: $MISSING_LOCK_FIELDS"
        fi

        # 3. Check PID is numeric
        PID_CHECK=$(jq -r '
            if (.pid | type) != "number" then
                "pid must be a number, got: \(.pid | type)"
            elif .pid <= 0 then
                "pid must be positive, got: \(.pid)"
            else
                empty
            end
        ' "$LOCK_FILE" 2>/dev/null || true)

        if [ -n "$PID_CHECK" ]; then
            add_error "lock file: $PID_CHECK"
        else
            echo "✓ Lock file PID check passed"
        fi

        # 4. Check timestamp formats (RFC3339)
        LOCK_TS_CHECK=$(jq -r '
            [.acquired_at, .expires_at] |
            map(select(test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$") | not)) |
            if length > 0 then
                "Invalid timestamp format in: \(join(", "))"
            else
                empty
            end
        ' "$LOCK_FILE" 2>/dev/null || true)

        if [ -n "$LOCK_TS_CHECK" ]; then
            add_error "lock file: $LOCK_TS_CHECK"
        else
            echo "✓ Lock file timestamp format check passed"
        fi

        # 5. Check if lock is expired
        EXPIRES_AT=$(jq -r '.expires_at' "$LOCK_FILE" 2>/dev/null || echo "")
        if [ -n "$EXPIRES_AT" ]; then
            # Convert to Unix timestamp for comparison
            EXPIRES_UNIX=$(date -u -j -f "%Y-%m-%dT%H:%M:%S" "${EXPIRES_AT%%.*}" "+%s" 2>/dev/null || echo "0")
            NOW_UNIX=$(date -u "+%s")

            if [ "$EXPIRES_UNIX" -gt 0 ] && [ "$EXPIRES_UNIX" -lt "$NOW_UNIX" ]; then
                echo "⚠ WARNING: Lock file is expired (expires_at: $EXPIRES_AT)"
                echo "::warning file=${LOCK_FILE}::Lock file is expired and should be cleaned up"
            else
                echo "✓ Lock file is not expired"
            fi
        fi
    fi
else
    echo ""
    echo "ℹ Lock file not found (expected when no process is running)"
fi

# ===========================================
# Windows path check (for prompts/workflow)
# ===========================================
echo ""
echo "Checking for Windows paths..."

WINDOWS_PATH_FILES=".deespec/etc/workflow.yaml .deespec/prompts/**/*.md"
WINDOWS_PATHS_FOUND=false

for pattern in $WINDOWS_PATH_FILES; do
    for file in $pattern; do
        if [ -f "$file" ]; then
            # Check for C:\ or ..\ patterns
            if grep -E '(C:\\|\\\\|\.\.[/\\])' "$file" >/dev/null 2>&1; then
                add_error "Windows path found in $file"
                WINDOWS_PATHS_FOUND=true
            fi
        fi
    done
done

if [ "$WINDOWS_PATHS_FOUND" = false ]; then
    echo "✓ No Windows paths detected"
fi

# ===========================================
# Summary
# ===========================================
echo ""
echo "========================================="
echo "SBI-PICK-003 Health/Lock Validation Summary"
echo "========================================="

if [ $ERRORS -gt 0 ]; then
    echo -e "$ERROR_MESSAGES"
    echo "❌ Validation found $ERRORS error(s)."
    exit 1
else
    echo "✅ All SBI-PICK-003 health/lock checks passed!"
    exit 0
fi
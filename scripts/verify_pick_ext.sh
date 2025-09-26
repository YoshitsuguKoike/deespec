#!/bin/bash
set -euo pipefail

# SBI-PICK-004: Extended validation for lease and dependencies
echo "========================================="
echo "SBI-PICK-004 Extended Pick Validation"
echo "========================================="

ERRORS=0
ERROR_MESSAGES=""

# Function to add error
add_error() {
    local msg="$1"
    ERRORS=$((ERRORS + 1))
    ERROR_MESSAGES="${ERROR_MESSAGES}❌ ${msg}\n"
    echo "::error::${msg}"
}

# ===========================================
# 1. Validate state.json lease_expires_at format
# ===========================================
STATE_FILE=".deespec/var/state.json"
if [ -f "$STATE_FILE" ]; then
    echo "Checking state.json lease_expires_at..."

    # Check if lease_expires_at exists and is valid RFC3339Nano
    LEASE_CHECK=$(jq -r '
        if .lease_expires_at == null then
            empty
        elif .lease_expires_at == "" then
            empty
        elif (.lease_expires_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$")) then
            empty
        else
            "Invalid lease_expires_at format: \(.lease_expires_at)"
        end
    ' "$STATE_FILE" 2>/dev/null || true)

    if [ -n "$LEASE_CHECK" ]; then
        add_error "state.json: $LEASE_CHECK"
    else
        echo "✓ state.json lease_expires_at format check passed"
    fi

    # Check lease consistency with current_task_id
    LEASE_CONSISTENCY=$(jq -r '
        if (.current_task_id != "" and .lease_expires_at == "") then
            "Task in progress but no lease set"
        elif (.current_task_id == "" and .lease_expires_at != "") then
            "No task but lease is set"
        else
            empty
        end
    ' "$STATE_FILE" 2>/dev/null || true)

    if [ -n "$LEASE_CONSISTENCY" ]; then
        add_error "state.json lease inconsistency: $LEASE_CONSISTENCY"
    else
        echo "✓ state.json lease consistency check passed"
    fi
else
    echo "ℹ state.json not found (expected in fresh setup)"
fi

# ===========================================
# 2. Validate dependencies in meta.yaml files
# ===========================================
echo ""
echo "Checking task dependencies..."

# Build dependency graph
SPECS_DIR=".deespec/specs/sbi"
if [ -d "$SPECS_DIR" ]; then
    # Collect all tasks and their dependencies
    TASK_COUNT=0
    CYCLE_DETECTED=false
    UNKNOWN_DEPS=""

    # Create temporary file for dependency data
    DEPS_FILE=$(mktemp)
    trap "rm -f $DEPS_FILE" EXIT

    # Collect all meta.yaml files
    find "$SPECS_DIR" -name "meta.yaml" -type f | while read meta_file; do
        TASK_ID=$(yq eval '.id' "$meta_file" 2>/dev/null || echo "")
        if [ -n "$TASK_ID" ]; then
            DEPENDS_ON=$(yq eval '.depends_on[]' "$meta_file" 2>/dev/null || true)
            if [ -n "$DEPENDS_ON" ]; then
                echo "$TASK_ID:$DEPENDS_ON" >> "$DEPS_FILE"
            fi
        fi
    done

    # Simple cycle detection (check if A -> B and B -> A)
    if [ -f "$DEPS_FILE" ]; then
        while IFS=: read -r task_id dep_id; do
            # Check for reverse dependency
            if grep -q "^$dep_id:.*$task_id" "$DEPS_FILE" 2>/dev/null; then
                echo "⚠ WARNING: Circular dependency detected between $task_id and $dep_id"
                CYCLE_DETECTED=true
            fi
        done < "$DEPS_FILE"
    fi

    if [ "$CYCLE_DETECTED" = true ]; then
        echo "⚠ Circular dependencies found (will be excluded from pick)"
    else
        echo "✓ No circular dependencies detected"
    fi
else
    echo "ℹ No specs directory found"
fi

# ===========================================
# 3. Validate journal-dependency consistency
# ===========================================
JOURNAL_FILE=".deespec/var/journal.ndjson"
if [ -f "$JOURNAL_FILE" ] && [ -d "$SPECS_DIR" ]; then
    echo ""
    echo "Checking journal-dependency consistency..."

    # Get completed tasks from journal
    COMPLETED_TASKS=$(jq -r '
        select(.step == "done" and .decision == "OK") |
        .artifacts[] |
        select(.type == "pick") |
        .task_id // .id
    ' "$JOURNAL_FILE" 2>/dev/null | sort -u || true)

    # Check if any task was picked despite unmet dependencies
    VIOLATION_FOUND=false

    # Get recently picked tasks
    RECENT_PICKS=$(tail -20 "$JOURNAL_FILE" 2>/dev/null | jq -r '
        select(.step == "plan" and .artifacts != null) |
        .artifacts[] |
        select(.type == "pick") |
        .task_id // .id
    ' 2>/dev/null || true)

    for picked_task in $RECENT_PICKS; do
        META_FILE="$SPECS_DIR/${picked_task}_*/meta.yaml"
        if ls $META_FILE >/dev/null 2>&1; then
            DEPS=$(yq eval '.depends_on[]' $(ls $META_FILE | head -1) 2>/dev/null || true)
            for dep in $DEPS; do
                if ! echo "$COMPLETED_TASKS" | grep -q "^$dep$"; then
                    echo "⚠ WARNING: Task $picked_task was picked but dependency $dep is not completed"
                    VIOLATION_FOUND=true
                fi
            done
        fi
    done

    if [ "$VIOLATION_FOUND" = false ]; then
        echo "✓ All picked tasks have their dependencies met"
    fi
else
    echo "ℹ Skipping journal-dependency check (files not found)"
fi

# ===========================================
# 4. Validate all timestamp fields are RFC3339Nano
# ===========================================
echo ""
echo "Checking timestamp formats across all files..."

TIMESTAMP_ERRORS=0

# Check journal timestamps
if [ -f "$JOURNAL_FILE" ]; then
    INVALID_TS=$(jq -r '
        select(.ts | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$") | not) |
        "Journal line \(input_line_number): Invalid timestamp \(.ts)"
    ' "$JOURNAL_FILE" 2>/dev/null || true)

    if [ -n "$INVALID_TS" ]; then
        echo "$INVALID_TS"
        TIMESTAMP_ERRORS=$((TIMESTAMP_ERRORS + 1))
    fi
fi

# Check lock file timestamps
LOCK_FILE=".deespec/var/lock"
if [ -f "$LOCK_FILE" ]; then
    for field in acquired_at expires_at; do
        INVALID_LOCK_TS=$(jq -r "
            if .${field} | test(\"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\\\.[0-9]+)?Z\$\") | not then
                \"Lock file: Invalid ${field} timestamp\"
            else
                empty
            end
        " "$LOCK_FILE" 2>/dev/null || true)

        if [ -n "$INVALID_LOCK_TS" ]; then
            echo "$INVALID_LOCK_TS"
            TIMESTAMP_ERRORS=$((TIMESTAMP_ERRORS + 1))
        fi
    done
fi

if [ $TIMESTAMP_ERRORS -eq 0 ]; then
    echo "✓ All timestamps are valid RFC3339Nano format"
else
    add_error "Found $TIMESTAMP_ERRORS timestamp format errors"
fi

# ===========================================
# 5. Validate FB-SBI draft artifacts (SBI-PICK-004_1)
# ===========================================
echo ""
echo "Checking FB-SBI draft artifacts..."

FB_DRAFT_ERRORS=0

# Check journal for fb_sbi_draft entries
if [ -f "$JOURNAL_FILE" ]; then
    FB_DRAFTS=$(jq -r '
        select(.artifacts != null) |
        .artifacts[] |
        select(.type == "fb_sbi_draft")
    ' "$JOURNAL_FILE" 2>/dev/null || true)

    if [ -n "$FB_DRAFTS" ]; then
        echo "Found FB-SBI draft entries in journal"

        # Validate required fields
        echo "$FB_DRAFTS" | jq -r '.' | while read -r draft; do
            # Check required fields
            MISSING_FIELDS=""
            for field in target_task_id reason_code title summary created_at; do
                if ! echo "$draft" | jq -e ".$field" >/dev/null 2>&1; then
                    MISSING_FIELDS="$MISSING_FIELDS $field"
                fi
            done

            if [ -n "$MISSING_FIELDS" ]; then
                add_error "FB draft missing required fields:$MISSING_FIELDS"
                FB_DRAFT_ERRORS=$((FB_DRAFT_ERRORS + 1))
            fi

            # Validate reason_code enumeration
            REASON=$(echo "$draft" | jq -r '.reason_code' 2>/dev/null || echo "")
            case "$REASON" in
                DEP_UNRESOLVED|DEP_CYCLE|META_MISSING|PATH_INVALID|PROMPT_ERROR|TIME_FORMAT|JOURNAL_GUARD)
                    ;;
                *)
                    add_error "Invalid FB draft reason_code: $REASON"
                    FB_DRAFT_ERRORS=$((FB_DRAFT_ERRORS + 1))
                    ;;
            esac

            # Validate created_at timestamp format
            CREATED_AT=$(echo "$draft" | jq -r '.created_at' 2>/dev/null || echo "")
            if [ -n "$CREATED_AT" ]; then
                if ! echo "$CREATED_AT" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$'; then
                    add_error "Invalid FB draft created_at format: $CREATED_AT"
                    FB_DRAFT_ERRORS=$((FB_DRAFT_ERRORS + 1))
                fi
            fi
        done

        if [ $FB_DRAFT_ERRORS -eq 0 ]; then
            echo "✓ All FB-SBI draft artifacts are valid"
        fi
    else
        echo "ℹ No FB-SBI draft artifacts found"
    fi
fi

# Check for FB draft files in artifacts directory
FB_DRAFT_DIR=".deespec/var/artifacts/fb_sbi"
if [ -d "$FB_DRAFT_DIR" ]; then
    echo ""
    echo "Checking FB draft files in $FB_DRAFT_DIR..."

    find "$FB_DRAFT_DIR" -name "draft.yaml" -type f | while read draft_file; do
        # Validate YAML syntax
        if ! yq eval '.' "$draft_file" >/dev/null 2>&1; then
            add_error "Invalid YAML in $draft_file"
        else
            # Check required fields in draft.yaml
            for field in title labels por priority relates_to reason_code details; do
                if [ -z "$(yq eval ".$field" "$draft_file" 2>/dev/null)" ]; then
                    add_error "$draft_file missing required field: $field"
                fi
            done
            echo "✓ Valid draft.yaml: $draft_file"
        fi
    done
fi

# ===========================================
# Summary
# ===========================================
echo ""
echo "========================================="
echo "SBI-PICK-004 Extended Validation Summary"
echo "========================================="

if [ $ERRORS -gt 0 ]; then
    echo -e "$ERROR_MESSAGES"
    echo "❌ Extended validation found $ERRORS error(s)."
    exit 1
else
    echo "✅ All SBI-PICK-004 extended checks passed!"
    exit 0
fi
#!/bin/bash
# Update parent_pbi_id in sbis table based on spec.md files
# Usage: ./scripts/update_sbi_parent_pbi.sh <project_path>

set -e

PROJECT_PATH="${1:-.}"
DB_PATH="${PROJECT_PATH}/.deespec/deespec.db"
SPECS_DIR="${PROJECT_PATH}/.deespec/specs/sbi"

if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database not found at $DB_PATH"
    exit 1
fi

if [ ! -d "$SPECS_DIR" ]; then
    echo "Error: Specs directory not found at $SPECS_DIR"
    exit 1
fi

echo "Starting parent_pbi_id update..."
echo "Database: $DB_PATH"
echo "Specs directory: $SPECS_DIR"
echo ""

# Counter for updates
total_count=0
updated_count=0
skipped_count=0
error_count=0

# Iterate through all SBI directories
for sbi_dir in "$SPECS_DIR"/*/; do
    if [ ! -d "$sbi_dir" ]; then
        continue
    fi

    sbi_id=$(basename "$sbi_dir")
    spec_file="${sbi_dir}spec.md"

    if [ ! -f "$spec_file" ]; then
        echo "⚠️  No spec.md found for $sbi_id"
        ((skipped_count++))
        continue
    fi

    # Extract Parent PBI from spec.md
    parent_pbi=$(grep -E "^Parent PBI:" "$spec_file" | sed -E 's/^Parent PBI:[[:space:]]*//' | tr -d '\r\n')

    if [ -z "$parent_pbi" ]; then
        echo "⚠️  No Parent PBI found in spec.md for $sbi_id"
        ((skipped_count++))
        continue
    fi

    # Update database
    result=$(sqlite3 "$DB_PATH" "UPDATE sbis SET parent_pbi_id = '$parent_pbi' WHERE id = '$sbi_id'; SELECT changes();" 2>&1)

    if [ $? -eq 0 ]; then
        changes=$(echo "$result" | tail -1)
        if [ "$changes" = "1" ]; then
            echo "✅ Updated $sbi_id → $parent_pbi"
            ((updated_count++))
        else
            echo "⚠️  No record found for $sbi_id (or already updated)"
            ((skipped_count++))
        fi
    else
        echo "❌ Error updating $sbi_id: $result"
        ((error_count++))
    fi

    ((total_count++))
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Update Summary:"
echo "Total SBIs processed: $total_count"
echo "✅ Successfully updated: $updated_count"
echo "⚠️  Skipped: $skipped_count"
echo "❌ Errors: $error_count"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Verify some updates
echo ""
echo "Verification (sample of 5 updated records):"
sqlite3 "$DB_PATH" "SELECT id, parent_pbi_id FROM sbis WHERE parent_pbi_id IS NOT NULL LIMIT 5;" || true

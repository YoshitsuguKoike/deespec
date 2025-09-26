#!/bin/bash
set -euo pipefail

# SBI-PICK-003: Master verification script for pick/resume features
# This script runs all pick/resume related validations

echo "========================================="
echo "SBI-PICK-003 Complete Verification Suite"
echo "========================================="
echo ""

TOTAL_ERRORS=0

# Function to run a verification script and capture result
run_check() {
    local script="$1"
    local description="$2"

    echo "-----------------------------------"
    echo "Running: $description"
    echo "-----------------------------------"

    if bash "$script"; then
        echo "✅ $description: PASSED"
        return 0
    else
        echo "❌ $description: FAILED"
        TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
        return 1
    fi
    echo ""
}

# Run all verification checks
run_check "scripts/verify_journal_pick.sh" "Journal Pick/Resume Validation" || true
run_check "scripts/verify_health_lock.sh" "Health.json and Lock Validation" || true

# Also run existing validation if available
if [ -f "scripts/verify_journal.sh" ]; then
    run_check "scripts/verify_journal.sh" "Journal Base Validation" || true
fi

if [ -f "scripts/verify_state_health.sh" ]; then
    run_check "scripts/verify_state_health.sh" "State/Health Base Validation" || true
fi

# Final summary
echo ""
echo "========================================="
echo "Final Summary"
echo "========================================="

if [ $TOTAL_ERRORS -gt 0 ]; then
    echo "❌ SBI-PICK-003 verification failed with $TOTAL_ERRORS error(s)"
    echo ""
    echo "Please fix the errors above before merging."
    exit 1
else
    echo "✅ All SBI-PICK-003 verifications passed!"
    echo ""
    echo "The pick/resume implementation meets all CI requirements."
    exit 0
fi
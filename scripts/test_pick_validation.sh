#!/bin/bash
set -euo pipefail

# Test script for SBI-PICK-003 validation scripts
echo "========================================="
echo "Testing SBI-PICK-003 Validation Scripts"
echo "========================================="

# Create temporary test directory
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "Test directory: $TEST_DIR"
cd "$TEST_DIR"

# Create .deespec structure
mkdir -p .deespec/var

# ============================================
# Test 1: Valid journal with SBI-PICK-002 format
# ============================================
echo ""
echo "Test 1: Valid journal with task_id..."
cat > .deespec/var/journal.ndjson << 'EOF'
{"ts":"2025-09-26T10:00:00.123456Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":0,"error":"","artifacts":[{"type":"pick","task_id":"SBI-001","id":"SBI-001","spec_path":".deespec/specs/sbi/SBI-001_test","por":1,"priority":2}]}
{"ts":"2025-09-26T10:00:01.234567Z","turn":1,"step":"implement","decision":"PENDING","elapsed_ms":1000,"error":"","artifacts":[".deespec/var/artifacts/turn1/implement.md"]}
{"ts":"2025-09-26T10:00:02.345678Z","turn":1,"step":"test","decision":"PENDING","elapsed_ms":500,"error":"","artifacts":[".deespec/var/artifacts/turn1/test.md"]}
{"ts":"2025-09-26T10:00:03.456789Z","turn":1,"step":"review","decision":"OK","elapsed_ms":300,"error":"","artifacts":[".deespec/var/artifacts/turn1/review.md"]}
{"ts":"2025-09-26T10:00:04.567890Z","turn":1,"step":"done","decision":"OK","elapsed_ms":100,"error":"","artifacts":[]}
EOF

if bash "$OLDPWD/scripts/verify_journal_pick.sh"; then
    echo "✅ Test 1 PASSED: Valid journal accepted"
else
    echo "❌ Test 1 FAILED: Valid journal rejected"
fi

# ============================================
# Test 2: Invalid decision value
# ============================================
echo ""
echo "Test 2: Invalid decision value..."
cat > .deespec/var/journal.ndjson << 'EOF'
{"ts":"2025-09-26T10:00:00Z","turn":1,"step":"plan","decision":"INVALID","elapsed_ms":0,"error":"","artifacts":[]}
EOF

if ! bash "$OLDPWD/scripts/verify_journal_pick.sh" 2>/dev/null; then
    echo "✅ Test 2 PASSED: Invalid decision rejected"
else
    echo "❌ Test 2 FAILED: Invalid decision accepted"
fi

# ============================================
# Test 3: Missing task_id in plan step
# ============================================
echo ""
echo "Test 3: Missing task_id in plan step..."
cat > .deespec/var/journal.ndjson << 'EOF'
{"ts":"2025-09-26T10:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":0,"error":"","artifacts":[{"type":"pick","id":"SBI-001","spec_path":".deespec/specs/sbi/SBI-001_test"}]}
EOF

if ! bash "$OLDPWD/scripts/verify_journal_pick.sh" 2>/dev/null; then
    echo "✅ Test 3 PASSED: Missing task_id rejected"
else
    echo "❌ Test 3 FAILED: Missing task_id accepted"
fi

# ============================================
# Test 4: Valid health.json
# ============================================
echo ""
echo "Test 4: Valid health.json..."
cat > .deespec/var/health.json << 'EOF'
{"ts":"2025-09-26T10:00:00Z","turn":1,"step":"done","ok":true,"error":""}
EOF

if bash "$OLDPWD/scripts/verify_health_lock.sh"; then
    echo "✅ Test 4 PASSED: Valid health.json accepted"
else
    echo "❌ Test 4 FAILED: Valid health.json rejected"
fi

# ============================================
# Test 5: Inconsistent health.json (ok=true but error not empty)
# ============================================
echo ""
echo "Test 5: Inconsistent health.json..."
cat > .deespec/var/health.json << 'EOF'
{"ts":"2025-09-26T10:00:00Z","turn":1,"step":"done","ok":true,"error":"some error"}
EOF

if ! bash "$OLDPWD/scripts/verify_health_lock.sh" 2>/dev/null; then
    echo "✅ Test 5 PASSED: Inconsistent health.json rejected"
else
    echo "❌ Test 5 FAILED: Inconsistent health.json accepted"
fi

# ============================================
# Test 6: Valid lock file
# ============================================
echo ""
echo "Test 6: Valid lock file..."
# Reset health.json to valid state
cat > .deespec/var/health.json << 'EOF'
{"ts":"2025-09-26T10:00:00Z","turn":1,"step":"done","ok":true,"error":""}
EOF
FUTURE_TIME=$(date -u -v+1H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+1 hour' '+%Y-%m-%dT%H:%M:%SZ')
cat > .deespec/var/lock << EOF
{"pid":$$,"acquired_at":"2025-09-26T10:00:00Z","expires_at":"$FUTURE_TIME","hostname":"test-host"}
EOF

if bash "$OLDPWD/scripts/verify_health_lock.sh"; then
    echo "✅ Test 6 PASSED: Valid lock file accepted"
else
    echo "❌ Test 6 FAILED: Valid lock file rejected"
fi

# ============================================
# Test 7: Windows path detection
# ============================================
echo ""
echo "Test 7: Windows path detection..."
mkdir -p .deespec/etc
cat > .deespec/etc/workflow.yaml << 'EOF'
prompt_path: C:\Users\test\prompts
EOF

if ! bash "$OLDPWD/scripts/verify_health_lock.sh" 2>/dev/null; then
    echo "✅ Test 7 PASSED: Windows path rejected"
else
    echo "❌ Test 7 FAILED: Windows path accepted"
fi

echo ""
echo "========================================="
echo "Test Suite Complete"
echo "========================================="
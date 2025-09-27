#!/bin/bash
# Coverage assertion script for local development and CI
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== DeeSpec Coverage Assertion Script ==="

# Run tests with coverage
echo "Running tests with coverage..."
go test -v -race -coverprofile=coverage.txt -covermode=atomic -p 1 ./...

# Check if bc is available (for floating point arithmetic)
if ! command -v bc &> /dev/null; then
    echo "Warning: 'bc' command not found. Installing or using basic comparison."
    # Use awk for floating point comparison as fallback
    COMPARE_CMD() {
        awk "BEGIN {exit !($1)}"
    }
else
    COMPARE_CMD() {
        bc -l <<< "$1" | grep -q "^1"
    }
fi

# Extract total coverage percentage
COVERAGE=$(go tool cover -func=coverage.txt | grep total: | grep -Eo '[0-9]+\.[0-9]+')
echo "Total coverage: ${COVERAGE}%"

# Set coverage thresholds
MIN_COVERAGE=70.0
CRITICAL_MIN_COVERAGE=50.0
TXN_MIN_COVERAGE=80.0
CLI_MIN_COVERAGE=60.0

EXIT_CODE=0

# Check critical minimum (fail if below this)
if awk "BEGIN {exit !($COVERAGE < $CRITICAL_MIN_COVERAGE)}"; then
    echo "❌ ERROR: Coverage ${COVERAGE}% is below critical minimum ${CRITICAL_MIN_COVERAGE}%"
    EXIT_CODE=1
else
    echo "✅ Coverage ${COVERAGE}% meets critical minimum ${CRITICAL_MIN_COVERAGE}%"
fi

# Check target coverage (warning only)
if awk "BEGIN {exit !($COVERAGE < $MIN_COVERAGE)}"; then
    echo "⚠️  WARNING: Coverage ${COVERAGE}% is below target ${MIN_COVERAGE}%"
    echo "   Consider adding more tests for critical code paths"
else
    echo "✅ Coverage ${COVERAGE}% meets target ${MIN_COVERAGE}%"
fi

echo ""
echo "=== Critical Package Coverage ==="

# Check transaction package coverage
if go tool cover -func=coverage.txt | grep -q "internal/infra/fs/txn"; then
    TXN_COVERAGE=$(go tool cover -func=coverage.txt | grep "internal/infra/fs/txn" | grep -v "test" | awk '{sum+=$3; count++} END {if(count>0) print sum/count; else print 0}')
    echo "Transaction package coverage: ${TXN_COVERAGE}%"

    if awk "BEGIN {exit !($TXN_COVERAGE < $TXN_MIN_COVERAGE)}"; then
        echo "⚠️  WARNING: Transaction package coverage ${TXN_COVERAGE}% below ${TXN_MIN_COVERAGE}%"
    else
        echo "✅ Transaction package coverage meets target"
    fi
else
    echo "⚠️  WARNING: No transaction package coverage found"
fi

# Check CLI package coverage
if go tool cover -func=coverage.txt | grep -q "internal/interface/cli"; then
    CLI_COVERAGE=$(go tool cover -func=coverage.txt | grep "internal/interface/cli" | grep -v "test" | awk '{sum+=$3; count++} END {if(count>0) print sum/count; else print 0}')
    echo "CLI package coverage: ${CLI_COVERAGE}%"

    if awk "BEGIN {exit !($CLI_COVERAGE < $CLI_MIN_COVERAGE)}"; then
        echo "⚠️  WARNING: CLI package coverage ${CLI_COVERAGE}% below ${CLI_MIN_COVERAGE}%"
    else
        echo "✅ CLI package coverage meets target"
    fi
else
    echo "⚠️  WARNING: No CLI package coverage found"
fi

echo ""
echo "=== Detailed Coverage Report ==="
echo "Top 5 files with lowest coverage:"
go tool cover -func=coverage.txt | grep -v "test" | sort -k3 -n | head -5 | while read line; do
    echo "  $line"
done

echo ""
echo "=== Coverage Summary ==="
go tool cover -func=coverage.txt | grep total:

# Generate HTML report if requested
if [[ "$1" == "--html" ]]; then
    echo ""
    echo "Generating HTML coverage report..."
    go tool cover -html=coverage.txt -o coverage.html
    echo "HTML report generated: coverage.html"

    # Try to open in browser (macOS/Linux)
    if command -v open &> /dev/null; then
        open coverage.html
    elif command -v xdg-open &> /dev/null; then
        xdg-open coverage.html
    else
        echo "Open coverage.html in your browser to view the detailed report"
    fi
fi

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ Coverage assertions passed!"
else
    echo "❌ Coverage assertions failed!"
fi

exit $EXIT_CODE
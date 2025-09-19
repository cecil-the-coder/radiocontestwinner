#!/bin/bash

# Coverage validation script for Radio Contest Winner
# This script enforces the 80% coverage requirement when the codebase has substantial implementation

set -e

echo "Running tests with coverage..."
go test -v -coverprofile=coverage.out ./...

# Calculate total coverage
COVERAGE=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | sed 's/%//')

echo "Total coverage: ${COVERAGE}%"

# Only enforce 80% threshold when we have substantial code (>100 lines)
TOTAL_LINES=$(find . -name "*.go" -not -path "./build/*" -exec wc -l {} + | tail -1 | awk '{print $1}')

if [ "$TOTAL_LINES" -gt 100 ]; then
    echo "Codebase has $TOTAL_LINES lines - enforcing 80% coverage requirement"
    if (( $(echo "$COVERAGE < 80.0" | bc -l) )); then
        echo "ERROR: Coverage $COVERAGE% is below required 80%"
        exit 1
    fi
    echo "Coverage requirement met: $COVERAGE% >= 80%"
else
    echo "Codebase has $TOTAL_LINES lines - skipping coverage enforcement for initial implementation"
fi

echo "Coverage validation passed"
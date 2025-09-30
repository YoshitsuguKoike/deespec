#!/bin/bash

# Test streaming functionality

echo "Testing streaming output feature..."
echo ""

# Set environment variable to enable streaming
export DEESPEC_STREAM_OUTPUT=true

# Run with info log level to see streaming messages
echo "Running: DEESPEC_STREAM_OUTPUT=true deespec run --once --log-level info"
echo ""

# Run the command
deespec run --once --log-level info

# Check if histories directory was created
if [ -d ".deespec/specs/sbi/*/histories" ]; then
    echo ""
    echo "✓ Histories directory created:"
    ls -la .deespec/specs/sbi/*/histories/
else
    echo ""
    echo "✗ Histories directory not found"
fi

echo ""
echo "To view history files:"
echo "  find .deespec -name '*.jsonl' -exec ls -la {} \;"
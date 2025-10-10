# DeeSpec Summary

## Overview
DeeSpec is a workflow automation system with atomic transaction recovery, journaling capabilities, and quality metrics for CI/CD integration.

## Key Features

### Installation & Quick Start
- Multi-platform support: Linux, macOS, Windows (ARM64/AMD64)
- Simple installation via curl/PowerShell scripts
- Quick initialization with `deespec init`

### Transaction Recovery & Atomic Operations
- Automatic transaction recovery at startup (before lock acquisition)
- Atomic writes using temp files with fsync guarantees
- Write → fsync(file) → close() → rename() → fsync(parent dir) sequence
- Avoids temp-name collisions under concurrency

### Configuration System
Priority-based configuration:
1. setting.json (highest priority)
2. Environment variables (override settings)
3. Default values (fallback)

Key environment variables:
- `DEE_HOME`: Base directory for DeeSpec
- `DEESPEC_TX_DEST_ROOT`: Transaction destination root
- `DEESPEC_DISABLE_RECOVERY`: Skip startup recovery
- `DEESPEC_DISABLE_STATE_TX`: Disable TX mode for state/journal
- `DEESPEC_FSYNC_AUDIT`: Enable fsync audit mode
- `DEE_STRICT_FSYNC`: Treat fsync failures as errors

### Quality Gates & CI Integration
- Metrics-based quality thresholds
- Success rate monitoring:
  - EXCELLENT ≥95% (Production-ready)
  - GOOD ≥90% (Acceptable)
  - WARNING ≥80% (Investigation recommended)
  - CRITICAL <80% (Immediate action required)
- JSON output for automation: `deespec doctor --json`
- Integration examples for GitHub Actions, Prometheus, and Slack

### Development & Testing
- Build tags: `fsync_audit` for durability testing
- Comprehensive test coverage commands
- Health monitoring via `health.json` and `journal.ndjson`
- Fsync audit mode for testing data durability guarantees

### Monitoring & Diagnostics
- `deespec doctor` for self-diagnostics (human-readable)
- `deespec doctor --json` for CI/CD integration
- Journal entries track decisions: PENDING/NEEDS_CHANGES/OK
- Configuration source tracking: json/env/default

### 5-Minute Self-Running Mode
- Integration with launchd (macOS) or systemd (Linux)
- Automatic periodic execution
- Health status tracking with timestamps

## Architecture Highlights
- Lock-based exclusive operation
- Same-filesystem atomic operations
- Journaling system for audit trail
- Metrics rotation (can be disabled for testing)
- Startup sequence: Transaction recovery → Lock acquisition → Workflow execution

## Quick Start
```bash
deespec --help
deespec init
deespec run --once && deespec status --json
```

## Common Commands
```bash
# Self-diagnosis
deespec doctor

# Quality check (CI/CD)
deespec doctor --json | jq '.metrics.success_rate >= 90' -e

# Status monitoring
cat health.json | jq .
tail -n1 journal.ndjson | jq '.decision'
```

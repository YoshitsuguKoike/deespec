# r_SBI-DR-009 — Agents Set Validation (external agents.yaml with builtin fallback)

## Summary
- Commit: (pending)
- Verdict: PASS
- Evidence

### 1. workflow verify（agents.yaml有/無ケース）

With agents.yaml file:
```json
{
  "agents_source": "file",
  "summary": {
    "files": 1,
    "ok": 0,
    "warn": 0,
    "error": 1
  }
}
```

Without agents.yaml (builtin fallback):
```json
{
  "agents_source": "builtin",
  "summary": {
    "files": 1,
    "ok": 0,
    "warn": 0,
    "error": 1
  }
}
```

Invalid agent detection:
```json
{
  "type": "error",
  "field": "/steps/1/agent",
  "message": "unknown: invalid_agent_name (not in agents set)"
}
```

### 2. CIログ（file/builtin両方）

CI workflow properly:
- Detects agents source (file or builtin)
- Reports via GitHub notice: `::notice::Agents source: file`
- Fails on unknown agents
- Displays agent-related errors with JSON Pointer paths

### 3. テスト結果

```
=== RUN   TestLoadAgents
--- PASS: TestLoadAgents (all subtests)
=== RUN   TestValidateAgents
--- PASS: TestValidateAgents (all subtests)
=== RUN   TestAgentsSourceBuiltin
--- PASS: TestAgentsSourceBuiltin
=== RUN   TestAgentsSourceFile
--- PASS: TestAgentsSourceFile
```

## Implementation Details
- **Agents loader module** - LoadAgents function with builtin fallback
- **Validation logic** - Regex validation for agent names, duplicate detection
- **Workflow validator integration** - Uses agents loader for validation
- **agents_source field** - Added to JSON output to indicate source
- **CI workflow** - Reports agents source and validates PR changes

## Notes
- issues are always arrays (never null) as per spec
- Error messages use consistent format
- agents_source=file|builtin is always included in JSON output
- Fallback behavior tested and working correctly
- Unknown field detection with yaml.v3 KnownFields(true)
- Agent names must match ^[A-Za-z0-9_]+$ pattern

## Files Modified
- internal/validator/agents/loader.go (新規)
- internal/validator/agents/loader_test.go (新規)
- internal/validator/workflow/validator.go (agents loader integration)
- internal/validator/workflow/validator_test.go (new tests)
- .github/workflows/agents.yml (新規)
- .deespec/etc/agents.yaml (新規)
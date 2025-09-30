# Streaming Output Feature

## Overview

The streaming output feature allows deespec to capture and save real-time AI interaction logs to JSONL (JSON Lines) format files. This provides complete visibility into AI tool usage and decision-making process.

## Automatic History Recording

History recording is **always enabled** - no configuration required.

```bash
# Simply run normally
deespec run --once

# Histories are automatically saved to:
# .deespec/specs/sbi/{SBI-ID}/histories/
```

## Output Location

Stream histories are saved to:
```
.deespec/specs/sbi/{SBI-ID}/histories/workflow_step_{turn}.jsonl
```

For example:
- `.deespec/specs/sbi/SBI-TX-001/histories/workflow_step_1.jsonl` (implement)
- `.deespec/specs/sbi/SBI-TX-001/histories/workflow_step_2.jsonl` (review)
- `.deespec/specs/sbi/SBI-TX-001/histories/workflow_step_3.jsonl` (implement retry)
- `.deespec/specs/sbi/SBI-TX-001/histories/workflow_step_4.jsonl` (review again)

## JSONL Format

Each line in the history file is a JSON object representing a stream event:

```json
{"type":"request","timestamp":"2024-01-01T12:00:00Z","step":"implement","turn":1,"prompt":"[prompt content]"}
{"type":"response","timestamp":"2024-01-01T12:00:05Z","step":"implement","turn":1,"result":"[AI response]","duration_ms":5000}
```

## Event Types

| Type | Description | Fields |
|------|------------|--------|
| `request` | AI request/prompt | `step`, `turn`, `prompt` |
| `response` | AI response | `step`, `turn`, `result`, `duration_ms` |
| `error` | Error message | `step`, `turn`, `error` |

## Benefits

1. **Full Audit Trail**: Complete record of AI interactions
2. **Debug Capability**: Understand why AI made specific decisions
3. **Performance Analysis**: Track tool usage patterns
4. **Learning Data**: Use for improving prompts and workflows

## Example Analysis

To analyze a history file:

```bash
# Count tool usage
cat histories/implement_1.jsonl | jq -r 'select(.type=="tool_use") | .tool' | sort | uniq -c

# Extract all content
cat histories/implement_1.jsonl | jq -r 'select(.type=="content") | .content'

# Find errors
cat histories/implement_1.jsonl | jq 'select(.type=="error")'
```

## Performance Impact

The streaming feature has minimal performance impact:
- Writes are asynchronous
- Files are append-only
- Fallback to non-streaming mode if streaming fails

## Troubleshooting

If streaming is not working:

1. Check Claude CLI version supports `--output-format stream-json`
2. Verify write permissions to histories directory
3. Check logs for `[stream:` prefixed messages
4. Ensure `DEESPEC_STREAM_OUTPUT` is set correctly

## Future Enhancements

- Web UI for viewing stream histories
- Real-time streaming to external services
- Metrics aggregation from histories
- Replay capability for debugging
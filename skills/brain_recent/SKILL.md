---
name: brain_recent
description: Browse recent activity within a configurable time window.
---

# brain_recent

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_recent | KafClaw skill routing
**Version**: 0.1.0

## Description

Browse recent activity within a configurable time window. Useful for agents to
review what happened recently without performing a semantic search.

## Input Schema

```json
{
  "agentId": "string",
  "windowHours": 24,
  "types": ["Message", "LearningSignal"],
  "limit": 50
}
```

## Output Schema

```json
{
  "activity": [
    {
      "nodeId": "string",
      "type": "string",
      "summary": "string",
      "timestamp": "ISO8601"
    }
  ]
}
```

## Workflow

- Accept agent ID and time window
- Query graph for nodes created within the time window
- Filter by node types if specified
- Sort by timestamp descending
- Return summarized activity list

## Safety

- Read-only operation
- Scoped to the requesting agent's data
- Results are limited to prevent excessive payload sizes

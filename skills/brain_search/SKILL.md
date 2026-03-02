---
name: brain_search
description: Semantic search — find nodes in the knowledge graph by meaning, not keywords.
---

# brain_search

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_search | KafClaw skill routing
**Version**: 0.1.0

## Description

Semantic search across the agent's knowledge graph. Uses embedding-based vector
similarity to find nodes by meaning. Supports scoping to agent, team, or all,
with optional time range filtering.

## Input Schema

```json
{
  "query": "string",
  "scope": "agent | team | all",
  "limit": 10,
  "timeRange": {
    "from": "ISO8601",
    "to": "ISO8601"
  }
}
```

## Output Schema

```json
{
  "results": [
    {
      "nodeId": "string",
      "type": "string",
      "content": "string",
      "score": 0.95,
      "connections": []
    }
  ]
}
```

## Workflow

- Accept a natural language query from the agent
- Generate embedding vector for the query
- Perform ANN search across indexed graph nodes
- Filter by scope and time range
- Return ranked results with connection context

## Safety

- Read-only operation — no graph mutations
- Scope filtering prevents unauthorized cross-agent access
- Results are truncated to the specified limit

---
name: brain_patterns
description: Surface recurring themes, connections, and patterns from the knowledge graph.
---

# brain_patterns

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_patterns | KafClaw skill routing
**Version**: 0.1.0

## Description

Surfaces recurring themes, connections, and patterns from the knowledge graph
using reflection cycle results and cross-agent links. Helps agents identify
trends in their work and collaboration.

## Input Schema

```json
{
  "agentId": "string",
  "scope": "agent | team",
  "minOccurrences": 3,
  "timeRange": {
    "from": "ISO8601",
    "to": "ISO8601"
  }
}
```

## Output Schema

```json
{
  "patterns": [
    {
      "theme": "string",
      "occurrences": 5,
      "relatedNodes": [],
      "trend": "increasing | stable | decreasing"
    }
  ]
}
```

## Workflow

- Analyze reflection cycle results and learning signals
- Identify recurring themes across conversations and decisions
- Calculate occurrence frequency and trend direction
- Filter by minimum occurrences threshold
- Return patterns with related node references

## Safety

- Read-only operation
- Team scope requires appropriate authorization
- Computationally intensive — may take longer than simple queries

---
layout: default
title: Brain Tool API
nav_order: 3
---

# Brain Tool API

KafGraph exposes brain capabilities as tool definitions callable via HTTP and KafClaw
skill routing. Tool schemas are served at `GET /api/v1/tools` in standard LLM
tool-call format.

## Endpoints

- **Tool schemas**: `GET /api/v1/tools`
- **Tool execution**: `POST /api/v1/tools/{toolName}`
- **KafClaw skill**: `kafgraph_brain` (via skill routing)

## brain_search

Semantic search — find nodes by meaning, not keywords.

**Input**:
```json
{
  "query": "string",
  "scope": "agent | team | all",
  "limit": 10,
  "timeRange": { "from": "ISO8601", "to": "ISO8601" }
}
```

**Output**:
```json
{
  "results": [
    { "nodeId": "string", "type": "string", "content": "string", "score": 0.95, "properties": {} }
  ]
}
```

## brain_recall

Load accumulated context for a specific agent: active conversations, recent decisions,
pending feedback, team context, and unresolved threads.

**Input**:
```json
{
  "agentId": "string",
  "depth": "shallow | deep",
  "includeTeamContext": true
}
```

**Output**:
```json
{
  "context": {
    "activeConversations": [{ "nodeId": "string", "type": "string", "summary": "string", "timestamp": "ISO8601" }],
    "recentDecisions": [],
    "pendingFeedback": [],
    "unresolvedThreads": []
  }
}
```

## brain_capture

Write insights, decisions, and observations into the brain. Captured items are
auto-embedded, auto-classified, and auto-linked to related graph nodes.

**Input**:
```json
{
  "agentId": "string",
  "type": "insight | decision | observation",
  "content": "string",
  "tags": ["string"],
  "linkedNodes": ["nodeId"]
}
```

**Output**:
```json
{
  "nodeId": "string",
  "linkedTo": ["nodeId"]
}
```

## brain_recent

Browse recent activity within a configurable time window.

**Input**:
```json
{
  "agentId": "string",
  "windowHours": 24,
  "types": ["Message", "LearningSignal"],
  "limit": 50
}
```

**Output**:
```json
{
  "activity": [
    { "nodeId": "string", "type": "string", "summary": "string", "timestamp": "ISO8601" }
  ]
}
```

## brain_patterns

Surface recurring themes, connections, and patterns from the knowledge graph.

**Input**:
```json
{
  "agentId": "string",
  "scope": "agent | team",
  "minOccurrences": 3,
  "timeRange": { "from": "ISO8601", "to": "ISO8601" }
}
```

**Output**:
```json
{
  "patterns": [
    { "theme": "string", "occurrences": 5, "relatedNodes": [], "trend": "increasing | stable | decreasing" }
  ]
}
```

## brain_reflect

Trigger an on-demand reflection cycle and return results inline.

**Input**:
```json
{
  "agentId": "string",
  "scope": "self | cross",
  "windowHours": 24
}
```

**Output**:
```json
{
  "cycleId": "string",
  "learningSignals": [],
  "summary": "string",
  "humanFeedbackStatus": "PENDING"
}
```

## brain_feedback

Submit human feedback on reflection cycles.

**Input**:
```json
{
  "cycleId": "string",
  "feedbackType": "confirm | override | waive",
  "scores": { "impact": 0.8, "relevance": 0.9, "valueContribution": 0.7 },
  "comment": "string"
}
```

**Output**:
```json
{
  "feedbackId": "string",
  "cycleStatus": "RECEIVED"
}
```

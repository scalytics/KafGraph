---
name: brain_reflect
description: Trigger an on-demand reflection cycle and return results inline.
---

# brain_reflect

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_reflect | KafClaw skill routing
**Version**: 0.1.0

## Description

Triggers an on-demand reflection cycle for a specific agent and returns the
results inline. Supports both self-directed reflection (agent's own activity)
and cross-directed reflection (other agents' contributions).

## Input Schema

```json
{
  "agentId": "string",
  "scope": "self | cross",
  "windowHours": 24
}
```

## Output Schema

```json
{
  "cycleId": "string",
  "learningSignals": [],
  "summary": "string",
  "humanFeedbackStatus": "PENDING"
}
```

## Workflow

- Validate the reflection request
- Determine the time window for reflection
- Execute reflection over the agent's activity graph
- Generate learning signals and scores
- Create ReflectionCycle and LearningSignal nodes
- Return results inline with PENDING feedback status

## Safety

- Write operation — creates ReflectionCycle and LearningSignal nodes
- Idempotent — re-running for the same window updates existing nodes
- May trigger a human feedback request after the grace period

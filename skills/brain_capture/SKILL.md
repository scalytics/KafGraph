---
name: brain_capture
description: Write insights, decisions, and observations directly into the brain.
---

# brain_capture

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_capture | KafClaw skill routing
**Version**: 0.1.0

## Description

Allows agents to write insights, decisions, and observations into the brain.
Captured items are automatically embedded (vector representation), classified,
and linked to related graph nodes via vector similarity.

## Input Schema

```json
{
  "agentId": "string",
  "type": "insight | decision | observation",
  "content": "string",
  "tags": ["string"],
  "linkedNodes": ["nodeId"]
}
```

## Output Schema

```json
{
  "nodeId": "string",
  "linkedTo": ["nodeId"],
  "embeddingGenerated": true
}
```

## Workflow

- Validate the capture payload
- Create a new graph node with the captured content
- Generate embedding vector for the content
- Auto-link to related nodes via vector similarity
- Publish to `kafgraph.brain-captures` topic for cluster sync
- Return the created node ID and links

## Safety

- Write operation — creates new graph nodes
- Agent identity is validated before capture
- Content is auto-embedded but not auto-shared beyond the agent's scope unless configured

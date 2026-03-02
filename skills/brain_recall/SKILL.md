---
name: brain_recall
description: Load accumulated context for a specific agent — no more starting from zero.
---

# brain_recall

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_recall | KafClaw skill routing
**Version**: 0.1.0

## Description

Loads the accumulated context for a specific agent: active conversations, recent
decisions, pending feedback, team context, and unresolved threads. This is the
"no more starting from zero" capability.

## Input Schema

```json
{
  "agentId": "string",
  "depth": "shallow | deep",
  "includeTeamContext": true
}
```

## Output Schema

```json
{
  "context": {
    "activeConversations": [],
    "recentDecisions": [],
    "pendingFeedback": [],
    "teamContext": {},
    "unresolvedThreads": []
  }
}
```

## Workflow

- Identify the requesting agent
- Gather active conversation threads
- Collect recent decisions and learning signals
- Check for pending human feedback
- Optionally include team-wide context
- Return structured context summary

## Safety

- Read-only operation
- Agent can only recall its own context (or team context if authorized)
- Deep mode may return larger payloads — use shallow for quick context loads

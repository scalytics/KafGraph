---
name: brain_feedback
description: Submit human feedback on reflection cycles.
---

# brain_feedback

**Type**: kafgraph_brain
**Transport**: HTTP POST /api/v1/tools/brain_feedback | KafClaw skill routing
**Version**: 0.1.0

## Description

Submit human feedback on reflection cycles. Feedback can confirm, override,
or waive automatically computed scores for impact, relevance, and value
contribution.

## Input Schema

```json
{
  "cycleId": "string",
  "feedbackType": "confirm | override | waive",
  "scores": {
    "impact": 0.8,
    "relevance": 0.9,
    "valueContribution": 0.7
  },
  "comment": "string"
}
```

## Output Schema

```json
{
  "feedbackId": "string",
  "cycleStatus": "RECEIVED"
}
```

## Workflow

- Validate the feedback payload and cycle ID
- Create a HumanFeedback node linked to the ReflectionCycle
- Update edge weights if feedbackType is "override"
- Update ReflectionCycle humanFeedbackStatus to RECEIVED (or WAIVED)
- Publish feedback event to the feedback Kafka topic

## Safety

- Write operation — modifies graph scores
- Requires authorized identity (team leader or designated expert)
- Override feedback permanently changes computed scores
- Waive feedback marks the cycle as complete without validation

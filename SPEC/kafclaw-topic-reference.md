# KafClaw Topic Reference for KafGraph

*Version: 0.1 — 2026-03-02*

---

## 1. Purpose

This document captures the **actual topic structure** used by KafClaw agent groups,
as found in the KafClaw codebase (`internal/group/topics.go`). KafGraph must consume
from these topics — not from the placeholder names originally assumed in the SPEC.

This is the authoritative mapping between KafClaw's wire format and KafGraph's
ingestion layer.

---

## 2. KafClaw Topic Naming Convention

All KafClaw topics follow a hierarchical pattern rooted in the **group name**:

```
group.<group_name>.<category>[.<subcategory>]
```

The group name is the logical identifier for an agent team (e.g., `workshop`,
`research-team`, `team-alpha`).

---

## 3. Core Topics per Agent Group

KafClaw defines **11 core topics** per group via the `ExtendedTopicNames` struct
in `internal/group/topics.go`:

| # | Topic Pattern | Category | Purpose |
|---|--------------|----------|---------|
| 1 | `group.<name>.announce` | Control | Join / leave / heartbeat announcements |
| 2 | `group.<name>.requests` | Tasks | General task delegation requests |
| 3 | `group.<name>.responses` | Tasks | General task completion responses |
| 4 | `group.<name>.tasks.status` | Tasks | Task progress updates (accepted / in_progress / completed / failed) |
| 5 | `group.<name>.traces` | Observe | Distributed trace spans |
| 6 | `group.<name>.control.roster` | Control | Topic registry manifest + member capabilities |
| 7 | `group.<name>.control.onboarding` | Control | Agent onboarding protocol (4-step handshake) |
| 8 | `group.<name>.observe.audit` | Observe | Admin audit trail events |
| 9 | `group.<name>.memory.shared` | Memory | Persistent shared knowledge (via LFS / S3) |
| 10 | `group.<name>.memory.context` | Memory | Ephemeral context sharing (TTL-based) |
| 11 | `group.<name>.orchestrator` | Orchestrator | Hierarchy discovery + zone coordination |

### 3.1 Dynamic Skill Topics

In addition to the 11 core topics, KafClaw dynamically creates **skill topic pairs**
when agents register capabilities:

```
group.<name>.skill.<skill_name>.requests
group.<name>.skill.<skill_name>.responses
```

These are tracked in a `TopicManifest` published to the roster topic.

---

## 4. Wire Format — GroupEnvelope

All KafClaw messages share a common JSON envelope:

```json
{
  "Type":          "<envelope_type>",
  "CorrelationID": "<uuid>",
  "SenderID":      "<agent_id>",
  "Timestamp":     "<RFC3339>",
  "Payload":       { ... }
}
```

### 4.1 Envelope Types

| Type | Topic(s) | Payload Struct | KafGraph Relevance |
|------|----------|---------------|-------------------|
| `announce` | `group.*.announce` | `AnnouncePayload` (Identity + action) | **HIGH** — Agent join/leave/heartbeat for Agent nodes |
| `request` | `group.*.requests` | `TaskRequestPayload` (description, content, requester) | **HIGH** — Task conversations to ingest |
| `response` | `group.*.responses` | `TaskResponsePayload` (status, content) | **HIGH** — Task completions to ingest |
| `task_status` | `group.*.tasks.status` | `TaskStatusPayload` (accepted/in_progress/completed/failed) | **MEDIUM** — Enriches task graph edges |
| `trace` | `group.*.traces` | `TracePayload` (traceId, spanId, timing, content) | **MEDIUM** — Performance and interaction timing |
| `onboard` | `group.*.control.onboarding` | `OnboardPayload` (action, identity, challenge/response) | **LOW** — Agent lifecycle events |
| `roster` | `group.*.control.roster` | `TopicManifest` (topic registry, version, consumers) | **HIGH** — Skill and topic discovery |
| `memory` | `group.*.memory.*` | `MemoryItem` (title, tags, LFSEnvelope) | **HIGH** — Shared knowledge artifacts |
| `audit` | `group.*.observe.audit` | Map with event_type, detail, agent_id | **MEDIUM** — Compliance audit trail |
| `skill_request` | `group.*.skill.*.requests` | `TaskRequestPayload` for specific skill | **HIGH** — Skill-specific conversations |
| `skill_response` | `group.*.skill.*.responses` | `TaskResponsePayload` for specific skill | **HIGH** — Skill-specific results |

### 4.2 Agent Identity (within AnnouncePayload)

```json
{
  "agentId":      "agent-researcher",
  "agentName":    "Research Agent",
  "soulSummary":  "Specializes in deep web research...",
  "capabilities": ["web_search", "summarize", "code_review"],
  "channels":     ["whatsapp", "telegram"],
  "model":        "claude-opus-4-6",
  "role":         "worker",
  "parentId":     "agent-orchestrator",
  "zoneId":       "zone-research"
}
```

---

## 5. Topic Categories and KafGraph Mapping

### 5.1 Topics KafGraph MUST Consume (Primary)

These topics carry the core conversation data that forms the knowledge graph:

| KafClaw Topic | Graph Nodes Created | Graph Edges Created |
|---------------|-------------------|-------------------|
| `group.<name>.announce` | `Agent` (upsert on join) | — |
| `group.<name>.requests` | `Conversation`, `Message` | `AUTHORED`, `BELONGS_TO` |
| `group.<name>.responses` | `Message` | `REPLIED_TO`, `AUTHORED`, `BELONGS_TO` |
| `group.<name>.tasks.status` | — (enrichment) | Updates `BELONGS_TO` edge properties |
| `group.<name>.skill.*.requests` | `Conversation`, `Message`, `Skill` | `AUTHORED`, `BELONGS_TO`, `USES_SKILL` |
| `group.<name>.skill.*.responses` | `Message` | `REPLIED_TO`, `AUTHORED` |
| `group.<name>.memory.shared` | `SharedMemory` | `SHARED_BY`, `REFERENCES` |

### 5.2 Topics KafGraph SHOULD Consume (Enrichment)

These topics provide supplementary context for richer graph analysis:

| KafClaw Topic | Graph Enrichment |
|---------------|-----------------|
| `group.<name>.traces` | Timing annotations on `Message` and `Conversation` nodes |
| `group.<name>.observe.audit` | `AuditEvent` nodes linked to `Agent` |
| `group.<name>.control.roster` | Dynamic skill discovery — auto-subscribe to new skill topics |
| `group.<name>.memory.context` | Ephemeral context annotations (TTL-based, not persisted long-term) |
| `group.<name>.orchestrator` | Hierarchy edges between `Agent` nodes (`DELEGATES_TO`, `REPORTS_TO`) |

### 5.3 Topics KafGraph Does NOT Consume

| KafClaw Topic | Reason |
|---------------|--------|
| `group.<name>.control.onboarding` | Internal handshake protocol; KafGraph observes join via `announce` |

---

## 6. Topic Manifest and Auto-Discovery

KafClaw publishes a `TopicManifest` to the roster topic whenever a skill is
registered. The manifest contains:

```json
{
  "GroupName":   "workshop",
  "Version":     3,
  "CoreTopics":  [ ... ],
  "SkillTopics": [
    { "Name": "group.workshop.skill.code_review.requests", "AgentID": "agent-reviewer" },
    { "Name": "group.workshop.skill.code_review.responses", "AgentID": "agent-reviewer" }
  ],
  "UpdatedAt":   "2026-03-02T10:00:00Z",
  "UpdatedBy":   "agent-reviewer"
}
```

**KafGraph MUST subscribe to the roster topic** and dynamically add subscriptions
for newly registered skill topics. This ensures that skill-based conversations are
captured in the graph without manual reconfiguration.

---

## 7. LFS Envelope (Large Payloads)

KafClaw uses the KafScale LFS (Large File Storage) "Claim Check" pattern for large
messages. When a `MemoryItem` or large task payload exceeds the inline size limit,
the Kafka message contains an LFS envelope instead of the raw content:

```json
{
  "bucket":    "kafscale-data",
  "key":       "lfs/workshop/agent-researcher/artifact-123.json",
  "size":      1048576,
  "checksum":  "sha256:abc123..."
}
```

KafGraph must resolve LFS envelopes during ingestion by fetching the actual content
from S3 via the KafScale LFS API.

---

## 8. Mapping to KafGraph's Original Topic Assumptions

The original SPEC assumed two placeholder topic names. Here is the corrected mapping:

| Original SPEC Assumption | Actual KafClaw Topics |
|--------------------------|----------------------|
| `kafclaw.conversations.<team>` | `group.<group_name>.requests`, `group.<group_name>.responses`, `group.<group_name>.skill.*.requests`, `group.<group_name>.skill.*.responses` |
| `kafclaw.audits.<team>` | `group.<group_name>.observe.audit` |

The actual topic surface is significantly broader than originally assumed.

---

*Source: KafClaw repository at `/Users/kamir/GITHUB.kamir/KafClaw/internal/group/topics.go`,
`types.go`, `consumer.go`, `skills.go`, `memory.go`.*

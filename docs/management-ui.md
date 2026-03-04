---
layout: page
title: Management UI
permalink: /management-ui/
---

# Management UI

KafGraph includes an embedded management UI served on the same HTTP port
(default `:7474`). The UI provides real-time visibility into the graph
database, reflection engine, and cluster topology.

## Accessing the UI

Start KafGraph and open your browser:

```
http://localhost:7474/
```

No additional configuration is required. The UI is embedded in the binary
via Go's `embed.FS` and works in air-gapped environments.

## Views

### Dashboard

The dashboard provides an overview of system health:

- **Metric cards**: Total nodes, edges, active reflection cycles, pending feedback
- **Activity timeline**: Recent graph events from the last 24 hours
- **Node distribution**: Bar chart of nodes by label
- **Service health**: Version, uptime, Go version, storage engine

The dashboard auto-refreshes every 30 seconds.

### Graph Browser

Interactive graph visualization powered by Cytoscape.js:

- **Search**: Find nodes by ID or property values
- **Label filter**: Browse nodes by type (Agent, Message, etc.)
- **Depth selection**: Explore 1 or 2 hops from a focal node
- **Layout options**: Force-directed, concentric, or breadth-first
- **Node detail panel**: Click a node to inspect its properties
- **Double-click**: Re-center exploration around a node

Node colors follow the label color scheme:

| Label | Color |
|-------|-------|
| Agent | Blue |
| Conversation | Indigo |
| Message | Slate |
| LearningSignal | Emerald |
| ReflectionCycle | Amber |
| HumanFeedback | Violet |
| Skill | Cyan |
| SharedMemory | Gray |
| AuditEvent | Red |

### Data Stats

Storage and distribution metrics:

- **Node/edge distribution**: Bar charts by label type
- **Storage metrics**: LSM tree size, value log size, data directory
- **Audit events**: Recent audit event table (if available)

### Configuration

Read-only view of the running configuration:

- **Node Configuration**: Server, storage, Kafka, S3, ingest, reflection settings
- **Cluster**: Member table, partition map grid, self-node info

S3 credentials are redacted server-side before display.

### Reflection

Reflection engine insights:

- **Summary cards**: Total cycles, signals, average scores
- **Feedback pipeline**: Pie chart of cycle feedback statuses
- **Cycle type distribution**: Daily, weekly, monthly breakdown
- **Cycle table**: Filterable, paginated list with top signals
- **Score distribution**: Average impact, relevance, value contribution

## Management API

All management endpoints live under `/api/v2/mgmt/` and return JSON.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v2/mgmt/info` | Service info (version, uptime, Go version) |
| GET | `/api/v2/mgmt/storage` | Storage engine metrics |
| GET | `/api/v2/mgmt/stats/graph` | Node/edge counts by label |
| GET | `/api/v2/mgmt/graph/explore` | Subgraph exploration |
| GET | `/api/v2/mgmt/graph/search` | Node search |
| GET | `/api/v2/mgmt/config` | Configuration (secrets redacted) |
| GET | `/api/v2/mgmt/cluster` | Cluster topology |
| GET | `/api/v2/mgmt/reflect/summary` | Reflection summary stats |
| GET | `/api/v2/mgmt/reflect/cycles` | Paginated cycle list |
| GET | `/api/v2/mgmt/activity` | Recent activity timeline |

### Graph Explore Parameters

| Parameter | Description |
|-----------|-------------|
| `nodeId` | Focal node ID for neighborhood exploration |
| `label` | Sample nodes by label |
| `depth` | Exploration depth (1 or 2) |
| `limit` | Max nodes to return (default 50, max 200) |

### Graph Search Parameters

| Parameter | Description |
|-----------|-------------|
| `q` | Search query (matches ID, label, property values) |
| `label` | Filter by node label |
| `limit` | Max results (default 20, max 100) |

### Reflection Cycles Parameters

| Parameter | Description |
|-----------|-------------|
| `limit` | Page size (default 20, max 100) |
| `offset` | Pagination offset |
| `type` | Filter by cycle type (daily, weekly, monthly) |
| `status` | Filter by cycle status |

## Technology Stack

| Component | Technology |
|-----------|------------|
| Graph visualization | Cytoscape.js |
| Charts | Apache ECharts |
| Application logic | Vanilla JS + ES Modules |
| CSS | Custom design system with CSS variables |
| Delivery | Go `embed.FS` (single binary) |
| Routing | Hash-based (`#/view`) |

## Future CLI Integration

The `/api/v2/mgmt/` endpoints return machine-readable JSON suitable for
a future `kafgraph-ctl` CLI tool:

```bash
kafgraph-ctl info          # GET /api/v2/mgmt/info
kafgraph-ctl stats         # GET /api/v2/mgmt/stats/graph
kafgraph-ctl cluster       # GET /api/v2/mgmt/cluster
kafgraph-ctl reflect       # GET /api/v2/mgmt/reflect/summary
```

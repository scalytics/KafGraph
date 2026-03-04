---
title: Distribution Architecture
layout: default
nav_order: 7
---

# Distribution Architecture

KafGraph Phase 7 adds multi-node clustering via gossip-based membership,
agent-ID-based graph partitioning, internal RPC, and a query router that
fans out queries across partitions and merges results.

## Overview

```
 ┌────────────────────────┐     ┌────────────────────────┐
 │       Node A           │     │       Node B           │
 │  ┌──────────────────┐  │     │  ┌──────────────────┐  │
 │  │  Bolt / HTTP     │  │     │  │  Bolt / HTTP     │  │
 │  │  (QueryExecutor) │  │     │  │  (QueryExecutor) │  │
 │  └────────┬─────────┘  │     │  └────────┬─────────┘  │
 │           │             │     │           │             │
 │  ┌────────▼─────────┐  │     │  ┌────────▼─────────┐  │
 │  │  QueryRouter     │──┼─RPC─┼──│  QueryRouter     │  │
 │  └────────┬─────────┘  │     │  └────────┬─────────┘  │
 │           │             │     │           │             │
 │  ┌────────▼─────────┐  │     │  ┌────────▼─────────┐  │
 │  │  Local Executor   │  │     │  │  Local Executor   │  │
 │  │  (BadgerDB shard) │  │     │  │  (BadgerDB shard) │  │
 │  └──────────────────┘  │     │  └──────────────────┘  │
 │                         │     │                         │
 │  ┌──────────────────┐  │     │  ┌──────────────────┐  │
 │  │  Memberlist      │◄─┼─────┼──│  Memberlist      │  │
 │  │  (gossip)        │──┼─────┼──│  (gossip)        │  │
 │  └──────────────────┘  │     │  └──────────────────┘  │
 └────────────────────────┘     └────────────────────────┘
```

## Components

### Partition Strategy

Agent IDs are mapped to partitions using FNV-1a hashing:

```
partition = FNV-1a(agentID) % numPartitions
```

Default: 16 partitions. Partitions are assigned to nodes via deterministic
round-robin over the sorted member list. All nodes compute the same partition
map independently — no coordination required.

### Membership (Gossip)

Uses [hashicorp/memberlist](https://github.com/hashicorp/memberlist) for
decentralized cluster discovery. Nodes join by contacting seed addresses.
Member metadata (RPC port, Bolt port, HTTP port) is encoded in node metadata.

Join/leave events trigger automatic partition rebalancing.

### Internal RPC

Length-prefixed JSON over TCP. Wire format:

```
[4-byte big-endian length][JSON payload]
```

Used for cross-node query forwarding. Each node runs an RPC server that
accepts query requests and executes them against its local shard.

### Query Router

The `QueryRouter` implements the `QueryExecutor` interface. On query execution:

1. Identifies all unique partition owners from the partition map
2. If single-node: executes locally (zero overhead)
3. If multi-node: fans out to all nodes concurrently
   - Local shard: direct execution (no RPC)
   - Remote shards: RPC calls in parallel
4. Merges results: concatenates rows, uses first shard's columns
5. Partial failure: returns available results if at least one shard succeeds

## Configuration

```yaml
cluster:
  enabled: false          # Enable cluster mode
  node_name: ""           # Unique node name (auto-generated if empty)
  bind_addr: "0.0.0.0"   # Gossip bind address
  gossip_port: 7946       # Gossip protocol port
  rpc_port: 7948          # Internal RPC port
  seeds: []               # Seed node addresses for joining
  num_partitions: 16      # Number of graph partitions
```

Environment variables: `KAFGRAPH_CLUSTER_ENABLED`, `KAFGRAPH_CLUSTER_NODE_NAME`, etc.

## Backward Compatibility

When `cluster.enabled = false` (default), KafGraph runs in single-node mode
identical to Phase 6. The `QueryExecutor` interface (`cluster.QueryExecutor`)
is satisfied by both `*query.Executor` (local) and `*cluster.QueryRouter`
(distributed) via Go structural typing.

## Requirements

| Requirement | Description | Status |
|-------------|-------------|--------|
| FR-DM-01 | Multi-node cluster | Complete |
| FR-DM-02 | Agent-ID partitioning | Complete |
| FR-DM-06 | Cross-partition queries | Complete |
| FR-DM-07 | Cluster-wide Bolt endpoint | Complete |

### Deferred to Phase 8

| Requirement | Description |
|-------------|-------------|
| FR-DM-03 | Kafka coordination topic |
| FR-DM-04 | Read availability during failures |
| FR-DM-05 | Synchronous replication (factor ≥ 2) |

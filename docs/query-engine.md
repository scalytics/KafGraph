---
layout: default
title: Query Engine
nav_order: 5
---

# Query Engine

KafGraph includes an OpenCypher subset query engine for querying the property graph.
Queries can be executed via the HTTP API or the Bolt v4 protocol.

## Supported Cypher

### Reading

```cypher
MATCH (n:Agent) RETURN n
MATCH (n:Agent {name: 'alice'}) RETURN n
MATCH (n:Agent) WHERE n.name = 'alice' RETURN n
MATCH (n:Agent)-[:KNOWS]->(m:Agent) RETURN n, m
MATCH (n:Agent)<-[:FOLLOWS]-(m:Agent) RETURN n, m
MATCH (n:Agent) RETURN n.name AS name
MATCH (n:Agent) RETURN count(*) AS total
MATCH (n:Agent) RETURN n ORDER BY n.name ASC
MATCH (n:Agent) RETURN n LIMIT 10 SKIP 5
MATCH (n:Agent) RETURN *
```

### Writing

```cypher
CREATE (n:Agent {name: 'alice'})
CREATE (n)-[:KNOWS]->(m)
MERGE (n:Agent {name: 'alice'})
MATCH (n:Agent) SET n.role = 'leader'
MATCH (n:Agent) DELETE n
```

### WHERE Operators

| Operator | Example |
|----------|---------|
| `=` | `n.name = 'alice'` |
| `<>` | `n.name <> 'bob'` |
| `<`, `>` | `n.age > 25` |
| `<=`, `>=` | `n.score >= 80` |
| `AND` | `n.name = 'alice' AND n.role = 'leader'` |
| `OR` | `n.name = 'alice' OR n.name = 'bob'` |
| `NOT` | `NOT n.active = true` |
| `CONTAINS` | `n.text CONTAINS 'hello'` |
| `IN` | `n.name IN ['alice', 'bob']` |

### Parameters

Use `$param` syntax with a params map:

```cypher
MATCH (n:Agent) WHERE n.name = $name RETURN n
```

## Full-Text Search

Full-text search is available via the `kafgraph.fullTextSearch` procedure:

```cypher
CALL kafgraph.fullTextSearch('Message', 'text', 'hello world') YIELD node, score
```

Powered by [bleve](https://github.com/blevesearch/bleve). Text properties are indexed
for configured label/property pairs.

## Vector Search

Vector similarity search uses brute-force cosine similarity:

```cypher
CALL kafgraph.vectorSearch('Message', 'embedding', [0.1, 0.2, 0.3], 10) YIELD node, score
```

Vectors are stored as binary float32 arrays in BadgerDB at key prefix `v:`.

## HTTP API

```
POST /api/v1/query
Content-Type: application/json

{
    "cypher": "MATCH (n:Agent) WHERE n.name = $name RETURN n",
    "params": {"name": "alice"}
}
```

Response:

```json
{
    "columns": ["n"],
    "rows": [[{"id": "...", "label": "Agent", "properties": {"name": "alice"}}]]
}
```

## Bolt v4 Protocol

KafGraph implements the Neo4j Bolt v4.4 protocol on port 7687 (configurable).
Any Neo4j-compatible driver can connect and execute queries.

### Message Types

| Type | Code | Description |
|------|------|-------------|
| HELLO | 0x01 | Client authentication |
| RUN | 0x10 | Submit query + parameters |
| PULL | 0x3F | Request result records |
| RECORD | 0x71 | Single result row |
| SUCCESS | 0x70 | Operation completed |
| FAILURE | 0x7F | Operation failed |
| RESET | 0x0F | Reset connection state |

### PackStream Encoding

The Bolt protocol uses PackStream binary encoding for all data:
- Integers (tiny, 8, 16, 32-bit)
- Strings (tiny, 8, 16-bit length)
- Lists and Maps (tiny, 8-bit length)
- Booleans, Null
- Struct markers for message framing

## Secondary Indexes

BadgerDB secondary indexes are maintained transactionally alongside writes:

| Key Prefix | Purpose |
|-----------|---------|
| `i:lbl:<label>:<nodeID>` | Nodes by label |
| `i:out:<nodeID>:<edgeID>` | Outgoing edges per node |
| `i:in:<nodeID>:<edgeID>` | Incoming edges per node |
| `i:elbl:<label>:<edgeID>` | Edges by label |

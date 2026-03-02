---
layout: default
title: Configuration
nav_order: 5
---

# Configuration

KafGraph uses a layered configuration system:

1. **Defaults** — built into the binary
2. **Config file** — `kafgraph.yaml` in `.` or `/etc/kafgraph/`
3. **Environment variables** — `KAFGRAPH_` prefix, override everything

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KAFGRAPH_HOST` | `0.0.0.0` | HTTP API listen address |
| `KAFGRAPH_PORT` | `7474` | HTTP API port |
| `KAFGRAPH_BOLT_PORT` | `7687` | Bolt protocol port |
| `KAFGRAPH_DATA_DIR` | `./data` | BadgerDB data directory |
| `KAFGRAPH_STORAGE_ENGINE` | `badger` | Storage backend |
| `KAFGRAPH_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `KAFGRAPH_LOG_FORMAT` | `json` | Log format (json, text) |
| `KAFGRAPH_KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `KAFGRAPH_KAFKA_GROUP_ID` | `kafgraph` | Kafka consumer group ID |
| `KAFGRAPH_KAFKA_TOPIC_PREFIX` | `group` | KafClaw topic prefix |
| `KAFGRAPH_S3_ENDPOINT` | `localhost:9000` | MinIO/S3 endpoint |
| `KAFGRAPH_S3_ACCESS_KEY` | — | S3 access key |
| `KAFGRAPH_S3_SECRET_KEY` | — | S3 secret key |
| `KAFGRAPH_S3_BUCKET` | `kafgraph` | S3 bucket name |
| `KAFGRAPH_S3_USE_SSL` | `false` | Enable TLS for S3 |
| `KAFGRAPH_EMBEDDING_ENDPOINT` | `http://localhost:11434` | Embedding API endpoint |
| `KAFGRAPH_EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model name |
| `KAFGRAPH_METRICS_PORT` | `9090` | Prometheus metrics port |

## Config File

Create `kafgraph.yaml`:

```yaml
host: 0.0.0.0
port: 7474
bolt_port: 7687
data_dir: /var/lib/kafgraph
storage_engine: badger
log_level: info
log_format: json

kafka:
  brokers: kafka:9092
  group_id: kafgraph
  topic_prefix: group

s3:
  endpoint: minio:9000
  access_key: minioadmin
  secret_key: minioadmin
  bucket: kafgraph
  use_ssl: false
```

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 7474 | HTTP | REST API, health checks, metrics |
| 7687 | TCP | Bolt v4 protocol (Neo4j compatible) |
| 9090 | HTTP | Prometheus metrics |

# ai-json

Live class-camera event ingestion and analytics with local SQLite.

## What It Does

- Reads class/camera topology from `stream.json`
- Scans camera event folders for newly produced JSON files
- Periodically ingests new/changed files into local SQLite
- Exposes advanced HTTP endpoints for ingestion, querying, and analytics

## Quick Start

```bash
# 1) Start API server + scheduler
go run ./cmd/ai-json-api --stream ./stream.json --poll-seconds 5

# 2) Trigger immediate ingestion (optional)
curl -X POST 'http://127.0.0.1:8080/v1/ingest/stream?stream_path=./stream.json'

# 3) Read analytics
curl 'http://127.0.0.1:8080/v1/summary'
```

## Docs

- API reference: `docs/API.md`
- Stream config guide: `docs/STREAM.md`

## CLI Analyzer (existing)

You can still run the offline analytics CLI:

```bash
go run ./cmd/ai-json --stream stream.json
```

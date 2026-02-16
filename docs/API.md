# AI-JSON API Documentation

## Overview

`ai-json-api` provides local HTTP endpoints backed by SQLite for:

- live ingestion from class/camera event directories
- direct event ingestion by payload
- querying stored events with filters and pagination
- aggregated summary analytics

The service is designed for continuously produced event JSON files.

## Architecture

### Components

- `cmd/ai-json-api`: HTTP server process and periodic scheduler
- `internal/ingest`: live scanner/ingestion runner for stream directories
- `internal/store`: SQLite schema, insert logic, query filters, summary SQL
- `internal/api`: HTTP handlers, request validation, and JSON responses

### Data flow

1. Event producer writes JSON files to camera event folders.
2. Periodic runner scans folders from `stream.json`.
3. New/changed files are detected using `ingested_files` state table.
4. Events are parsed and inserted into `events` table.
5. API query endpoints read analytics and event rows from SQLite.

## Run

```bash
go run ./cmd/ai-json-api \
  --addr :8080 \
  --db ./data/ai-json.db \
  --stream ./stream.json \
  --poll-seconds 5 \
  --min-file-age-seconds 2
```

### Runtime flags

- `--addr`: HTTP bind address (default `:8080`)
- `--db`: SQLite file path (default `./data/ai-json.db`)
- `--stream`: stream config path used by scheduler and stream ingestion endpoint
- `--poll-seconds`: periodic scan interval (`0` disables scheduler)
- `--min-file-age-seconds`: skip files younger than this age to avoid partial writes

## stream.json Structure

### Required model

- `classes[]`
  - `class_id` (required)
  - `name` (optional)
  - `base_dir` (required/recommended)
  - `cameras[]`
    - required IDs: `front` and `back`
    - `images_dir` (optional, defaults to `<base_dir>/<camera>/images`)
    - `events_dir` (optional, defaults to `<base_dir>/<camera>/events`)
    - `file_pattern` (optional, default `*.json`)

### Optional compatibility fields

- `event_files[]`
- `event_globs[]`

These are still supported but directory-based ingestion is the primary live mode.

## Endpoints

## `GET /health`

Health check.

### Response 200

```json
{
  "status": "ok",
  "timestamp": "2026-02-16T09:46:05.985091193Z"
}
```

## `POST /v1/ingest/events`

Ingest raw event payload directly.

### Query parameters

- `class_id` (optional): inject/override `stream_class_id`
- `camera_id` (optional): inject/override `stream_camera_id`
- `source` (optional): source label stored in DB

### Body

- JSON array of events
- JSON object (single event)
- NDJSON

### Response 200

```json
{
  "inserted": 120,
  "source": "api:/v1/ingest/events"
}
```

## `POST /v1/ingest/stream`

Run one live scan ingestion cycle against stream-config camera folders.

### Query parameters

- `stream_path` (optional, default server `--stream`)
- `min_file_age_seconds` (optional, default server `--min-file-age-seconds`)

### Response 200

```json
{
  "inserted": 199,
  "processed_files": 4,
  "skipped_files": 0,
  "stream_path": "/abs/path/stream.json"
}
```

## `GET /v1/events`

List ingested events with pagination and filters.

### Query parameters

- `event_types`: csv (`person_tracked,proximity_event`)
- `class_ids`: csv (uses `stream_class_id` fallback `room_id`)
- `camera_ids`: csv (uses `stream_camera_id` fallback `camera_id`)
- `min_confidence`: float
- `from_ts`: float timestamp lower bound
- `to_ts`: float timestamp upper bound
- `limit`: int (`1..1000`, default `200`)
- `offset`: int (default `0`)

### Response 200

```json
{
  "total": 199,
  "limit": 200,
  "offset": 0,
  "events": [
    {
      "id": 1,
      "ingested_at": "2026-02-16T09:46:08.120Z",
      "source_file": "/path/front/events/1771233054.json",
      "stream_class_id": "classroom-a",
      "stream_camera_id": "front",
      "event_type": "person_tracked",
      "raw": {"...": "..."}
    }
  ]
}
```

## `GET /v1/summary`

Aggregated analytics over the filtered event set.

### Query parameters

Same filter set as `GET /v1/events`.

### Response 200

```json
{
  "summary": {
    "total_events": 199,
    "distinct_classes": 1,
    "distinct_cameras": 2,
    "avg_confidence": 0.579,
    "min_timestamp": 1771233054.2231407,
    "max_timestamp": 1771233089.5232406,
    "event_type_counts": [
      {"key": "person_tracked", "count": 77}
    ],
    "stream_class_counts": [
      {"key": "classroom-a", "count": 199}
    ],
    "stream_camera_counts": [
      {"key": "front", "count": 100},
      {"key": "back", "count": 99}
    ]
  }
}
```

## Error Model

All handler errors are JSON:

```json
{
  "error": {
    "code": "invalid_query",
    "message": "invalid limit"
  }
}
```

### Typical error codes

- `method_not_allowed`
- `read_body_failed`
- `invalid_events_payload`
- `invalid_query`
- `invalid_min_file_age_seconds`
- `stream_load_failed`
- `stream_ingest_failed`
- `insert_failed`
- `query_failed`
- `summary_failed`

### HTTP status usage

- `200`: success
- `400`: invalid input/query/body/config
- `405`: invalid HTTP method
- `500`: internal processing/storage failure

## SQLite Schema

### `events`

- event metadata columns (`event_type`, `class/camera`, `confidence`, `timestamp`, ...)
- raw JSON payload in `raw_json`

### `ingested_files`

- `path` (PK)
- `size_bytes`
- `mod_unix`
- `ingested_at`

Used to ensure periodic scans only ingest new/changed files.

## Operational Notes

- Keep event producers writing atomic files when possible.
- Use `min_file_age_seconds` to avoid ingesting files mid-write.
- For high throughput, run scheduler (`--poll-seconds > 0`) and call query endpoints only.
- You can still trigger manual ingestion with `POST /v1/ingest/stream`.

## Example Workflow

1. Start API server with scheduler enabled.
2. Event producer writes JSON files into configured camera event directories.
3. Scheduler ingests new files every polling interval.
4. Client dashboard polls `GET /v1/summary` and `GET /v1/events`.

# AI-JSON Live API

## Purpose

This service ingests live classroom camera events into local SQLite and provides query/analysis/image endpoints.

It is designed for:

- `class_id` with 2 cameras (`front`, `back`)
- event JSON files named `{unix_epoch_seconds}.json`
- image files named `{unix_epoch_seconds}.jpg` or `.jpeg`
- continuous live ingestion from camera folders

## Runtime

```bash
go run ./cmd/ai-json-api \
  --addr :8080 \
  --db ./data/ai-json.db \
  --stream ./stream.json \
  --poll-seconds 5 \
  --min-file-age-seconds 2 \
  --max-past-seconds 60
```

### Flags

- `--addr`: API bind address
- `--db`: SQLite DB path
- `--stream`: stream config path (can be `stream.json` or `camera.json`)
- `--poll-seconds`: periodic ingestion interval (`0` disables scheduler)
- `--min-file-age-seconds`: skip files too new (avoid partial writes)
- `--max-past-seconds`: ingest only files not older than this by filename epoch

## Stream Config

`stream.json` (or any path passed to `--stream`):

- classes[]
  - class_id
  - base_dir
  - cameras[]
    - id: `front`, `back`
    - images_dir
    - events_dir
    - file_pattern (default `*.json`)

## Ingestion Behavior

Each scan cycle:

1. read classes/cameras from stream config
2. list event JSON files per camera
3. process oldest to newest
4. keep only files in allowed time window (`max_past_seconds`)
5. skip files already ingested with same size+mtime
6. parse JSON events and insert into SQLite

Error tolerance:

- missing/newly-not-ready files are skipped safely
- malformed event files return detailed ingestion errors
- missing image files do not break event ingestion

## Endpoints

## `GET /health`

Health check.

### 200

```json
{"status":"ok","timestamp":"2026-02-16T10:00:00Z"}
```

## `POST /v1/ingest/stream`

Run one ingestion cycle immediately.

### Query

- `stream_path` optional
- `min_file_age_seconds` optional
- `max_past_seconds` optional

### 200

```json
{
  "inserted": 199,
  "processed_files": 4,
  "skipped_files": 0,
  "stream_path": "./stream.json",
  "max_past_seconds": 60
}
```

## `POST /v1/ingest/events`

Direct payload ingestion.

### Query

- `class_id` optional
- `camera_id` optional
- `source` optional

### Body

- JSON array, JSON object, or NDJSON

### 200

```json
{"inserted":12,"source":"api:/v1/ingest/events"}
```

## `GET /v1/events`

General event query.

### Query

- `event_types` csv
- `class_ids` csv
- `camera_ids` csv
- `min_confidence` float
- `from_ts` float
- `to_ts` float
- `limit` int
- `offset` int

### 200

```json
{
  "total": 199,
  "limit": 50,
  "offset": 0,
  "events": [ ... ]
}
```

## `GET /v1/special-events`

Special events for a day (default: current UTC day).

Default special event types:

- `sleeping_suspected`
- `posture_changed`
- `proximity_event`
- `role_assigned`

### Query

- `date` (`YYYY-MM-DD`) optional
- `event_types` csv optional override
- all filters from `/v1/events` (`class_ids`, `camera_ids`, `limit`, `offset`, etc)

### 200

```json
{
  "date": "2026-02-16",
  "event_types": ["sleeping_suspected","posture_changed","proximity_event","role_assigned"],
  "total": 55,
  "events": [ ... ]
}
```

## `GET /v1/special-events-with-images`

Returns special events and pre-expanded image context in one call.

### Query

- All parameters from `GET /v1/special-events`
- `window_seconds` optional (default `5`, range `0..120`)
- `stream_path` optional

### 200

```json
{
  "date": "2026-02-16",
  "event_types": ["sleeping_suspected","posture_changed","proximity_event","role_assigned"],
  "total": 55,
  "window_seconds": 5,
  "events": [
    {
      "event": { "...": "original event row" },
      "images": [
        {"offset_seconds":-1,"timestamp":1771233088,"exists":false},
        {"offset_seconds":0,"timestamp":1771233089,"exists":true,"url":"/v1/image?..."},
        {"offset_seconds":1,"timestamp":1771233090,"exists":false}
      ]
    }
  ]
}
```

## `GET /v1/event-images`

Returns image context around one event.

### Query

- `event_id` required
- `window_seconds` optional (default `5`, range `0..120`)
- `stream_path` optional

### 200

```json
{
  "event_id": 198,
  "class_id": "classroom-a",
  "camera_id": "back",
  "event_ts": 1771233089,
  "window_seconds": 5,
  "images": [
    {"offset_seconds":-1,"timestamp":1771233088,"exists":false},
    {"offset_seconds":0,"timestamp":1771233089,"exists":true,"path":".../1771233089.jpg","url":"/v1/image?class_id=classroom-a&camera_id=back&ts=1771233089"},
    {"offset_seconds":1,"timestamp":1771233090,"exists":false}
  ]
}
```

Notes:

- Missing files are returned with `exists:false` (not an endpoint error).
- This endpoint is the main way to get "5 seconds past/future" image context.

## `GET /v1/image`

Serves a single JPEG by class/camera/second.

### Query

- `class_id` required
- `camera_id` required
- `ts` required (unix seconds)
- `stream_path` optional

### Responses

- `200` with `Content-Type: image/jpeg`
- `404` when image file not found

## `GET /v1/student-metrics/daily`

Cleaned student detection metrics per class for a day.

### Query

- `date` optional (`YYYY-MM-DD`, default current UTC day)
- `class_ids` optional csv

### Cleaning logic

- Uses `person_tracked` and `person_detected`
- Keeps student rows only (`person_role` or `role` == `student`)
- Per class/camera/second: count distinct student identities
- Per class/second: use max across cameras
- Returns:
  - `max_students`: highest per-second class count of day
  - `average_students`: average per-second class count across sampled seconds

### 200

```json
{
  "date":"2026-02-16",
  "metrics":[
    {"class_id":"classroom-a","max_students":7,"average_students":6,"sampled_seconds":4}
  ]
}
```

## `GET /v1/summary`

Aggregated analytics over filtered event set.

### Query

Same filters as `/v1/events`.

### 200

```json
{"summary":{...}}
```

## Error Contract

All non-image errors are JSON:

```json
{
  "error": {
    "code": "invalid_query",
    "message": "invalid limit"
  }
}
```

### Common Codes

- `method_not_allowed`
- `invalid_events_payload`
- `stream_ingest_failed`
- `invalid_min_file_age_seconds`
- `invalid_max_past_seconds`
- `invalid_query`
- `invalid_date`
- `event_not_found`
- `event_without_timestamp`
- `invalid_image_request`
- `image_not_found`
- `daily_metrics_failed`
- `summary_failed`

## SQLite Data

### `events`

Stores event metadata + full raw JSON (`raw_json`).

### `ingested_files`

Tracks ingested event files by `path + size + mtime` to avoid duplicate ingestion.

## Recommended Client Flow

1. Keep API server running with scheduler.
2. Producers write `{epoch}.json` and `{epoch}.jpg` files to camera folders.
3. Use `/v1/special-events` for daily critical incidents.
4. For selected event, call `/v1/event-images?event_id=...&window_seconds=5`.
5. Download/render individual images with `/v1/image` URLs.

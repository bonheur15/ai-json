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

1. Read classes/cameras from stream config
2. List event JSON files per camera
3. Process oldest to newest
4. Keep only files in allowed time window (`max_past_seconds`)
5. Skip files already ingested with same size+mtime
6. Parse JSON events and insert into SQLite

Error tolerance:

- missing/newly-not-ready files are skipped safely
- malformed event files return detailed ingestion errors
- missing image files do not break event ingestion

## Endpoints

## `GET /health`

Health check.

### 200

```json
{
  "status": "ok",
  "timestamp": "2026-02-16T10:00:00.123456789Z"
}
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
{
  "inserted": 12,
  "source": "api:/v1/ingest/events"
}
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
  "limit": 2,
  "offset": 0,
  "events": [
    {
      "id": 199,
      "ingested_at": "2026-02-16T10:03:14.827688582Z",
      "source_file": "/home/bonheur/Desktop/Projects/ai/ai-json/.material/classes/classroom-a/back/events/1771233089.json",
      "stream_class_id": "classroom-a",
      "stream_camera_id": "back",
      "event_type": "frame_tick",
      "room_id": "room1",
      "camera_id": "front",
      "global_person_id": 0,
      "confidence": 1,
      "timestamp": 1771233089.5232406,
      "raw": {
        "event_type": "frame_tick",
        "room_id": "room1",
        "camera_id": "front",
        "detections_count": 20,
        "objects_count": 0,
        "stream_class_id": "classroom-a",
        "stream_camera_id": "back"
      }
    },
    {
      "id": 198,
      "ingested_at": "2026-02-16T10:03:14.827688582Z",
      "source_file": "/home/bonheur/Desktop/Projects/ai/ai-json/.material/classes/classroom-a/back/events/1771233089.json",
      "stream_class_id": "classroom-a",
      "stream_camera_id": "back",
      "event_type": "proximity_event",
      "room_id": "room1",
      "camera_id": "front",
      "global_person_id": 0,
      "confidence": 0.5,
      "timestamp": 1771233089.5232406,
      "raw": {
        "event_type": "proximity_event",
        "status": "close",
        "distance": 104.88207663848004,
        "track_ids": [61, 65],
        "stream_class_id": "classroom-a",
        "stream_camera_id": "back"
      }
    }
  ]
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
  "event_types": [
    "sleeping_suspected",
    "posture_changed",
    "proximity_event",
    "role_assigned"
  ],
  "total": 55,
  "limit": 2,
  "offset": 0,
  "events": [
    {
      "id": 198,
      "ingested_at": "2026-02-16T10:03:14.827688582Z",
      "source_file": "/home/bonheur/Desktop/Projects/ai/ai-json/.material/classes/classroom-a/back/events/1771233089.json",
      "stream_class_id": "classroom-a",
      "stream_camera_id": "back",
      "event_type": "proximity_event",
      "room_id": "room1",
      "camera_id": "front",
      "confidence": 0.5,
      "timestamp": 1771233089.5232406,
      "raw": {
        "event_type": "proximity_event",
        "status": "close",
        "distance": 104.88207663848004
      }
    },
    {
      "id": 197,
      "ingested_at": "2026-02-16T10:03:14.827688582Z",
      "source_file": "/home/bonheur/Desktop/Projects/ai/ai-json/.material/classes/classroom-a/back/events/1771233089.json",
      "stream_class_id": "classroom-a",
      "stream_camera_id": "back",
      "event_type": "proximity_event",
      "room_id": "room1",
      "camera_id": "front",
      "confidence": 0.5,
      "timestamp": 1771233089.5232406,
      "raw": {
        "event_type": "proximity_event",
        "status": "close",
        "distance": 80.1826040983953
      }
    }
  ]
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
  "event_types": [
    "sleeping_suspected",
    "posture_changed",
    "proximity_event",
    "role_assigned"
  ],
  "total": 55,
  "limit": 1,
  "offset": 0,
  "window_seconds": 5,
  "events": [
    {
      "event": {
        "id": 198,
        "stream_class_id": "classroom-a",
        "stream_camera_id": "back",
        "event_type": "proximity_event",
        "camera_id": "front",
        "timestamp": 1771233089.5232406
      },
      "images": [
        {
          "offset_seconds": -1,
          "timestamp": 1771233088,
          "exists": false
        },
        {
          "offset_seconds": 0,
          "timestamp": 1771233089,
          "exists": true,
          "path": "/home/bonheur/Desktop/Projects/ai/ai-json/.material/classes/classroom-a/back/images/1771233089.jpg",
          "url": "/v1/image?class_id=classroom-a&camera_id=back&ts=1771233089"
        },
        {
          "offset_seconds": 1,
          "timestamp": 1771233090,
          "exists": false
        }
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
    {
      "offset_seconds": -1,
      "timestamp": 1771233088,
      "exists": false
    },
    {
      "offset_seconds": 0,
      "timestamp": 1771233089,
      "exists": true,
      "path": "/home/bonheur/Desktop/Projects/ai/ai-json/.material/classes/classroom-a/back/images/1771233089.jpg",
      "url": "/v1/image?class_id=classroom-a&camera_id=back&ts=1771233089"
    },
    {
      "offset_seconds": 1,
      "timestamp": 1771233090,
      "exists": false
    }
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
  "date": "2026-02-16",
  "metrics": [
    {
      "class_id": "classroom-a",
      "max_students": 7,
      "average_students": 6,
      "sampled_seconds": 4
    }
  ]
}
```

## `GET /v1/summary`

Aggregated analytics over filtered event set.

### Query

Same filters as `/v1/events`.

### 200

```json
{
  "summary": {
    "total_events": 199,
    "distinct_classes": 1,
    "distinct_cameras": 2,
    "avg_confidence": 0.5790431245128748,
    "min_timestamp": 1771233054.2231407,
    "max_timestamp": 1771233089.5232406,
    "event_type_counts": [
      {"key": "person_tracked", "count": 77},
      {"key": "head_orientation_changed", "count": 32},
      {"key": "proximity_event", "count": 29}
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

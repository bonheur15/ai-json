# ai-json

Go service for live classroom event ingestion, SQLite storage, analytics, and image context APIs.

## Core Features

- Periodic ingestion from camera event directories
- SQLite-backed event storage and summaries
- Daily special events endpoint
- Event-centered image context endpoint (past/future seconds)
- JPEG serving endpoint
- Daily class student metrics (max and average cleaned counts)

## Start API

```bash
go run ./cmd/ai-json-api \
  --stream ./stream.json \
  --db ./data/ai-json.db \
  --poll-seconds 5 \
  --min-file-age-seconds 2 \
  --max-past-seconds 60
```

## Quick Calls

```bash
# Force one ingestion cycle
curl -X POST 'http://127.0.0.1:8080/v1/ingest/stream?stream_path=./stream.json'

# Special events for today
curl 'http://127.0.0.1:8080/v1/special-events'

# Get +/-5s images for one special event
curl 'http://127.0.0.1:8080/v1/event-images?event_id=198&window_seconds=5'

# Serve one image
curl 'http://127.0.0.1:8080/v1/image?class_id=classroom-a&camera_id=front&ts=1771233054' --output frame.jpg

# Daily student metrics
curl 'http://127.0.0.1:8080/v1/student-metrics/daily?date=2026-02-16'
```

## Documentation

- API: `docs/API.md`
- Stream config: `docs/STREAM.md`

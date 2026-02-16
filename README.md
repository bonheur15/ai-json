# ai-json

Advanced analytics for class-based camera streams.

`ai-json` is now configuration-driven through `stream.json`, where you define classes and camera sources (`front` and `back`), including image folders and event JSON files.

## Core model

- `stream.json` stores classes and all stream configurations.
- Each class must define two cameras: `front` and `back`.
- Each camera points to:
  - an `images_dir` containing `.jpg/.jpeg` frames
  - event JSON sources (`event_files`, `event_globs`, or `events_dir`)

The CLI validates this structure and reports inventory + analytics.

## Quick start

```bash
go run ./cmd/ai-json --stream stream.json
```

If `--stream` is omitted and `./stream.json` exists, it is used automatically.

## stream.json schema

```json
{
  "version": "1.0",
  "classes": [
    {
      "class_id": "classroom-a",
      "name": "Classroom A",
      "base_dir": "./data/classroom-a",
      "cameras": [
        {
          "id": "front",
          "images_dir": "./data/classroom-a/front/images",
          "events_dir": "./data/classroom-a/front/events",
          "event_files": ["./extra/front-events.json"],
          "event_globs": ["./imports/front/*.json"]
        },
        {
          "id": "back",
          "images_dir": "./data/classroom-a/back/images",
          "events_dir": "./data/classroom-a/back/events"
        }
      ]
    }
  ]
}
```

## CLI

```bash
go run ./cmd/ai-json [flags]
```

### Important flags

- `--stream <path>`: stream config JSON path
- `--format text|json`: output format
- `--class-ids <csv>`: filter by class IDs
- `--camera-ids <csv>`: filter by camera IDs
- `--event-types <csv>`: filter by event types
- `--min-confidence <float>`: confidence threshold
- `--max-issues <n>`: limit text issue lines
- `--strict`: exit code `1` if validation errors exist

### Examples

```bash
# Analyze configured classes/cameras
go run ./cmd/ai-json --stream stream.json

# JSON report with stream inventory
go run ./cmd/ai-json --stream stream.json --format json

# Front camera only for one class and selected event types
go run ./cmd/ai-json --stream stream.json \
  --class-ids classroom-a \
  --camera-ids front \
  --event-types person_tracked,role_assigned,proximity_event \
  --min-confidence 0.6
```

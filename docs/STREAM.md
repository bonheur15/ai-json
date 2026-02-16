# Stream Config

## Objective

Declare classes and camera directories for live ingestion.

## File Naming

- Event files: `{unix_epoch_seconds}.json`
- Image files: `{unix_epoch_seconds}.jpg` (or `.jpeg`)

## Example

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
          "file_pattern": "*.json"
        },
        {
          "id": "back",
          "images_dir": "./data/classroom-a/back/images",
          "events_dir": "./data/classroom-a/back/events",
          "file_pattern": "*.json"
        }
      ]
    }
  ]
}
```

## Rules

- Each class must include both `front` and `back` cameras.
- Directories must exist.
- Ingestion scans `events_dir` and uses image folder for context serving.
- Old files can be automatically excluded by `max_past_seconds`.

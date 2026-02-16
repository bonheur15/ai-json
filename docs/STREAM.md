# Stream Configuration Guide

## Purpose

`stream.json` declares your classes and camera directories. Event files are discovered from directories at runtime; you do not need to list each generated event file.

## Minimal Example

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

- Each class must define both `front` and `back` cameras.
- `base_dir` and camera directories must exist.
- Camera event folders are scanned repeatedly by the ingestion runner.
- Changed files are re-ingested; unchanged files are skipped.

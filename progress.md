# Progress Log

## 2026-02-16

### Session start
- Initialized implementation workflow.
- Confirmed sample event files are available in `.material/samples/`.
- Defined staged build plan with per-step commits.

### Step 1 completed
- Added base documentation and task tracking files.
- Added initial CLI entrypoint at `cmd/ai-json/main.go`.
- Established step-by-step commit workflow.

### Step 2 completed
- Implemented robust event parser supporting JSON array, single object, and NDJSON.
- Added schema helper methods for typed field extraction and common-field normalization.
- Added parser tests against sample data and NDJSON fallback.

### Step 3 completed
- Added analysis engine for event distributions, confidence/latency summaries, and identity statistics.
- Added validation logic for common fields, bbox integrity, person-event fields, proximity constraints, and frame tick fields.
- Added analysis tests covering sample data and invalid proximity scenarios.

### Step 4 completed
- Added input loader for explicit files and glob patterns with deterministic file resolution.
- Implemented complete CLI flags for format selection, filtering, issue limits, and strict mode.
- Added text and JSON report output including top entities, latency metrics, and validation issue summaries.
- Added tests for input loading and text report rendering.

### Step 5 completed
- Added threshold-based transport delay validation to reduce false positives from small timing jitter.
- Added tests for warning vs. error behavior on negative transport delay.
- Updated README with complete usage, filtering, and report examples.
- Verified project with `go test ./...` and sample CLI execution.

### Step 6 completed
- Added `stream.json` domain model for classes and camera configurations.
- Enforced required camera pair (`front`, `back`) per class with strict validation.
- Added ingestion of camera image folders (JPG count) and event JSON files.
- Attached stream metadata (`stream_class_id`, `stream_camera_id`) to decoded events.
- Added stream-config tests for success path and missing-camera validation.

### Step 7 completed
- Integrated `--stream` mode in CLI with auto-detection of local `stream.json`.
- Added class and camera filtering flags (`--class-ids`, `--camera-ids`).
- Extended JSON/text output to include stream inventory and per-camera event/image stats.
- Added ready-to-use `stream.json` and camera folder scaffold for class-based workflows.
- Updated README to document stream schema and advanced usage patterns.

### Step 8 completed
- Added stream-aware analytics counters for `stream_class_id` and `stream_camera_id`.
- Extended text report with class and camera event distribution blocks.
- Added test coverage for stream class/camera count aggregation.
- Re-ran full test suite and stream CLI report checks.

### Step 9 completed
- Added SQLite storage layer (`internal/store`) using a local file database.
- Added schema migration with indexes for event type, class, camera, and timestamp.
- Implemented transaction-based event ingestion from decoded event streams.
- Implemented paginated event query API and SQL-based summary aggregations.
- Added storage tests covering insert, list filtering, and summary counts.

### Step 10 completed
- Added HTTP API server (`cmd/ai-json-api`) with health, ingest, events, and summary endpoints.
- Added live ingestion runner that scans stream camera event directories and ingests only new/changed JSON files.
- Added SQLite file-ingestion state tracking (`ingested_files`) to prevent duplicate inserts.
- Added periodic scheduler mode in API server for continuous background ingestion.
- Updated sample `stream.json` and class camera folders to use directory-based live event ingestion.
- Added API and ingestion tests; validated with end-to-end local endpoint smoke run.

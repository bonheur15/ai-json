# ai-json

Advanced JSON analytics CLI for classroom/computer-vision event streams.

It ingests JSON event arrays like those in `.material/samples/` and produces:

- schema-aware validation errors and warnings
- event distribution and confidence analytics
- latency/timestamp quality metrics
- person/track and proximity insights
- text or JSON reports

## Status

Track implementation progress in:

- `todo.md`
- `progress.md`

## Usage

```bash
go run ./cmd/ai-json [flags]
```

If no input flags are passed, the CLI automatically analyzes `.material/samples/*.json`.

### Inputs

- `--input <file>`: direct file input (repeatable or comma-separated)
- `--glob <pattern>`: glob input (repeatable or comma-separated)

### Filtering

- `--event-types <csv>`: include only specific event types
- `--min-confidence <float>`: include only events with confidence >= threshold

### Output

- `--format text|json`: report format
- `--max-issues <n>`: limit printed issues in text output (`0` prints all)
- `--strict`: exit with code `1` if validation errors are found

## Examples

```bash
# Default: analyze sample files with text report
go run ./cmd/ai-json

# Analyze all samples with JSON output
go run ./cmd/ai-json --glob '.material/samples/*.json' --format json

# Filter to key event types and higher confidence
go run ./cmd/ai-json --glob '.material/samples/*.json' \
  --event-types person_tracked,role_assigned,proximity_event \
  --min-confidence 0.6
```

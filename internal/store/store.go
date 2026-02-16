package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"ai-json/internal/input"
	"ai-json/internal/model"
)

type Store struct {
	db *sql.DB
}

type EventFilter struct {
	ClassIDs      []string
	CameraIDs     []string
	EventTypes    []string
	MinConfidence *float64
	FromTS        *float64
	ToTS          *float64
	Limit         int
	Offset        int
}

type EventRecord struct {
	ID             int64           `json:"id"`
	IngestedAt     string          `json:"ingested_at"`
	SourceFile     string          `json:"source_file"`
	StreamClassID  string          `json:"stream_class_id,omitempty"`
	StreamCameraID string          `json:"stream_camera_id,omitempty"`
	EventType      string          `json:"event_type"`
	RoomID         string          `json:"room_id,omitempty"`
	CameraID       string          `json:"camera_id,omitempty"`
	PersonID       string          `json:"person_id,omitempty"`
	GlobalPersonID *int64          `json:"global_person_id,omitempty"`
	TrackID        *int64          `json:"track_id,omitempty"`
	Confidence     *float64        `json:"confidence,omitempty"`
	Timestamp      *float64        `json:"timestamp,omitempty"`
	Raw            json.RawMessage `json:"raw"`
}

type Summary struct {
	TotalEvents        int64       `json:"total_events"`
	DistinctClasses    int64       `json:"distinct_classes"`
	DistinctCameras    int64       `json:"distinct_cameras"`
	AvgConfidence      float64     `json:"avg_confidence"`
	MinTimestamp       float64     `json:"min_timestamp"`
	MaxTimestamp       float64     `json:"max_timestamp"`
	EventTypeCounts    []CountItem `json:"event_type_counts"`
	StreamClassCounts  []CountItem `json:"stream_class_counts"`
	StreamCameraCounts []CountItem `json:"stream_camera_counts"`
}

type CountItem struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ingested_at TEXT NOT NULL,
  source_file TEXT NOT NULL,
  stream_class_id TEXT,
  stream_camera_id TEXT,
  event_type TEXT,
  room_id TEXT,
  camera_id TEXT,
  person_id TEXT,
  global_person_id INTEGER,
  track_id INTEGER,
  confidence REAL,
  timestamp REAL,
  raw_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_stream_class ON events(stream_class_id);
CREATE INDEX IF NOT EXISTS idx_events_stream_camera ON events(stream_camera_id);
CREATE INDEX IF NOT EXISTS idx_events_camera ON events(camera_id);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
CREATE TABLE IF NOT EXISTS ingested_files (
  path TEXT PRIMARY KEY,
  size_bytes INTEGER NOT NULL,
  mod_unix INTEGER NOT NULL,
  ingested_at TEXT NOT NULL
);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}

func (s *Store) IngestDataset(ds input.Dataset) (int, error) {
	total := 0
	src := "dataset"
	if ds.Stream != nil {
		src = ds.Stream.ConfigPath
	}
	n, err := s.InsertEvents(ds.Events, src)
	if err != nil {
		return 0, err
	}
	total += n
	return total, nil
}

func (s *Store) InsertEvents(events []model.Event, source string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
INSERT INTO events(
  ingested_at, source_file, stream_class_id, stream_camera_id,
  event_type, room_id, camera_id, person_id, global_person_id,
  track_id, confidence, timestamp, raw_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	count := 0
	for _, ev := range events {
		raw, err := json.Marshal(ev.Raw)
		if err != nil {
			return count, fmt.Errorf("marshal event raw: %w", err)
		}

		eventType, _ := ev.String("event_type")
		roomID, _ := ev.String("room_id")
		cameraID, _ := ev.String("camera_id")
		personID, _ := ev.String("person_id")
		streamClassID, _ := ev.String("stream_class_id")
		streamCameraID, _ := ev.String("stream_camera_id")
		globalID, _ := ev.Int64("global_person_id")
		trackID, _ := ev.Int64("track_id")
		confidence, _ := ev.Float64("confidence")
		ts, _ := ev.Float64("timestamp")

		var (
			globalPtr any
			trackPtr  any
			confPtr   any
			tsPtr     any
		)
		if _, ok := ev.Raw["global_person_id"]; ok {
			globalPtr = globalID
		}
		if _, ok := ev.Raw["track_id"]; ok {
			trackPtr = trackID
		}
		if _, ok := ev.Raw["confidence"]; ok {
			confPtr = confidence
		}
		if _, ok := ev.Raw["timestamp"]; ok {
			tsPtr = ts
		}

		if _, err := stmt.Exec(now, source, streamClassID, streamCameraID, eventType, roomID, cameraID, personID, globalPtr, trackPtr, confPtr, tsPtr, string(raw)); err != nil {
			return count, fmt.Errorf("insert event: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("commit tx: %w", err)
	}
	return count, nil
}

func (s *Store) ShouldIngestFile(path string, sizeBytes int64, modUnix int64) (bool, error) {
	var (
		oldSize int64
		oldMod  int64
	)
	err := s.db.QueryRow("SELECT size_bytes, mod_unix FROM ingested_files WHERE path = ?", path).Scan(&oldSize, &oldMod)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check ingested file: %w", err)
	}
	if oldSize == sizeBytes && oldMod == modUnix {
		return false, nil
	}
	return true, nil
}

func (s *Store) MarkFileIngested(path string, sizeBytes int64, modUnix int64) error {
	_, err := s.db.Exec(`INSERT INTO ingested_files(path, size_bytes, mod_unix, ingested_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
  size_bytes = excluded.size_bytes,
  mod_unix = excluded.mod_unix,
  ingested_at = excluded.ingested_at`, path, sizeBytes, modUnix, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("mark file ingested: %w", err)
	}
	return nil
}

func (s *Store) ListEvents(f EventFilter) ([]EventRecord, int64, error) {
	where, args := buildWhere(f)
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	countQuery := "SELECT COUNT(*) FROM events" + where
	var total int64
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}

	query := `SELECT id, ingested_at, source_file, stream_class_id, stream_camera_id, event_type, room_id, camera_id, person_id, global_person_id, track_id, confidence, timestamp, raw_json
FROM events` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	out := make([]EventRecord, 0)
	for rows.Next() {
		var r EventRecord
		var (
			globalID sql.NullInt64
			trackID  sql.NullInt64
			conf     sql.NullFloat64
			ts       sql.NullFloat64
			raw      string
		)
		if err := rows.Scan(&r.ID, &r.IngestedAt, &r.SourceFile, &r.StreamClassID, &r.StreamCameraID, &r.EventType, &r.RoomID, &r.CameraID, &r.PersonID, &globalID, &trackID, &conf, &ts, &raw); err != nil {
			return nil, 0, fmt.Errorf("scan event: %w", err)
		}
		if globalID.Valid {
			v := globalID.Int64
			r.GlobalPersonID = &v
		}
		if trackID.Valid {
			v := trackID.Int64
			r.TrackID = &v
		}
		if conf.Valid {
			v := conf.Float64
			r.Confidence = &v
		}
		if ts.Valid {
			v := ts.Float64
			r.Timestamp = &v
		}
		r.Raw = json.RawMessage(raw)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}
	return out, total, nil
}

func (s *Store) Summary(f EventFilter) (Summary, error) {
	where, args := buildWhere(f)
	var out Summary

	query := `SELECT COUNT(*), COUNT(DISTINCT stream_class_id), COUNT(DISTINCT COALESCE(stream_camera_id, camera_id)),
COALESCE(AVG(confidence),0), COALESCE(MIN(timestamp),0), COALESCE(MAX(timestamp),0)
FROM events` + where
	if err := s.db.QueryRow(query, args...).Scan(&out.TotalEvents, &out.DistinctClasses, &out.DistinctCameras, &out.AvgConfidence, &out.MinTimestamp, &out.MaxTimestamp); err != nil {
		return out, fmt.Errorf("summary totals: %w", err)
	}

	var err error
	if out.EventTypeCounts, err = s.groupCounts("event_type", where, args); err != nil {
		return out, err
	}
	if out.StreamClassCounts, err = s.groupCounts("stream_class_id", where, args); err != nil {
		return out, err
	}
	if out.StreamCameraCounts, err = s.groupCounts("COALESCE(stream_camera_id, camera_id)", where, args); err != nil {
		return out, err
	}

	return out, nil
}

func (s *Store) groupCounts(keyExpr string, where string, args []any) ([]CountItem, error) {
	query := "SELECT " + keyExpr + " AS k, COUNT(*) FROM events" + where + " GROUP BY k ORDER BY COUNT(*) DESC, k ASC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("group counts for %s: %w", keyExpr, err)
	}
	defer rows.Close()

	out := make([]CountItem, 0)
	for rows.Next() {
		var k sql.NullString
		var c int64
		if err := rows.Scan(&k, &c); err != nil {
			return nil, fmt.Errorf("scan group row: %w", err)
		}
		if !k.Valid || strings.TrimSpace(k.String) == "" {
			continue
		}
		out = append(out, CountItem{Key: k.String, Count: c})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group rows: %w", err)
	}
	return out, nil
}

func buildWhere(f EventFilter) (string, []any) {
	clauses := make([]string, 0)
	args := make([]any, 0)

	if len(f.EventTypes) > 0 {
		clauses = append(clauses, "event_type IN ("+placeholders(len(f.EventTypes))+")")
		for _, v := range f.EventTypes {
			args = append(args, v)
		}
	}
	if len(f.ClassIDs) > 0 {
		clauses = append(clauses, "COALESCE(stream_class_id, room_id) IN ("+placeholders(len(f.ClassIDs))+")")
		for _, v := range f.ClassIDs {
			args = append(args, v)
		}
	}
	if len(f.CameraIDs) > 0 {
		clauses = append(clauses, "COALESCE(stream_camera_id, camera_id) IN ("+placeholders(len(f.CameraIDs))+")")
		for _, v := range f.CameraIDs {
			args = append(args, v)
		}
	}
	if f.MinConfidence != nil {
		clauses = append(clauses, "confidence >= ?")
		args = append(args, *f.MinConfidence)
	}
	if f.FromTS != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, *f.FromTS)
	}
	if f.ToTS != nil {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, *f.ToTS)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	items := make([]string, n)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

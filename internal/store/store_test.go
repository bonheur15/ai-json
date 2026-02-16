package store

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"ai-json/internal/model"
)

func TestStoreInsertListSummary(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	events, err := model.ParseEvents([]byte(`[
		{"event_type":"person_tracked","room_id":"class-a","camera_id":"front","stream_class_id":"class-a","stream_camera_id":"front","pipeline":"p","confidence":0.8,"timestamp":10,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1},
		{"event_type":"proximity_event","room_id":"class-a","camera_id":"back","stream_class_id":"class-a","stream_camera_id":"back","pipeline":"p","confidence":0.5,"timestamp":20,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1}
	]`))
	if err != nil {
		t.Fatalf("parse events: %v", err)
	}

	n, err := s.InsertEvents(events, "test.json")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 inserts, got %d", n)
	}

	rows, total, err := s.ListEvents(EventFilter{ClassIDs: []string{"class-a"}, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("expected 2 rows, total=%d len=%d", total, len(rows))
	}

	summary, err := s.Summary(EventFilter{ClassIDs: []string{"class-a"}})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalEvents != 2 {
		t.Fatalf("expected total 2, got %d", summary.TotalEvents)
	}
	if len(summary.StreamCameraCounts) != 2 {
		t.Fatalf("expected 2 camera counts, got %d", len(summary.StreamCameraCounts))
	}
}

func TestStoreGetEventByIDAndDailyMetrics(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	ts := float64(time.Now().UTC().Unix())
	events, err := model.ParseEvents([]byte(`[
		{"event_type":"person_tracked","room_id":"class-a","camera_id":"front","stream_class_id":"class-a","stream_camera_id":"front","pipeline":"p","confidence":0.8,"timestamp":` + strconvF(ts) + `,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1,"person_role":"student","person_id":"s1"},
		{"event_type":"person_tracked","room_id":"class-a","camera_id":"front","stream_class_id":"class-a","stream_camera_id":"front","pipeline":"p","confidence":0.8,"timestamp":` + strconvF(ts) + `,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1,"person_role":"student","person_id":"s2"}
	]`))
	if err != nil {
		t.Fatalf("parse events: %v", err)
	}
	if _, err := s.InsertEvents(events, "test.json"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, _, err := s.ListEvents(EventFilter{Limit: 10})
	if err != nil || len(rows) == 0 {
		t.Fatalf("list events: %v", err)
	}
	got, err := s.GetEventByID(rows[0].ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID == 0 {
		t.Fatalf("expected non-zero id")
	}

	day := time.Unix(int64(ts), 0).UTC()
	start := float64(time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Unix())
	end := float64(time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour).Unix())
	metrics, err := s.DailyStudentMetrics(start, end, []string{"class-a"})
	if err != nil {
		t.Fatalf("daily metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatalf("expected metrics rows")
	}
}

func TestStoreInsertInferenceTypeField(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	events, err := model.ParseEvents([]byte(`[{
		"type":"teacher_engagement",
		"room_id":"class-a",
		"camera_id":"front",
		"stream_class_id":"class-a",
		"stream_camera_id":"front",
		"confidence":0.7,
		"timestamp":123
	}]`))
	if err != nil {
		t.Fatalf("parse events: %v", err)
	}
	if _, err := s.InsertEvents(events, "inference.json"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, total, err := s.ListEvents(EventFilter{EventTypes: []string{"teacher_engagement"}, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected one row, total=%d len=%d", total, len(rows))
	}
	if rows[0].EventType != "teacher_engagement" {
		t.Fatalf("unexpected stored event_type: %q", rows[0].EventType)
	}
}

func strconvF(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

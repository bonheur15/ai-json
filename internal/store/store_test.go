package store

import (
	"path/filepath"
	"testing"

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

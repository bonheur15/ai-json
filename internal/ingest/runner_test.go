package ingest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-json/internal/store"
)

func TestRunOnceSkipsAlreadyIngested(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "c", "front", "images"))
	mustMkdir(t, filepath.Join(root, "c", "back", "images"))
	mustMkdir(t, filepath.Join(root, "c", "front", "events"))
	mustMkdir(t, filepath.Join(root, "c", "back", "events"))
	mustWrite(t, filepath.Join(root, "c", "front", "images", "a.jpg"), []byte("x"))
	mustWrite(t, filepath.Join(root, "c", "back", "images", "b.jpg"), []byte("x"))

	e := `[{"event_type":"person_tracked","room_id":"r","camera_id":"front","pipeline":"p","confidence":0.9,"timestamp":1,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1}]`
	frontFile := filepath.Join(root, "c", "front", "events", "e1.json")
	backFile := filepath.Join(root, "c", "back", "events", "e1.json")
	mustWrite(t, frontFile, []byte(e))
	mustWrite(t, backFile, []byte(e))

	cfg := `{"classes":[{"class_id":"c","base_dir":"c","cameras":[{"id":"front"},{"id":"back"}]}]}`
	cfgPath := filepath.Join(root, "stream.json")
	mustWrite(t, cfgPath, []byte(cfg))

	dbPath := filepath.Join(root, "events.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	r := Runner{Store: st, StreamPath: cfgPath, MinFileAge: 0}
	if _, err := r.RunOnce(); err != nil {
		t.Fatalf("run once first: %v", err)
	}
	stats2, err := r.RunOnce()
	if err != nil {
		t.Fatalf("run once second: %v", err)
	}
	if stats2.ProcessedFiles != 0 {
		t.Fatalf("expected 0 processed files on second run, got %d", stats2.ProcessedFiles)
	}
	_ = os.Chtimes(frontFile, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))
	if _, err := r.RunOnce(); err != nil {
		t.Fatalf("run once after touch: %v", err)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func mustWrite(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

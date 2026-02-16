package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromStreamConfig(t *testing.T) {
	root := t.TempDir()
	classDir := filepath.Join(root, "class-a")
	mustMkdir(t, filepath.Join(classDir, "front", "images"))
	mustMkdir(t, filepath.Join(classDir, "back", "images"))
	mustMkdir(t, filepath.Join(classDir, "front", "events"))
	mustMkdir(t, filepath.Join(classDir, "back", "events"))

	mustWrite(t, filepath.Join(classDir, "front", "images", "f1.jpg"), []byte("x"))
	mustWrite(t, filepath.Join(classDir, "back", "images", "b1.jpg"), []byte("x"))

	eventJSON := `[{"event_type":"person_tracked","room_id":"r","camera_id":"front","pipeline":"p","confidence":0.9,"timestamp":1,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1,"person_id":"unknown:1","global_person_id":1,"track_id":1}]`
	mustWrite(t, filepath.Join(classDir, "front", "events", "e1.json"), []byte(eventJSON))
	mustWrite(t, filepath.Join(classDir, "back", "events", "e1.json"), []byte(eventJSON))

	cfg := `{
  "version": "1",
  "classes": [
    {
      "class_id": "class-a",
      "name": "A",
      "base_dir": "class-a",
      "cameras": [
        {"id":"front"},
        {"id":"back"}
      ]
    }
  ]
}`
	cfgPath := filepath.Join(root, "stream.json")
	mustWrite(t, cfgPath, []byte(cfg))

	ds, err := LoadFromStreamConfig(cfgPath)
	if err != nil {
		t.Fatalf("load stream config: %v", err)
	}
	if ds.Stream == nil {
		t.Fatalf("expected stream summary")
	}
	if ds.Stream.TotalClasses != 1 {
		t.Fatalf("expected 1 class, got %d", ds.Stream.TotalClasses)
	}
	if ds.Stream.TotalImages != 2 {
		t.Fatalf("expected 2 images, got %d", ds.Stream.TotalImages)
	}
	if len(ds.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(ds.Events))
	}

	classID, ok := ds.Events[0].String("stream_class_id")
	if !ok || classID != "class-a" {
		t.Fatalf("missing stream_class_id")
	}
	camID, ok := ds.Events[0].String("stream_camera_id")
	if !ok || (camID != "front" && camID != "back") {
		t.Fatalf("missing stream_camera_id")
	}
}

func TestLoadFromStreamConfigRequiresFrontBack(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "c", "front", "images"))
	mustMkdir(t, filepath.Join(root, "c", "front", "events"))
	mustWrite(t, filepath.Join(root, "c", "front", "images", "x.jpg"), []byte("x"))
	mustWrite(t, filepath.Join(root, "c", "front", "events", "x.json"), []byte(`[{"event_type":"x","room_id":"r","camera_id":"c","pipeline":"p","confidence":1,"timestamp":1,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0,"frame_transport_delay_seconds":0}]`))

	cfg := `{"classes":[{"class_id":"c","base_dir":"c","cameras":[{"id":"front"}]}]}`
	cfgPath := filepath.Join(root, "stream.json")
	mustWrite(t, cfgPath, []byte(cfg))

	if _, err := LoadFromStreamConfig(cfgPath); err == nil {
		t.Fatalf("expected error for missing back camera")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

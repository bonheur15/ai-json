package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromGlob(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "events.json")
	if err := os.WriteFile(p, []byte(`[{"event_type":"x","room_id":"r","camera_id":"c","pipeline":"p","confidence":1,"timestamp":1,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0,"frame_transport_delay_seconds":0}]`), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	ds, err := Load(nil, []string{filepath.Join(dir, "*.json")})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(ds.Files) != 1 || len(ds.Events) != 1 {
		t.Fatalf("unexpected dataset sizes: files=%d events=%d", len(ds.Files), len(ds.Events))
	}
}

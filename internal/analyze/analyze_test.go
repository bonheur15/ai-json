package analyze

import (
	"os"
	"testing"

	"ai-json/internal/model"
)

func TestRunOnSamples(t *testing.T) {
	b, err := os.ReadFile("../../.material/samples/1771233054.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	events, err := model.ParseEvents(b)
	if err != nil {
		t.Fatalf("parse sample: %v", err)
	}

	res := Run(events)
	if res.TotalEvents != len(events) {
		t.Fatalf("total mismatch: got %d want %d", res.TotalEvents, len(events))
	}
	if len(res.EventTypeCounts) == 0 {
		t.Fatalf("expected event types")
	}
	if res.Confidence.Count == 0 {
		t.Fatalf("expected confidence stats")
	}
}

func TestRunFindsInvalidProximity(t *testing.T) {
	data := []byte(`[
	  {
		"event_type":"proximity_event",
		"room_id":"r1",
		"camera_id":"c1",
		"pipeline":"p1",
		"confidence":0.5,
		"timestamp":1,
		"frame_timestamp":1,
		"frame_source_timestamp":1,
		"emitted_at":1,
		"timestamp_offset_seconds":0,
		"timestamp_stabilizer_skew_seconds":0,
		"frame_age_seconds":0,
		"frame_transport_delay_seconds":0,
		"track_ids":[1],
		"global_ids":[1,2],
		"person_ids":["a"],
		"distance":-1,
		"duration_seconds":-2
	  }
	]`)
	events, err := model.ParseEvents(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res := Run(events)
	if res.ErrorCount == 0 {
		t.Fatalf("expected validation errors")
	}
}

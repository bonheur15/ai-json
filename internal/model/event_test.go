package model

import (
	"os"
	"testing"
)

func TestParseEventsFromSampleArray(t *testing.T) {
	b, err := os.ReadFile("../../.material/samples/1771233054.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	events, err := ParseEvents(b)
	if err != nil {
		t.Fatalf("parse events: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected events, got 0")
	}

	common, probs := events[0].ParseCommonFields()
	if len(probs) != 0 {
		t.Fatalf("unexpected common field problems: %v", probs)
	}
	if common.EventType == "" {
		t.Fatalf("expected event_type")
	}
}

func TestParseEventsNDJSON(t *testing.T) {
	input := "{\"event_type\":\"x\",\"room_id\":\"r\",\"camera_id\":\"c\",\"pipeline\":\"p\",\"confidence\":1,\"timestamp\":1,\"frame_timestamp\":1,\"frame_source_timestamp\":1,\"emitted_at\":1,\"timestamp_offset_seconds\":0,\"timestamp_stabilizer_skew_seconds\":0,\"frame_age_seconds\":0,\"frame_transport_delay_seconds\":0}\n"
	events, err := ParseEvents([]byte(input))
	if err != nil {
		t.Fatalf("parse ndjson: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

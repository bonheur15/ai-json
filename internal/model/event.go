package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Event stores one decoded event while keeping all original keys.
type Event struct {
	Raw map[string]any
}

// CommonFields captures normalized fields that appear in nearly all events.
type CommonFields struct {
	EventType                     string
	RoomID                        string
	CameraID                      string
	Pipeline                      string
	Confidence                    float64
	Timestamp                     float64
	FrameTimestamp                float64
	FrameSourceTimestamp          float64
	EmittedAt                     float64
	TimestampOffsetSeconds        float64
	TimestampStabilizerSkewSecond float64
	FrameAgeSeconds               float64
	FrameTransportDelaySeconds    float64
	PersonID                      *string
	GlobalPersonID                *int64
	TrackID                       *int64
}

// ParseEvents decodes one input that can be an array, a single object, or NDJSON.
func ParseEvents(data []byte) ([]Event, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("empty input")
	}

	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]any
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, fmt.Errorf("decode JSON array: %w", err)
		}
		return wrap(arr), nil
	}

	if strings.HasPrefix(trimmed, "{") {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("decode JSON object: %w", err)
		}
		return []Event{{Raw: obj}}, nil
	}

	// Fallback: NDJSON
	lines := strings.Split(trimmed, "\n")
	out := make([]Event, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, fmt.Errorf("decode NDJSON line %d: %w", i+1, err)
		}
		out = append(out, Event{Raw: obj})
	}
	if len(out) == 0 {
		return nil, errors.New("no events decoded")
	}
	return out, nil
}

func wrap(arr []map[string]any) []Event {
	out := make([]Event, 0, len(arr))
	for _, it := range arr {
		out = append(out, Event{Raw: it})
	}
	return out
}

func (e Event) Keys() []string {
	keys := make([]string, 0, len(e.Raw))
	for k := range e.Raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (e Event) Has(key string) bool {
	_, ok := e.Raw[key]
	return ok
}

func (e Event) String(key string) (string, bool) {
	v, ok := e.Raw[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (e Event) NullableString(key string) (*string, bool) {
	v, ok := e.Raw[key]
	if !ok {
		return nil, false
	}
	if v == nil {
		return nil, true
	}
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	return &s, true
}

func (e Event) Float64(key string) (float64, bool) {
	v, ok := e.Raw[key]
	if !ok || v == nil {
		return 0, false
	}
	n, ok := v.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

func (e Event) Int64(key string) (int64, bool) {
	n, ok := e.Float64(key)
	if !ok {
		return 0, false
	}
	if math.Mod(n, 1) != 0 {
		return 0, false
	}
	return int64(n), true
}

func (e Event) NullableInt64(key string) (*int64, bool) {
	v, ok := e.Raw[key]
	if !ok {
		return nil, false
	}
	if v == nil {
		return nil, true
	}
	n, ok := v.(float64)
	if !ok || math.Mod(n, 1) != 0 {
		return nil, false
	}
	i := int64(n)
	return &i, true
}

func (e Event) StringSlice(key string) ([]string, bool) {
	v, ok := e.Raw[key]
	if !ok || v == nil {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func (e Event) Int64Slice(key string) ([]int64, bool) {
	v, ok := e.Raw[key]
	if !ok || v == nil {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]int64, 0, len(arr))
	for _, item := range arr {
		n, ok := item.(float64)
		if !ok || math.Mod(n, 1) != 0 {
			return nil, false
		}
		out = append(out, int64(n))
	}
	return out, true
}

func (e Event) ParseCommonFields() (CommonFields, []string) {
	var (
		c        CommonFields
		problems []string
	)

	var ok bool
	if c.EventType, ok = e.String("event_type"); !ok || c.EventType == "" {
		problems = append(problems, "missing event_type")
	}
	if c.RoomID, ok = e.String("room_id"); !ok || c.RoomID == "" {
		problems = append(problems, "missing room_id")
	}
	if c.CameraID, ok = e.String("camera_id"); !ok || c.CameraID == "" {
		problems = append(problems, "missing camera_id")
	}
	if c.Pipeline, ok = e.String("pipeline"); !ok || c.Pipeline == "" {
		problems = append(problems, "missing pipeline")
	}

	readFloat := func(key string, dst *float64) {
		v, ok := e.Float64(key)
		if !ok {
			problems = append(problems, "invalid or missing "+key)
			return
		}
		*dst = v
	}

	readFloat("confidence", &c.Confidence)
	readFloat("timestamp", &c.Timestamp)
	readFloat("frame_timestamp", &c.FrameTimestamp)
	readFloat("frame_source_timestamp", &c.FrameSourceTimestamp)
	readFloat("emitted_at", &c.EmittedAt)
	readFloat("timestamp_offset_seconds", &c.TimestampOffsetSeconds)
	readFloat("timestamp_stabilizer_skew_seconds", &c.TimestampStabilizerSkewSecond)
	readFloat("frame_age_seconds", &c.FrameAgeSeconds)
	readFloat("frame_transport_delay_seconds", &c.FrameTransportDelaySeconds)

	if personID, ok := e.NullableString("person_id"); ok {
		c.PersonID = personID
	}
	if globalID, ok := e.NullableInt64("global_person_id"); ok {
		c.GlobalPersonID = globalID
	}
	if trackID, ok := e.NullableInt64("track_id"); ok {
		c.TrackID = trackID
	}

	return c, problems
}

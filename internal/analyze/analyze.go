package analyze

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"ai-json/internal/model"
)

type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
)

type ValidationIssue struct {
	Severity   Severity `json:"severity"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	EventIndex int      `json:"event_index"`
	EventType  string   `json:"event_type"`
}

type StatSummary struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
}

type KeyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type PairCount struct {
	Pair  string `json:"pair"`
	Count int    `json:"count"`
}

type Analysis struct {
	TotalEvents          int               `json:"total_events"`
	EventTypeCounts      []KeyCount        `json:"event_type_counts"`
	RoomCounts           []KeyCount        `json:"room_counts"`
	CameraCounts         []KeyCount        `json:"camera_counts"`
	RoleCounts           []KeyCount        `json:"role_counts"`
	OrientationCounts    []KeyCount        `json:"orientation_counts"`
	TopPersonEventCounts []KeyCount        `json:"top_person_event_counts"`
	TopProximityPairs    []PairCount       `json:"top_proximity_pairs"`
	UniquePersonIDs      int               `json:"unique_person_ids"`
	UniqueGlobalIDs      int               `json:"unique_global_ids"`
	UniqueTrackIDs       int               `json:"unique_track_ids"`
	Confidence           StatSummary       `json:"confidence"`
	FrameAgeSeconds      StatSummary       `json:"frame_age_seconds"`
	TransportDelay       StatSummary       `json:"transport_delay_seconds"`
	TimestampOffset      StatSummary       `json:"timestamp_offset_seconds"`
	StabilizerSkew       StatSummary       `json:"timestamp_stabilizer_skew_seconds"`
	Issues               []ValidationIssue `json:"issues"`
	ErrorCount           int               `json:"error_count"`
	WarningCount         int               `json:"warning_count"`
}

func Run(events []model.Event) Analysis {
	typeCounts := map[string]int{}
	roomCounts := map[string]int{}
	cameraCounts := map[string]int{}
	roleCounts := map[string]int{}
	orientationCounts := map[string]int{}
	personEventCounts := map[string]int{}
	proximityCounts := map[string]int{}
	uniquePerson := map[string]struct{}{}
	uniqueGlobal := map[int64]struct{}{}
	uniqueTrack := map[int64]struct{}{}
	issues := make([]ValidationIssue, 0)

	conf := make([]float64, 0, len(events))
	frameAge := make([]float64, 0, len(events))
	delay := make([]float64, 0, len(events))
	offset := make([]float64, 0, len(events))
	skew := make([]float64, 0, len(events))

	for i, ev := range events {
		common, commonProblems := ev.ParseCommonFields()

		eventType := common.EventType
		if eventType == "" {
			eventType = "unknown"
		}
		typeCounts[eventType]++

		if common.RoomID != "" {
			roomCounts[common.RoomID]++
		}
		if common.CameraID != "" {
			cameraCounts[common.CameraID]++
		}

		for _, p := range commonProblems {
			issues = append(issues, ValidationIssue{
				Severity:   SeverityError,
				Code:       "missing_common_field",
				Message:    p,
				EventIndex: i,
				EventType:  eventType,
			})
		}

		if common.Confidence < 0 || common.Confidence > 1 {
			issues = append(issues, ValidationIssue{Severity: SeverityWarn, Code: "confidence_out_of_range", Message: fmt.Sprintf("confidence %.3f out of [0,1]", common.Confidence), EventIndex: i, EventType: eventType})
		}

		if common.PersonID != nil && *common.PersonID != "" {
			uniquePerson[*common.PersonID] = struct{}{}
			personEventCounts[*common.PersonID]++
		}
		if common.GlobalPersonID != nil {
			uniqueGlobal[*common.GlobalPersonID] = struct{}{}
		}
		if common.TrackID != nil {
			uniqueTrack[*common.TrackID] = struct{}{}
		}

		if c, ok := ev.Float64("confidence"); ok {
			conf = append(conf, c)
		}
		if v, ok := ev.Float64("frame_age_seconds"); ok {
			frameAge = append(frameAge, v)
			if v < 0 {
				issues = append(issues, ValidationIssue{Severity: SeverityError, Code: "negative_frame_age", Message: "frame_age_seconds must be >= 0", EventIndex: i, EventType: eventType})
			}
		}
		if v, ok := ev.Float64("frame_transport_delay_seconds"); ok {
			delay = append(delay, v)
			if v < 0 {
				issues = append(issues, ValidationIssue{Severity: SeverityError, Code: "negative_transport_delay", Message: "frame_transport_delay_seconds must be >= 0", EventIndex: i, EventType: eventType})
			}
		}
		if v, ok := ev.Float64("timestamp_offset_seconds"); ok {
			offset = append(offset, v)
		}
		if v, ok := ev.Float64("timestamp_stabilizer_skew_seconds"); ok {
			skew = append(skew, v)
		}

		if role, ok := ev.String("role"); ok && role != "" {
			roleCounts[role]++
		}
		if orientation, ok := ev.String("orientation"); ok && orientation != "" {
			orientationCounts[orientation]++
		}

		validateShape(ev, eventType, i, &issues)
		validateEventSpecific(ev, eventType, i, &issues)

		if eventType == "proximity_event" {
			pairKeys := proximityPairKeys(ev)
			for _, k := range pairKeys {
				proximityCounts[k]++
			}
		}
	}

	errCount := 0
	warnCount := 0
	for _, issue := range issues {
		switch issue.Severity {
		case SeverityError:
			errCount++
		case SeverityWarn:
			warnCount++
		}
	}

	return Analysis{
		TotalEvents:          len(events),
		EventTypeCounts:      toKeyCounts(typeCounts),
		RoomCounts:           toKeyCounts(roomCounts),
		CameraCounts:         toKeyCounts(cameraCounts),
		RoleCounts:           toKeyCounts(roleCounts),
		OrientationCounts:    toKeyCounts(orientationCounts),
		TopPersonEventCounts: limitKeyCounts(toKeyCounts(personEventCounts), 10),
		TopProximityPairs:    limitPairCounts(toPairCounts(proximityCounts), 10),
		UniquePersonIDs:      len(uniquePerson),
		UniqueGlobalIDs:      len(uniqueGlobal),
		UniqueTrackIDs:       len(uniqueTrack),
		Confidence:           summarize(conf),
		FrameAgeSeconds:      summarize(frameAge),
		TransportDelay:       summarize(delay),
		TimestampOffset:      summarize(offset),
		StabilizerSkew:       summarize(skew),
		Issues:               issues,
		ErrorCount:           errCount,
		WarningCount:         warnCount,
	}
}

func validateShape(ev model.Event, eventType string, idx int, issues *[]ValidationIssue) {
	if bbox, ok := ev.Raw["bbox"]; ok {
		arr, ok := bbox.([]any)
		if !ok || len(arr) != 4 {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_bbox", Message: "bbox must contain 4 numbers", EventIndex: idx, EventType: eventType})
			return
		}
		vals := make([]float64, 0, 4)
		for _, item := range arr {
			n, ok := item.(float64)
			if !ok {
				*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_bbox", Message: "bbox values must be numeric", EventIndex: idx, EventType: eventType})
				return
			}
			vals = append(vals, n)
		}
		if vals[0] >= vals[2] || vals[1] >= vals[3] {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_bbox_order", Message: "bbox requires x1<x2 and y1<y2", EventIndex: idx, EventType: eventType})
		}
	}
}

func validateEventSpecific(ev model.Event, eventType string, idx int, issues *[]ValidationIssue) {
	personEvents := map[string]bool{
		"person_tracked":           true,
		"person_detected":          true,
		"person_lost":              true,
		"head_orientation_changed": true,
		"posture_changed":          true,
		"sleeping_suspected":       true,
		"role_assigned":            true,
	}

	if personEvents[eventType] {
		if _, ok := ev.NullableInt64("track_id"); !ok {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "missing_track_id", Message: "person event missing track_id", EventIndex: idx, EventType: eventType})
		}
		if _, ok := ev.NullableString("person_id"); !ok {
			*issues = append(*issues, ValidationIssue{Severity: SeverityWarn, Code: "invalid_person_id", Message: "person_id has invalid type", EventIndex: idx, EventType: eventType})
		}
	}

	if eventType == "proximity_event" {
		tIDs, ok1 := ev.Int64Slice("track_ids")
		gIDs, ok2 := ev.Int64Slice("global_ids")
		pIDs, ok3 := ev.StringSlice("person_ids")
		if !ok1 || !ok2 || !ok3 || len(tIDs) == 0 {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_proximity_ids", Message: "proximity_event requires non-empty track_ids/global_ids/person_ids", EventIndex: idx, EventType: eventType})
		}
		if ok1 && ok2 && len(tIDs) != len(gIDs) {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "proximity_id_length_mismatch", Message: "track_ids and global_ids length mismatch", EventIndex: idx, EventType: eventType})
		}
		if ok1 && ok3 && len(tIDs) != len(pIDs) {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "proximity_person_length_mismatch", Message: "track_ids and person_ids length mismatch", EventIndex: idx, EventType: eventType})
		}
		if d, ok := ev.Float64("distance"); !ok || d < 0 {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_distance", Message: "distance must be >= 0", EventIndex: idx, EventType: eventType})
		}
		if d, ok := ev.Float64("duration_seconds"); !ok || d < 0 {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_duration", Message: "duration_seconds must be >= 0", EventIndex: idx, EventType: eventType})
		}
	}

	if eventType == "frame_tick" {
		if v, ok := ev.Float64("detections_count"); !ok || v < 0 || math.Mod(v, 1) != 0 {
			*issues = append(*issues, ValidationIssue{Severity: SeverityError, Code: "invalid_detections_count", Message: "detections_count must be a non-negative integer", EventIndex: idx, EventType: eventType})
		}
	}
}

func proximityPairKeys(ev model.Event) []string {
	trackIDs, ok := ev.Int64Slice("track_ids")
	if !ok || len(trackIDs) < 2 {
		return nil
	}
	pairs := make([]string, 0)
	for i := 0; i < len(trackIDs)-1; i++ {
		for j := i + 1; j < len(trackIDs); j++ {
			a := trackIDs[i]
			b := trackIDs[j]
			if a > b {
				a, b = b, a
			}
			pairs = append(pairs, fmt.Sprintf("%d-%d", a, b))
		}
	}
	return pairs
}

func summarize(values []float64) StatSummary {
	if len(values) == 0 {
		return StatSummary{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	return StatSummary{
		Count: len(sorted),
		Min:   sorted[0],
		Max:   sorted[len(sorted)-1],
		Avg:   sum / float64(len(sorted)),
		P50:   quantile(sorted, 0.50),
		P95:   quantile(sorted, 0.95),
	}
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

func toKeyCounts(in map[string]int) []KeyCount {
	out := make([]KeyCount, 0, len(in))
	for k, v := range in {
		out = append(out, KeyCount{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return strings.Compare(out[i].Key, out[j].Key) < 0
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func toPairCounts(in map[string]int) []PairCount {
	out := make([]PairCount, 0, len(in))
	for k, v := range in {
		out = append(out, PairCount{Pair: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return strings.Compare(out[i].Pair, out[j].Pair) < 0
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func limitKeyCounts(in []KeyCount, n int) []KeyCount {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func limitPairCounts(in []PairCount, n int) []PairCount {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

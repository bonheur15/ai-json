package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ai-json/internal/ingest"
	"ai-json/internal/media"
	"ai-json/internal/model"
	"ai-json/internal/store"
)

type Server struct {
	Store             *store.Store
	DefaultStream     string
	DefaultMinAge     time.Duration
	DefaultMaxPastAge time.Duration
}

func New(s *store.Store) *Server {
	return &Server{Store: s, DefaultStream: "stream.json", DefaultMinAge: 2 * time.Second, DefaultMaxPastAge: 1 * time.Minute}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/ingest/events", s.handleIngestEvents)
	mux.HandleFunc("/v1/ingest/stream", s.handleIngestStream)
	mux.HandleFunc("/v1/events", s.handleListEvents)
	mux.HandleFunc("/v1/special-events", s.handleSpecialEvents)
	mux.HandleFunc("/v1/special-events-with-images", s.handleSpecialEventsWithImages)
	mux.HandleFunc("/v1/event-images", s.handleEventImages)
	mux.HandleFunc("/v1/image", s.handleImage)
	mux.HandleFunc("/v1/student-metrics/daily", s.handleStudentDailyMetrics)
	mux.HandleFunc("/v1/summary", s.handleSummary)
	return withHeaders(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) handleIngestEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST allowed")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body_failed", err.Error())
		return
	}
	events, err := model.ParseEvents(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_events_payload", err.Error())
		return
	}

	classID := strings.TrimSpace(r.URL.Query().Get("class_id"))
	cameraID := strings.TrimSpace(r.URL.Query().Get("camera_id"))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if source == "" {
		source = "api:/v1/ingest/events"
	}

	for i := range events {
		if classID != "" {
			events[i].Raw["stream_class_id"] = classID
		}
		if cameraID != "" {
			events[i].Raw["stream_camera_id"] = cameraID
		}
	}

	n, err := s.Store.InsertEvents(events, source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"inserted": n,
		"source":   source,
	})
}

func (s *Server) handleIngestStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST allowed")
		return
	}

	streamPath := strings.TrimSpace(r.URL.Query().Get("stream_path"))
	if streamPath == "" {
		streamPath = s.DefaultStream
	}
	minAge := s.DefaultMinAge
	if v := strings.TrimSpace(r.URL.Query().Get("min_file_age_seconds")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid_min_file_age_seconds", "min_file_age_seconds must be >= 0")
			return
		}
		minAge = time.Duration(n) * time.Second
	}
	maxPast := s.DefaultMaxPastAge
	if v := strings.TrimSpace(r.URL.Query().Get("max_past_seconds")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_max_past_seconds", "max_past_seconds must be > 0")
			return
		}
		maxPast = time.Duration(n) * time.Second
	}

	runner := ingest.Runner{Store: s.Store, StreamPath: streamPath, MinFileAge: minAge, MaxPastAge: maxPast}
	stats, err := runner.RunOnce()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stream_ingest_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"inserted":         stats.InsertedEvents,
		"processed_files":  stats.ProcessedFiles,
		"skipped_files":    stats.SkippedFiles,
		"stream_path":      streamPath,
		"max_past_seconds": int(maxPast.Seconds()),
	})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	rows, total, err := s.Store.ListEvents(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
		"events": rows,
	})
}

func (s *Server) handleSpecialEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	eventTypes := splitCSV(r.URL.Query().Get("event_types"))
	if len(eventTypes) == 0 {
		eventTypes = defaultSpecialEventTypes()
	}
	filter.EventTypes = eventTypes

	dayStart, dayEnd, err := parseDayRange(r.URL.Query().Get("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}
	filter.FromTS = &dayStart
	filter.ToTS = &dayEnd

	events, total, err := s.Store.ListEvents(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":        time.Unix(int64(dayStart), 0).UTC().Format("2006-01-02"),
		"event_types": eventTypes,
		"total":       total,
		"limit":       filter.Limit,
		"offset":      filter.Offset,
		"events":      events,
	})
}

func (s *Server) handleSpecialEventsWithImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	window := 5
	if v := strings.TrimSpace(r.URL.Query().Get("window_seconds")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 120 {
			writeError(w, http.StatusBadRequest, "invalid_window_seconds", "window_seconds must be 0..120")
			return
		}
		window = n
	}
	streamPath := strings.TrimSpace(r.URL.Query().Get("stream_path"))
	if streamPath == "" {
		streamPath = s.DefaultStream
	}
	resolver, err := media.NewStreamImageResolver(streamPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "stream_resolve_failed", err.Error())
		return
	}

	eventTypes := splitCSV(r.URL.Query().Get("event_types"))
	if len(eventTypes) == 0 {
		eventTypes = defaultSpecialEventTypes()
	}
	filter.EventTypes = eventTypes

	dayStart, dayEnd, err := parseDayRange(r.URL.Query().Get("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}
	filter.FromTS = &dayStart
	filter.ToTS = &dayEnd

	events, total, err := s.Store.ListEvents(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}

	type item struct {
		Event  store.EventRecord        `json:"event"`
		Images []media.ImageContextItem `json:"images"`
	}
	out := make([]item, 0, len(events))
	for _, ev := range events {
		classID := firstNonEmpty(ev.StreamClassID, ev.RoomID)
		cameraID := firstNonEmpty(ev.StreamCameraID, ev.CameraID)
		eventTS := int64(0)
		if ev.Timestamp != nil {
			eventTS = int64(*ev.Timestamp)
		}
		ctx := resolver.BuildContext(classID, cameraID, eventTS, window, "/v1/image")
		out = append(out, item{Event: ev, Images: ctx})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":           time.Unix(int64(dayStart), 0).UTC().Format("2006-01-02"),
		"event_types":    eventTypes,
		"total":          total,
		"limit":          filter.Limit,
		"offset":         filter.Offset,
		"window_seconds": window,
		"events":         out,
	})
}

func (s *Server) handleEventImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	eventID, err := parseInt64Required(r.URL.Query().Get("event_id"), "event_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event_id", err.Error())
		return
	}
	window := 5
	if v := strings.TrimSpace(r.URL.Query().Get("window_seconds")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 120 {
			writeError(w, http.StatusBadRequest, "invalid_window_seconds", "window_seconds must be 0..120")
			return
		}
		window = n
	}
	streamPath := strings.TrimSpace(r.URL.Query().Get("stream_path"))
	if streamPath == "" {
		streamPath = s.DefaultStream
	}

	ev, err := s.Store.GetEventByID(eventID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_lookup_failed", err.Error())
		return
	}
	if ev.Timestamp == nil {
		writeError(w, http.StatusBadRequest, "event_without_timestamp", "event has no timestamp")
		return
	}
	classID := firstNonEmpty(ev.StreamClassID, ev.RoomID)
	cameraID := firstNonEmpty(ev.StreamCameraID, ev.CameraID)

	resolver, err := media.NewStreamImageResolver(streamPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "stream_resolve_failed", err.Error())
		return
	}
	ctx := resolver.BuildContext(classID, cameraID, int64(*ev.Timestamp), window, "/v1/image")
	writeJSON(w, http.StatusOK, map[string]any{
		"event_id":       ev.ID,
		"class_id":       classID,
		"camera_id":      cameraID,
		"event_ts":       int64(*ev.Timestamp),
		"window_seconds": window,
		"images":         ctx,
	})
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	classID := strings.TrimSpace(r.URL.Query().Get("class_id"))
	cameraID := strings.TrimSpace(r.URL.Query().Get("camera_id"))
	ts, err := parseInt64Required(r.URL.Query().Get("ts"), "ts")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ts", err.Error())
		return
	}
	if err := media.ValidateImageRequest(classID, cameraID, ts); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image_request", err.Error())
		return
	}
	streamPath := strings.TrimSpace(r.URL.Query().Get("stream_path"))
	if streamPath == "" {
		streamPath = s.DefaultStream
	}
	resolver, err := media.NewStreamImageResolver(streamPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "stream_resolve_failed", err.Error())
		return
	}
	path, ok := resolver.ResolveImagePath(classID, cameraID, ts)
	if !ok {
		writeError(w, http.StatusNotFound, "image_not_found", "image file does not exist for requested timestamp")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "image_open_failed", err.Error())
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "image/jpeg")
	if st != nil {
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
		return
	}
	_, _ = io.Copy(w, f)
}

func (s *Server) handleStudentDailyMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	dayStart, dayEnd, err := parseDayRange(r.URL.Query().Get("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}
	classIDs := splitCSV(r.URL.Query().Get("class_ids"))
	metrics, err := s.Store.DailyStudentMetrics(dayStart, dayEnd, classIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "daily_metrics_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":    time.Unix(int64(dayStart), 0).UTC().Format("2006-01-02"),
		"metrics": metrics,
	})
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	summary, err := s.Store.Summary(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "summary_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

func parseFilter(r *http.Request) (store.EventFilter, error) {
	q := r.URL.Query()
	f := store.EventFilter{
		EventTypes: splitCSV(q.Get("event_types")),
		ClassIDs:   splitCSV(q.Get("class_ids")),
		CameraIDs:  splitCSV(q.Get("camera_ids")),
		Limit:      200,
		Offset:     0,
	}
	if v := strings.TrimSpace(q.Get("min_confidence")); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return f, fmt.Errorf("invalid min_confidence")
		}
		f.MinConfidence = &n
	}
	if v := strings.TrimSpace(q.Get("from_ts")); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return f, fmt.Errorf("invalid from_ts")
		}
		f.FromTS = &n
	}
	if v := strings.TrimSpace(q.Get("to_ts")); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return f, fmt.Errorf("invalid to_ts")
		}
		f.ToTS = &n
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, fmt.Errorf("invalid limit")
		}
		f.Limit = n
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, fmt.Errorf("invalid offset")
		}
		f.Offset = n
	}
	return f, nil
}

func parseDayRange(dateParam string) (float64, float64, error) {
	var day time.Time
	if strings.TrimSpace(dateParam) == "" {
		now := time.Now().UTC()
		day = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(dateParam))
		if err != nil {
			return 0, 0, fmt.Errorf("date must be YYYY-MM-DD")
		}
		day = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	start := float64(day.Unix())
	end := float64(day.Add(24 * time.Hour).Unix())
	return start, end, nil
}

func parseInt64Required(raw string, name string) (int64, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return n, nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func defaultSpecialEventTypes() []string {
	return []string{"sleeping_suspected", "posture_changed", "proximity_event", "role_assigned"}
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

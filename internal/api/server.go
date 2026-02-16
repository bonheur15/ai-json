package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-json/internal/ingest"
	"ai-json/internal/input"
	"ai-json/internal/model"
	"ai-json/internal/store"
)

type Server struct {
	Store         *store.Store
	DefaultStream string
	DefaultMinAge time.Duration
}

func New(s *store.Store) *Server {
	return &Server{Store: s, DefaultStream: "stream.json", DefaultMinAge: 2 * time.Second}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/ingest/events", s.handleIngestEvents)
	mux.HandleFunc("/v1/ingest/stream", s.handleIngestStream)
	mux.HandleFunc("/v1/events", s.handleListEvents)
	mux.HandleFunc("/v1/summary", s.handleSummary)
	return withJSONHeaders(mux)
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

	runner := ingest.Runner{Store: s.Store, StreamPath: streamPath, MinFileAge: minAge}
	stats, err := runner.RunOnce()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stream_ingest_failed", err.Error())
		return
	}
	resolved, err := input.ResolveStreamConfig(streamPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "stream_load_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"inserted":        stats.InsertedEvents,
		"processed_files": stats.ProcessedFiles,
		"skipped_files":   stats.SkippedFiles,
		"stream_path":     resolved.ConfigPath,
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

func withJSONHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
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

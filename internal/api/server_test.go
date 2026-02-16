package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ai-json/internal/store"
)

func TestHealth(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestIngestAndQuery(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()

	payload := []byte(`[{"event_type":"person_tracked","room_id":"class-a","camera_id":"front","pipeline":"p1","confidence":0.9,"timestamp":1,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1}]`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events?class_id=class-a&camera_id=front", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ingest status: %d body=%s", rr.Code, rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/events?class_ids=class-a&camera_ids=front", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("events status: %d body=%s", rr2.Code, rr2.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if resp["total"].(float64) < 1 {
		t.Fatalf("expected total >= 1")
	}
}

func testServer(t *testing.T) (*Server, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "api.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return New(st), func() { _ = st.Close() }
}

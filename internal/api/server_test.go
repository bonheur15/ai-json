package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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

func TestSpecialEventsAndEventImagesAndImageServe(t *testing.T) {
	root := t.TempDir()
	classDir := filepath.Join(root, "class-a")
	mustMkdir(t, filepath.Join(classDir, "front", "images"))
	mustMkdir(t, filepath.Join(classDir, "back", "images"))
	mustMkdir(t, filepath.Join(classDir, "front", "events"))
	mustMkdir(t, filepath.Join(classDir, "back", "events"))

	nowTs := time.Now().UTC().Unix()
	mustWrite(t, filepath.Join(classDir, "front", "images", strconvI(nowTs)+".jpg"), []byte("img"))
	mustWrite(t, filepath.Join(classDir, "back", "images", strconvI(nowTs)+".jpg"), []byte("img"))

	cfg := `{"classes":[{"class_id":"class-a","base_dir":"class-a","cameras":[{"id":"front"},{"id":"back"}]}]}`
	cfgPath := filepath.Join(root, "stream.json")
	mustWrite(t, cfgPath, []byte(cfg))

	st, err := store.Open(filepath.Join(root, "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	s := New(st)
	s.DefaultStream = cfgPath

	payload := []byte(`[{"event_type":"sleeping_suspected","room_id":"class-a","camera_id":"front","pipeline":"p1","confidence":0.9,"timestamp":` + strconvI(nowTs) + `,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1,"person_role":"student"}]`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events?class_id=class-a&camera_id=front", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ingest status: %d body=%s", rr.Code, rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/special-events", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("special events status: %d body=%s", rr2.Code, rr2.Body.String())
	}
	var specials map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &specials); err != nil {
		t.Fatalf("decode specials: %v", err)
	}
	evs := specials["events"].([]any)
	if len(evs) == 0 {
		t.Fatalf("expected at least one special event")
	}
	eventID := int64(evs[0].(map[string]any)["id"].(float64))

	req3 := httptest.NewRequest(http.MethodGet, "/v1/event-images?event_id="+strconvI(eventID)+"&window_seconds=5", nil)
	rr3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("event images status: %d body=%s", rr3.Code, rr3.Body.String())
	}
	var imagesResp map[string]any
	if err := json.Unmarshal(rr3.Body.Bytes(), &imagesResp); err != nil {
		t.Fatalf("decode event images: %v", err)
	}
	imgs := imagesResp["images"].([]any)
	if len(imgs) != 11 {
		t.Fatalf("expected 11 context images, got %d", len(imgs))
	}

	req4 := httptest.NewRequest(http.MethodGet, "/v1/image?class_id=class-a&camera_id=front&ts="+strconvI(nowTs)+"&stream_path="+cfgPath, nil)
	rr4 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusOK {
		t.Fatalf("image serve status: %d body=%s", rr4.Code, rr4.Body.String())
	}

	req5 := httptest.NewRequest(http.MethodGet, "/v1/special-events-with-images?window_seconds=5&stream_path="+cfgPath, nil)
	rr5 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr5, req5)
	if rr5.Code != http.StatusOK {
		t.Fatalf("special-events-with-images status: %d body=%s", rr5.Code, rr5.Body.String())
	}
	var enriched map[string]any
	if err := json.Unmarshal(rr5.Body.Bytes(), &enriched); err != nil {
		t.Fatalf("decode special-events-with-images: %v", err)
	}
	enrichedEvents := enriched["events"].([]any)
	if len(enrichedEvents) == 0 {
		t.Fatalf("expected enriched special events")
	}
	first := enrichedEvents[0].(map[string]any)
	context := first["images"].([]any)
	if len(context) != 11 {
		t.Fatalf("expected 11 context entries, got %d", len(context))
	}
}

func TestStudentDailyMetrics(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	ts := float64(time.Now().UTC().Unix())
	payload := []byte(`[
		{"event_type":"person_tracked","room_id":"class-a","camera_id":"front","pipeline":"p1","confidence":0.9,"timestamp":` + strconvF(ts) + `,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1,"person_role":"student","person_id":"s1"},
		{"event_type":"person_tracked","room_id":"class-a","camera_id":"front","pipeline":"p1","confidence":0.9,"timestamp":` + strconvF(ts) + `,"frame_timestamp":1,"frame_source_timestamp":1,"emitted_at":1,"timestamp_offset_seconds":0,"timestamp_stabilizer_skew_seconds":0,"frame_age_seconds":0.1,"frame_transport_delay_seconds":0.1,"person_role":"student","person_id":"s2"}
	]`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events?class_id=class-a&camera_id=front", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ingest status: %d", rr.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/student-metrics/daily?class_ids=class-a", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("metrics status: %d body=%s", rr2.Code, rr2.Body.String())
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

func strconvI(v int64) string   { return strconv.FormatInt(v, 10) }
func strconvF(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

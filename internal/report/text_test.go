package report

import (
	"strings"
	"testing"

	"ai-json/internal/analyze"
	"ai-json/internal/input"
)

func TestRenderTextIncludesHeadline(t *testing.T) {
	result := analyze.Analysis{TotalEvents: 2}
	out := RenderText(result, []string{"a.json"}, nil, 10)
	if !strings.Contains(out, "AI JSON Analysis Report") {
		t.Fatalf("missing report headline")
	}
	if !strings.Contains(out, "Total Events: 2") {
		t.Fatalf("missing total events")
	}
}

func TestRenderTextIncludesStreamInventory(t *testing.T) {
	result := analyze.Analysis{TotalEvents: 2}
	stream := &input.StreamSummary{
		ConfigPath:      "/tmp/stream.json",
		TotalClasses:    1,
		TotalImages:     20,
		TotalEventFiles: 2,
		Classes: []input.ClassSummary{{
			ClassID: "class-a",
			Name:    "A",
			Cameras: []input.CameraSummary{{ID: "front", ImageCount: 10, EventFileCount: 1, EventCount: 100}},
		}},
	}
	out := RenderText(result, []string{"a.json"}, stream, 10)
	if !strings.Contains(out, "Stream Inventory") {
		t.Fatalf("missing stream inventory")
	}
	if !strings.Contains(out, "class-a") {
		t.Fatalf("missing class id")
	}
}

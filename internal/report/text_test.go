package report

import (
	"strings"
	"testing"

	"ai-json/internal/analyze"
)

func TestRenderTextIncludesHeadline(t *testing.T) {
	result := analyze.Analysis{TotalEvents: 2}
	out := RenderText(result, []string{"a.json"}, 10)
	if !strings.Contains(out, "AI JSON Analysis Report") {
		t.Fatalf("missing report headline")
	}
	if !strings.Contains(out, "Total Events: 2") {
		t.Fatalf("missing total events")
	}
}

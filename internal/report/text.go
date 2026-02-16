package report

import (
	"fmt"
	"strings"

	"ai-json/internal/analyze"
	"ai-json/internal/input"
)

func RenderText(result analyze.Analysis, files []string, stream *input.StreamSummary, maxIssues int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "AI JSON Analysis Report\n")
	fmt.Fprintf(&b, "=======================\n")
	fmt.Fprintf(&b, "Files: %d\n", len(files))
	for _, f := range files {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	fmt.Fprintf(&b, "\n")

	if stream != nil {
		fmt.Fprintf(&b, "Stream Inventory\n")
		fmt.Fprintf(&b, "- Config: %s\n", stream.ConfigPath)
		fmt.Fprintf(&b, "- Classes: %d | Images: %d | Event Files: %d\n", stream.TotalClasses, stream.TotalImages, stream.TotalEventFiles)
		for _, cls := range stream.Classes {
			fmt.Fprintf(&b, "- Class %s (%s)\n", cls.ClassID, cls.Name)
			for _, cam := range cls.Cameras {
				fmt.Fprintf(&b, "  camera=%s images=%d event_files=%d events=%d\n", cam.ID, cam.ImageCount, cam.EventFileCount, cam.EventCount)
			}
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "Total Events: %d\n", result.TotalEvents)
	fmt.Fprintf(&b, "Unique Person IDs: %d | Global IDs: %d | Track IDs: %d\n", result.UniquePersonIDs, result.UniqueGlobalIDs, result.UniqueTrackIDs)
	fmt.Fprintf(&b, "Validation: %d errors, %d warnings\n", result.ErrorCount, result.WarningCount)
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "Event Types\n")
	for _, c := range result.EventTypeCounts {
		fmt.Fprintf(&b, "- %-28s %d\n", c.Key, c.Count)
	}
	fmt.Fprintf(&b, "\n")

	if len(result.StreamClassCounts) > 0 {
		fmt.Fprintf(&b, "Stream Classes\n")
		for _, c := range result.StreamClassCounts {
			fmt.Fprintf(&b, "- %-28s %d\n", c.Key, c.Count)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(result.StreamCameraCounts) > 0 {
		fmt.Fprintf(&b, "Stream Cameras\n")
		for _, c := range result.StreamCameraCounts {
			fmt.Fprintf(&b, "- %-28s %d\n", c.Key, c.Count)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "Confidence Stats (min/avg/p50/p95/max)\n")
	fmt.Fprintf(&b, "- %.4f / %.4f / %.4f / %.4f / %.4f\n", result.Confidence.Min, result.Confidence.Avg, result.Confidence.P50, result.Confidence.P95, result.Confidence.Max)
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "Frame Age Seconds (min/avg/p50/p95/max)\n")
	fmt.Fprintf(&b, "- %.4f / %.4f / %.4f / %.4f / %.4f\n", result.FrameAgeSeconds.Min, result.FrameAgeSeconds.Avg, result.FrameAgeSeconds.P50, result.FrameAgeSeconds.P95, result.FrameAgeSeconds.Max)
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "Transport Delay Seconds (min/avg/p50/p95/max)\n")
	fmt.Fprintf(&b, "- %.4f / %.4f / %.4f / %.4f / %.4f\n", result.TransportDelay.Min, result.TransportDelay.Avg, result.TransportDelay.P50, result.TransportDelay.P95, result.TransportDelay.Max)
	fmt.Fprintf(&b, "\n")

	if len(result.TopPersonEventCounts) > 0 {
		fmt.Fprintf(&b, "Top Person Event Counts\n")
		for _, c := range result.TopPersonEventCounts {
			fmt.Fprintf(&b, "- %-28s %d\n", c.Key, c.Count)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(result.TopProximityPairs) > 0 {
		fmt.Fprintf(&b, "Top Proximity Pairs\n")
		for _, c := range result.TopProximityPairs {
			fmt.Fprintf(&b, "- %-28s %d\n", c.Pair, c.Count)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(result.Issues) > 0 {
		fmt.Fprintf(&b, "Issues\n")
		limit := len(result.Issues)
		if maxIssues > 0 && maxIssues < limit {
			limit = maxIssues
		}
		for i := 0; i < limit; i++ {
			it := result.Issues[i]
			fmt.Fprintf(&b, "- [%s] %s (event #%d, type=%s): %s\n", strings.ToUpper(string(it.Severity)), it.Code, it.EventIndex, it.EventType, it.Message)
		}
		if limit < len(result.Issues) {
			fmt.Fprintf(&b, "- ... %d more issues not shown\n", len(result.Issues)-limit)
		}
	}

	return b.String()
}

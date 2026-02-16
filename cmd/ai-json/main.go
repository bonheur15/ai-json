package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"ai-json/internal/analyze"
	"ai-json/internal/input"
	"ai-json/internal/model"
	"ai-json/internal/report"
)

func main() {
	var (
		inputPaths     multiFlag
		globPatterns   multiFlag
		streamPath     string
		format         string
		eventTypesFlag string
		classIDsFlag   string
		cameraIDsFlag  string
		minConfidence  float64
		maxIssues      int
		strict         bool
		help           bool
	)

	flag.Var(&inputPaths, "input", "input JSON file path (repeatable or comma-separated)")
	flag.Var(&globPatterns, "glob", "input glob pattern (repeatable or comma-separated)")
	flag.StringVar(&streamPath, "stream", "", "stream config JSON path (defaults to ./stream.json when present)")
	flag.StringVar(&format, "format", "text", "output format: text|json")
	flag.StringVar(&eventTypesFlag, "event-types", "", "comma-separated event types to include")
	flag.StringVar(&classIDsFlag, "class-ids", "", "comma-separated class IDs to include (uses stream_class_id or room_id)")
	flag.StringVar(&cameraIDsFlag, "camera-ids", "", "comma-separated camera IDs to include (uses stream_camera_id or camera_id)")
	flag.Float64Var(&minConfidence, "min-confidence", 0, "minimum confidence threshold")
	flag.IntVar(&maxIssues, "max-issues", 50, "max issues to print in text report (0 = all)")
	flag.BoolVar(&strict, "strict", false, "exit with code 1 when validation errors are found")
	flag.BoolVar(&help, "help", false, "show usage")
	flag.Parse()

	if help {
		printUsage()
		return
	}

	if streamPath == "" {
		if _, err := os.Stat("stream.json"); err == nil {
			streamPath = "stream.json"
		}
	}

	var (
		ds  input.Dataset
		err error
	)
	if streamPath != "" {
		ds, err = input.LoadFromStreamConfig(streamPath)
	} else {
		if len(inputPaths) == 0 && len(globPatterns) == 0 {
			globPatterns = append(globPatterns, ".material/samples/*.json")
		}
		ds, err = input.Load(inputPaths, globPatterns)
	}
	if err != nil {
		exitf("input error: %v", err)
	}

	allowedTypes := parseSet(eventTypesFlag)
	allowedClasses := parseSet(classIDsFlag)
	allowedCameras := parseSet(cameraIDsFlag)
	filtered := filterEvents(ds.Events, allowedTypes, allowedClasses, allowedCameras, minConfidence)

	res := analyze.Run(filtered)

	switch strings.ToLower(format) {
	case "text":
		fmt.Fprintln(os.Stdout, report.RenderText(res, ds.Files, ds.Stream, maxIssues))
	case "json":
		obj := struct {
			Files  []string             `json:"files"`
			Stream *input.StreamSummary `json:"stream,omitempty"`
			Result analyze.Analysis     `json:"result"`
		}{Files: ds.Files, Stream: ds.Stream, Result: res}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(obj); err != nil {
			exitf("encode json: %v", err)
		}
	default:
		exitf("invalid --format %q (expected text or json)", format)
	}

	if strict && res.ErrorCount > 0 {
		os.Exit(1)
	}
}

func parseSet(csv string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[p] = struct{}{}
	}
	return out
}

func filterEvents(events []model.Event, allowedTypes, allowedClasses, allowedCameras map[string]struct{}, minConfidence float64) []model.Event {
	if len(allowedTypes) == 0 && len(allowedClasses) == 0 && len(allowedCameras) == 0 && minConfidence <= 0 {
		return events
	}
	out := make([]model.Event, 0, len(events))
	for _, ev := range events {
		if len(allowedTypes) > 0 {
			eventType, _ := ev.String("event_type")
			if _, ok := allowedTypes[eventType]; !ok {
				continue
			}
		}
		if len(allowedClasses) > 0 {
			classID, ok := ev.String("stream_class_id")
			if !ok || classID == "" {
				classID, _ = ev.String("room_id")
			}
			if _, ok := allowedClasses[classID]; !ok {
				continue
			}
		}
		if len(allowedCameras) > 0 {
			cameraID, ok := ev.String("stream_camera_id")
			if !ok || cameraID == "" {
				cameraID, _ = ev.String("camera_id")
			}
			if _, ok := allowedCameras[cameraID]; !ok {
				continue
			}
		}
		if minConfidence > 0 {
			c, ok := ev.Float64("confidence")
			if !ok || c < minConfidence {
				continue
			}
		}
		out = append(out, ev)
	}
	return out
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "ai-json - advanced class/camera event stream analytics")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json [flags]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Examples:")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json --stream stream.json")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json --stream stream.json --format json")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json --stream stream.json --class-ids class-a --camera-ids front --event-types person_tracked,role_assigned --min-confidence 0.6")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json --glob '.material/samples/*.json' --format text")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Flags:")
	flag.PrintDefaults()
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

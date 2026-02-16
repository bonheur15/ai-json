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
		format         string
		eventTypesFlag string
		minConfidence  float64
		maxIssues      int
		strict         bool
		help           bool
	)

	flag.Var(&inputPaths, "input", "input JSON file path (repeatable or comma-separated)")
	flag.Var(&globPatterns, "glob", "input glob pattern (repeatable or comma-separated)")
	flag.StringVar(&format, "format", "text", "output format: text|json")
	flag.StringVar(&eventTypesFlag, "event-types", "", "comma-separated event types to include")
	flag.Float64Var(&minConfidence, "min-confidence", 0, "minimum confidence threshold")
	flag.IntVar(&maxIssues, "max-issues", 50, "max issues to print in text report (0 = all)")
	flag.BoolVar(&strict, "strict", false, "exit with code 1 when validation errors are found")
	flag.BoolVar(&help, "help", false, "show usage")
	flag.Parse()

	if help {
		printUsage()
		return
	}

	if len(inputPaths) == 0 && len(globPatterns) == 0 {
		globPatterns = append(globPatterns, ".material/samples/*.json")
	}

	ds, err := input.Load(inputPaths, globPatterns)
	if err != nil {
		exitf("input error: %v", err)
	}

	allowedTypes := parseSet(eventTypesFlag)
	filtered := filterEvents(ds.Events, allowedTypes, minConfidence)

	res := analyze.Run(filtered)

	switch strings.ToLower(format) {
	case "text":
		fmt.Fprintln(os.Stdout, report.RenderText(res, ds.Files, maxIssues))
	case "json":
		obj := struct {
			Files  []string         `json:"files"`
			Result analyze.Analysis `json:"result"`
		}{Files: ds.Files, Result: res}
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

func filterEvents(events []model.Event, allowedTypes map[string]struct{}, minConfidence float64) []model.Event {
	if len(allowedTypes) == 0 && minConfidence <= 0 {
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
	fmt.Fprintln(os.Stdout, "ai-json - advanced event stream analytics")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json [flags]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Examples:")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json --glob '.material/samples/*.json' --format json")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json --input a.json,b.json --event-types person_tracked,role_assigned --min-confidence 0.6")
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

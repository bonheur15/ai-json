package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"ai-json/internal/api"
	"ai-json/internal/ingest"
	"ai-json/internal/store"
)

func main() {
	var (
		addr             string
		dbPath           string
		streamPath       string
		pollSeconds      int
		minFileAgeSecond int
		maxPastSeconds   int
	)
	flag.StringVar(&addr, "addr", ":8080", "HTTP listen address")
	flag.StringVar(&dbPath, "db", "./data/ai-json.db", "sqlite database path")
	flag.StringVar(&streamPath, "stream", "stream.json", "stream config path used for periodic ingestion and stream ingest endpoint")
	flag.IntVar(&pollSeconds, "poll-seconds", 5, "periodic stream scan interval in seconds (0 disables scheduler)")
	flag.IntVar(&minFileAgeSecond, "min-file-age-seconds", 2, "minimum file age before ingesting JSON files")
	flag.IntVar(&maxPastSeconds, "max-past-seconds", 60, "maximum age (by epoch filename) allowed for ingestion")
	flag.Parse()

	if err := os.MkdirAll("./data", 0o755); err != nil {
		fatalf("create data dir: %v", err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		fatalf("open store: %v", err)
	}
	defer s.Close()

	minAge := time.Duration(minFileAgeSecond) * time.Second
	maxPast := time.Duration(maxPastSeconds) * time.Second
	if pollSeconds > 0 {
		go runPeriodicIngestion(s, streamPath, time.Duration(pollSeconds)*time.Second, minAge, maxPast)
	}

	h := api.New(s)
	h.DefaultStream = streamPath
	h.DefaultMinAge = minAge
	h.DefaultMaxPastAge = maxPast

	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	fmt.Fprintf(os.Stdout, "ai-json-api listening on %s using db %s\n", addr, dbPath)
	fmt.Fprintf(os.Stdout, "stream=%s poll_seconds=%d min_file_age_seconds=%d max_past_seconds=%d\n", streamPath, pollSeconds, minFileAgeSecond, maxPastSeconds)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatalf("listen: %v", err)
	}
}

func runPeriodicIngestion(st *store.Store, streamPath string, interval time.Duration, minFileAge time.Duration, maxPast time.Duration) {
	runner := ingest.Runner{Store: st, StreamPath: streamPath, MinFileAge: minFileAge, MaxPastAge: maxPast}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		stats, err := runner.RunOnce()
		if err != nil {
			fmt.Fprintf(os.Stderr, "periodic ingestion error: %v\n", err)
		} else if stats.ProcessedFiles > 0 {
			fmt.Fprintf(os.Stdout, "periodic ingestion processed=%d inserted=%d skipped=%d\n", stats.ProcessedFiles, stats.InsertedEvents, stats.SkippedFiles)
		}
		<-ticker.C
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"ai-json/internal/input"
	"ai-json/internal/model"
	"ai-json/internal/store"
)

type Runner struct {
	Store      *store.Store
	StreamPath string
	MinFileAge time.Duration
}

type RunStats struct {
	ProcessedFiles int `json:"processed_files"`
	InsertedEvents int `json:"inserted_events"`
	SkippedFiles   int `json:"skipped_files"`
}

func (r *Runner) RunOnce() (RunStats, error) {
	if r.Store == nil {
		return RunStats{}, fmt.Errorf("store is required")
	}
	if r.StreamPath == "" {
		r.StreamPath = "stream.json"
	}
	if r.MinFileAge <= 0 {
		r.MinFileAge = 2 * time.Second
	}

	resolved, err := input.ResolveStreamConfig(r.StreamPath)
	if err != nil {
		return RunStats{}, err
	}

	stats := RunStats{}
	now := time.Now()
	for _, cls := range resolved.Classes {
		for _, cam := range cls.Cameras {
			files, err := input.ResolveCameraEventFiles(resolved.ConfigDir, cls.BaseDir, cam)
			if err != nil {
				return stats, fmt.Errorf("resolve event files class=%s camera=%s: %w", cls.ClassID, cam.ID, err)
			}
			sort.Strings(files)
			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					continue
				}
				if now.Sub(info.ModTime()) < r.MinFileAge {
					stats.SkippedFiles++
					continue
				}
				absFile, err := filepath.Abs(file)
				if err != nil {
					continue
				}
				should, err := r.Store.ShouldIngestFile(absFile, info.Size(), info.ModTime().Unix())
				if err != nil {
					return stats, err
				}
				if !should {
					stats.SkippedFiles++
					continue
				}

				b, err := os.ReadFile(absFile)
				if err != nil {
					return stats, fmt.Errorf("read %s: %w", absFile, err)
				}
				events, err := model.ParseEvents(b)
				if err != nil {
					return stats, fmt.Errorf("parse %s: %w", absFile, err)
				}
				for i := range events {
					events[i].Raw["stream_class_id"] = cls.ClassID
					events[i].Raw["stream_camera_id"] = cam.ID
				}
				n, err := r.Store.InsertEvents(events, absFile)
				if err != nil {
					return stats, fmt.Errorf("insert from %s: %w", absFile, err)
				}
				if err := r.Store.MarkFileIngested(absFile, info.Size(), info.ModTime().Unix()); err != nil {
					return stats, err
				}
				stats.ProcessedFiles++
				stats.InsertedEvents += n
			}
		}
	}

	return stats, nil
}

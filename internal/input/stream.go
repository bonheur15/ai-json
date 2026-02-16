package input

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ai-json/internal/model"
)

const (
	CameraFront = "front"
	CameraBack  = "back"
)

// StreamConfig defines top-level stream.json layout.
type StreamConfig struct {
	Version string        `json:"version"`
	Classes []ClassConfig `json:"classes"`
}

type ClassConfig struct {
	ClassID string         `json:"class_id"`
	Name    string         `json:"name"`
	BaseDir string         `json:"base_dir"`
	Cameras []CameraConfig `json:"cameras"`
}

type CameraConfig struct {
	ID         string   `json:"id"`
	ImagesDir  string   `json:"images_dir"`
	EventsDir  string   `json:"events_dir"`
	EventFiles []string `json:"event_files"`
	EventGlobs []string `json:"event_globs"`
}

type StreamSummary struct {
	ConfigPath      string         `json:"config_path"`
	TotalClasses    int            `json:"total_classes"`
	TotalImages     int            `json:"total_images"`
	TotalEventFiles int            `json:"total_event_files"`
	Classes         []ClassSummary `json:"classes"`
}

type ClassSummary struct {
	ClassID string          `json:"class_id"`
	Name    string          `json:"name"`
	BaseDir string          `json:"base_dir"`
	Cameras []CameraSummary `json:"cameras"`
}

type CameraSummary struct {
	ID             string   `json:"id"`
	ImagesDir      string   `json:"images_dir"`
	ImageCount     int      `json:"image_count"`
	EventFiles     []string `json:"event_files"`
	EventFileCount int      `json:"event_file_count"`
	EventCount     int      `json:"event_count"`
}

func LoadFromStreamConfig(path string) (Dataset, error) {
	absCfg, err := filepath.Abs(path)
	if err != nil {
		return Dataset{}, fmt.Errorf("resolve stream config path: %w", err)
	}

	b, err := os.ReadFile(absCfg)
	if err != nil {
		return Dataset{}, fmt.Errorf("read stream config %s: %w", absCfg, err)
	}

	var cfg StreamConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Dataset{}, fmt.Errorf("decode stream config %s: %w", absCfg, err)
	}
	if len(cfg.Classes) == 0 {
		return Dataset{}, fmt.Errorf("stream config must contain at least one class")
	}

	cfgDir := filepath.Dir(absCfg)
	allEvents := make([]model.Event, 0)
	allFiles := make([]string, 0)
	stream := StreamSummary{ConfigPath: absCfg, TotalClasses: len(cfg.Classes), Classes: make([]ClassSummary, 0, len(cfg.Classes))}

	classSeen := map[string]struct{}{}
	for _, classCfg := range cfg.Classes {
		if strings.TrimSpace(classCfg.ClassID) == "" {
			return Dataset{}, fmt.Errorf("class_id is required for each class")
		}
		if _, exists := classSeen[classCfg.ClassID]; exists {
			return Dataset{}, fmt.Errorf("duplicate class_id %q", classCfg.ClassID)
		}
		classSeen[classCfg.ClassID] = struct{}{}

		baseDir := classCfg.BaseDir
		if strings.TrimSpace(baseDir) == "" {
			baseDir = classCfg.ClassID
		}
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Join(cfgDir, baseDir)
		}
		if _, err := os.Stat(baseDir); err != nil {
			return Dataset{}, fmt.Errorf("class base_dir %s: %w", baseDir, err)
		}

		classSummary := ClassSummary{ClassID: classCfg.ClassID, Name: classCfg.Name, BaseDir: baseDir, Cameras: make([]CameraSummary, 0, len(classCfg.Cameras))}
		cameraByID := map[string]CameraConfig{}
		for _, c := range classCfg.Cameras {
			if strings.TrimSpace(c.ID) == "" {
				return Dataset{}, fmt.Errorf("class %s has camera with empty id", classCfg.ClassID)
			}
			if _, exists := cameraByID[c.ID]; exists {
				return Dataset{}, fmt.Errorf("class %s has duplicate camera id %s", classCfg.ClassID, c.ID)
			}
			cameraByID[c.ID] = c
		}
		for _, required := range []string{CameraFront, CameraBack} {
			if _, ok := cameraByID[required]; !ok {
				return Dataset{}, fmt.Errorf("class %s must define %q and %q cameras", classCfg.ClassID, CameraFront, CameraBack)
			}
		}

		for _, camID := range []string{CameraFront, CameraBack} {
			cam := cameraByID[camID]
			imagesDir := cam.ImagesDir
			if strings.TrimSpace(imagesDir) == "" {
				imagesDir = filepath.Join(baseDir, camID, "images")
			}
			if !filepath.IsAbs(imagesDir) {
				imagesDir = filepath.Join(cfgDir, imagesDir)
			}
			if _, err := os.Stat(imagesDir); err != nil {
				return Dataset{}, fmt.Errorf("class %s camera %s images_dir %s: %w", classCfg.ClassID, camID, imagesDir, err)
			}
			imageCount, err := countJPG(imagesDir)
			if err != nil {
				return Dataset{}, fmt.Errorf("class %s camera %s images scan: %w", classCfg.ClassID, camID, err)
			}

			eventFiles, err := resolveCameraEventFiles(cfgDir, baseDir, cam)
			if err != nil {
				return Dataset{}, fmt.Errorf("class %s camera %s events: %w", classCfg.ClassID, camID, err)
			}
			if len(eventFiles) == 0 {
				return Dataset{}, fmt.Errorf("class %s camera %s has no event JSON files", classCfg.ClassID, camID)
			}

			cameraEventCount := 0
			for _, f := range eventFiles {
				b, err := os.ReadFile(f)
				if err != nil {
					return Dataset{}, fmt.Errorf("read %s: %w", f, err)
				}
				events, err := model.ParseEvents(b)
				if err != nil {
					return Dataset{}, fmt.Errorf("parse %s: %w", f, err)
				}
				for i := range events {
					// Attach stream metadata for downstream filtering/reporting.
					events[i].Raw["stream_class_id"] = classCfg.ClassID
					events[i].Raw["stream_camera_id"] = camID
				}
				cameraEventCount += len(events)
				allEvents = append(allEvents, events...)
			}

			stream.TotalImages += imageCount
			stream.TotalEventFiles += len(eventFiles)
			allFiles = append(allFiles, eventFiles...)
			classSummary.Cameras = append(classSummary.Cameras, CameraSummary{
				ID:             camID,
				ImagesDir:      imagesDir,
				ImageCount:     imageCount,
				EventFiles:     eventFiles,
				EventFileCount: len(eventFiles),
				EventCount:     cameraEventCount,
			})
		}

		stream.Classes = append(stream.Classes, classSummary)
	}

	sort.Strings(allFiles)
	return Dataset{Files: allFiles, Events: allEvents, Stream: &stream}, nil
}

func resolveCameraEventFiles(cfgDir string, baseDir string, cam CameraConfig) ([]string, error) {
	paths := make([]string, 0)
	for _, f := range cam.EventFiles {
		for _, part := range splitCSV(f) {
			if strings.TrimSpace(part) == "" {
				continue
			}
			if !filepath.IsAbs(part) {
				part = filepath.Join(cfgDir, part)
			}
			paths = append(paths, part)
		}
	}

	globs := make([]string, 0)
	for _, g := range cam.EventGlobs {
		for _, part := range splitCSV(g) {
			if strings.TrimSpace(part) == "" {
				continue
			}
			if !filepath.IsAbs(part) {
				part = filepath.Join(cfgDir, part)
			}
			globs = append(globs, part)
		}
	}

	if cam.EventsDir != "" {
		eventsDir := cam.EventsDir
		if !filepath.IsAbs(eventsDir) {
			eventsDir = filepath.Join(cfgDir, eventsDir)
		}
		globs = append(globs, filepath.Join(eventsDir, "*.json"))
	}

	if len(paths) == 0 && len(globs) == 0 {
		globs = append(globs, filepath.Join(baseDir, cam.ID, "events", "*.json"))
	}

	return resolveFiles(paths, globs)
}

func countJPG(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

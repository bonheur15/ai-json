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
	ID          string   `json:"id"`
	ImagesDir   string   `json:"images_dir"`
	EventsDir   string   `json:"events_dir"`
	FilePattern string   `json:"file_pattern"`
	EventFiles  []string `json:"event_files,omitempty"`
	EventGlobs  []string `json:"event_globs,omitempty"`
}

type ResolvedStream struct {
	ConfigPath string
	ConfigDir  string
	Classes    []ResolvedClass
}

type ResolvedClass struct {
	ClassID string
	Name    string
	BaseDir string
	Cameras []ResolvedCamera
}

type ResolvedCamera struct {
	ID          string
	ImagesDir   string
	EventsDir   string
	FilePattern string
	EventFiles  []string
	EventGlobs  []string
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
	EventDir       string   `json:"event_dir"`
	ImageCount     int      `json:"image_count"`
	EventFiles     []string `json:"event_files"`
	EventFileCount int      `json:"event_file_count"`
	EventCount     int      `json:"event_count"`
}

func LoadFromStreamConfig(path string) (Dataset, error) {
	resolved, err := ResolveStreamConfig(path)
	if err != nil {
		return Dataset{}, err
	}

	allEvents := make([]model.Event, 0)
	allFiles := make([]string, 0)
	stream := StreamSummary{ConfigPath: resolved.ConfigPath, TotalClasses: len(resolved.Classes), Classes: make([]ClassSummary, 0, len(resolved.Classes))}

	for _, cls := range resolved.Classes {
		classSummary := ClassSummary{ClassID: cls.ClassID, Name: cls.Name, BaseDir: cls.BaseDir, Cameras: make([]CameraSummary, 0, len(cls.Cameras))}
		for _, cam := range cls.Cameras {
			imageCount, err := countJPG(cam.ImagesDir)
			if err != nil {
				return Dataset{}, fmt.Errorf("class %s camera %s images scan: %w", cls.ClassID, cam.ID, err)
			}

			eventFiles, err := ResolveCameraEventFiles(resolved.ConfigDir, cls.BaseDir, cam)
			if err != nil {
				return Dataset{}, fmt.Errorf("class %s camera %s events: %w", cls.ClassID, cam.ID, err)
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
					events[i].Raw["stream_class_id"] = cls.ClassID
					events[i].Raw["stream_camera_id"] = cam.ID
				}
				cameraEventCount += len(events)
				allEvents = append(allEvents, events...)
			}

			stream.TotalImages += imageCount
			stream.TotalEventFiles += len(eventFiles)
			allFiles = append(allFiles, eventFiles...)
			classSummary.Cameras = append(classSummary.Cameras, CameraSummary{
				ID:             cam.ID,
				ImagesDir:      cam.ImagesDir,
				EventDir:       cam.EventsDir,
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

func ResolveStreamConfig(path string) (ResolvedStream, error) {
	absCfg, err := filepath.Abs(path)
	if err != nil {
		return ResolvedStream{}, fmt.Errorf("resolve stream config path: %w", err)
	}
	b, err := os.ReadFile(absCfg)
	if err != nil {
		return ResolvedStream{}, fmt.Errorf("read stream config %s: %w", absCfg, err)
	}

	var cfg StreamConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return ResolvedStream{}, fmt.Errorf("decode stream config %s: %w", absCfg, err)
	}
	if len(cfg.Classes) == 0 {
		return ResolvedStream{}, fmt.Errorf("stream config must contain at least one class")
	}

	cfgDir := filepath.Dir(absCfg)
	resolved := ResolvedStream{ConfigPath: absCfg, ConfigDir: cfgDir, Classes: make([]ResolvedClass, 0, len(cfg.Classes))}
	classSeen := map[string]struct{}{}

	for _, classCfg := range cfg.Classes {
		if strings.TrimSpace(classCfg.ClassID) == "" {
			return ResolvedStream{}, fmt.Errorf("class_id is required for each class")
		}
		if _, exists := classSeen[classCfg.ClassID]; exists {
			return ResolvedStream{}, fmt.Errorf("duplicate class_id %q", classCfg.ClassID)
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
			return ResolvedStream{}, fmt.Errorf("class %s base_dir %s: %w", classCfg.ClassID, baseDir, err)
		}

		cameraByID := map[string]CameraConfig{}
		for _, c := range classCfg.Cameras {
			if strings.TrimSpace(c.ID) == "" {
				return ResolvedStream{}, fmt.Errorf("class %s has camera with empty id", classCfg.ClassID)
			}
			if _, exists := cameraByID[c.ID]; exists {
				return ResolvedStream{}, fmt.Errorf("class %s has duplicate camera id %s", classCfg.ClassID, c.ID)
			}
			cameraByID[c.ID] = c
		}
		for _, required := range []string{CameraFront, CameraBack} {
			if _, ok := cameraByID[required]; !ok {
				return ResolvedStream{}, fmt.Errorf("class %s must define %q and %q cameras", classCfg.ClassID, CameraFront, CameraBack)
			}
		}

		resolvedClass := ResolvedClass{ClassID: classCfg.ClassID, Name: classCfg.Name, BaseDir: baseDir, Cameras: make([]ResolvedCamera, 0, 2)}
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
				return ResolvedStream{}, fmt.Errorf("class %s camera %s images_dir %s: %w", classCfg.ClassID, camID, imagesDir, err)
			}

			eventsDir := cam.EventsDir
			if strings.TrimSpace(eventsDir) == "" {
				eventsDir = filepath.Join(baseDir, camID, "events")
			}
			if !filepath.IsAbs(eventsDir) {
				eventsDir = filepath.Join(cfgDir, eventsDir)
			}
			if _, err := os.Stat(eventsDir); err != nil {
				return ResolvedStream{}, fmt.Errorf("class %s camera %s events_dir %s: %w", classCfg.ClassID, camID, eventsDir, err)
			}

			pattern := cam.FilePattern
			if strings.TrimSpace(pattern) == "" {
				pattern = "*.json"
			}

			resolvedClass.Cameras = append(resolvedClass.Cameras, ResolvedCamera{
				ID:          camID,
				ImagesDir:   imagesDir,
				EventsDir:   eventsDir,
				FilePattern: pattern,
				EventFiles:  cam.EventFiles,
				EventGlobs:  cam.EventGlobs,
			})
		}
		resolved.Classes = append(resolved.Classes, resolvedClass)
	}
	return resolved, nil
}

func ResolveCameraEventFiles(cfgDir string, baseDir string, cam ResolvedCamera) ([]string, error) {
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

	globs = append(globs, filepath.Join(cam.EventsDir, cam.FilePattern))
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

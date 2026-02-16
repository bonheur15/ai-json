package media

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ai-json/internal/input"
)

type StreamImageResolver struct {
	byClassCamera map[string]string
}

type ImageContextItem struct {
	OffsetSeconds int    `json:"offset_seconds"`
	Timestamp     int64  `json:"timestamp"`
	Exists        bool   `json:"exists"`
	Path          string `json:"path,omitempty"`
	URL           string `json:"url,omitempty"`
}

func NewStreamImageResolver(streamPath string) (*StreamImageResolver, error) {
	resolved, err := input.ResolveStreamConfig(streamPath)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, cls := range resolved.Classes {
		for _, cam := range cls.Cameras {
			key := classCameraKey(cls.ClassID, cam.ID)
			m[key] = cam.ImagesDir
		}
	}
	return &StreamImageResolver{byClassCamera: m}, nil
}

func (r *StreamImageResolver) ResolveImagePath(classID, cameraID string, ts int64) (string, bool) {
	key := classCameraKey(classID, cameraID)
	imagesDir, ok := r.byClassCamera[key]
	if !ok {
		return "", false
	}
	base := strconv.FormatInt(ts, 10)
	jpg := filepath.Join(imagesDir, base+".jpg")
	if fileExists(jpg) {
		return jpg, true
	}
	jpeg := filepath.Join(imagesDir, base+".jpeg")
	if fileExists(jpeg) {
		return jpeg, true
	}
	return jpg, false
}

func (r *StreamImageResolver) BuildContext(classID, cameraID string, eventTS int64, windowSeconds int, imageEndpoint string) []ImageContextItem {
	if windowSeconds < 0 {
		windowSeconds = 0
	}
	out := make([]ImageContextItem, 0, 2*windowSeconds+1)
	for offset := -windowSeconds; offset <= windowSeconds; offset++ {
		ts := eventTS + int64(offset)
		path, exists := r.ResolveImagePath(classID, cameraID, ts)
		item := ImageContextItem{OffsetSeconds: offset, Timestamp: ts, Exists: exists}
		if exists {
			item.Path = path
			if imageEndpoint != "" {
				item.URL = imageEndpoint + "?class_id=" + url.QueryEscape(classID) + "&camera_id=" + url.QueryEscape(cameraID) + "&ts=" + strconv.FormatInt(ts, 10)
			}
		}
		out = append(out, item)
	}
	return out
}

func classCameraKey(classID, cameraID string) string {
	return strings.TrimSpace(classID) + "::" + strings.TrimSpace(cameraID)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ValidateImageRequest(classID, cameraID string, ts int64) error {
	if strings.TrimSpace(classID) == "" {
		return fmt.Errorf("class_id is required")
	}
	if strings.TrimSpace(cameraID) == "" {
		return fmt.Errorf("camera_id is required")
	}
	if ts <= 0 {
		return fmt.Errorf("ts must be > 0")
	}
	return nil
}

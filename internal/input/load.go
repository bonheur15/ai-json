package input

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ai-json/internal/model"
)

type Dataset struct {
	Files  []string
	Events []model.Event
}

func Load(paths []string, globs []string) (Dataset, error) {
	files, err := resolveFiles(paths, globs)
	if err != nil {
		return Dataset{}, err
	}
	if len(files) == 0 {
		return Dataset{}, fmt.Errorf("no input files resolved")
	}

	all := make([]model.Event, 0)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return Dataset{}, fmt.Errorf("read %s: %w", f, err)
		}
		events, err := model.ParseEvents(b)
		if err != nil {
			return Dataset{}, fmt.Errorf("parse %s: %w", f, err)
		}
		all = append(all, events...)
	}

	return Dataset{Files: files, Events: all}, nil
}

func resolveFiles(paths []string, globs []string) ([]string, error) {
	set := map[string]struct{}{}

	for _, p := range paths {
		for _, item := range splitCSV(p) {
			if item == "" {
				continue
			}
			info, err := os.Stat(item)
			if err != nil {
				return nil, fmt.Errorf("stat %s: %w", item, err)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("%s is a directory; pass files or globs", item)
			}
			abs, err := filepath.Abs(item)
			if err != nil {
				return nil, fmt.Errorf("abs path %s: %w", item, err)
			}
			set[abs] = struct{}{}
		}
	}

	for _, g := range globs {
		for _, pattern := range splitCSV(g) {
			if pattern == "" {
				continue
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("glob %s: %w", pattern, err)
			}
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil || info.IsDir() {
					continue
				}
				abs, err := filepath.Abs(m)
				if err != nil {
					return nil, fmt.Errorf("abs path %s: %w", m, err)
				}
				set[abs] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

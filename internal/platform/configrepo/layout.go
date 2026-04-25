package configrepo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type layoutResolution struct {
	SourcePath string
	Entries    []layoutEntry
}

type layoutEntry struct {
	Name     string
	DiskPath string
}

func resolveLayout(rootDir, sourcePath, env string) (*layoutResolution, error) {
	normalizedSource := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(sourcePath)), "./")
	if normalizedSource == "." || normalizedSource == "" {
		return nil, ErrSourcePathNotFound
	}
	dir := filepath.Join(rootDir, filepath.FromSlash(normalizedSource))
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, ErrSourcePathNotFound
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entryMap := make(map[string]string, len(entries)+4)
	serviceLayout := strings.HasPrefix(normalizedSource, "applications/devflow-platform/services/")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if serviceLayout && isServiceLayoutExcludedFile(entry.Name()) {
			continue
		}
		entryMap[entry.Name()] = filepath.Join(dir, entry.Name())
	}
	if serviceLayout {
		for name, diskPath := range resolveEnvironmentOverlayFiles(dir, env) {
			entryMap[name] = diskPath
		}
	}
	if len(entryMap) == 0 {
		return nil, ErrSourcePathNotFound
	}
	names := make([]string, 0, len(entryMap))
	for name := range entryMap {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		leftWeight, rightWeight := layoutFileOrder(names[i]), layoutFileOrder(names[j])
		if leftWeight != rightWeight {
			return leftWeight < rightWeight
		}
		return names[i] < names[j]
	})
	resolvedEntries := make([]layoutEntry, 0, len(names))
	for _, name := range names {
		resolvedEntries = append(resolvedEntries, layoutEntry{Name: name, DiskPath: entryMap[name]})
	}

	return &layoutResolution{
		SourcePath: normalizedSource,
		Entries:    resolvedEntries,
	}, nil
}

func resolveEnvironmentOverlayFiles(dir, env string) map[string]string {
	trimmedEnv := strings.TrimSpace(env)
	if trimmedEnv == "" || strings.EqualFold(trimmedEnv, "base") {
		return nil
	}
	envDir := filepath.Join(dir, "environments", trimmedEnv)
	entryMap := map[string]string{}
	if relativeFiles, err := collectRelativeFiles(envDir); err == nil {
		for _, relative := range relativeFiles {
			entryMap[relative] = filepath.Join(envDir, filepath.FromSlash(relative))
		}
	}
	return entryMap
}

func collectRelativeFiles(targetDir string) ([]string, error) {
	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		return nil, err
	}
	files := make([]string, 0, 8)
	err = filepath.WalkDir(targetDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(targetDir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isServiceLayoutExcludedFile(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "deployment.yaml", "service.yaml":
		return true
	default:
		return false
	}
}

func layoutFileOrder(name string) int {
	return 0
}

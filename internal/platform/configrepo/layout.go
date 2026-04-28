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
	if strings.TrimSpace(env) != "" && !strings.EqualFold(strings.TrimSpace(env), "base") {
		candidate := strings.TrimSuffix(normalizedSource, "/") + "/" + strings.Trim(strings.TrimSpace(env), "/")
		if info, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(candidate))); err == nil && info.IsDir() {
			normalizedSource = candidate
		}
	}
	dir := filepath.Join(rootDir, filepath.FromSlash(normalizedSource))
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, ErrSourcePathNotFound
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	resolvedEntries := make([]layoutEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		resolvedEntries = append(resolvedEntries, layoutEntry{
			Name:     entry.Name(),
			DiskPath: filepath.Join(dir, entry.Name()),
		})
	}
	if len(resolvedEntries) == 0 {
		return nil, ErrSourcePathNotFound
	}
	sort.Slice(resolvedEntries, func(i, j int) bool {
		leftWeight, rightWeight := layoutFileOrder(resolvedEntries[i].Name), layoutFileOrder(resolvedEntries[j].Name)
		if leftWeight != rightWeight {
			return leftWeight < rightWeight
		}
		return resolvedEntries[i].Name < resolvedEntries[j].Name
	})
	return &layoutResolution{
		SourcePath: normalizedSource,
		Entries:    resolvedEntries,
	}, nil
}

func layoutFileOrder(name string) int {
	return 0
}

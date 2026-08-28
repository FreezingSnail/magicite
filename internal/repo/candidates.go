package repo

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/connorfranc/magicite/internal/config"
)

const maxScanEntries = 512

// Finder locates and filters repository candidates.
type Finder struct {
	Repos config.ReposConfig
	Probe *Prober
	Dir   string
}

// Candidates returns configured or ambient repository candidates.
func (f Finder) Candidates(ctx context.Context) []string {
	candidates := make([]string, 0)
	switch f.Repos.Discover {
	case "explicit":
		for _, root := range f.Repos.Roots {
			if directory, ok := Directory(root); ok {
				candidates = append(candidates, directory)
			}
		}
	case "project":
		if f.Probe == nil {
			break
		}
		dir := f.Dir
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				break
			}
		}
		ambient, ok := f.Probe.GitRoot(ctx, dir)
		if !ok {
			break
		}
		candidates = append(candidates, ambient)
		parent := filepath.Dir(strings.TrimSuffix(ambient, string(filepath.Separator)))
		entries, err := os.ReadDir(parent)
		if err != nil {
			break
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		if len(entries) > maxScanEntries {
			entries = entries[:maxScanEntries]
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if directory, ok := Directory(filepath.Join(parent, entry.Name())); ok {
				candidates = append(candidates, directory)
			}
		}
	}
	return unique(f.Filter(candidates))
}

// Filter applies the configured include and exclude entries to candidates.
func (f Finder) Filter(candidates []string) []string {
	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if len(f.Repos.Include) > 0 && !matchesAny(candidate, f.Repos.Include) {
			continue
		}
		if matchesAny(candidate, f.Repos.Exclude) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func matchesAny(candidate string, entries []string) bool {
	for _, entry := range entries {
		if strings.TrimSpace(entry) != "" && matches(candidate, entry) {
			return true
		}
	}
	return false
}

func matches(candidate, entry string) bool {
	if candidate == entry {
		return true
	}
	directory, ok := Directory(candidate)
	if !ok {
		return false
	}
	return entry == directory ||
		entry == strings.TrimSuffix(directory, string(filepath.Separator)) ||
		entry == filepath.Base(strings.TrimSuffix(directory, string(filepath.Separator)))
}

func unique(candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

package repo

import (
	"context"
	"os"
	"path/filepath"
)

// AmbientCandidates returns worktree roots surrounding the process.
func (f Finder) AmbientCandidates(ctx context.Context) []string {
	candidates := make([]string, 0, 2)
	if f.Repos.Discover == "explicit" || f.Probe == nil {
		return candidates
	}

	seen := make(map[string]struct{}, 2)
	appendRoot := func(dir string) {
		root, ok := f.Probe.GitRoot(ctx, dir)
		if !ok {
			return
		}
		normalized, ok := Directory(root)
		if !ok {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		candidates = append(candidates, normalized)
	}

	dir := f.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			dir = ""
		}
	}
	if dir != "" {
		appendRoot(dir)
	}

	executable, err := os.Executable()
	if err == nil {
		appendRoot(filepath.Dir(executable))
	}

	return f.Filter(candidates)
}

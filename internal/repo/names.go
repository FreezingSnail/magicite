package repo

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Records builds deterministic, uniquely named repository records.
func Records(roots []string) []Repo {
	normalized := make([]string, 0, len(roots))
	seenRoots := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		directory, ok := Directory(root)
		if !ok {
			continue
		}
		if _, seen := seenRoots[directory]; seen {
			continue
		}
		seenRoots[directory] = struct{}{}
		normalized = append(normalized, directory)
	}
	sort.Strings(normalized)

	baseCounts := make(map[string]int, len(normalized))
	for _, root := range normalized {
		baseCounts[pathBase(root)]++
	}

	records := make([]Repo, 0, len(normalized))
	usedNames := make(map[string]struct{}, len(normalized))
	for _, root := range normalized {
		base := pathBase(root)
		stem := base
		if baseCounts[base] > 1 {
			stem = pathBase(filepath.Dir(trimDirectory(root))) + "-" + base
		}
		if _, used := usedNames[stem]; used {
			for suffix := 2; ; suffix++ {
				candidate := stem + "-" + itoa(suffix)
				if _, used := usedNames[candidate]; !used {
					stem = candidate
					break
				}
			}
		}
		usedNames[stem] = struct{}{}
		record, ok := Make(root, stem, stem, "")
		if ok {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})
	return records
}

func pathBase(directory string) string {
	return filepath.Base(trimDirectory(directory))
}

func trimDirectory(directory string) string {
	trimmed := strings.TrimSuffix(directory, string(filepath.Separator))
	if trimmed == "" {
		return string(filepath.Separator)
	}
	return trimmed
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

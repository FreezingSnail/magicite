package dispatch

import (
	"sort"
	"strconv"
	"strings"

	"github.com/connorfranc/magicite/internal/repo"
)

const defaultReadyPriority = "2"

// NormalizeReady validates one repository-scoped ready entry and fills its
// repository and default priority.
func NormalizeReady(repository repo.Repo, entry ReadyEntry) (ReadyEntry, bool) {
	if !repository.Valid() || strings.TrimSpace(entry.Task) == "" {
		return ReadyEntry{}, false
	}
	if entry.Repo != (repo.Repo{}) && entry.Repo != repository {
		return ReadyEntry{}, false
	}
	if entry.Priority == "" {
		entry.Priority = defaultReadyPriority
	} else if _, err := strconv.Atoi(entry.Priority); err != nil {
		entry.Priority = defaultReadyPriority
	}
	entry.Repo = repository
	return entry, true
}

// MergeReady normalizes and fairly merges repository ready queues. Priorities
// ascend; equal-priority entries round-robin in group order.
func MergeReady(groups []RepoReady) []ReadyEntry {
	queues := make(map[int][][]ReadyEntry)
	priorities := make(map[int]struct{})

	for groupIndex, group := range groups {
		for _, entry := range group.Entries {
			normalized, ok := NormalizeReady(group.Repo, entry)
			if !ok {
				continue
			}
			priority, _ := strconv.Atoi(normalized.Priority)
			if _, exists := queues[priority]; !exists {
				queues[priority] = make([][]ReadyEntry, len(groups))
			}
			queues[priority][groupIndex] = append(queues[priority][groupIndex], normalized)
			priorities[priority] = struct{}{}
		}
	}

	orderedPriorities := make([]int, 0, len(priorities))
	for priority := range priorities {
		orderedPriorities = append(orderedPriorities, priority)
	}
	sort.Ints(orderedPriorities)

	merged := make([]ReadyEntry, 0)
	for _, priority := range orderedPriorities {
		perRepository := queues[priority]
		positions := make([]int, len(perRepository))
		for {
			drained := true
			for groupIndex, queue := range perRepository {
				if positions[groupIndex] == len(queue) {
					continue
				}
				drained = false
				merged = append(merged, queue[positions[groupIndex]])
				positions[groupIndex]++
			}
			if drained {
				break
			}
		}
	}
	return merged
}

// TakeReady returns at most n entries in a new slice.
func TakeReady(entries []ReadyEntry, n int) []ReadyEntry {
	if n <= 0 {
		return []ReadyEntry{}
	}
	if n > len(entries) {
		n = len(entries)
	}
	selected := make([]ReadyEntry, n)
	copy(selected, entries[:n])
	return selected
}

package server

import (
	"sort"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// normalizeSnapshot returns an independently owned, deterministically ordered
// snapshot suitable for transport. It performs no daemon or repository reads.
func normalizeSnapshot(result wire.SnapshotResult) wire.SnapshotResult {
	result.Repositories = append([]wire.RepoResult{}, result.Repositories...)
	result.Seats = append([]wire.SeatResult{}, result.Seats...)
	result.Sessions = append([]wire.SessionResult{}, result.Sessions...)
	result.Runtime.Sessions = append([]wire.SessionResult{}, result.Runtime.Sessions...)
	result.Beads = append([]wire.BeadResult{}, result.Beads...)
	result.StatusCounts = append([]wire.StatusCount{}, result.StatusCounts...)
	result.RepositoryErrors = append([]wire.RepositoryError{}, result.RepositoryErrors...)

	for index := range result.Beads {
		result.Beads[index].Dependencies = append([]wire.DependencyResult{}, result.Beads[index].Dependencies...)
		result.Beads[index].Labels = append([]string{}, result.Beads[index].Labels...)
	}

	sort.Slice(result.Repositories, func(i, j int) bool {
		left, right := result.Repositories[i], result.Repositories[j]
		return compareStrings([]string{left.Name, left.Path, left.Prefix, left.Branch}, []string{right.Name, right.Path, right.Prefix, right.Branch}) < 0
	})
	sort.Slice(result.Seats, func(i, j int) bool {
		left, right := result.Seats[i], result.Seats[j]
		return compareStrings([]string{left.Role, left.Name, left.Repo, left.Worktree, left.Task, boolString(left.Busy)}, []string{right.Role, right.Name, right.Repo, right.Worktree, right.Task, boolString(right.Busy)}) < 0
	})
	sort.Slice(result.Sessions, sessionLess(result.Sessions))
	sort.Slice(result.Runtime.Sessions, sessionLess(result.Runtime.Sessions))
	sort.Slice(result.Beads, func(i, j int) bool {
		left, right := result.Beads[i], result.Beads[j]
		if left.Repo != right.Repo {
			return left.Repo < right.Repo
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return compareStrings([]string{left.ID, left.Title, left.Status, left.IssueType}, []string{right.ID, right.Title, right.Status, right.IssueType}) < 0
	})
	sort.Slice(result.StatusCounts, func(i, j int) bool {
		left, right := result.StatusCounts[i], result.StatusCounts[j]
		if left.Status != right.Status {
			return left.Status < right.Status
		}
		return left.Count < right.Count
	})
	sort.Slice(result.RepositoryErrors, func(i, j int) bool {
		left, right := result.RepositoryErrors[i], result.RepositoryErrors[j]
		return compareStrings([]string{left.Repository, left.Error}, []string{right.Repository, right.Error}) < 0
	})
	return result
}

func sessionLess(sessions []wire.SessionResult) func(int, int) bool {
	return func(i, j int) bool {
		left, right := sessions[i], sessions[j]
		if left.UptimeSeconds != right.UptimeSeconds {
			return left.UptimeSeconds > right.UptimeSeconds
		}
		return compareStrings([]string{left.Handle, left.Repo, left.Task, left.Role, left.Seat, left.Backend, left.Model, left.Status, left.Phase}, []string{right.Handle, right.Repo, right.Task, right.Role, right.Seat, right.Backend, right.Model, right.Status, right.Phase}) < 0
	}
}

func compareStrings(left, right []string) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

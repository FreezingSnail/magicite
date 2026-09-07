package daemon

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/repo"
)

// snapshotRecord is one repository's raw contribution to a snapshot. Its value
// deliberately remains uninterpreted so the final composer owns conversion.
type snapshotRecord[T any] struct {
	Repo  repo.Repo
	Value T
}

// snapshotRepositoryError identifies a repository read failure without
// discarding that repository's last good raw contribution.
type snapshotRepositoryError struct {
	Repo repo.Repo
	Err  error
}

// snapshotState is the coordinator-owned raw input to final snapshot
// composition. Records and Errors are ordered by the requested repositories.
type snapshotState[T any] struct {
	Records    []snapshotRecord[T]
	Errors     []snapshotRepositoryError
	Generation uint64
	Fresh      bool
	Stale      bool
}

// snapshotCoordinator combines cache freshness with concurrent raw reads and
// owns last-good retention policy. The cache clone must deep-copy snapshotState
// values so every returned state is independently owned by its caller.
type snapshotCoordinator[T any] struct {
	cache     *snapshotCache[snapshotState[T]]
	refresher *snapshotRefresher[T]
}

func newSnapshotCoordinator[T any](cache *snapshotCache[snapshotState[T]], refresher *snapshotRefresher[T]) *snapshotCoordinator[T] {
	if cache == nil {
		panic("daemon: snapshot coordinator cache is nil")
	}
	if refresher == nil {
		panic("daemon: snapshot coordinator refresher is nil")
	}
	return &snapshotCoordinator[T]{cache: cache, refresher: refresher}
}

// Snapshot returns fresh cached state when possible. A partial refresh accepts
// current successes and retained failed records as fresh state. A total source
// failure can return only a prior complete state; that result is stale and
// retains its generation. Caller cancellation and other refresh errors do not
// alter retained state.
func (c *snapshotCoordinator[T]) Snapshot(ctx context.Context, repositories []repo.Repo) (snapshotState[T], error) {
	if state, generation, ok := c.cache.Fresh(); ok {
		state.Generation = generation
		state.Fresh = true
		state.Stale = false
		return state, nil
	}

	outcomes, err := c.refresher.Refresh(ctx, repositories)
	if err != nil {
		return snapshotState[T]{}, err
	}

	previous, generation, hasPrevious := c.cache.Latest()
	latest := snapshotLatestByRepo(previous.Records)
	state, successes := snapshotMerge(repositories, outcomes, latest)
	if successes > 0 {
		state.Fresh = true
		generation = c.cache.Accept(state)
		accepted, _, _ := c.cache.Latest()
		accepted.Generation = generation
		accepted.Fresh = true
		accepted.Stale = false
		return accepted, nil
	}

	if hasPrevious && snapshotComplete(repositories, latest) {
		state.Records = snapshotRecordsFor(repositories, latest)
		state.Generation = generation
		state.Fresh = false
		state.Stale = true
		c.cache.Invalidate()
		return c.cache.clone(state), nil
	}

	state.Fresh = false
	state.Stale = false
	return c.cache.clone(state), nil
}

func snapshotMerge[T any](repositories []repo.Repo, outcomes []snapshotOutcome[T], latest map[repo.Repo]T) (snapshotState[T], int) {
	byRepo := make(map[repo.Repo]snapshotOutcome[T], len(outcomes))
	for _, outcome := range outcomes {
		byRepo[outcome.Repo] = outcome
	}

	state := snapshotState[T]{
		Records: make([]snapshotRecord[T], 0, len(repositories)),
		Errors:  make([]snapshotRepositoryError, 0),
	}
	successes := 0
	for _, repository := range repositories {
		outcome, found := byRepo[repository]
		if found && outcome.Err == nil {
			state.Records = append(state.Records, snapshotRecord[T]{Repo: repository, Value: outcome.Value})
			successes++
			continue
		}
		if found {
			state.Errors = append(state.Errors, snapshotRepositoryError{Repo: repository, Err: outcome.Err})
		}
		if value, ok := latest[repository]; ok {
			state.Records = append(state.Records, snapshotRecord[T]{Repo: repository, Value: value})
		}
	}
	return state, successes
}

func snapshotLatestByRepo[T any](records []snapshotRecord[T]) map[repo.Repo]T {
	latest := make(map[repo.Repo]T, len(records))
	for _, record := range records {
		latest[record.Repo] = record.Value
	}
	return latest
}

func snapshotComplete[T any](repositories []repo.Repo, values map[repo.Repo]T) bool {
	for _, repository := range repositories {
		if _, ok := values[repository]; !ok {
			return false
		}
	}
	return true
}

func snapshotRecordsFor[T any](repositories []repo.Repo, values map[repo.Repo]T) []snapshotRecord[T] {
	records := make([]snapshotRecord[T], 0, len(repositories))
	for _, repository := range repositories {
		if value, ok := values[repository]; ok {
			records = append(records, snapshotRecord[T]{Repo: repository, Value: value})
		}
	}
	return records
}

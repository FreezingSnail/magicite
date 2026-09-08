package daemon

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/dispatch"
	"github.com/FreezingSnail/magicite/internal/repo"
)

// QueueSampler captures the ready-task queues for every configured repository.
type QueueSampler struct {
	repos dispatch.Repos
	beads dispatch.Beads
	now   func() time.Time
}

// QueueSample is one point-in-time ready-task queue reading.
type QueueSample struct {
	Repositories []QueueRepository
	Failures     []QueueFailure
	Total        int
	CapturedAt   time.Time
}

// QueueRepository is a successful ready-task count for one repository.
type QueueRepository struct {
	Repo  repo.Repo
	Ready int
}

// QueueFailure identifies a repository whose ready-task count could not be read.
type QueueFailure struct {
	Repo repo.Repo
	Err  error
}

// NewQueueSampler constructs a sampler over the configured repository and bead ports.
func NewQueueSampler(repos dispatch.Repos, beads dispatch.Beads) *QueueSampler {
	return &QueueSampler{repos: repos, beads: beads, now: time.Now}
}

// Sample reads every configured repository concurrently. Failed repositories are
// retained as named failures; only successful reads contribute to Total.
func (s *QueueSampler) Sample(ctx context.Context) QueueSample {
	repositories := append([]repo.Repo(nil), s.repos.List(ctx)...)
	sort.Slice(repositories, func(left, right int) bool {
		if repositories[left].Name != repositories[right].Name {
			return repositories[left].Name < repositories[right].Name
		}
		return repositories[left].Root < repositories[right].Root
	})

	type outcome struct {
		ready int
		err   error
	}
	outcomes := make([]outcome, len(repositories))
	var group sync.WaitGroup
	group.Add(len(repositories))
	for index, repository := range repositories {
		go func() {
			defer group.Done()
			ready, err := s.beads.Ready(ctx, repository)
			outcomes[index] = outcome{ready: len(ready), err: err}
		}()
	}
	group.Wait()

	sample := QueueSample{
		Repositories: make([]QueueRepository, 0, len(repositories)),
		Failures:     make([]QueueFailure, 0),
		CapturedAt:   s.now().UTC(),
	}
	for index, repository := range repositories {
		outcome := outcomes[index]
		if outcome.err != nil {
			sample.Failures = append(sample.Failures, QueueFailure{Repo: repository, Err: outcome.err})
			continue
		}
		sample.Repositories = append(sample.Repositories, QueueRepository{Repo: repository, Ready: outcome.ready})
		sample.Total += outcome.ready
	}
	return sample
}

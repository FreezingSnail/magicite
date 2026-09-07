package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/FreezingSnail/magicite/internal/repo"
)

var errNilSnapshotRead = errors.New("daemon: nil snapshot read")

type snapshotRead[T any] func(context.Context, repo.Repo) (T, error)

type snapshotOutcome[T any] struct {
	Repo  repo.Repo
	Value T
	Err   error
}

type snapshotRefresher[T any] struct {
	mu     sync.Mutex
	limit  int
	read   snapshotRead[T]
	active *snapshotRefresh[T]
}

type snapshotRefresh[T any] struct {
	done     chan struct{}
	outcomes []snapshotOutcome[T]
}

func newSnapshotRefresher[T any](limit int, read snapshotRead[T]) *snapshotRefresher[T] {
	if limit < 1 {
		limit = 1
	}
	return &snapshotRefresher[T]{limit: limit, read: read}
}

// Refresh reads every repository once, sharing an in-flight read with concurrent callers.
func (r *snapshotRefresher[T]) Refresh(ctx context.Context, repositories []repo.Repo) ([]snapshotOutcome[T], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	flight := r.active
	if flight == nil {
		flight = &snapshotRefresh[T]{done: make(chan struct{})}
		r.active = flight
		inputs := append([]repo.Repo(nil), repositories...)
		go r.refresh(context.WithoutCancel(ctx), inputs, flight)
	}
	r.mu.Unlock()

	select {
	case <-flight.done:
		return cloneSnapshotOutcomes(flight.outcomes), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *snapshotRefresher[T]) refresh(ctx context.Context, repositories []repo.Repo, flight *snapshotRefresh[T]) {
	outcomes := make([]snapshotOutcome[T], len(repositories))
	jobs := make(chan int)
	workers := r.limit
	if workers > len(repositories) {
		workers = len(repositories)
	}

	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for index := range jobs {
				repository := repositories[index]
				value, err := r.readRepository(ctx, repository)
				outcomes[index] = snapshotOutcome[T]{Repo: repository, Value: value, Err: err}
			}
		}()
	}
	for index := range repositories {
		jobs <- index
	}
	close(jobs)
	group.Wait()

	r.mu.Lock()
	if r.active == flight {
		r.active = nil
	}
	flight.outcomes = outcomes
	r.mu.Unlock()
	close(flight.done)
}

func (r *snapshotRefresher[T]) readRepository(ctx context.Context, repository repo.Repo) (value T, err error) {
	if r.read == nil {
		return value, errNilSnapshotRead
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("daemon: snapshot read panicked: %v", recovered)
		}
	}()
	return r.read(ctx, repository)
}

func cloneSnapshotOutcomes[T any](outcomes []snapshotOutcome[T]) []snapshotOutcome[T] {
	return append([]snapshotOutcome[T]{}, outcomes...)
}

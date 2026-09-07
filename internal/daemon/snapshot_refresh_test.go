package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/repo"
)

func TestSnapshotRefresherBoundsFanoutAndPreservesOrder(t *testing.T) {
	repositories := snapshotRepos(5)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var mu sync.Mutex
	current, maximum := 0, 0
	refresher := newSnapshotRefresher(2, func(_ context.Context, repository repo.Repo) (string, error) {
		mu.Lock()
		current++
		if current > maximum {
			maximum = current
		}
		mu.Unlock()
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		mu.Lock()
		current--
		mu.Unlock()
		return repository.Name, nil
	})

	result := make(chan []snapshotOutcome[string], 1)
	go func() {
		outcomes, err := refresher.Refresh(context.Background(), repositories)
		if err != nil {
			t.Errorf("Refresh() error = %v", err)
		}
		result <- outcomes
	}()
	for range 2 {
		waitSnapshotStart(t, started)
	}
	mu.Lock()
	gotMaximum := maximum
	mu.Unlock()
	if gotMaximum != 2 {
		t.Fatalf("maximum reads = %d, want 2", gotMaximum)
	}
	close(release)

	outcomes := <-result
	if len(outcomes) != len(repositories) {
		t.Fatalf("outcomes = %#v, want %d entries", outcomes, len(repositories))
	}
	for index, outcome := range outcomes {
		if outcome.Repo != repositories[index] || outcome.Value != repositories[index].Name || outcome.Err != nil {
			t.Errorf("outcome %d = %#v, want %q success for %#v", index, outcome, repositories[index].Name, repositories[index])
		}
	}
}

func TestSnapshotRefresherCoalescesAndCopiesOutcomes(t *testing.T) {
	repositories := snapshotRepos(2)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	calls := 0
	refresher := newSnapshotRefresher(2, func(_ context.Context, repository repo.Repo) (string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		once.Do(func() { close(started) })
		<-release
		return repository.Name, nil
	})

	const callers = 8
	results := make([][]snapshotOutcome[string], callers)
	errs := make([]error, callers)
	begin := make(chan struct{})
	var group sync.WaitGroup
	for index := range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-begin
			results[index], errs[index] = refresher.Refresh(context.Background(), repositories)
		}()
	}
	close(begin)
	waitSnapshotStart(t, started)
	close(release)
	group.Wait()

	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != len(repositories) {
		t.Fatalf("reads = %d, want %d", gotCalls, len(repositories))
	}
	for index, err := range errs {
		if err != nil {
			t.Errorf("Refresh() caller %d error = %v", index, err)
		}
	}
	results[0][0].Value = "changed"
	for index := 1; index < callers; index++ {
		if results[index][0].Value != repositories[0].Name {
			t.Errorf("caller %d shares outcome collection: %#v", index, results[index])
		}
	}
}

func TestSnapshotRefresherKeepsSourceErrorsInOrderedOutcomes(t *testing.T) {
	repositories := snapshotRepos(3)
	source := errors.New("source unavailable")
	refresher := newSnapshotRefresher(2, func(_ context.Context, repository repo.Repo) (string, error) {
		if repository.Name == "repo-1" {
			return "", source
		}
		return repository.Name, nil
	})

	outcomes, err := refresher.Refresh(context.Background(), repositories)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(outcomes) != len(repositories) {
		t.Fatalf("outcomes = %#v, want %d entries", outcomes, len(repositories))
	}
	for index, outcome := range outcomes {
		if outcome.Repo != repositories[index] {
			t.Errorf("outcome %d repo = %#v, want %#v", index, outcome.Repo, repositories[index])
		}
	}
	if !errors.Is(outcomes[1].Err, source) {
		t.Errorf("outcome error = %v, want source error", outcomes[1].Err)
	}
	if outcomes[0].Err != nil || outcomes[2].Err != nil {
		t.Errorf("successful outcomes = %#v", outcomes)
	}
}

func TestSnapshotRefresherCancelledCallerDoesNotOwnFlight(t *testing.T) {
	repositories := snapshotRepos(1)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	calls := 0
	refresher := newSnapshotRefresher(1, func(_ context.Context, repository repo.Repo) (string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		once.Do(func() { close(started) })
		<-release
		return repository.Name, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() {
		_, err := refresher.Refresh(ctx, repositories)
		first <- err
	}()
	waitSnapshotStart(t, started)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Refresh() error = %v, want context.Canceled", err)
	}

	second := make(chan struct {
		outcomes []snapshotOutcome[string]
		err      error
	}, 1)
	go func() {
		outcomes, err := refresher.Refresh(context.Background(), repositories)
		second <- struct {
			outcomes []snapshotOutcome[string]
			err      error
		}{outcomes, err}
	}()
	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("reads after caller cancellation = %d, want 1", gotCalls)
	}
	close(release)
	got := <-second
	if got.err != nil || len(got.outcomes) != 1 || got.outcomes[0].Value != repositories[0].Name {
		t.Fatalf("shared Refresh() = (%#v, %v), want successful outcome", got.outcomes, got.err)
	}
}

func snapshotRepos(count int) []repo.Repo {
	repositories := make([]repo.Repo, count)
	for index := range repositories {
		repositories[index] = repo.Repo{Name: "repo-" + string(rune('0'+index))}
	}
	return repositories
}

func waitSnapshotStart(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("snapshot read did not start")
	}
}

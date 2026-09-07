package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/repo"
)

type snapshotRawRecord struct {
	Items []string
	Meta  map[string][]string
}

type snapshotReadResult struct {
	Value snapshotRawRecord
	Err   error
}

func cloneSnapshotState(state snapshotState[snapshotRawRecord]) snapshotState[snapshotRawRecord] {
	clone := snapshotState[snapshotRawRecord]{
		Records:    make([]snapshotRecord[snapshotRawRecord], len(state.Records)),
		Errors:     append([]snapshotRepositoryError(nil), state.Errors...),
		Generation: state.Generation,
		Fresh:      state.Fresh,
		Stale:      state.Stale,
	}
	for index, record := range state.Records {
		clone.Records[index] = snapshotRecord[snapshotRawRecord]{Repo: record.Repo, Value: cloneSnapshotRawRecord(record.Value)}
	}
	return clone
}

func cloneSnapshotRawRecord(record snapshotRawRecord) snapshotRawRecord {
	clone := snapshotRawRecord{Items: append([]string(nil), record.Items...), Meta: make(map[string][]string, len(record.Meta))}
	for key, values := range record.Meta {
		clone.Meta[key] = append([]string(nil), values...)
	}
	return clone
}

func TestSnapshotCoordinatorRetainsLastGoodRecordsOnPartialFailure(t *testing.T) {
	repositories := snapshotRepos(2)
	failure := errors.New("repo unavailable")
	reader := &snapshotCoordinatorReader{results: []map[repo.Repo]snapshotReadResult{
		{
			repositories[0]: {Value: snapshotRaw("old-0")},
			repositories[1]: {Value: snapshotRaw("old-1")},
		},
		{
			repositories[0]: {Value: snapshotRaw("new-0")},
			repositories[1]: {Err: failure},
		},
	}}
	coordinator, cache := newSnapshotCoordinatorForTest(reader)

	warm, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("warm Snapshot() error = %v", err)
	}
	if warm.Generation != 1 || !warm.Fresh || warm.Stale {
		t.Fatalf("warm state = %#v, want fresh generation 1", warm)
	}
	cache.Invalidate()

	got, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("partial Snapshot() error = %v", err)
	}
	assertSnapshotState(t, got, []string{"new-0", "old-1"}, []snapshotRepositoryError{{Repo: repositories[1], Err: failure}}, 2, true, false)
}

func TestSnapshotCoordinatorColdFailuresRetainNoFabricatedRecords(t *testing.T) {
	repositories := snapshotRepos(2)
	first := errors.New("first unavailable")
	second := errors.New("second unavailable")
	reader := &snapshotCoordinatorReader{results: []map[repo.Repo]snapshotReadResult{{
		repositories[0]: {Err: first},
		repositories[1]: {Err: second},
	}}}
	coordinator, _ := newSnapshotCoordinatorForTest(reader)

	got, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	assertSnapshotState(t, got, nil, []snapshotRepositoryError{{Repo: repositories[0], Err: first}, {Repo: repositories[1], Err: second}}, 0, false, false)
}

func TestSnapshotCoordinatorMarksOnlyCompleteTotalFailureStale(t *testing.T) {
	repositories := snapshotRepos(2)
	failure := errors.New("unavailable")
	reader := &snapshotCoordinatorReader{results: []map[repo.Repo]snapshotReadResult{
		{
			repositories[0]: {Value: snapshotRaw("old-0")},
			repositories[1]: {Value: snapshotRaw("old-1")},
		},
		{
			repositories[0]: {Err: failure},
			repositories[1]: {Err: failure},
		},
	}}
	coordinator, cache := newSnapshotCoordinatorForTest(reader)
	if _, err := coordinator.Snapshot(context.Background(), repositories); err != nil {
		t.Fatalf("warm Snapshot() error = %v", err)
	}
	cache.Invalidate()

	got, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("total failure Snapshot() error = %v", err)
	}
	assertSnapshotState(t, got, []string{"old-0", "old-1"}, []snapshotRepositoryError{{Repo: repositories[0], Err: failure}, {Repo: repositories[1], Err: failure}}, 1, false, true)
}

func TestSnapshotCoordinatorDoesNotMarkIncompletePriorStateStale(t *testing.T) {
	repositories := snapshotRepos(2)
	failure := errors.New("unavailable")
	reader := &snapshotCoordinatorReader{results: []map[repo.Repo]snapshotReadResult{
		{
			repositories[0]: {Value: snapshotRaw("only-good")},
			repositories[1]: {Err: failure},
		},
		{
			repositories[0]: {Err: failure},
			repositories[1]: {Err: failure},
		},
	}}
	coordinator, cache := newSnapshotCoordinatorForTest(reader)
	if _, err := coordinator.Snapshot(context.Background(), repositories); err != nil {
		t.Fatalf("partial Snapshot() error = %v", err)
	}
	cache.Invalidate()

	got, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("total failure Snapshot() error = %v", err)
	}
	assertSnapshotState(t, got, []string{"only-good"}, []snapshotRepositoryError{{Repo: repositories[0], Err: failure}, {Repo: repositories[1], Err: failure}}, 0, false, false)
}

func TestSnapshotCoordinatorReturnsIndependentCachedRawState(t *testing.T) {
	repositories := snapshotRepos(1)
	reader := &snapshotCoordinatorReader{results: []map[repo.Repo]snapshotReadResult{{
		repositories[0]: {Value: snapshotRaw("original")},
	}}}
	coordinator, _ := newSnapshotCoordinatorForTest(reader)

	first, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("first Snapshot() error = %v", err)
	}
	first.Records[0].Value.Items[0] = "mutated"
	first.Records[0].Value.Meta["labels"][0] = "mutated"

	second, err := coordinator.Snapshot(context.Background(), repositories)
	if err != nil {
		t.Fatalf("second Snapshot() error = %v", err)
	}
	assertSnapshotState(t, second, []string{"original"}, nil, 1, true, false)
	if reader.Calls() != 1 {
		t.Fatalf("reads = %d, want 1 cached read", reader.Calls())
	}
}

func TestSnapshotCoordinatorReturnsRefreshErrorWithoutMutatingCache(t *testing.T) {
	repositories := snapshotRepos(1)
	reader := &snapshotCoordinatorReader{results: []map[repo.Repo]snapshotReadResult{{
		repositories[0]: {Value: snapshotRaw("good")},
	}}}
	coordinator, cache := newSnapshotCoordinatorForTest(reader)
	if _, err := coordinator.Snapshot(context.Background(), repositories); err != nil {
		t.Fatalf("warm Snapshot() error = %v", err)
	}
	cache.Invalidate()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := coordinator.Snapshot(cancelled, repositories); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Snapshot() error = %v, want context.Canceled", err)
	}
	latest, generation, ok := cache.Latest()
	if !ok || generation != 1 || latest.Records[0].Value.Items[0] != "good" {
		t.Fatalf("Latest() after cancelled Snapshot() = (%#v, %d, %t), want retained generation 1", latest, generation, ok)
	}
}

func newSnapshotCoordinatorForTest(reader *snapshotCoordinatorReader) (*snapshotCoordinator[snapshotRawRecord], *snapshotCache[snapshotState[snapshotRawRecord]]) {
	clock := &snapshotCacheClock{now: time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)}
	cache := newSnapshotCache(time.Minute, clock.Now, cloneSnapshotState)
	refresher := newSnapshotRefresher(2, reader.Read)
	return newSnapshotCoordinator(cache, refresher), cache
}

type snapshotCoordinatorReader struct {
	mu      sync.Mutex
	results []map[repo.Repo]snapshotReadResult
	calls   int
}

func (r *snapshotCoordinatorReader) Read(_ context.Context, repository repo.Repo) (snapshotRawRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cycle := r.calls / 2
	if cycle >= len(r.results) {
		cycle = len(r.results) - 1
	}
	r.calls++
	result := r.results[cycle][repository]
	return result.Value, result.Err
}

func (r *snapshotCoordinatorReader) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func snapshotRaw(value string) snapshotRawRecord {
	return snapshotRawRecord{Items: []string{value}, Meta: map[string][]string{"labels": {value}}}
}

func assertSnapshotState(t *testing.T, got snapshotState[snapshotRawRecord], values []string, wantErrors []snapshotRepositoryError, generation uint64, fresh, stale bool) {
	t.Helper()
	if got.Generation != generation || got.Fresh != fresh || got.Stale != stale {
		t.Errorf("metadata = (generation %d, fresh %t, stale %t), want (%d, %t, %t)", got.Generation, got.Fresh, got.Stale, generation, fresh, stale)
	}
	if len(got.Records) != len(values) {
		t.Fatalf("records = %#v, want %d records", got.Records, len(values))
	}
	for index, value := range values {
		if got.Records[index].Value.Items[0] != value {
			t.Errorf("record %d value = %#v, want %q", index, got.Records[index].Value, value)
		}
	}
	if len(got.Errors) != len(wantErrors) {
		t.Fatalf("errors = %#v, want %#v", got.Errors, wantErrors)
	}
	for index, want := range wantErrors {
		if got.Errors[index].Repo != want.Repo || !errors.Is(got.Errors[index].Err, want.Err) {
			t.Errorf("error %d = %#v, want %#v", index, got.Errors[index], want)
		}
	}
}

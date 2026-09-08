package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/dispatch"
	"github.com/FreezingSnail/magicite/internal/repo"
)

type queueBeads struct {
	testBeads
	ready func(context.Context, repo.Repo) ([]dispatch.ReadyEntry, error)
}

func (b queueBeads) Ready(ctx context.Context, repository repo.Repo) ([]dispatch.ReadyEntry, error) {
	return b.ready(ctx, repository)
}

func queueRepository(t *testing.T, name string) repo.Repo {
	t.Helper()
	record, ok := repo.Make(t.TempDir(), name, name, "main")
	if !ok {
		t.Fatal("repo.Make() failed")
	}
	return record
}

func TestQueueSamplerSampleEmpty(t *testing.T) {
	sampler := NewQueueSampler(testRepos{}, queueBeads{ready: func(context.Context, repo.Repo) ([]dispatch.ReadyEntry, error) {
		t.Fatal("Ready() called for empty repository list")
		return nil, nil
	}})
	captured := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.FixedZone("test", -4*60*60))
	sampler.now = func() time.Time { return captured }

	sample := sampler.Sample(context.Background())
	if sample.Total != 0 || len(sample.Repositories) != 0 || len(sample.Failures) != 0 || !sample.CapturedAt.Equal(captured) {
		t.Fatalf("Sample() = %#v", sample)
	}
	if sample.Repositories == nil || sample.Failures == nil {
		t.Fatalf("Sample() collections = %#v, want nonnil", sample)
	}
}

func TestQueueSamplerSampleOrdersPartialResultsAndKeepsZero(t *testing.T) {
	alpha := queueRepository(t, "alpha")
	beta := queueRepository(t, "beta")
	broken := queueRepository(t, "broken")
	failed := errors.New("bd unavailable")
	replies := map[string]struct {
		count int
		err   error
	}{
		"alpha":  {count: 2},
		"beta":   {count: 0},
		"broken": {err: failed},
	}
	sampler := NewQueueSampler(testRepos{records: []repo.Repo{broken, beta, alpha}}, queueBeads{ready: func(_ context.Context, repository repo.Repo) ([]dispatch.ReadyEntry, error) {
		reply := replies[repository.Name]
		return make([]dispatch.ReadyEntry, reply.count), reply.err
	}})

	sample := sampler.Sample(context.Background())
	if sample.Total != 2 || len(sample.Repositories) != 2 || len(sample.Failures) != 1 {
		t.Fatalf("Sample() = %#v", sample)
	}
	if got := sample.Repositories[0]; got.Repo.Name != "alpha" || got.Ready != 2 {
		t.Fatalf("first repository = %#v", got)
	}
	if got := sample.Repositories[1]; got.Repo.Name != "beta" || got.Ready != 0 {
		t.Fatalf("second repository = %#v", got)
	}
	if got := sample.Failures[0]; got.Repo.Name != "broken" || !errors.Is(got.Err, failed) {
		t.Fatalf("failure = %#v", got)
	}
}

func TestQueueSamplerSamplePassesCancellationToEveryRead(t *testing.T) {
	alpha := queueRepository(t, "alpha")
	beta := queueRepository(t, "beta")
	gamma := queueRepository(t, "gamma")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var mu sync.Mutex
	reads := make(map[string]bool)
	sampler := NewQueueSampler(testRepos{records: []repo.Repo{alpha, beta, gamma}}, queueBeads{ready: func(ctx context.Context, repository repo.Repo) ([]dispatch.ReadyEntry, error) {
		if err := ctx.Err(); !errors.Is(err, context.Canceled) {
			t.Errorf("Ready(%s) context error = %v, want canceled", repository.Name, err)
		}
		mu.Lock()
		reads[repository.Name] = true
		mu.Unlock()
		return nil, ctx.Err()
	}})

	sample := sampler.Sample(ctx)
	if len(reads) != 3 || !reads["alpha"] || !reads["beta"] || !reads["gamma"] {
		t.Fatalf("Ready() reads = %#v", reads)
	}
	if sample.Total != 0 || len(sample.Repositories) != 0 || len(sample.Failures) != 3 {
		t.Fatalf("Sample() = %#v", sample)
	}
	for index, name := range []string{"alpha", "beta", "gamma"} {
		if got := sample.Failures[index]; got.Repo.Name != name || !errors.Is(got.Err, context.Canceled) {
			t.Fatalf("failure[%d] = %#v", index, got)
		}
	}
}

func TestQueueSamplerSampleRetainsAllFailedRepositories(t *testing.T) {
	alpha := queueRepository(t, "alpha")
	beta := queueRepository(t, "beta")
	failed := errors.New("bd unavailable")
	sampler := NewQueueSampler(testRepos{records: []repo.Repo{beta, alpha}}, queueBeads{ready: func(context.Context, repo.Repo) ([]dispatch.ReadyEntry, error) {
		return nil, failed
	}})

	sample := sampler.Sample(context.Background())
	if sample.Total != 0 || len(sample.Repositories) != 0 || len(sample.Failures) != 2 {
		t.Fatalf("Sample() = %#v", sample)
	}
	for index, name := range []string{"alpha", "beta"} {
		if got := sample.Failures[index]; got.Repo.Name != name || !errors.Is(got.Err, failed) {
			t.Fatalf("failure[%d] = %#v", index, got)
		}
	}
}

package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/dispatch"
	"github.com/FreezingSnail/magicite/internal/metrics"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestCoreMetricsComposesSharedRegistryQueueAndBus(t *testing.T) {
	alpha := queueRepository(t, "alpha")
	broken := queueRepository(t, "broken")
	captured := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC)
	now := captured
	registry := metrics.NewRegistry(func() time.Time { return now })
	now = captured.Add(90 * time.Second)
	registry.RecordLifecycle(metrics.LifecyclePickup)
	registry.RecordLand(metrics.LandConflict)
	registry.StartSession()
	registry.FinishSession(metrics.RoleImplementer, 3*time.Second, false)

	var mu sync.Mutex
	reads := make(map[string]int)
	deps := coreDeps(t)
	deps.Repos = testRepos{records: []repo.Repo{broken, alpha}}
	deps.Beads = queueBeads{ready: func(_ context.Context, record repo.Repo) ([]dispatch.ReadyEntry, error) {
		mu.Lock()
		reads[record.Name]++
		mu.Unlock()
		if record.Name == "broken" {
			return nil, errors.New("bd unavailable")
		}
		return []dispatch.ReadyEntry{{Task: "ready-1"}, {Task: "ready-2"}}, nil
	}}
	deps.Metrics = registry
	deps.QueueSampler = NewQueueSampler(deps.Repos, deps.Beads)
	deps.QueueSampler.now = func() time.Time { return captured }
	deps.Bus.Publish(wire.Event{Kind: wire.KindWarn})

	capability, err := NewCore(deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := capability.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics() error = %v", err)
	}
	if reads["alpha"] != 1 || reads["broken"] != 1 {
		t.Fatalf("queue reads = %#v, want one sample", reads)
	}
	if result.StartedAt.IsZero() || result.UptimeSeconds != 90 || result.Bus.Published != 1 || result.Bus.Dropped != 0 {
		t.Fatalf("Metrics() metadata = %#v", result)
	}
	if len(result.Lifecycle) != 9 || result.Lifecycle[0].Key != metrics.LifecyclePickup || result.Lifecycle[0].Count != 1 || result.Lifecycle[8].Key != metrics.LifecycleError {
		t.Fatalf("Metrics() lifecycle = %#v", result.Lifecycle)
	}
	if len(result.Land) != 4 || result.Land[1].Key != metrics.LandConflict || result.Land[1].Count != 1 {
		t.Fatalf("Metrics() land = %#v", result.Land)
	}
	if result.Sessions.Completed != 1 || len(result.Roles) != 5 || result.Roles[2].Role != metrics.RoleImplementer || result.Roles[2].TotalSeconds != 3 {
		t.Fatalf("Metrics() sessions and roles = %#v %#v", result.Sessions, result.Roles)
	}
	if len(result.Queue) != 1 || result.Queue[0].Repo != "alpha" || result.Queue[0].Depth != 2 || result.QueueTotal != 2 || result.QueueSampledAt == nil || !result.QueueSampledAt.Equal(captured) {
		t.Fatalf("Metrics() partial queue = %#v", result)
	}
}

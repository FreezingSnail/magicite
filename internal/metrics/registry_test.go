package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestRegistrySnapshotZeroValuesAndUptime(t *testing.T) {
	started := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.FixedZone("test", -4*60*60))
	now := started
	registry := NewRegistry(func() time.Time { return now })
	now = started.Add(90*time.Second + 999*time.Millisecond)

	snapshot := registry.Snapshot()
	if !snapshot.StartedAt.Equal(started) || snapshot.Uptime != 90*time.Second+999*time.Millisecond {
		t.Fatalf("Snapshot() time = %#v", snapshot)
	}
	if len(snapshot.Lifecycle) != len(lifecycleKeys) || len(snapshot.Land) != len(landKeys) || len(snapshot.Roles) != len(roleKeys) {
		t.Fatalf("Snapshot() fixed maps = %#v", snapshot)
	}
	if snapshot.Lifecycle[LifecyclePickup] != 0 || snapshot.Land[LandOK] != 0 || snapshot.Roles[RoleImplementer] != (RoleTotal{}) {
		t.Fatalf("Snapshot() zero metrics = %#v", snapshot)
	}
	if snapshot.Queue == nil || len(snapshot.Queue) != 0 || snapshot.QueueSampledAt != nil {
		t.Fatalf("Snapshot() queue = %#v", snapshot)
	}
}

func TestRegistryRecordsFixedLifecycleAndLandKeys(t *testing.T) {
	registry := NewRegistry(func() time.Time { return time.Time{} })
	if !registry.RecordLifecycle(LifecyclePickup) || !registry.RecordLifecycle(LifecyclePickup) || !registry.RecordLand(LandConflict) {
		t.Fatal("known keys rejected")
	}
	if registry.RecordLifecycle("unknown") || registry.RecordLand("unknown") {
		t.Fatal("unknown key accepted")
	}

	snapshot := registry.Snapshot()
	if snapshot.Lifecycle[LifecyclePickup] != 2 || snapshot.Land[LandConflict] != 1 {
		t.Fatalf("Snapshot() counters = %#v", snapshot)
	}
	if len(snapshot.Lifecycle) != len(lifecycleKeys) || len(snapshot.Land) != len(landKeys) {
		t.Fatalf("unknown key grew labels: %#v", snapshot)
	}
}

func TestRegistrySessionGaugesAndRoleDurations(t *testing.T) {
	registry := NewRegistry(func() time.Time { return time.Time{} })
	registry.StartSession()
	registry.StartSession()
	if !registry.FinishSession(RoleImplementer, 3*time.Second, false) {
		t.Fatal("FinishSession() rejected implementer")
	}
	if !registry.FinishSession(RoleImplementer, -time.Second, true) {
		t.Fatal("FinishSession() rejected implementer")
	}
	if registry.FinishSession("unknown", time.Second, false) {
		t.Fatal("FinishSession() accepted unknown role")
	}

	snapshot := registry.Snapshot()
	if got, want := snapshot.Sessions, (SessionGauges{Peak: 2, Completed: 1, Failed: 1}); got != want {
		t.Fatalf("session gauges = %#v, want %#v", got, want)
	}
	if got, want := snapshot.Roles[RoleImplementer], (RoleTotal{Sessions: 2, Duration: 3 * time.Second}); got != want {
		t.Fatalf("implementer total = %#v, want %#v", got, want)
	}
	registry.StartSession()
	if got := registry.Snapshot().Sessions.Peak; got != 2 {
		t.Fatalf("peak after restart = %d, want 2", got)
	}
}

func TestRegistrySetQueueReplacesSampleAndClampsDepth(t *testing.T) {
	registry := NewRegistry(func() time.Time { return time.Time{} })
	sampled := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC)
	registry.SetQueue(map[string]int{"beta": 2, "alpha": -1, "": 8}, sampled)

	snapshot := registry.Snapshot()
	if got, want := snapshot.Queue, []QueueDepth{{Repository: "alpha", Depth: 0}, {Repository: "beta", Depth: 2}}; !equalQueue(got, want) {
		t.Fatalf("queue = %#v, want %#v", got, want)
	}
	if snapshot.QueueTotal != 2 || snapshot.QueueSampledAt == nil || !snapshot.QueueSampledAt.Equal(sampled) {
		t.Fatalf("queue sample = %#v", snapshot)
	}

	registry.SetQueue(map[string]int{"gamma": 3}, sampled.Add(time.Minute))
	snapshot = registry.Snapshot()
	if got, want := snapshot.Queue, []QueueDepth{{Repository: "gamma", Depth: 3}}; !equalQueue(got, want) {
		t.Fatalf("replaced queue = %#v, want %#v", got, want)
	}
	if snapshot.QueueTotal != 3 {
		t.Fatalf("replaced queue total = %d, want 3", snapshot.QueueTotal)
	}
}

func TestRegistrySnapshotsOwnCollections(t *testing.T) {
	registry := NewRegistry(func() time.Time { return time.Time{} })
	registry.RecordLifecycle(LifecyclePickup)
	registry.FinishSession(RoleDesigner, time.Second, false)
	sampled := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC)
	registry.SetQueue(map[string]int{"magicite": 1}, sampled)

	first := registry.Snapshot()
	first.Lifecycle[LifecyclePickup] = 100
	first.Roles[RoleDesigner] = RoleTotal{}
	first.Queue[0].Depth = 100
	*first.QueueSampledAt = time.Time{}

	second := registry.Snapshot()
	if second.Lifecycle[LifecyclePickup] != 1 || second.Roles[RoleDesigner] != (RoleTotal{Sessions: 1, Duration: time.Second}) || second.Queue[0].Depth != 1 || !second.QueueSampledAt.Equal(sampled) {
		t.Fatalf("snapshot mutation leaked: %#v", second)
	}
}

func TestRegistryClampsNegativeUptime(t *testing.T) {
	started := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC)
	now := started
	registry := NewRegistry(func() time.Time { return now })
	now = started.Add(-time.Second)
	if got := registry.Snapshot().Uptime; got != 0 {
		t.Fatalf("negative uptime = %s, want 0", got)
	}
}

func TestRegistryConcurrentRecordAndSnapshot(t *testing.T) {
	registry := NewRegistry(func() time.Time { return time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC) })
	const workers = 16
	const rounds = 100

	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			for range rounds {
				registry.RecordLifecycle(LifecyclePickup)
				registry.StartSession()
				registry.FinishSession(RoleReviewer, time.Second, false)
				registry.SetQueue(map[string]int{"magicite": 1}, time.Time{})
				_ = registry.Snapshot()
			}
		})
	}
	group.Wait()

	snapshot := registry.Snapshot()
	want := uint64(workers * rounds)
	if snapshot.Lifecycle[LifecyclePickup] != want || snapshot.Sessions.Completed != want || snapshot.Sessions.Active != 0 || snapshot.Sessions.Peak == 0 || snapshot.Roles[RoleReviewer].Sessions != want {
		t.Fatalf("concurrent snapshot = %#v", snapshot)
	}
}

func equalQueue(got, want []QueueDepth) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

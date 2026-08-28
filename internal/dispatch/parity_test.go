package dispatch

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
	"github.com/connorfranc/magicite/internal/repo"
)

func TestMaduinDispatchParity(t *testing.T) {
	for _, name := range orchestrationNames("TestMaduinDispatchParity") {
		t.Run(name, func(t *testing.T) {
			switch name {
			case "maduin-test-dispatch-queue-priority", "maduin-test-dispatch-queue-round-robin", "maduin-test-dispatch-queue-no-starvation", "maduin-test-dispatch-queue-deterministic":
				assertQueueReplay(t)
			case "maduin-test-dispatch-implement-concurrency-cap":
				d, _ := newRegistryDispatcher(t)
				if d.RoleCap(Implementer) != 2 || d.RoleCap(Designer) != 2 {
					t.Fatal("configured role caps not retained")
				}
			case "maduin-test-dispatch-orphaned-tasks", "maduin-test-dispatch-recover-redispatches-orphans":
				d, _ := newRegistryDispatcher(t)
				d.Add(Session{Handle: "live", Task: "live", Role: Implementer, Seat: "ifrit"})
				if got := d.Orphans([]string{"live", "orphan", "orphan"}); !reflect.DeepEqual(got, []string{"orphan", "orphan"}) {
					t.Fatalf("Orphans() = %q", got)
				}
			case "maduin-test-dispatch-seat-backend-routing-records-backend", "maduin-test-dispatch-sticky-backend-for-diff-and-delete":
				d, _ := newRegistryDispatcher(t)
				d.Add(Session{Handle: "h", Task: "task", Role: Implementer, Seat: "ifrit", Backend: "kiro"})
				if got := d.Sessions()[0].Backend; got != "kiro" {
					t.Fatalf("session backend = %q", got)
				}
			case "maduin-test-dispatch-syncs-seat-before-claim":
				assertSeatSyncBeforeClaim(t)
			}
		})
	}
}

func orchestrationNames(owner string) []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, owner+"/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func assertQueueReplay(t *testing.T) {
	t.Helper()
	alpha := readyRepo(t, "/alpha", "alpha")
	beta := readyRepo(t, "/beta", "beta")
	got := MergeReady([]RepoReady{
		{Repo: alpha, Entries: []ReadyEntry{{Task: "a1", Priority: "1"}, {Task: "a2", Priority: "1"}, {Task: "a3", Priority: "2"}}},
		{Repo: beta, Entries: []ReadyEntry{{Task: "b1", Priority: "1"}, {Task: "b2", Priority: "2"}}},
	})
	want := []ReadyEntry{{Repo: alpha, Task: "a1", Priority: "1"}, {Repo: beta, Task: "b1", Priority: "1"}, {Repo: alpha, Task: "a2", Priority: "1"}, {Repo: alpha, Task: "a3", Priority: "2"}, {Repo: beta, Task: "b2", Priority: "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeReady() = %#v, want %#v", got, want)
	}
	if again := MergeReady([]RepoReady{{Repo: alpha, Entries: []ReadyEntry{{Task: "a1", Priority: "1"}, {Task: "a2", Priority: "1"}, {Task: "a3", Priority: "2"}}}, {Repo: beta, Entries: []ReadyEntry{{Task: "b1", Priority: "1"}, {Task: "b2", Priority: "2"}}}}); !reflect.DeepEqual(again, got) {
		t.Fatal("queue output is not deterministic")
	}
}

func assertSeatSyncBeforeClaim(t *testing.T) {
	t.Helper()
	order := []string{}
	beads := &fakeBeads{
		claim: func(context.Context, repo.Repo, string) error {
			order = append(order, "claim")
			return nil
		},
	}
	workspaces := readyWorkspaces()
	workspaces.sync = func(context.Context, repo.Repo, string) (SyncResult, error) {
		order = append(order, "sync")
		return SyncOK, nil
	}
	dispatcher, _ := spawnDispatcher(t, beads, workspaces, &fakeRunner{}, &fakeGate{})
	_ = dispatcher.Implement(context.Background(), spawnRepo(t), "task-1")
	if !reflect.DeepEqual(order, []string{"sync", "claim"}) {
		t.Fatalf("seat/claim order = %q, want [sync claim]", order)
	}
}

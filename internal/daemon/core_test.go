package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/decomp"
	"github.com/FreezingSnail/magicite/internal/dispatch"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestNewCoreRejectsNilDependencies(t *testing.T) {
	base := coreDeps(t)
	for _, test := range []struct {
		name  string
		set   func(*Deps)
		field string
	}{
		{"dispatcher", func(d *Deps) { d.Dispatcher = nil }, "Dispatcher"},
		{"beads", func(d *Deps) { d.Beads = nil }, "Beads"},
		{"repos", func(d *Deps) { d.Repos = nil }, "Repos"},
		{"gate", func(d *Deps) { d.Gate = nil }, "Gate"},
		{"bus", func(d *Deps) { d.Bus = nil }, "Bus"},
	} {
		t.Run(test.name, func(t *testing.T) {
			deps := base
			test.set(&deps)
			_, err := NewCore(deps)
			var dependency *DepsError
			if !errors.As(err, &dependency) || dependency.Field != test.field {
				t.Fatalf("NewCore() error = %T %v, want %s dependency error", err, err, test.field)
			}
		})
	}
}

func TestCoreTasksReturnsReadyTasksAndMissingRepo(t *testing.T) {
	deps := coreDeps(t)
	record := testRepo(t)
	deps.Repos = testRepos{records: []repo.Repo{record}}
	deps.Beads = testBeads{ready: []dispatch.ReadyEntry{{Repo: record, Task: "magicite-1", Priority: "1"}}}
	capability, err := NewCore(deps)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := capability.Tasks(context.Background(), wire.TasksParams{Repo: record.Name})
	if err != nil {
		t.Fatalf("Tasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "magicite-1" || tasks[0].Repo != record.Name || tasks[0].Priority != 1 {
		t.Fatalf("Tasks() = %#v", tasks)
	}
	_, err = capability.Tasks(context.Background(), wire.TasksParams{Repo: "missing"})
	if !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("Tasks(missing) error = %v, want not found", err)
	}
}

func TestCoreSeatsIncludesIdleConfiguredSeat(t *testing.T) {
	deps := coreDeps(t)
	deps.Config.Fleet.Seats = []config.SeatConfig{{Name: "ifrit"}}
	capability, err := NewCore(deps)
	if err != nil {
		t.Fatal(err)
	}
	seats, err := capability.Seats(context.Background())
	if err != nil {
		t.Fatalf("Seats() error = %v", err)
	}
	if len(seats) == 0 || seats[0].Name != "ifrit" || seats[0].Role != "implementer" || seats[0].Busy {
		t.Fatalf("Seats() = %#v", seats)
	}
}

func TestCoreSnapshotComposesCachedFleetState(t *testing.T) {
	deps := coreDeps(t)
	record := testRepo(t)
	deps.Repos = testRepos{records: []repo.Repo{record}}
	deps.Beads = testBeads{all: []bd.Bead{
		{ID: "closed", Title: "closed", Status: "closed", Priority: 2, Labels: []string{"staged"}, Dependencies: []bd.Dependency{}},
		{ID: "open", Title: "open", Status: "open", Priority: 1, Labels: []string{}, Dependencies: []bd.Dependency{}},
	}}
	deps.Bus.Publish(wire.Event{Kind: wire.KindWarn})
	capability, err := NewCore(deps)
	if err != nil {
		t.Fatal(err)
	}

	result, err := capability.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if result.ModelVersion != snapshotModelVersion || result.Generation != 1 || !result.Fresh || result.Stale || result.Cursor != deps.Bus.Last() || result.CapturedAt.IsZero() {
		t.Fatalf("Snapshot() metadata = %#v", result)
	}
	if result.Runtime.Repos != 1 || len(result.Repositories) != 1 || len(result.Seats) == 0 || len(result.Sessions) != 0 {
		t.Fatalf("Snapshot() fleet = %#v", result)
	}
	if len(result.Beads) != 2 || result.Beads[0].ID != "closed" || result.Beads[0].Dispatch.Reason == nil || *result.Beads[0].Dispatch.Reason != unknownEligibilityReason {
		t.Fatalf("Snapshot() beads = %#v", result.Beads)
	}
	if len(result.StatusCounts) != 2 || result.StatusCounts[0].Status != "closed" || result.StatusCounts[0].Count != 1 || result.StatusCounts[1].Status != "open" || result.StatusCounts[1].Count != 1 {
		t.Fatalf("Snapshot() status counts = %#v", result.StatusCounts)
	}
	result.Beads[0].Labels[0] = "mutated"
	second, err := capability.Snapshot(context.Background())
	if err != nil || second.Beads[0].Labels[0] != "staged" {
		t.Fatalf("second Snapshot() = %#v, %v", second, err)
	}
}

func coreDeps(t *testing.T) Deps {
	t.Helper()
	return Deps{Config: config.Default(), Dispatcher: &dispatch.Dispatcher{}, Beads: testBeads{}, Repos: testRepos{}, Gate: testGate{}, Bus: server.NewBus(1), Version: "test"}
}

func testRepo(t *testing.T) repo.Repo {
	t.Helper()
	record, ok := repo.Make(t.TempDir(), "magicite", "magicite", "main")
	if !ok {
		t.Fatal("repo.Make() failed")
	}
	return record
}

func TestCoreSnapshotReportsExactRepositoryError(t *testing.T) {
	deps := coreDeps(t)
	record := testRepo(t)
	deps.Repos = testRepos{records: []repo.Repo{record}}
	deps.Beads = testBeads{queryErr: errors.New("bd unavailable")}
	capability, err := NewCore(deps)
	if err != nil {
		t.Fatal(err)
	}

	result, err := capability.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if result.Generation != 0 || result.Fresh || result.Stale || len(result.Beads) != 0 || len(result.RepositoryErrors) != 1 {
		t.Fatalf("Snapshot() failure state = %#v", result)
	}
	if got := result.RepositoryErrors[0]; got.Repository != record.Name || got.Error != "bd unavailable" {
		t.Fatalf("Snapshot() repository error = %#v", got)
	}
}

type testBeads struct {
	ready    []dispatch.ReadyEntry
	all      []bd.Bead
	queryErr error
}

func (b testBeads) Query(context.Context, repo.Repo, string) ([]bd.Bead, error) {
	return append([]bd.Bead{}, b.all...), b.queryErr
}

func (b testBeads) Ready(context.Context, repo.Repo) ([]dispatch.ReadyEntry, error) {
	return b.ready, nil
}
func (testBeads) Show(context.Context, repo.Repo, string) (dispatch.Spec, error) {
	return dispatch.Spec{}, nil
}
func (testBeads) Claim(context.Context, repo.Repo, string) error                    { return nil }
func (testBeads) Release(context.Context, repo.Repo, string) error                  { return nil }
func (testBeads) Close(context.Context, repo.Repo, string, string) error            { return nil }
func (testBeads) Comment(context.Context, repo.Repo, string, string) error          { return nil }
func (testBeads) Difficulty(context.Context, repo.Repo, string) (string, error)     { return "", nil }
func (testBeads) HumanOnly(context.Context, repo.Repo, string) (bool, error)        { return false, nil }
func (testBeads) InProgress(context.Context, repo.Repo) ([]string, error)           { return nil, nil }
func (testBeads) OpenEpics(context.Context, repo.Repo) ([]string, error)            { return nil, nil }
func (testBeads) EpicChildren(context.Context, repo.Repo, string) ([]string, error) { return nil, nil }
func (testBeads) EpicOpenChildren(context.Context, repo.Repo, string) ([]string, error) {
	return nil, nil
}
func (testBeads) DriftFixTasks(context.Context, repo.Repo) ([]string, error) { return nil, nil }
func (testBeads) CancelAll(context.Context) error                            { return nil }

type testRepos struct{ records []repo.Repo }

func (r testRepos) List(context.Context) []repo.Repo { return r.records }
func (r testRepos) Current(context.Context, string) (repo.Repo, error) {
	if len(r.records) == 0 {
		return repo.Repo{}, &repo.NotFoundError{}
	}
	return r.records[0], nil
}

type testGate struct{}

func (testGate) Hold(context.Context, repo.Repo) (bool, error)               { return false, nil }
func (testGate) DueEpic(context.Context, repo.Repo, string) (string, error)  { return "", nil }
func (testGate) GateEpic(context.Context, repo.Repo, string) (string, error) { return "", nil }
func (testGate) ReviewPlan(context.Context, repo.Repo, string) (dispatch.RunSpec, error) {
	return dispatch.RunSpec{}, nil
}
func (testGate) NoteSession(string, repo.Repo, string)                {}
func (testGate) CompleteReview(context.Context, string, string) error { return nil }
func (testGate) AbortReview(context.Context, string, string) error    { return nil }
func (testGate) DecompositionVerdict(context.Context, repo.Repo, string) ([]decomp.Violation, error) {
	return nil, nil
}

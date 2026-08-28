package daemon

import (
	"context"
	"errors"
	"testing"

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

type testBeads struct{ ready []dispatch.ReadyEntry }

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

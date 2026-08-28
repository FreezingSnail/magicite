package dispatch

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/repo"
)

func TestDecomposeEpicMarksSessionAndLogs(t *testing.T) {
	repository := spawnRepo(t)
	dispatcher, logs := spawnDispatcher(t, &fakeBeads{}, readyWorkspaces(), &fakeRunner{}, &fakeGate{})

	if got := dispatcher.DecomposeEpic(context.Background(), repository, "epic-1"); got != "handle" {
		t.Fatalf("DecomposeEpic() = %q, want handle", got)
	}
	sessions := dispatcher.Sessions()
	if len(sessions) != 1 || !sessions[0].Decomposition || sessions[0].Task != "epic-1" || sessions[0].Role != Designer {
		t.Fatalf("sessions = %#v, want marked designer decomposition", sessions)
	}
	last := (*logs)[len(*logs)-1]
	if last.kind != "decompose" || last.fields["epic"] != "epic-1" || last.fields["repo"] != repository.LogName() || last.fields["handle"] != "handle" {
		t.Fatalf("last log = %#v, want decomposition event", last)
	}
}

func TestDecomposeEpicLogsAtDesignerCap(t *testing.T) {
	repository := spawnRepo(t)
	dispatcher, logs := spawnDispatcher(t, &fakeBeads{}, readyWorkspaces(), &fakeRunner{}, &fakeGate{})
	dispatcher.Add(Session{Handle: "busy", Role: Designer, Seat: "ramuh"})

	if got := dispatcher.DecomposeEpic(context.Background(), repository, "epic-1"); got != "" {
		t.Fatalf("DecomposeEpic() = %q, want no handle", got)
	}
	if got := len(*logs); got != 1 {
		t.Fatalf("logs = %#v, want one at-cap event", *logs)
	}
	fields := (*logs)[0].fields
	if (*logs)[0].kind != "decompose" || fields["result"] != "at-cap" || fields["active"] != 1 || fields["seats"] != 1 || fields["cap"] != 1 {
		t.Fatalf("log = %#v, want at-cap occupancy", (*logs)[0])
	}
}

func TestDecomposeEpicWarnsWhenDispatchRefused(t *testing.T) {
	repository := spawnRepo(t)
	workspaces := readyWorkspaces()
	workspaces.sync = func(context.Context, repo.Repo, string) (SyncResult, error) { return SyncConflict, nil }
	dispatcher, logs := spawnDispatcher(t, &fakeBeads{}, workspaces, &fakeRunner{}, &fakeGate{})

	if got := dispatcher.DecomposeEpic(context.Background(), repository, "epic-1"); got != "" {
		t.Fatalf("DecomposeEpic() = %q, want no handle", got)
	}
	last := (*logs)[len(*logs)-1]
	if last.kind != "decompose" || last.fields["reason"] != "sync refusal or claim failure" {
		t.Fatalf("last log = %#v, want decomposition refusal", last)
	}
}

func TestEpicPassPartitionsDecomposeAndGate(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{
		epicChildren: func(_ context.Context, _ repo.Repo, epic string) ([]string, error) {
			switch epic {
			case "new":
				return nil, nil
			case "closed", "open":
				return []string{"child"}, nil
			default:
				t.Fatalf("EpicChildren(%q)", epic)
				return nil, nil
			}
		},
		epicOpenChildren: func(_ context.Context, _ repo.Repo, epic string) ([]string, error) {
			if epic == "closed" {
				return nil, nil
			}
			return []string{"child"}, nil
		},
	}
	gate := &fakeGate{}
	dispatcher, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, gate)

	if ok := dispatcher.EpicPass(context.Background(), repository, []string{"new", "closed", "open", "new"}); !ok {
		t.Fatal("EpicPass() = false, want complete pass")
	}
	sessions := dispatcher.Sessions()
	if len(sessions) != 1 || sessions[0].Task != "new" || !sessions[0].Decomposition {
		t.Fatalf("sessions = %#v, want one new decomposition", sessions)
	}
	if got := epicCalls(gate.Calls(), "GateEpic"); !reflect.DeepEqual(got, []string{"closed"}) {
		t.Fatalf("gated = %#v, want closed", got)
	}
	if got := epicCalls(beads.Calls(), "EpicChildren"); !sameEpics(got, []string{"new", "closed", "open"}) {
		t.Fatalf("children queries = %#v, want each unique epic", got)
	}
}

func TestEpicPassPartialChildrenGatesNothing(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{
		epicChildren: func(_ context.Context, _ repo.Repo, epic string) ([]string, error) {
			if epic == "broken" {
				return nil, errors.New("bd unavailable")
			}
			return []string{"child"}, nil
		},
		epicOpenChildren: func(context.Context, repo.Repo, string) ([]string, error) { return nil, nil },
	}
	gate := &fakeGate{}
	dispatcher, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, gate)

	if ok := dispatcher.EpicPass(context.Background(), repository, []string{"closed", "broken"}); ok {
		t.Fatal("EpicPass() = true with failed children query")
	}
	if got := epicCalls(gate.Calls(), "GateEpic"); len(got) != 0 {
		t.Fatalf("gated = %#v, want none after partial pass", got)
	}
}

func TestEpicPassPartialOpenChildrenGatesNothing(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{
		epicChildren: func(context.Context, repo.Repo, string) ([]string, error) { return []string{"child"}, nil },
		epicOpenChildren: func(_ context.Context, _ repo.Repo, epic string) ([]string, error) {
			if epic == "broken" {
				return nil, errors.New("bd unavailable")
			}
			return nil, nil
		},
	}
	gate := &fakeGate{}
	dispatcher, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, gate)

	if ok := dispatcher.EpicPass(context.Background(), repository, []string{"closed", "broken"}); ok {
		t.Fatal("EpicPass() = true with failed open-children query")
	}
	if got := epicCalls(gate.Calls(), "GateEpic"); len(got) != 0 {
		t.Fatalf("gated = %#v, want none after partial pass", got)
	}
}

func TestEpicPassDrainingDoesNothing(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{}
	gate := &fakeGate{}
	dispatcher, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, gate)
	dispatcher.stateMu.Lock()
	dispatcher.draining = true
	dispatcher.stateMu.Unlock()

	if ok := dispatcher.EpicPass(context.Background(), repository, []string{"epic-1"}); ok {
		t.Fatal("EpicPass() = true while draining")
	}
	if got := beads.Calls(); len(got) != 0 {
		t.Fatalf("bead calls = %#v, want none while draining", got)
	}
	if got := gate.Calls(); len(got) != 0 {
		t.Fatalf("gate calls = %#v, want none while draining", got)
	}
}

func TestEpicPassesRunsEachRepository(t *testing.T) {
	alpha := readyRepo(t, "/alpha", "alpha")
	beta := readyRepo(t, "/beta", "beta")
	beads := &fakeBeads{
		epicChildren:     func(context.Context, repo.Repo, string) ([]string, error) { return []string{"child"}, nil },
		epicOpenChildren: func(context.Context, repo.Repo, string) ([]string, error) { return []string{"child"}, nil },
	}
	dispatcher, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, &fakeGate{})

	if ok := dispatcher.EpicPasses(context.Background(), []RepoEpics{{Repo: alpha, Epics: []string{"alpha-epic"}}, {Repo: beta, Epics: []string{"beta-epic"}}}); !ok {
		t.Fatal("EpicPasses() = false, want complete passes")
	}
	if got := len(dispatcher.Sessions()); got != 0 {
		t.Fatalf("sessions = %d, want no decompositions", got)
	}
	if got := epicCalls(beads.Calls(), "EpicChildren"); len(got) != 2 {
		t.Fatalf("children queries = %#v, want both repositories", got)
	}
}

func TestTickRunsEpicPassAfterReadyResults(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{
		ready:        func(context.Context, repo.Repo) ([]ReadyEntry, error) { return nil, nil },
		openEpics:    func(context.Context, repo.Repo) ([]string, error) { return []string{"epic-1"}, nil },
		epicChildren: func(context.Context, repo.Repo, string) ([]string, error) { return nil, nil },
	}
	deps := completeDeps()
	deps.Beads = beads
	deps.Workspaces = readyWorkspaces()
	deps.Runner = &fakeRunner{}
	deps.Repos = &fakeRepos{list: func(context.Context) []repo.Repo { return []repo.Repo{repository} }}
	deps.Gate = &fakeGate{}
	deps.Config = config.Default()
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}

	dispatcher.Tick(context.Background())
	if got := epicCalls(beads.Calls(), "OpenEpics"); !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("open epic calls = %#v, want one repository sweep", got)
	}
	sessions := dispatcher.Sessions()
	if len(sessions) != 1 || sessions[0].Task != "epic-1" || !sessions[0].Decomposition {
		t.Fatalf("sessions = %#v, want decomposition from tick", sessions)
	}
}

func epicCalls(calls []fakeCall, method string) []string {
	var epics []string
	for _, call := range calls {
		if call.Method != method {
			continue
		}
		if method == "OpenEpics" {
			epics = append(epics, "")
			continue
		}
		epics = append(epics, call.Args[1].(string))
	}
	return epics
}

func sameEpics(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[string]int, len(want))
	for _, epic := range want {
		counts[epic]++
	}
	for _, epic := range got {
		if counts[epic] == 0 {
			return false
		}
		counts[epic]--
	}
	return true
}

package dispatch

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

type logCall struct {
	level  logging.Level
	kind   string
	fields map[string]any
}

func spawnDispatcher(t *testing.T, beads *fakeBeads, workspaces *fakeWorkspaces, runner *fakeRunner, gate *fakeGate) (*Dispatcher, *[]logCall) {
	t.Helper()
	logs := []logCall{}
	deps := completeDeps()
	deps.Beads = beads
	deps.Workspaces = workspaces
	deps.Runner = runner
	deps.Gate = gate
	deps.Config = config.Default()
	deps.Logger = func(level logging.Level, kind string, fields map[string]any) {
		copied := make(map[string]any, len(fields))
		for key, value := range fields {
			copied[key] = value
		}
		logs = append(logs, logCall{level: level, kind: kind, fields: copied})
	}
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	return dispatcher, &logs
}

func spawnRepo(t *testing.T) repo.Repo { return planRepo(t) }

func readyWorkspaces() *fakeWorkspaces {
	return &fakeWorkspaces{path: func(_ repo.Repo, seat string) (string, error) { return "/work/" + seat, nil }}
}

func TestImplementRefusesHumanOnlyBeforeSeatOrClaim(t *testing.T) {
	beads := &fakeBeads{humanOnly: func(context.Context, repo.Repo, string) (bool, error) { return true, nil }}
	workspaces := readyWorkspaces()
	runner := &fakeRunner{}
	dispatcher, logs := spawnDispatcher(t, beads, workspaces, runner, &fakeGate{})

	if handle := dispatcher.Implement(context.Background(), spawnRepo(t), "task-1"); handle != "" {
		t.Fatalf("Implement() = %q, want no dispatch", handle)
	}
	if got := callMethods(beads.Calls()); !reflect.DeepEqual(got, []string{"HumanOnly"}) {
		t.Fatalf("bead calls = %#v, want HumanOnly", got)
	}
	if len(workspaces.Calls()) != 0 || len(runner.Calls()) != 0 {
		t.Fatalf("seat or runner called after human hold: workspaces=%#v runner=%#v", workspaces.Calls(), runner.Calls())
	}
	if len(*logs) != 1 || (*logs)[0].kind != "human-hold" {
		t.Fatalf("logs = %#v, want human-hold", *logs)
	}
}

func TestImplementStopsAtRoleCap(t *testing.T) {
	beads := &fakeBeads{}
	workspaces := readyWorkspaces()
	dispatcher, _ := spawnDispatcher(t, beads, workspaces, &fakeRunner{}, &fakeGate{})
	for _, seat := range []string{"ifrit", "shiva", "titan"} {
		dispatcher.Add(Session{Handle: seat, Role: Implementer, Seat: seat})
	}

	if handle := dispatcher.Implement(context.Background(), spawnRepo(t), "task-1"); handle != "" {
		t.Fatalf("Implement() = %q, want no dispatch", handle)
	}
	if got := callMethods(beads.Calls()); !reflect.DeepEqual(got, []string{"HumanOnly"}) {
		t.Fatalf("bead calls = %#v, want HumanOnly", got)
	}
	if len(workspaces.Calls()) != 0 {
		t.Fatalf("workspace calls = %#v, want none", workspaces.Calls())
	}
}

func TestSeatReadySyncConflictRefusesAndComments(t *testing.T) {
	beads := &fakeBeads{}
	workspaces := readyWorkspaces()
	workspaces.sync = func(context.Context, repo.Repo, string) (SyncResult, error) { return SyncConflict, nil }
	dispatcher, logs := spawnDispatcher(t, beads, workspaces, &fakeRunner{}, &fakeGate{})

	if dispatcher.SeatReady(context.Background(), spawnRepo(t), Implementer, "ifrit", "task-1") {
		t.Fatal("SeatReady() true after sync conflict")
	}
	if got := callMethods(workspaces.Calls()); !reflect.DeepEqual(got, []string{"Ensure", "Sync"}) {
		t.Fatalf("workspace calls = %#v, want Ensure, Sync", got)
	}
	calls := beads.Calls()
	if len(calls) != 1 || calls[0].Method != "Comment" || calls[0].Args[2] != "seat ifrit holds unlanded work conflicting with main." {
		t.Fatalf("bead calls = %#v, want conflict comment", calls)
	}
	if len(*logs) != 1 || (*logs)[0].kind != "seat-refused" {
		t.Fatalf("logs = %#v, want seat-refused", *logs)
	}
}

func TestRepairSkipsSyncAndMarksSessionRepairing(t *testing.T) {
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil }}
	workspaces := readyWorkspaces()
	dispatcher, _ := spawnDispatcher(t, beads, workspaces, &fakeRunner{}, &fakeGate{})

	if handle := dispatcher.Repair(context.Background(), spawnRepo(t), "phoenix", "task-1"); handle != "handle" {
		t.Fatalf("Repair() = %q, want handle", handle)
	}
	if got := callMethods(workspaces.Calls()); !reflect.DeepEqual(got, []string{"Ensure", "Path"}) {
		t.Fatalf("workspace calls = %#v, want Ensure, Path", got)
	}
	if sessions := dispatcher.Sessions(); len(sessions) != 1 || sessions[0].Status != Repairing {
		t.Fatalf("sessions = %#v, want repairing session", sessions)
	}
}

func TestImplementRunFailureReleasesClaimAndLeavesTaskOpen(t *testing.T) {
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil }}
	workspaces := readyWorkspaces()
	runner := &fakeRunner{run: func(context.Context, RunSpec) (string, error) { return "", errors.New("runner failed") }}
	dispatcher, logs := spawnDispatcher(t, beads, workspaces, runner, &fakeGate{})

	if handle := dispatcher.Implement(context.Background(), spawnRepo(t), "task-1"); handle != "" {
		t.Fatalf("Implement() = %q, want no dispatch", handle)
	}
	if got := callMethods(beads.Calls()); !reflect.DeepEqual(got, []string{"HumanOnly", "Claim", "Difficulty", "Show", "Comment", "Release"}) {
		t.Fatalf("bead calls = %#v", got)
	}
	if len(dispatcher.Sessions()) != 0 {
		t.Fatal("failed run registered a session")
	}
	if len(*logs) != 1 || (*logs)[0].kind != "dispatch-failed" {
		t.Fatalf("logs = %#v, want dispatch-failed", *logs)
	}
}

func TestReviewUsesRepositoryRootWithoutClaimOrRelease(t *testing.T) {
	beads := &fakeBeads{}
	workspaces := readyWorkspaces()
	gate := &fakeGate{reviewPlan: func(context.Context, repo.Repo, string) (RunSpec, error) {
		return RunSpec{Model: "review-model", Plan: "review plan"}, nil
	}}
	runner := &fakeRunner{}
	dispatcher, _ := spawnDispatcher(t, beads, workspaces, runner, gate)
	repository := spawnRepo(t)

	if handle := dispatcher.Review(context.Background(), repository, "epic-1"); handle != "handle" {
		t.Fatalf("Review() = %q, want handle", handle)
	}
	if got := callMethods(beads.Calls()); !reflect.DeepEqual(got, []string{"HumanOnly"}) {
		t.Fatalf("bead calls = %#v, want HumanOnly", got)
	}
	runs := runner.Calls()
	if len(runs) != 1 || runs[0].Args[0].(RunSpec).Workdir != repository.Root || runs[0].Args[0].(RunSpec).Plan != "review plan" {
		t.Fatalf("runner calls = %#v, want root review run", runs)
	}
	if got := callMethods(gate.Calls()); !reflect.DeepEqual(got, []string{"ReviewPlan", "NoteSession"}) {
		t.Fatalf("gate calls = %#v, want review plan and note", got)
	}
}

func TestImplementPickupCapturesResolvedRouting(t *testing.T) {
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil }}
	workspaces := readyWorkspaces()
	dispatcher, logs := spawnDispatcher(t, beads, workspaces, &fakeRunner{}, &fakeGate{})
	dispatcher.config.Crew.Backend = config.BackendKiro

	if handle := dispatcher.Implement(context.Background(), spawnRepo(t), "task-1"); handle != "handle" {
		t.Fatalf("Implement() = %q, want handle", handle)
	}
	if len(*logs) != 1 || (*logs)[0].kind != logging.KindPickup {
		t.Fatalf("logs = %#v, want pickup", *logs)
	}
	fields := (*logs)[0].fields
	for _, key := range []string{"task", "repo", "role", "seat", "backend", "model", "difficulty", "effort", "handle"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("pickup fields missing %q: %#v", key, fields)
		}
	}
	if fields["backend"] != config.BackendKiro || fields["difficulty"] != "low" || fields["model"] != "gpt-5.6-luna" || fields["effort"] != "medium" {
		t.Fatalf("pickup routing = %#v", fields)
	}
}

func callMethods(calls []fakeCall) []string {
	methods := make([]string, len(calls))
	for index, call := range calls {
		methods[index] = call.Method
	}
	return methods
}

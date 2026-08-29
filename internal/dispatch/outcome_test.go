package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func outcomeDispatcher(t *testing.T, beads *fakeBeads, lander *fakeLander, runner *fakeRunner, gate *fakeGate) *Dispatcher {
	t.Helper()
	deps := completeDeps()
	deps.Beads = beads
	deps.Lander = lander
	deps.Runner = runner
	deps.Gate = gate
	deps.Config = config.Default()
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	return dispatcher
}

func outcomeSession(handle string, role Role) Session {
	record, _ := repo.Make("/repo", "magicite", "magicite", "main")
	return Session{
		Handle: handle, Repo: record, Task: "task-1", Seat: "ifrit", Role: role,
		Model: "model", Backend: "kiro", Difficulty: "high", Effort: "high", Agent: "worker",
	}
}

func TestOnCompleteClosesOnlyVerifiedLandedTask(t *testing.T) {
	beads := &fakeBeads{}
	runner := &fakeRunner{diff: func(context.Context, string) ([]Diff, error) {
		return []Diff{{File: "outcome.go", Status: "modified", Additions: 2, Deletions: 1}}, nil
	}}
	lander := &fakeLander{}
	gate := &fakeGate{dueEpic: func(context.Context, repo.Repo, string) (string, error) { return "", nil }}
	dispatcher := outcomeDispatcher(t, beads, lander, runner, gate)
	dispatcher.Add(outcomeSession("done", Implementer))

	dispatcher.OnComplete(context.Background(), "done", Completed)

	if !dispatcher.Drained() {
		t.Fatal("dispatcher remains live after completion")
	}
	if got := callMethods(runner.Calls()); !sameMethods(got, []string{"Diff", "Delete"}) {
		t.Fatalf("runner calls = %#v", got)
	}
	if got := callMethods(lander.Calls()); !sameMethods(got, []string{"Land", "Landed", "TaskLanded"}) {
		t.Fatalf("lander calls = %#v", got)
	}
	calls := beads.Calls()
	if len(calls) != 1 || calls[0].Method != "Close" {
		t.Fatalf("bead calls = %#v, want one close", calls)
	}
	closeOutput := calls[0].Args[2].(string)
	for _, want := range []string{"Landed changes:", "Magicite-Task: task-1", "Magicite-Agent: worker"} {
		if !strings.Contains(closeOutput, want) {
			t.Errorf("close output missing %q: %q", want, closeOutput)
		}
	}
}

func TestOnCompleteReleasesUnprovenLandingAndDeletes(t *testing.T) {
	beads := &fakeBeads{}
	lander := &fakeLander{taskLanded: func(context.Context, repo.Repo, string) (bool, error) { return false, nil }}
	runner := &fakeRunner{}
	dispatcher := outcomeDispatcher(t, beads, lander, runner, &fakeGate{})
	dispatcher.Add(outcomeSession("unproven", Implementer))

	dispatcher.OnComplete(context.Background(), "unproven", Completed)

	if got := callMethods(beads.Calls()); !sameMethods(got, []string{"Comment", "Release"}) {
		t.Fatalf("bead calls = %#v, want comment and release", got)
	}
	if got := callMethods(runner.Calls()); !sameMethods(got, []string{"Diff", "Delete"}) {
		t.Fatalf("runner calls = %#v, want diff and delete", got)
	}
}

func TestOnCompleteConflictDispatchesOneRepair(t *testing.T) {
	beads := &fakeBeads{}
	lander := &fakeLander{land: func(context.Context, repo.Repo, string, Stamp) (LandResult, error) { return LandConflict, nil }}
	runner := &fakeRunner{}
	dispatcher := outcomeDispatcher(t, beads, lander, runner, &fakeGate{})
	dispatcher.Add(outcomeSession("conflict", Implementer))

	dispatcher.OnComplete(context.Background(), "conflict", Completed)

	if got := callMethods(beads.Calls()); len(got) < 2 || got[0] != "Comment" || got[len(got)-1] == "Release" {
		t.Fatalf("bead calls = %#v, want conflict comment and no release", got)
	}
	if sessions := dispatcher.Sessions(); len(sessions) != 1 || sessions[0].Role != Repairer {
		t.Fatalf("sessions = %#v, want repairer", sessions)
	}
}

func TestOnCompleteReviewerReadsOutputWithoutLanding(t *testing.T) {
	beads := &fakeBeads{}
	lander := &fakeLander{}
	runner := &fakeRunner{output: func(context.Context, string) (string, error) { return "verdict", nil }}
	gate := &fakeGate{}
	dispatcher := outcomeDispatcher(t, beads, lander, runner, gate)
	dispatcher.Add(outcomeSession("review", Reviewer))

	dispatcher.OnComplete(context.Background(), "review", Completed)

	if got := callMethods(runner.Calls()); !sameMethods(got, []string{"Output", "Delete"}) {
		t.Fatalf("runner calls = %#v", got)
	}
	if got := callMethods(gate.Calls()); !sameMethods(got, []string{"CompleteReview"}) {
		t.Fatalf("gate calls = %#v", got)
	}
	if len(lander.Calls()) != 0 || len(beads.Calls()) != 0 {
		t.Fatalf("review landed or touched beads: lander=%#v beads=%#v", lander.Calls(), beads.Calls())
	}
}

func TestOnCompleteFailureAndPanicReleaseThenDelete(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome Outcome
		diff    func(context.Context, string) ([]Diff, error)
	}{
		{"failed", Failed, nil},
		{"panicked route", Completed, func(context.Context, string) ([]Diff, error) { panic("broken runtime") }},
		{"diff error", Completed, func(context.Context, string) ([]Diff, error) { return nil, errors.New("diff failed") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			beads := &fakeBeads{}
			runner := &fakeRunner{diff: test.diff}
			dispatcher := outcomeDispatcher(t, beads, &fakeLander{}, runner, &fakeGate{})
			dispatcher.Add(outcomeSession(test.name, Implementer))

			dispatcher.OnComplete(context.Background(), test.name, test.outcome)

			if got := callMethods(beads.Calls()); !sameMethods(got[len(got)-2:], []string{"Comment", "Release"}) {
				t.Fatalf("bead calls = %#v, want final comment and release", got)
			}
			if got := callMethods(runner.Calls()); got[len(got)-1] != "Delete" {
				t.Fatalf("runner calls = %#v, want final delete", got)
			}
		})
	}
}

func TestOnCompleteIgnoresUnknownHandle(t *testing.T) {
	beads, lander, runner := &fakeBeads{}, &fakeLander{}, &fakeRunner{}
	dispatcher := outcomeDispatcher(t, beads, lander, runner, &fakeGate{})
	dispatcher.OnComplete(context.Background(), "unknown", Completed)
	if len(beads.Calls()) != 0 || len(lander.Calls()) != 0 || len(runner.Calls()) != 0 {
		t.Fatalf("unknown completion had side effects: beads=%#v lander=%#v runner=%#v", beads.Calls(), lander.Calls(), runner.Calls())
	}
}

func sameMethods(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/repo"
)

func TestFallbackRetryCarriesRoutingAndMarksAttempt(t *testing.T) {
	beads := &fakeBeads{}
	runner := &fakeRunner{run: func(context.Context, RunSpec) (string, error) { return "fallback", nil }}
	dispatcher := outcomeDispatcher(t, beads, &fakeLander{}, runner, &fakeGate{})
	dispatcher.config.Fleet.KiroFallback = "fallback-model"
	dispatcher.config.Crew.Backend = config.BackendOpenCode
	session := outcomeSession("limited", Implementer)
	session.Backend = config.BackendKiro
	session.Difficulty = config.DifficultyLow
	session.Effort = "old-effort"
	dispatcher.Add(session)

	dispatcher.OnComplete(context.Background(), session.Handle, Limited)

	if got := callMethods(runner.Calls()); !sameMethods(got, []string{"Run", "Delete"}) {
		t.Fatalf("runner calls = %#v, want retry run and old delete", got)
	}
	run := runner.Calls()[0].Args[0].(RunSpec)
	if run.Backend != config.BackendKiro || run.Model != "fallback-model" || run.Effort != "old-effort" || run.Agent != session.Agent {
		t.Fatalf("retry routing = %#v, want sticky backend/model/effort/agent", run)
	}
	sessions := dispatcher.Sessions()
	if len(sessions) != 1 || sessions[0].Handle != "fallback" || !sessions[0].FallbackAttempted || sessions[0].Difficulty != session.Difficulty {
		t.Fatalf("sessions = %#v, want marked sticky retry", sessions)
	}
	if got := callMethods(beads.Calls()); got[len(got)-1] == "Release" {
		t.Fatalf("bead calls = %#v, retry unexpectedly released claim", got)
	}
}

func TestFallbackRetrySecondFailureFallsThrough(t *testing.T) {
	beads := &fakeBeads{}
	runner := &fakeRunner{usageLimited: func(context.Context, string) (bool, error) {
		return true, errors.New("must not inspect second attempt")
	}}
	dispatcher := outcomeDispatcher(t, beads, &fakeLander{}, runner, &fakeGate{})
	session := outcomeSession("second", Implementer)
	session.FallbackAttempted = true
	dispatcher.Add(session)

	dispatcher.OnComplete(context.Background(), session.Handle, Failed)

	if got := callMethods(beads.Calls()); !sameMethods(got, []string{"Comment", "Release"}) {
		t.Fatalf("bead calls = %#v, want ordinary failure", got)
	}
	if got := callMethods(runner.Calls()); !sameMethods(got, []string{"Delete"}) {
		t.Fatalf("runner calls = %#v, want delete without usage probe", got)
	}
}

func TestFallbackRetryReviewerRenotesGate(t *testing.T) {
	gate := &fakeGate{reviewPlan: func(context.Context, repo.Repo, string) (RunSpec, error) {
		return RunSpec{Plan: "retry review"}, nil
	}}
	runner := &fakeRunner{run: func(context.Context, RunSpec) (string, error) { return "review-retry", nil }}
	dispatcher := outcomeDispatcher(t, &fakeBeads{}, &fakeLander{}, runner, gate)
	session := outcomeSession("review", Reviewer)
	dispatcher.Add(session)

	dispatcher.OnComplete(context.Background(), session.Handle, Limited)

	if got := callMethods(gate.Calls()); !sameMethods(got, []string{"ReviewPlan", "NoteSession"}) {
		t.Fatalf("gate calls = %#v, want retry plan and renote", got)
	}
	if sessions := dispatcher.Sessions(); len(sessions) != 1 || sessions[0].Handle != "review-retry" || !sessions[0].FallbackAttempted {
		t.Fatalf("sessions = %#v, want reviewer retry", sessions)
	}
}

func TestFallbackRetryStartFailureReleasesOnce(t *testing.T) {
	beads := &fakeBeads{}
	runner := &fakeRunner{run: func(context.Context, RunSpec) (string, error) { return "", errors.New("start failed") }}
	dispatcher := outcomeDispatcher(t, beads, &fakeLander{}, runner, &fakeGate{})
	session := outcomeSession("failed-start", Implementer)
	dispatcher.Add(session)

	dispatcher.OnComplete(context.Background(), session.Handle, Limited)

	calls := beads.Calls()
	if got := callMethods(calls); !sameMethods(got, []string{"Comment", "Show", "Comment", "Release"}) {
		t.Fatalf("bead calls = %#v, want retry comment then ordinary failure", got)
	}
	if len(dispatcher.Sessions()) != 0 {
		t.Fatal("failed fallback start left live session")
	}
}

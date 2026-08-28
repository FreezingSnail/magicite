package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/logging"
)

type lifecycleRunner struct {
	*fakeRunner
	complete          func(string, Outcome)
	phase             func(string, string)
	completes, phases int
}

func (r *lifecycleRunner) OnComplete(callback func(string, Outcome)) {
	r.completes++
	r.complete = callback
}

func (r *lifecycleRunner) OnPhase(callback func(string, string)) {
	r.phases++
	r.phase = callback
}

func lifecycleDispatcher(t *testing.T, beads *fakeBeads, runner *lifecycleRunner, gate *fakeGate) (*Dispatcher, *manualClock, *[]logCall) {
	t.Helper()
	clock := newManualClock(time.Unix(0, 0))
	logs := []logCall{}
	deps := completeDeps()
	deps.Beads = beads
	deps.Runner = runner
	deps.Gate = gate
	deps.Clock = clock
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
	return dispatcher, clock, &logs
}

func TestStartRegistersCallbacksClearsFreezeAndPolls(t *testing.T) {
	beads := &fakeBeads{}
	runner := &lifecycleRunner{fakeRunner: &fakeRunner{}}
	dispatcher, clock, logs := lifecycleDispatcher(t, beads, runner, &fakeGate{})
	dispatcher.stateMu.Lock()
	dispatcher.draining = true
	dispatcher.tickInFlight = true
	dispatcher.pendingNotify = true
	dispatcher.stateMu.Unlock()

	stop, err := dispatcher.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if runner.completes != 1 || runner.phases != 1 {
		t.Fatalf("callback registrations = (%d, %d), want (1, 1)", runner.completes, runner.phases)
	}
	if dispatcher.Draining() || dispatcher.TickInFlight() {
		t.Fatalf("freeze state remains: draining=%t in-flight=%t", dispatcher.Draining(), dispatcher.TickInFlight())
	}
	if got := callMethods(beads.Calls()); len(got) == 0 || got[0] != "CancelAll" {
		t.Fatalf("bead calls = %#v, want initial CancelAll", got)
	}
	if _, err := dispatcher.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.completes != 1 || runner.phases != 1 {
		t.Fatalf("duplicate callback registrations = (%d, %d)", runner.completes, runner.phases)
	}

	dispatcher.Add(outcomeSession("phase", Implementer))
	runner.phase("phase", "review")
	if got := dispatcher.Sessions()[0].Phase; got != "review" {
		t.Fatalf("phase = %q, want review", got)
	}
	before := len(dispatcher.repos.(*fakeRepos).Calls())
	clock.Advance(30 * time.Second)
	waitFor(t, func() bool { return len(dispatcher.repos.(*fakeRepos).Calls()) > before })
	stop(context.Background(), false)
	stopped := len(dispatcher.repos.(*fakeRepos).Calls())
	clock.Advance(30 * time.Second)
	if got := len(dispatcher.repos.(*fakeRepos).Calls()); got != stopped {
		t.Fatalf("ticker ran after stop: got %d calls, want %d", got, stopped)
	}
	if !hasLog(*logs, "start-cleared-freeze") || !hasLog(*logs, "start") {
		t.Fatalf("logs = %#v, want freeze clear and start", *logs)
	}
}

func TestStartRequiresLifecycleRunner(t *testing.T) {
	dispatcher, err := New(completeDeps())
	if err != nil {
		t.Fatal(err)
	}
	_, err = dispatcher.Start(context.Background())
	var lifecycle *LifecycleError
	if !errors.As(err, &lifecycle) || lifecycle.Operation != "start" {
		t.Fatalf("Start() error = %v, want LifecycleError for start", err)
	}
}

func TestStopDrainsUntilCompletion(t *testing.T) {
	beads := &fakeBeads{}
	runner := &lifecycleRunner{fakeRunner: &fakeRunner{}}
	dispatcher, _, logs := lifecycleDispatcher(t, beads, runner, &fakeGate{})
	if _, err := dispatcher.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	dispatcher.Add(outcomeSession("live", Implementer))
	done := dispatcher.Stop(context.Background(), false)
	if !dispatcher.Draining() {
		t.Fatal("soft stop did not drain")
	}
	select {
	case <-done:
		t.Fatal("drain completed while session remained live")
	default:
	}
	runner.complete("live", Failed)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("drain did not complete after final session")
	}
	if got := callMethods(runner.Calls()); len(got) == 0 || got[len(got)-1] != "Delete" {
		t.Fatalf("runner calls = %#v, want completion deletion", got)
	}
	if !hasLog(*logs, "stop") {
		t.Fatalf("logs = %#v, want stop", *logs)
	}
	if open := dispatcher.Stop(context.Background(), false); !channelClosed(open) {
		t.Fatal("idle soft stop did not return closed channel")
	}
}

func TestHardStopAbortsReviewsDeletesSessionsAndReleasesClaims(t *testing.T) {
	beads := &fakeBeads{}
	runner := &lifecycleRunner{fakeRunner: &fakeRunner{}}
	gate := &fakeGate{}
	dispatcher, _, _ := lifecycleDispatcher(t, beads, runner, gate)
	dispatcher.Add(outcomeSession("worker", Implementer))
	dispatcher.Add(outcomeSession("review", Reviewer))

	done := dispatcher.Stop(context.Background(), true)
	if !channelClosed(done) || !dispatcher.Idle() {
		t.Fatal("hard stop did not finish empty")
	}
	if got := callMethods(runner.Calls()); !sameMethods(got, []string{"Delete", "Delete"}) {
		t.Fatalf("runner calls = %#v, want every session deleted", got)
	}
	if got := callMethods(gate.Calls()); !sameMethods(got, []string{"AbortReview"}) {
		t.Fatalf("gate calls = %#v, want reviewer abort", got)
	}
	if got := callMethods(beads.Calls()); !sameMethods(got, []string{"CancelAll", "Release"}) {
		t.Fatalf("bead calls = %#v, want query cancellation and task release", got)
	}
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.After(time.Second)
	for !ready() {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for asynchronous lifecycle work")
		case <-time.After(time.Millisecond):
		}
	}
}

func channelClosed(channel <-chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
}

func hasLog(logs []logCall, kind string) bool {
	for _, entry := range logs {
		if entry.kind == kind {
			return true
		}
	}
	return false
}

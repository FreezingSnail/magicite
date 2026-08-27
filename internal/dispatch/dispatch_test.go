package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/repo"
)

func completeDeps() Deps {
	return Deps{
		Beads:      &fakeBeads{},
		Workspaces: &fakeWorkspaces{},
		Lander:     &fakeLander{},
		Runner:     &fakeRunner{},
		Repos:      &fakeRepos{},
		Gate:       &fakeGate{},
		Clock:      newManualClock(time.Unix(0, 0)),
	}
}

func TestNewRequiresEachPort(t *testing.T) {
	for _, test := range []struct {
		name  string
		clear func(*Deps)
	}{
		{"Beads", func(d *Deps) { d.Beads = nil }},
		{"Workspaces", func(d *Deps) { d.Workspaces = nil }},
		{"Lander", func(d *Deps) { d.Lander = nil }},
		{"Runner", func(d *Deps) { d.Runner = nil }},
		{"Repos", func(d *Deps) { d.Repos = nil }},
		{"Gate", func(d *Deps) { d.Gate = nil }},
		{"Clock", func(d *Deps) { d.Clock = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			deps := completeDeps()
			test.clear(&deps)
			_, err := New(deps)
			var missing *MissingDependencyError
			if !errors.As(err, &missing) || missing.Dependency != test.name {
				t.Fatalf("New() error = %v, want missing %s", err, test.name)
			}
		})
	}
}

func TestNewRejectsTypedNilPort(t *testing.T) {
	deps := completeDeps()
	var beads *fakeBeads
	deps.Beads = beads
	if _, err := New(deps); err == nil {
		t.Fatal("New() accepted typed nil Beads")
	}
}

func TestNewDefaultsLogger(t *testing.T) {
	dispatcher, err := New(completeDeps())
	if err != nil {
		t.Fatal(err)
	}
	if dispatcher.log == nil {
		t.Fatal("New() left logger nil")
	}
}

func TestPermissiveGate(t *testing.T) {
	gate := PermissiveGate{}
	ctx := context.Background()
	if held, err := gate.Hold(ctx, repo.Repo{}); err != nil || held {
		t.Fatalf("Hold() = (%v, %v), want (false, nil)", held, err)
	}
	if epic, err := gate.DueEpic(ctx, repo.Repo{}, "task"); err != nil || epic != "" {
		t.Fatalf("DueEpic() = (%q, %v), want (empty, nil)", epic, err)
	}
	if epic, err := gate.GateEpic(ctx, repo.Repo{}, "epic"); err != nil || epic != "" {
		t.Fatalf("GateEpic() = (%q, %v), want (empty, nil)", epic, err)
	}
	if _, err := gate.ReviewPlan(ctx, repo.Repo{}, "epic"); !errors.Is(err, ErrReviewUnsupported) {
		t.Fatalf("ReviewPlan() error = %v, want %v", err, ErrReviewUnsupported)
	}
}

func TestManualClockAdvancesOnlyWhenTold(t *testing.T) {
	clock := newManualClock(time.Unix(0, 0))
	ticker := clock.Ticker(time.Second)
	select {
	case <-ticker.Chan():
		t.Fatal("Ticker fired before Advance")
	default:
	}
	clock.Advance(time.Second)
	select {
	case got := <-ticker.Chan():
		if !got.Equal(time.Unix(1, 0)) {
			t.Fatalf("tick = %v, want %v", got, time.Unix(1, 0))
		}
	default:
		t.Fatal("Ticker did not fire after Advance")
	}
	ticker.Stop()
	clock.Advance(time.Second)
	select {
	case <-ticker.Chan():
		t.Fatal("stopped ticker fired")
	default:
	}
}

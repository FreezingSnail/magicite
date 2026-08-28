package dispatch

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/repo"
)

func newRegistryDispatcher(t *testing.T) (*Dispatcher, *manualClock) {
	t.Helper()
	deps := completeDeps()
	clock := newManualClock(time.Date(2026, time.August, 27, 20, 0, 0, 0, time.UTC))
	deps.Clock = clock
	deps.Config = config.Config{
		Fleet:    config.RoleConfig{Seats: []config.SeatConfig{{Name: "ifrit"}, {Name: "shiva"}}},
		Designer: config.RoleConfig{Seats: []config.SeatConfig{{Name: "ramuh"}, {Name: "leviathan"}}},
		Repairer: config.RoleConfig{Seats: []config.SeatConfig{{Name: "phoenix"}}},
		Reviewer: config.RoleConfig{Seats: []config.SeatConfig{{Name: "odin"}}},
	}
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	return dispatcher, clock
}

func TestRegistryRoleCapsAndFreeSeats(t *testing.T) {
	dispatcher, _ := newRegistryDispatcher(t)
	for _, test := range []struct {
		role Role
		want int
	}{
		{Implementer, 2}, {Role("fleet"), 2}, {Designer, 2}, {Repairer, 1}, {Reviewer, 1}, {Role("unknown"), 0},
	} {
		if got := dispatcher.RoleCap(test.role); got != test.want {
			t.Errorf("RoleCap(%q) = %d, want %d", test.role, got, test.want)
		}
	}
	if got := dispatcher.FreeSeat(Implementer); got != "ifrit" {
		t.Fatalf("initial FreeSeat() = %q, want ifrit", got)
	}
	dispatcher.Add(Session{Handle: "one", Role: Implementer, Seat: "ifrit"})
	if got := dispatcher.FreeSeat(Implementer); got != "shiva" {
		t.Errorf("FreeSeat() after ifrit = %q, want shiva", got)
	}
	dispatcher.Add(Session{Handle: "two", Role: Implementer, Seat: "shiva"})
	if got := dispatcher.FreeSeat(Implementer); got != "" {
		t.Errorf("FreeSeat() all occupied = %q, want empty", got)
	}
	if got := dispatcher.FreeSeat(Designer); got != "ramuh" {
		t.Errorf("FreeSeat(designer) = %q, want ramuh", got)
	}
}

func TestRegistryMutatesOnlyNamedSession(t *testing.T) {
	dispatcher, clock := newRegistryDispatcher(t)
	first := Session{Handle: "first", Repo: repo.Repo{Name: "magicite"}, Task: "task-1", Role: Implementer, Seat: "ifrit", Status: Working}
	dispatcher.Add(first)
	clock.Advance(time.Minute)
	dispatcher.Add(Session{Handle: "second", Task: "task-2", Role: Designer, Seat: "ramuh", Status: Running})

	if !dispatcher.SetStatus("first", Failed) || !dispatcher.SetPhase("first", "landing") || !dispatcher.MarkDecomposition("first") {
		t.Fatal("named mutation reported missing session")
	}
	if dispatcher.SetStatus("missing", Running) || dispatcher.SetPhase("missing", "ignored") || dispatcher.MarkDecomposition("missing") {
		t.Fatal("missing mutation reported success")
	}

	sessions := dispatcher.Sessions()
	if len(sessions) != 2 || sessions[0].Handle != "first" || sessions[1].Handle != "second" {
		t.Fatalf("Sessions() = %#v, want ordered first and second", sessions)
	}
	if sessions[0].Started != clock.now.Add(-time.Minute) || sessions[0].Status != Failed || sessions[0].Phase != "landing" || !sessions[0].Decomposition {
		t.Errorf("first = %#v, want stamped and fully updated", sessions[0])
	}
	if sessions[1].Status != Running || sessions[1].Phase != "" || sessions[1].Decomposition {
		t.Errorf("second changed during first mutation: %#v", sessions[1])
	}
	if got := dispatcher.ActiveCount(Implementer); got != 1 {
		t.Errorf("ActiveCount(implementer) = %d, want 1", got)
	}
	if got := dispatcher.ActiveCount(Designer); got != 1 {
		t.Errorf("ActiveCount(designer) = %d, want 1", got)
	}

	sessions[0].Status = Working
	if got := dispatcher.Sessions()[0].Status; got != Failed {
		t.Errorf("snapshot mutation changed live status to %q", got)
	}
	removed, ok := dispatcher.Remove("first")
	if !ok || removed.Handle != "first" || removed.Status != Failed {
		t.Errorf("Remove(first) = (%#v, %v)", removed, ok)
	}
	if _, ok := dispatcher.Remove("missing"); ok {
		t.Error("Remove(missing) reported present")
	}
	if dispatcher.Idle() {
		t.Error("Idle() true with second session live")
	}
	dispatcher.Remove("second")
	if !dispatcher.Idle() {
		t.Error("Idle() false after removals")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	dispatcher, _ := newRegistryDispatcher(t)
	const workers = 32
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			handle := fmt.Sprintf("session-%d", worker)
			dispatcher.Add(Session{Handle: handle, Role: Implementer, Seat: fmt.Sprintf("seat-%d", worker)})
			dispatcher.SetStatus(handle, Running)
			dispatcher.SetPhase(handle, "running")
			dispatcher.MarkDecomposition(handle)
			_ = dispatcher.ActiveCount(Implementer)
			_ = dispatcher.FreeSeat(Implementer)
			_ = dispatcher.Sessions()
			dispatcher.Remove(handle)
		}(worker)
	}
	group.Wait()
	if !dispatcher.Idle() {
		t.Error("Idle() false after concurrent removals")
	}
}

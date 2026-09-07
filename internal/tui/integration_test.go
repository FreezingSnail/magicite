package tui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestProgramComposesRepairAndSuppressesLateMessages(t *testing.T) {
	clock := time.Date(2026, time.September, 7, 19, 0, 0, 0, time.UTC)
	api := &programAPI{calls: make(chan struct{}, 4), results: make(chan programResult, 4)}
	stream := &programStream{subscriptions: make(chan *programSubscription, 2)}
	timers := &programTimers{made: make(chan *programTimer, 2)}
	program := NewProgram(ProgramOptions{API: api, Stream: stream, Now: func() time.Time { return clock }, Cadence: -1, NewTimer: timers.New, Width: 80, Height: 40, NoColor: true})
	t.Cleanup(program.Stop)

	command := program.Init()
	awaitProgram(t, api.calls, "initial snapshot")
	first := awaitProgram(t, stream.subscriptions, "initial stream")
	api.results <- programResult{snapshot: programSnapshot(2, 1)}
	program = programUpdate(t, program, command())
	if state := program.State(); state.Connection != ConnectionOnline || state.Generation != 2 {
		t.Fatalf("initial state = %#v", state)
	}

	command = programNext(t, program, StreamEventMsg{Event: transport.Event{Seq: 2, Kind: wire.KindComplete, Task: "magicite-nbr.6"}})
	if state := program.State(); state.Cursor != 2 || len(state.Events) != 1 {
		t.Fatalf("event state = %#v", state)
	}
	first.notices <- transport.StreamNotice{Kind: transport.StreamNoticeEOF, Cursor: 2}
	close(first.events)
	close(first.notices)
	program = programUpdate(t, program, command())
	awaitProgram(t, api.calls, "EOF repair")
	api.results <- programResult{snapshot: programSnapshot(3, 2)}
	command = programNext(t, program, RefreshResult{Generation: 2, Snapshot: programSnapshot(3, 2)})
	if state := program.State(); state.Connection != ConnectionOnline || state.Generation != 3 || state.Cursor != 2 {
		t.Fatalf("repaired state = %#v", state)
	}

	program = programUpdate(t, program, RefreshResult{Generation: 5, Snapshot: programSnapshot(5, 5)})
	program = programUpdate(t, program, RefreshResult{Generation: 4, Err: &transport.Error{Code: transport.ErrorUnavailable}})
	state := program.State()
	if state.Connection != ConnectionOnline || state.Generation != 5 {
		t.Fatalf("late result changed state = %#v", state)
	}
	if view := program.View(); !strings.Contains(view, "Dashboard") || !strings.Contains(view, "generation: 5") || strings.Contains(view, "\x1b[") {
		t.Fatalf("dashboard = %q", view)
	}
	_ = command
}

func programNext(t *testing.T, program *Program, message tea.Msg) tea.Cmd {
	t.Helper()
	_, command := program.Update(message)
	if command == nil {
		t.Fatal("Update returned no message command")
	}
	return command
}

func programUpdate(t *testing.T, program *Program, message tea.Msg) *Program {
	t.Helper()
	next, _ := program.Update(message)
	updated, ok := next.(*Program)
	if !ok {
		t.Fatalf("Update = %T, want *Program", next)
	}
	return updated
}

type programResult struct {
	snapshot transport.Snapshot
	err      error
}

type programAPI struct {
	calls   chan struct{}
	results chan programResult
}

func (api *programAPI) Snapshot(context.Context) (transport.Snapshot, error) {
	api.calls <- struct{}{}
	result := <-api.results
	return result.snapshot, result.err
}

type programStream struct{ subscriptions chan *programSubscription }

type programSubscription struct {
	events  chan transport.Event
	notices chan transport.StreamNotice
}

func (stream *programStream) Subscribe(_ context.Context, _ uint64) *transport.Subscription {
	subscription := &programSubscription{events: make(chan transport.Event), notices: make(chan transport.StreamNotice)}
	stream.subscriptions <- subscription
	return &transport.Subscription{Events: subscription.events, Notices: subscription.notices}
}

type programTimer struct {
	channel chan time.Time
	once    sync.Once
}

func (timer *programTimer) C() <-chan time.Time { return timer.channel }
func (timer *programTimer) Stop() bool {
	stopped := false
	timer.once.Do(func() { stopped = true })
	return stopped
}

type programTimers struct{ made chan *programTimer }

func (timers *programTimers) New(time.Duration) Timer {
	timer := &programTimer{channel: make(chan time.Time, 1)}
	timers.made <- timer
	return timer
}

func programSnapshot(generation, cursor uint64) transport.Snapshot {
	return transport.Snapshot{
		ModelVersion: wire.Schema, Generation: generation, Cursor: cursor, Fresh: true,
		Runtime: transport.Status{Running: true}, Repositories: []transport.Repository{{Name: "magicite"}},
		Seats: []transport.Seat{{Name: "ifrit", Role: "implementer", Busy: true, Task: "magicite-nbr.6", Repo: "magicite"}},
		Sessions: []transport.Session{{Handle: "ifrit", Phase: "run", Backend: "kiro", Model: "terra", UptimeSeconds: 60}},
		StatusCounts: []transport.StatusCount{{Status: "open", Count: 1}},
	}
}

func awaitProgram[T any](t *testing.T, channel <-chan T, name string) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
		var zero T
		return zero
	}
}

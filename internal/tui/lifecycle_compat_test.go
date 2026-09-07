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

func TestCoordinatorModelDashboardLifecycleCompatibility(t *testing.T) {
	clock := &lifecycleClock{now: time.Date(2026, time.September, 7, 19, 0, 0, 0, time.UTC)}
	api := &scriptedAPI{calls: make(chan struct{}, 4), releases: make(chan snapshotOutcome, 4)}
	stream := &scriptedStream{subscriptions: make(chan *scriptedSubscription, 2)}
	timers := &lifecycleTimerFactory{now: clock.Now, made: make(chan *lifecycleTimer, 2)}
	results := make(chan RefreshResult, 4)
	notices := make(chan RefreshNotice, 4)
	coordinator := NewRefreshCoordinator(api, stream, RefreshOptions{
		Cadence:  -1,
		NewTimer: timers.New,
		ResultSink: func(result RefreshResult) {
			results <- result
		},
		NoticeSink: func(notice RefreshNotice) {
			notices <- notice
		},
	})
	coordinator.Start(context.Background())
	t.Cleanup(coordinator.Cancel)

	model := NewModel(ModelOptions{Now: clock.Now})
	await(t, api.calls, "initial snapshot")
	first := await(t, stream.subscriptions, "initial subscription")
	first.events <- transport.Event{Seq: 9, Kind: wire.KindComplete, Task: "magicite-nbr.10"}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(await(t, notices, "stream event")))

	first.notices <- transport.StreamNotice{Kind: transport.StreamNoticeEOF, Cursor: 9}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(await(t, notices, "EOF notice")))
	close(first.events)
	close(first.notices)

	clock.now = clock.now.Add(time.Minute)
	api.releases <- snapshotOutcome{snapshot: lifecycleSnapshot(3, 9)}
	initial := await(t, results, "initial result")
	if !sameCauses(initial.Causes, []RefreshCause{RefreshInitial}) {
		t.Fatalf("initial causes = %#v", initial.Causes)
	}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(initial))

	await(t, api.calls, "EOF repair")
	clock.now = clock.now.Add(time.Minute)
	api.releases <- snapshotOutcome{snapshot: lifecycleSnapshot(4, 9)}
	eofRepair := await(t, results, "EOF repair result")
	if !sameCauses(eofRepair.Causes, []RefreshCause{RefreshEOF}) {
		t.Fatalf("EOF repair causes = %#v", eofRepair.Causes)
	}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(eofRepair))

	timer := await(t, timers.made, "reconnect timer")
	timer.fire()
	second := await(t, stream.subscriptions, "reconnect subscription")
	second.notices <- transport.StreamNotice{Kind: transport.StreamNoticeGap, Cursor: 11, MissingFrom: 10, MissingTo: 10}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(await(t, notices, "gap notice")))
	close(second.events)
	close(second.notices)

	await(t, api.calls, "gap repair")
	clock.now = clock.now.Add(time.Minute)
	api.releases <- snapshotOutcome{err: &transport.Error{Code: transport.ErrorUnavailable}}
	gapRepair := await(t, results, "gap repair result")
	if !sameCauses(gapRepair.Causes, []RefreshCause{RefreshGap}) {
		t.Fatalf("gap repair causes = %#v", gapRepair.Causes)
	}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(gapRepair))

	coordinator.Refresh()
	await(t, api.calls, "manual refresh")
	clock.now = clock.now.Add(time.Minute)
	api.releases <- snapshotOutcome{snapshot: lifecycleSnapshot(2, 12)}
	late := await(t, results, "late manual result")
	if !sameCauses(late.Causes, []RefreshCause{RefreshManual}) {
		t.Fatalf("manual causes = %#v", late.Causes)
	}
	model = lifecycleUpdate(t, model, lifecycleModelMessage(late))

	state := model.State()
	if state.Connection != ConnectionOffline || state.Freshness != SnapshotStale || state.Generation != 4 || state.Cursor != 11 || len(state.Events) != 1 || len(state.Notices) != 2 {
		t.Fatalf("lifecycle state = %#v", state)
	}
	if state.Snapshot.Generation != 4 || state.Snapshot.Cursor != 9 {
		t.Fatalf("late snapshot replaced state: %#v", state.Snapshot)
	}

	panels := lifecycleDashboardPanels(state, clock.now.Sub(state.ChangedAt))
	assertDashboardPanel(t, "header", RenderDashboardHeader(panels.header, 80), strings.Join([]string{
		"Dashboard",
		"connection: offline",
		"snapshot: stale generation: 4 age: 1m0s",
		"daemon: running; draining: yes",
		"refresh: error",
		"error: unavailable",
	}, "\n"))
	assertDashboardPanel(t, "summary", RenderDashboardSummary(panels.summary, 80), strings.Join([]string{
		"Summary",
		"repositories: 2",
		"sessions: 2",
		"beads: closed 1; open 4",
		"seats: free 1; busy 1",
	}, "\n"))
	assertDashboardPanel(t, "seats", RenderDashboardSeats(panels.seats, 80), "Seats\nifrit [implementer] assigned magicite-nbr.10 @magicite\nodin [reviewer] idle")
	assertDashboardPanel(t, "sessions", RenderDashboardSessions(panels.sessions, 80), "Sessions\nifrit [implementing] kiro/gpt-5.6-terra 1m1s\nodin [review] opencode/gpt-5.6-luna 1m0s")
	assertDashboardPanel(t, "events", RenderDashboardEvents(panels.events), "Recent events\n• complete: magicite-nbr.10\n! reconnect: eof\n! gap: cursor 10 to 10")
}

type lifecyclePanels struct {
	header   DashboardHeaderInput
	summary  DashboardSummaryInput
	seats    DashboardSeatsInput
	sessions DashboardSessionsInput
	events   DashboardEventsInput
}

func lifecycleDashboardPanels(state ModelState, age time.Duration) lifecyclePanels {
	free, busy := 0, 0
	for _, seat := range state.Snapshot.Seats {
		if seat.Busy {
			busy++
		} else {
			free++
		}
	}
	events := make([]DashboardEvent, 0, len(state.Events))
	for _, event := range state.Events {
		events = append(events, DashboardEvent{Kind: string(event.Event.Kind), Detail: event.Event.Task, Style: EventStyleSuccess})
	}
	notices := make([]DashboardNotice, 0, len(state.Notices))
	for _, notice := range state.Notices {
		switch notice.Kind {
		case transport.StreamNoticeEOF, transport.StreamNoticeUnavailable:
			notices = append(notices, DashboardNotice{Kind: DashboardNoticeReconnect, Detail: string(notice.Kind)})
		case transport.StreamNoticeGap:
			notices = append(notices, DashboardNotice{Kind: DashboardNoticeGap, Detail: "cursor 10 to 10"})
		case transport.StreamNoticeMiss:
			notices = append(notices, DashboardNotice{Kind: DashboardNoticeMiss, Detail: "events unavailable"})
		}
	}
	return lifecyclePanels{
		header: DashboardHeaderInput{
			Connection:  lifecycleConnection(state.Connection),
			Freshness:   lifecycleFreshness(state.Freshness),
			SnapshotAge: age,
			Generation:  state.Generation,
			Running:     state.Snapshot.Runtime.Running,
			Draining:    state.Snapshot.Runtime.Draining,
			Refresh:     DashboardRefreshError,
			Error:       (&transport.Error{Code: transport.ErrorUnavailable}).Error(),
		},
		summary: DashboardSummaryInput{
			Repositories: len(state.Snapshot.Repositories),
			Sessions:     len(state.Snapshot.Sessions),
			StatusCounts: state.Snapshot.StatusCounts,
			FreeSeats:    free,
			BusySeats:    busy,
		},
		seats:    DashboardSeatsInput{Seats: state.Snapshot.Seats},
		sessions: DashboardSessionsInput{Sessions: state.Snapshot.Sessions},
		events:   DashboardEventsInput{Events: events, Notices: notices, Width: 80},
	}
}

func lifecycleConnection(connection ConnectionState) DashboardConnectionState {
	switch connection {
	case ConnectionOnline:
		return DashboardConnectionConnected
	case ConnectionOffline:
		return DashboardConnectionOffline
	default:
		return DashboardConnectionLoading
	}
}

func lifecycleFreshness(freshness SnapshotFreshness) DashboardFreshness {
	switch freshness {
	case SnapshotFresh:
		return DashboardFreshnessFresh
	case SnapshotStale:
		return DashboardFreshnessStale
	default:
		return DashboardFreshnessLoading
	}
}

func lifecycleModelMessage(value any) tea.Msg {
	switch value := value.(type) {
	case RefreshResult:
		return SnapshotMsg{Snapshot: value.Snapshot, Error: value.Err}
	case RefreshNotice:
		if value.Kind == RefreshEvent {
			return StreamEventMsg{Event: value.Event}
		}
		return StreamNoticeMsg{Notice: value.Stream}
	default:
		panic("unexpected coordinator message")
	}
}

func lifecycleUpdate(t *testing.T, model Model, message tea.Msg) Model {
	t.Helper()
	next, command := model.Update(message)
	if command != nil {
		t.Fatal("Update returned unexpected command")
	}
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update model = %T, want tui.Model", next)
	}
	return updated
}

func lifecycleSnapshot(generation, cursor uint64) transport.Snapshot {
	return transport.Snapshot{
		ModelVersion: wire.Schema,
		Generation:   generation,
		Cursor:       cursor,
		Fresh:        true,
		Runtime:      transport.Status{Running: true, Draining: true},
		Repositories: []transport.Repository{{Name: "magicite"}, {Name: "other"}},
		Seats: []transport.Seat{
			{Name: "odin", Role: "reviewer"},
			{Name: "ifrit", Role: "implementer", Busy: true, Task: "magicite-nbr.10", Repo: "magicite"},
		},
		Sessions: []transport.Session{
			{Handle: "odin", Phase: "review", Backend: "opencode", Model: "gpt-5.6-luna", UptimeSeconds: 60},
			{Handle: "ifrit", Phase: "implementing", Backend: "kiro", Model: "gpt-5.6-terra", UptimeSeconds: 61},
		},
		StatusCounts: []transport.StatusCount{{Status: "open", Count: 4}, {Status: "closed", Count: 1}},
	}
}

func assertDashboardPanel(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

type lifecycleClock struct{ now time.Time }

func (clock *lifecycleClock) Now() time.Time { return clock.now }

type lifecycleTimer struct {
	channel chan time.Time
	stopped chan struct{}
	now     func() time.Time
	once    sync.Once
}

func (timer *lifecycleTimer) C() <-chan time.Time { return timer.channel }

func (timer *lifecycleTimer) Stop() bool {
	stopped := false
	timer.once.Do(func() {
		stopped = true
		close(timer.stopped)
	})
	return stopped
}

func (timer *lifecycleTimer) fire() { timer.channel <- timer.now() }

type lifecycleTimerFactory struct {
	now  func() time.Time
	made chan *lifecycleTimer
}

func (factory *lifecycleTimerFactory) New(time.Duration) Timer {
	timer := &lifecycleTimer{channel: make(chan time.Time, 1), stopped: make(chan struct{}), now: factory.now}
	factory.made <- timer
	return timer
}

package tui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

func TestRefreshCoordinatorCoalescesRequestsWithoutOverlap(t *testing.T) {
	api := &scriptedAPI{calls: make(chan struct{}, 4), releases: make(chan snapshotOutcome, 4)}
	results := make(chan RefreshResult, 4)
	coordinator := NewRefreshCoordinator(api, nil, RefreshOptions{
		Cadence: -1,
		ResultSink: func(result RefreshResult) {
			results <- result
		},
	})
	coordinator.Start(context.Background())
	t.Cleanup(coordinator.Cancel)

	await(t, api.calls, "initial snapshot")
	coordinator.Refresh()
	coordinator.Focus()
	if got := api.maximum(); got != 1 {
		t.Fatalf("overlapping Snapshot calls = %d, want 1", got)
	}
	api.releases <- snapshotOutcome{snapshot: transport.Snapshot{Cursor: 3}}
	initial := await(t, results, "initial result")
	if !sameCauses(initial.Causes, []RefreshCause{RefreshInitial}) {
		t.Fatalf("initial causes = %#v", initial.Causes)
	}

	await(t, api.calls, "coalesced snapshot")
	if got := api.maximum(); got != 1 {
		t.Fatalf("overlapping Snapshot calls = %d, want 1", got)
	}
	api.releases <- snapshotOutcome{snapshot: transport.Snapshot{Cursor: 4}}
	coalesced := await(t, results, "coalesced result")
	if !sameCauses(coalesced.Causes, []RefreshCause{RefreshManual, RefreshFocus}) {
		t.Fatalf("coalesced causes = %#v", coalesced.Causes)
	}
	if coalesced.Generation <= initial.Generation {
		t.Fatalf("generations = %d then %d", initial.Generation, coalesced.Generation)
	}
}

func TestRefreshCoordinatorRepairsStreamEOFAndCancelsTimers(t *testing.T) {
	api := &scriptedAPI{calls: make(chan struct{}, 4), releases: make(chan snapshotOutcome, 4)}
	stream := &scriptedStream{subscriptions: make(chan *scriptedSubscription, 4)}
	timers := &timerFactory{made: make(chan *manualTimer, 4)}
	results := make(chan RefreshResult, 4)
	notices := make(chan RefreshNotice, 4)
	coordinator := NewRefreshCoordinator(api, stream, RefreshOptions{
		Cadence: -1,
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

	await(t, api.calls, "initial snapshot")
	first := await(t, stream.subscriptions, "initial subscription")
	first.notices <- transport.StreamNotice{Kind: transport.StreamNoticeEOF, Cursor: 7}
	close(first.notices)
	close(first.events)
	notice := await(t, notices, "EOF notice")
	if notice.Kind != RefreshStream || notice.Stream.Kind != transport.StreamNoticeEOF {
		t.Fatalf("notice = %#v", notice)
	}

	api.releases <- snapshotOutcome{snapshot: transport.Snapshot{Cursor: 7}}
	await(t, results, "initial result")
	await(t, api.calls, "EOF repair")
	api.releases <- snapshotOutcome{snapshot: transport.Snapshot{Cursor: 7}}
	repair := await(t, results, "EOF repair result")
	if !sameCauses(repair.Causes, []RefreshCause{RefreshEOF}) {
		t.Fatalf("repair causes = %#v", repair.Causes)
	}

	timer := await(t, timers.made, "reconnect timer")
	timer.fire()
	second := await(t, stream.subscriptions, "reconnect subscription")
	second.notices <- transport.StreamNotice{Kind: transport.StreamNoticeUnavailable, Cursor: 7}
	close(second.notices)
	close(second.events)
	stopped := await(t, timers.made, "second reconnect timer")
	coordinator.Cancel()
	awaitClosed(t, stopped.stopped, "reconnect timer stop")
	select {
	case <-second.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Cancel() did not cancel owned stream")
	}
}

type snapshotOutcome struct {
	snapshot transport.Snapshot
	err      error
}

type scriptedAPI struct {
	calls    chan struct{}
	releases chan snapshotOutcome
	mu       sync.Mutex
	active   int
	peak     int
}

func (a *scriptedAPI) Snapshot(ctx context.Context) (transport.Snapshot, error) {
	a.mu.Lock()
	a.active++
	if a.active > a.peak {
		a.peak = a.active
	}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.active--
		a.mu.Unlock()
	}()
	select {
	case a.calls <- struct{}{}:
	case <-ctx.Done():
		return transport.Snapshot{}, ctx.Err()
	}
	select {
	case outcome := <-a.releases:
		return outcome.snapshot, outcome.err
	case <-ctx.Done():
		return transport.Snapshot{}, ctx.Err()
	}
}

func (a *scriptedAPI) maximum() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.peak
}

type scriptedStream struct{ subscriptions chan *scriptedSubscription }

type scriptedSubscription struct {
	ctx     context.Context
	events  chan transport.Event
	notices chan transport.StreamNotice
}

func (s *scriptedStream) Subscribe(ctx context.Context, _ uint64) *transport.Subscription {
	subscription := &scriptedSubscription{ctx: ctx, events: make(chan transport.Event, 2), notices: make(chan transport.StreamNotice, 2)}
	s.subscriptions <- subscription
	return &transport.Subscription{Events: subscription.events, Notices: subscription.notices}
}

type manualTimer struct {
	channel chan time.Time
	stopped chan struct{}
	mu      sync.Mutex
	isStop  bool
}

func (t *manualTimer) C() <-chan time.Time { return t.channel }

func (t *manualTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	wasStopped := t.isStop
	if !wasStopped {
		t.isStop = true
		close(t.stopped)
	}
	return !wasStopped
}

func (t *manualTimer) fire() { t.channel <- time.Now() }

type timerFactory struct{ made chan *manualTimer }

func (f *timerFactory) New(time.Duration) Timer {
	timer := &manualTimer{channel: make(chan time.Time, 1), stopped: make(chan struct{})}
	f.made <- timer
	return timer
}

func sameCauses(got, want []RefreshCause) bool {
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

func await[T any](t *testing.T, channel <-chan T, description string) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
		var zero T
		return zero
	}
}

func awaitClosed(t *testing.T, channel <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

var _ transport.DaemonAPI = (*scriptedAPI)(nil)
var _ transport.EventStream = (*scriptedStream)(nil)

package tui

import (
	"context"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

// RefreshCause identifies why the coordinator requested a daemon snapshot.
type RefreshCause string

const (
	RefreshInitial   RefreshCause = "initial"
	RefreshCadence   RefreshCause = "cadence"
	RefreshManual    RefreshCause = "manual"
	RefreshFocus     RefreshCause = "focus"
	RefreshReconnect RefreshCause = "reconnect"
	RefreshEOF       RefreshCause = "eof"
	RefreshMiss      RefreshCause = "miss"
	RefreshGap       RefreshCause = "gap"
)

// RefreshResult is one snapshot request outcome. Generation increases for
// each request, allowing a state owner to reject callbacks from an older run.
// Snapshot is populated only on a successful request; callers retain their
// own last-good snapshot after an error.
type RefreshResult struct {
	Generation uint64
	Causes     []RefreshCause
	Snapshot   transport.Snapshot
	Err        error
}

// RefreshNoticeKind identifies a stream annotation. Stream events and notices
// never update daemon state; snapshots remain the sole state authority.
type RefreshNoticeKind string

const (
	RefreshEvent  RefreshNoticeKind = "event"
	RefreshStream RefreshNoticeKind = "stream"
)

// RefreshNotice forwards one typed event-stream observation to the owner.
type RefreshNotice struct {
	Generation uint64
	Kind       RefreshNoticeKind
	Event      transport.Event
	Stream     transport.StreamNotice
}

// Timer is the cancellation-aware scheduling seam used by RefreshCoordinator.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// TimerFactory creates one coordinator timer. Tests may inject deterministic
// timers; production uses time.NewTimer.
type TimerFactory func(time.Duration) Timer

// RefreshOptions configures refresh cadence, reconnect retry, and callback
// delivery. A negative Cadence disables periodic refresh; zero uses five
// seconds. Callbacks may run from coordinator goroutines and must not block.
type RefreshOptions struct {
	Cadence        time.Duration
	InitialBackoff time.Duration
	MaximumBackoff time.Duration
	NewTimer       TimerFactory
	ResultSink     func(RefreshResult)
	NoticeSink     func(RefreshNotice)
}

// RefreshCoordinator serializes daemon snapshots and owns one event stream.
// It deliberately retains no snapshot or Bubble Tea state.
type RefreshCoordinator struct {
	api    transport.DaemonAPI
	stream transport.EventStream
	options RefreshOptions

	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	subscription *transport.Subscription
	cursor       uint64
	generation   uint64
	causes       map[RefreshCause]struct{}
	wake         chan struct{}
	started      bool
}

// NewRefreshCoordinator constructs a coordinator. Call Start before requesting
// refreshes; Cancel is safe before or after Start.
func NewRefreshCoordinator(api transport.DaemonAPI, stream transport.EventStream, options RefreshOptions) *RefreshCoordinator {
	if options.Cadence == 0 {
		options.Cadence = 5 * time.Second
	}
	if options.NewTimer == nil {
		options.NewTimer = newTimer
	}
	return &RefreshCoordinator{
		api: api, stream: stream, options: options,
		causes: make(map[RefreshCause]struct{}), wake: make(chan struct{}, 1),
	}
}

// Start begins cadence, snapshots, and event subscription ownership. Repeated
// calls are ignored. The initial snapshot is requested immediately.
func (c *RefreshCoordinator) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.started = true
	run := c.ctx
	c.mu.Unlock()

	go c.refreshLoop(run)
	if c.options.Cadence > 0 {
		go c.cadenceLoop(run)
	}
	if c.stream != nil {
		go c.streamLoop(run)
	}
	c.Request(RefreshInitial)
}

// Request coalesces an invalidation into the next serialized snapshot request.
func (c *RefreshCoordinator) Request(cause RefreshCause) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if !c.started || c.ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	c.causes[cause] = struct{}{}
	select {
	case c.wake <- struct{}{}:
	default:
	}
	c.mu.Unlock()
}

// Refresh requests a user-initiated snapshot.
func (c *RefreshCoordinator) Refresh() { c.Request(RefreshManual) }

// Focus requests repair after the TUI regains focus.
func (c *RefreshCoordinator) Focus() { c.Request(RefreshFocus) }

// Cancel stops cadence and reconnect timers and closes the owned stream.
func (c *RefreshCoordinator) Cancel() {
	if c == nil {
		return
	}
	c.mu.Lock()
	cancel := c.cancel
	subscription := c.subscription
	c.mu.Unlock()
	if subscription != nil {
		subscription.Cancel()
	}
	if cancel != nil {
		cancel()
	}
}

func (c *RefreshCoordinator) refreshLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		}
		causes, generation := c.takeRequest()
		if len(causes) == 0 {
			continue
		}
		var snapshot transport.Snapshot
		var err error
		if c.api == nil {
			err = &transport.Error{Code: transport.ErrorUnavailable}
		} else {
			snapshot, err = c.api.Snapshot(ctx)
		}
		if err == nil {
			c.advanceCursor(snapshot.Cursor)
		}
		c.sendResult(RefreshResult{Generation: generation, Causes: causes, Snapshot: snapshot, Err: err})
	}
}

func (c *RefreshCoordinator) takeRequest() ([]RefreshCause, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	causes := make([]RefreshCause, 0, len(c.causes))
	for _, cause := range []RefreshCause{RefreshInitial, RefreshCadence, RefreshManual, RefreshFocus, RefreshReconnect, RefreshEOF, RefreshMiss, RefreshGap} {
		if _, ok := c.causes[cause]; ok {
			causes = append(causes, cause)
		}
	}
	c.causes = make(map[RefreshCause]struct{})
	c.generation++
	return causes, c.generation
}

func (c *RefreshCoordinator) cadenceLoop(ctx context.Context) {
	for c.wait(ctx, c.options.Cadence) {
		c.Request(RefreshCadence)
	}
}

func (c *RefreshCoordinator) streamLoop(ctx context.Context) {
	backoff := NewBackoff(c.options.InitialBackoff, c.options.MaximumBackoff)
	for ctx.Err() == nil {
		subscription := c.subscribe(ctx)
		if subscription == nil {
			return
		}
		hadEvent, terminal := c.relaySubscription(ctx, subscription)
		c.clearSubscription(subscription)
		if ctx.Err() != nil || terminal == transport.StreamNoticeSchemaMismatch {
			return
		}
		if hadEvent {
			backoff.Reset()
		}
		if !c.wait(ctx, backoff.Next()) {
			return
		}
	}
}

func (c *RefreshCoordinator) subscribe(ctx context.Context) *transport.Subscription {
	c.mu.Lock()
	since := c.cursor
	c.mu.Unlock()
	subscription := c.stream.Subscribe(ctx, since)
	c.mu.Lock()
	if c.ctx == ctx && ctx.Err() == nil {
		c.subscription = subscription
	} else if subscription != nil {
		subscription.Cancel()
	}
	c.mu.Unlock()
	return subscription
}

func (c *RefreshCoordinator) relaySubscription(ctx context.Context, subscription *transport.Subscription) (bool, transport.StreamNoticeKind) {
	events, notices := subscription.Events, subscription.Notices
	hadEvent := false
	var terminal transport.StreamNoticeKind
	for events != nil || notices != nil {
		select {
		case <-ctx.Done():
			return hadEvent, terminal
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			hadEvent = true
			c.advanceCursor(event.Seq)
			c.sendNotice(RefreshNotice{Generation: c.currentGeneration(), Kind: RefreshEvent, Event: event})
		case notice, ok := <-notices:
			if !ok {
				notices = nil
				continue
			}
			terminal = notice.Kind
			c.sendNotice(RefreshNotice{Generation: c.currentGeneration(), Kind: RefreshStream, Stream: notice})
			switch notice.Kind {
			case transport.StreamNoticeEOF:
				c.Request(RefreshEOF)
			case transport.StreamNoticeMiss:
				c.Request(RefreshMiss)
			case transport.StreamNoticeGap:
				c.Request(RefreshGap)
			case transport.StreamNoticeUnavailable:
				c.Request(RefreshReconnect)
			}
		}
	}
	return hadEvent, terminal
}

func (c *RefreshCoordinator) wait(ctx context.Context, duration time.Duration) bool {
	timer := c.options.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C():
		return true
	}
}

func (c *RefreshCoordinator) advanceCursor(cursor uint64) {
	c.mu.Lock()
	if cursor > c.cursor {
		c.cursor = cursor
	}
	c.mu.Unlock()
}

func (c *RefreshCoordinator) clearSubscription(subscription *transport.Subscription) {
	c.mu.Lock()
	if c.subscription == subscription {
		c.subscription = nil
	}
	c.mu.Unlock()
}

func (c *RefreshCoordinator) currentGeneration() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}

func (c *RefreshCoordinator) sendResult(result RefreshResult) {
	if c.options.ResultSink != nil {
		c.options.ResultSink(result)
	}
}

func (c *RefreshCoordinator) sendNotice(notice RefreshNotice) {
	if c.options.NoticeSink != nil {
		c.options.NoticeSink(notice)
	}
}

type coordinatorTimer struct{ *time.Timer }

func (t coordinatorTimer) C() <-chan time.Time { return t.Timer.C }

func newTimer(duration time.Duration) Timer { return coordinatorTimer{time.NewTimer(duration)} }

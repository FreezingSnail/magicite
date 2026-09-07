package transport

import (
	"context"
	"errors"
	"math"
	"sync/atomic"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

// Event is one ordered daemon event delivered to the TUI.
type Event = wire.Event

// StreamClient is the daemon event capability required by EventStream.
type StreamClient interface {
	Stream(context.Context, uint64, func(wire.Event) error, ...bool) (uint64, error)
}

// EventStream starts independently owned daemon event subscriptions.
type EventStream interface {
	Subscribe(context.Context, uint64) *Subscription
}

type eventStream struct {
	client StreamClient
}

// NewEventStream adapts a daemon event client for the TUI.
func NewEventStream(client StreamClient) EventStream {
	return &eventStream{client: client}
}

// Subscription owns one daemon stream connection. Events retain daemon
// sequence order. Cancel closes its connection; Highest returns its latest
// delivered or reported cursor.
type Subscription struct {
	Events  <-chan Event
	Notices <-chan StreamNotice

	cancel  context.CancelFunc
	highest atomic.Uint64
}

// Cancel stops this subscription and closes its daemon connection.
func (s *Subscription) Cancel() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

// Highest returns the greatest cursor observed by this subscription.
func (s *Subscription) Highest() uint64 {
	if s == nil {
		return 0
	}
	return s.highest.Load()
}

// Subscribe starts one daemon subscription after since. The returned
// subscription is independent of every request and other subscription.
func (s *eventStream) Subscribe(ctx context.Context, since uint64) *Subscription {
	if ctx == nil {
		ctx = context.Background()
	}
	run, cancel := context.WithCancel(ctx)
	events := make(chan Event)
	notices := make(chan StreamNotice)
	subscription := &Subscription{Events: events, Notices: notices, cancel: cancel}
	subscription.highest.Store(since)

	go func() {
		defer close(events)
		defer close(notices)
		defer cancel()
		if s.client == nil {
			subscription.sendNotice(run, notices, StreamNotice{Kind: StreamNoticeUnavailable, Cursor: subscription.Highest(), Code: wire.CodeUnavailable})
			return
		}

		started := false
		highest, err := s.client.Stream(run, since, func(event wire.Event) error {
			previous := subscription.Highest()
			if expected, ok := nextSequence(previous); ok && event.Seq > expected {
				kind := StreamNoticeGap
				if !started {
					kind = StreamNoticeMiss
				}
				if !subscription.sendNotice(run, notices, StreamNotice{
					Kind:        kind,
					Cursor:      event.Seq,
					MissingFrom: expected,
					MissingTo:   event.Seq - 1,
				}) {
					return run.Err()
				}
			}
			if !subscription.sendEvent(run, events, event) {
				return run.Err()
			}
			subscription.highest.Store(event.Seq)
			started = true
			return nil
		})
		if highest != 0 || since == 0 {
			subscription.highest.Store(highest)
		}
		if run.Err() != nil {
			return
		}
		if err == nil {
			subscription.sendNotice(run, notices, StreamNotice{Kind: StreamNoticeEOF, Cursor: subscription.Highest()})
			return
		}
		subscription.sendNotice(run, notices, terminalNotice(subscription.Highest(), err))
	}()
	return subscription
}

func (s *Subscription) sendEvent(ctx context.Context, events chan<- Event, event Event) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Subscription) sendNotice(ctx context.Context, notices chan<- StreamNotice, notice StreamNotice) bool {
	select {
	case notices <- notice:
		return true
	case <-ctx.Done():
		return false
	}
}

func nextSequence(cursor uint64) (uint64, bool) {
	if cursor == math.MaxUint64 {
		return 0, false
	}
	return cursor + 1, true
}

func terminalNotice(cursor uint64, err error) StreamNotice {
	var daemonError *client.Error
	if errors.As(err, &daemonError) {
		switch daemonError.Code {
		case wire.CodeSchemaMismatch:
			return StreamNotice{Kind: StreamNoticeSchemaMismatch, Cursor: cursor, Code: daemonError.Code}
		case wire.CodeInternal:
			// client.Stream reserves internal errors for malformed or non-monotonic
			// frames. Both invalidate contiguous event history.
			return StreamNotice{Kind: StreamNoticeGap, Cursor: cursor, Code: daemonError.Code}
		default:
			return StreamNotice{Kind: StreamNoticeUnavailable, Cursor: cursor, Code: daemonError.Code}
		}
	}
	return StreamNotice{Kind: StreamNoticeUnavailable, Cursor: cursor, Code: wire.CodeUnavailable}
}

var _ EventStream = (*eventStream)(nil)

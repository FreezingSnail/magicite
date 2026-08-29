package server

import (
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const defaultBusCapacity = 1024

// Bus retains recent events and offers them to subscribers without waiting for
// a subscriber to consume them.
type Bus struct {
	mu       sync.Mutex
	capacity int
	ring     []wire.Event
	head     int
	count    int
	next     uint64
	closed   bool
	subs     map[*Subscription]struct{}
	clock    func() time.Time
}

// NewBus creates a bus retaining capacity events. Non-positive capacities use
// the default retention window.
func NewBus(capacity int) *Bus {
	if capacity <= 0 {
		capacity = defaultBusCapacity
	}
	return &Bus{
		capacity: capacity,
		ring:     make([]wire.Event, capacity),
		subs:     make(map[*Subscription]struct{}),
		clock:    time.Now,
	}
}

// Publish assigns an event sequence, records the event, and offers it to each
// subscriber. It returns zero after Close.
func (b *Bus) Publish(event wire.Event) uint64 {
	var warnings uint64

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return 0
	}
	b.next++
	event.Seq = b.next
	event.Schema = wire.Schema
	if event.Time.IsZero() {
		event.Time = b.clock()
	}
	b.appendLocked(event)
	for subscription := range b.subs {
		select {
		case subscription.events <- event:
		default:
			subscription.dropped++
			if !subscription.warned {
				subscription.warned = true
				warnings++
			}
		}
	}
	b.mu.Unlock()

	for range warnings {
		go logging.Event(logging.Warn, logging.KindWarn, map[string]any{"event": "subscriber.dropped"})
	}
	return event.Seq
}

// Log projects and publishes a structured log record when it maps to a wire
// event.
func (b *Bus) Log(level, kind string, fields map[string]string) {
	if event, ok := Project(level, kind, fields); ok {
		b.Publish(event)
	}
}

// Subscribe replays retained events newer than since before accepting live
// events. Missed reports the final unavailable sequence when retention has
// advanced past since.
func (b *Bus) Subscribe(since uint64, buffer int) *Subscription {
	if buffer < 0 {
		buffer = 0
	}

	b.mu.Lock()
	replay, missed, hasMissed := b.replayLocked(since)
	if len(replay) > buffer {
		buffer = len(replay)
	}
	subscription := &Subscription{
		bus:       b,
		events:    make(chan wire.Event, buffer),
		missed:    missed,
		hasMissed: hasMissed,
	}
	for _, event := range replay {
		subscription.events <- event
	}
	if b.closed {
		close(subscription.events)
	} else {
		b.subs[subscription] = struct{}{}
	}
	b.mu.Unlock()
	return subscription
}

// Last returns the most recently assigned sequence.
func (b *Bus) Last() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.next
}

// Close stops future publication and closes every subscription.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for subscription := range b.subs {
		close(subscription.events)
		delete(b.subs, subscription)
	}
}

func (b *Bus) appendLocked(event wire.Event) {
	if b.count < b.capacity {
		index := (b.head + b.count) % b.capacity
		b.ring[index] = event
		b.count++
		return
	}
	b.ring[b.head] = event
	b.head = (b.head + 1) % b.capacity
}

func (b *Bus) replayLocked(since uint64) ([]wire.Event, uint64, bool) {
	if b.count == 0 {
		return nil, 0, false
	}
	oldest := b.ring[b.head].Seq
	missed := uint64(0)
	hasMissed := oldest > 0 && since < oldest-1
	if hasMissed {
		missed = oldest - 1
	}
	start := since
	if hasMissed {
		start = oldest - 1
	}
	replay := make([]wire.Event, 0, b.count)
	for offset := 0; offset < b.count; offset++ {
		event := b.ring[(b.head+offset)%b.capacity]
		if event.Seq > start {
			replay = append(replay, event)
		}
	}
	return replay, missed, hasMissed
}

// Subscription receives one ordered event stream from a Bus.
type Subscription struct {
	bus       *Bus
	events    chan wire.Event
	dropped   uint64
	warned    bool
	missed    uint64
	hasMissed bool
}

// Events returns the subscription event stream.
func (s *Subscription) Events() <-chan wire.Event { return s.events }

// Dropped returns the number of events not queued because this subscription
// was full.
func (s *Subscription) Dropped() uint64 {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()
	return s.dropped
}

// Missed reports the final sequence unavailable from the initial replay.
func (s *Subscription) Missed() (uint64, bool) {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()
	return s.missed, s.hasMissed
}

// Close removes the subscription and closes its event stream.
func (s *Subscription) Close() {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()
	if _, ok := s.bus.subs[s]; !ok {
		return
	}
	delete(s.bus.subs, s)
	close(s.events)
}

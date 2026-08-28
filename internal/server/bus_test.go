package server

import (
	"sync"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/wire"
)

func TestBusPublishStampsSequenceSchemaAndTime(t *testing.T) {
	bus := NewBus(2)
	at := time.Date(2026, time.August, 28, 20, 0, 0, 0, time.UTC)
	bus.clock = func() time.Time { return at }
	if got := bus.Publish(wire.Event{Kind: wire.KindLand}); got != 1 {
		t.Fatalf("first sequence = %d, want 1", got)
	}
	if got := bus.Publish(wire.Event{Kind: wire.KindClose}); got != 2 {
		t.Fatalf("second sequence = %d, want 2", got)
	}
	subscription := bus.Subscribe(0, 2)
	defer subscription.Close()
	for want := uint64(1); want <= 2; want++ {
		event := <-subscription.Events()
		if event.Seq != want || event.Schema != wire.Schema || !event.Time.Equal(at) {
			t.Errorf("event %d = %#v", want, event)
		}
	}
	if got := bus.Last(); got != 2 {
		t.Errorf("Last = %d, want 2", got)
	}
}

func TestBusReplaysWindowThenLiveAndReportsMiss(t *testing.T) {
	bus := NewBus(2)
	for range 4 {
		bus.Publish(wire.Event{Kind: wire.KindLand})
	}
	subscription := bus.Subscribe(1, 2)
	defer subscription.Close()
	if missed, ok := subscription.Missed(); !ok || missed != 2 {
		t.Errorf("Missed = %d, %t, want 2, true", missed, ok)
	}
	if got := (<-subscription.Events()).Seq; got != 3 {
		t.Errorf("first replay = %d, want 3", got)
	}
	if got := (<-subscription.Events()).Seq; got != 4 {
		t.Errorf("second replay = %d, want 4", got)
	}
	bus.Publish(wire.Event{Kind: wire.KindClose})
	if got := (<-subscription.Events()).Seq; got != 5 {
		t.Errorf("live event = %d, want 5", got)
	}
}

func TestBusDropsFullSubscriptionWithoutBlocking(t *testing.T) {
	bus := NewBus(1)
	subscription := bus.Subscribe(0, 1)
	defer subscription.Close()
	bus.Publish(wire.Event{Kind: wire.KindLand})
	bus.Publish(wire.Event{Kind: wire.KindClose})
	if got := subscription.Dropped(); got != 1 {
		t.Errorf("Dropped = %d, want 1", got)
	}
	if got := (<-subscription.Events()).Seq; got != 1 {
		t.Errorf("queued event = %d, want 1", got)
	}
}

func TestBusCloseRacesSubscriptionCloseAndPublish(t *testing.T) {
	bus := NewBus(8)
	subscription := bus.Subscribe(0, 8)
	var group sync.WaitGroup
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			bus.Publish(wire.Event{Kind: wire.KindLand})
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		subscription.Close()
	}()
	group.Add(1)
	go func() {
		defer group.Done()
		bus.Close()
	}()
	group.Wait()
	if got := bus.Publish(wire.Event{Kind: wire.KindLand}); got != 0 {
		t.Errorf("Publish after Close = %d, want 0", got)
	}
}

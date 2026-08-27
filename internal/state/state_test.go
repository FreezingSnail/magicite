package state

import (
	"sync"
	"testing"
	"time"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

func TestStoreExpiresEntriesUsingInjectedClock(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)}
	store := New(time.Second, clock.Now)
	store.Put("status", "ready")

	value, fresh := store.Get("status")
	if !fresh || value != "ready" {
		t.Fatalf("Get before expiry = (%#v, %t), want (ready, true)", value, fresh)
	}

	clock.Advance(time.Second)
	value, fresh = store.Get("status")
	if fresh || value != nil {
		t.Errorf("Get at expiry = (%#v, %t), want (nil, false)", value, fresh)
	}
}

func TestStoreInvalidatesOneEntryAndResetsAll(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)}
	store := New(time.Minute, clock.Now)
	store.Put("one", 1)
	store.Put("two", 2)

	store.Invalidate("one")
	if value, fresh := store.Get("one"); fresh || value != nil {
		t.Errorf("Get(one) after Invalidate = (%#v, %t), want (nil, false)", value, fresh)
	}
	if value, fresh := store.Get("two"); !fresh || value != 2 {
		t.Errorf("Get(two) after Invalidate(one) = (%#v, %t), want (2, true)", value, fresh)
	}
	select {
	case <-store.Invalidations():
	default:
		t.Error("Invalidate did not notify")
	}

	store.Reset()
	if value, fresh := store.Get("two"); fresh || value != nil {
		t.Errorf("Get(two) after Reset = (%#v, %t), want (nil, false)", value, fresh)
	}
	select {
	case <-store.Invalidations():
	default:
		t.Error("Reset did not notify")
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)}
	store := New(time.Minute, clock.Now)

	var group sync.WaitGroup
	for worker := range 16 {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for iteration := range 100 {
				key := string(rune('a' + (worker+iteration)%4))
				store.Put(key, iteration)
				store.Get(key)
				if iteration%10 == 0 {
					store.Invalidate(key)
				}
				if iteration%25 == 0 {
					store.Reset()
				}
			}
		}(worker)
	}
	group.Wait()
}

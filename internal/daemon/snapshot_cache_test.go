package daemon

import (
	"sync"
	"testing"
	"time"
)

type snapshotCacheClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *snapshotCacheClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *snapshotCacheClock) Advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

type snapshotCacheRecord struct {
	items []string
	meta  map[string][]string
}

func cloneSnapshotCacheRecord(record snapshotCacheRecord) snapshotCacheRecord {
	clone := snapshotCacheRecord{
		items: append([]string(nil), record.items...),
		meta:  make(map[string][]string, len(record.meta)),
	}
	for key, values := range record.meta {
		clone.meta[key] = append([]string(nil), values...)
	}
	return clone
}

func TestSnapshotCacheClonesAcceptedAndReturnedValues(t *testing.T) {
	clock := &snapshotCacheClock{now: time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)}
	cache := newSnapshotCache(time.Minute, clock.Now, cloneSnapshotCacheRecord)
	input := snapshotCacheRecord{
		items: []string{"original"},
		meta:  map[string][]string{"labels": {"staged"}},
	}

	cache.Accept(input)
	input.items[0] = "mutated input"
	input.meta["labels"][0] = "mutated input"

	fresh, generation, ok := cache.Fresh()
	if !ok || generation != 1 {
		t.Fatalf("Fresh() = (%#v, %d, %t), want generation 1 and true", fresh, generation, ok)
	}
	if fresh.items[0] != "original" || fresh.meta["labels"][0] != "staged" {
		t.Fatalf("Fresh() retained caller mutation: %#v", fresh)
	}

	fresh.items[0] = "mutated result"
	fresh.meta["labels"][0] = "mutated result"
	latest, _, ok := cache.Latest()
	if !ok {
		t.Fatal("Latest() = no value")
	}
	if latest.items[0] != "original" || latest.meta["labels"][0] != "staged" {
		t.Fatalf("Latest() retained returned mutation: %#v", latest)
	}
}

func TestSnapshotCacheExpiresAtBoundaryAndRetainsLatest(t *testing.T) {
	clock := &snapshotCacheClock{now: time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)}
	cache := newSnapshotCache(time.Second, clock.Now, func(value string) string { return value })
	cache.Accept("ready")

	if value, generation, ok := cache.Fresh(); !ok || value != "ready" || generation != 1 {
		t.Fatalf("Fresh() before expiry = (%q, %d, %t), want (ready, 1, true)", value, generation, ok)
	}

	clock.Advance(time.Second)
	if value, generation, ok := cache.Fresh(); ok || value != "" || generation != 0 {
		t.Errorf("Fresh() at expiry = (%q, %d, %t), want (empty, 0, false)", value, generation, ok)
	}
	if value, generation, ok := cache.Latest(); !ok || value != "ready" || generation != 1 {
		t.Errorf("Latest() at expiry = (%q, %d, %t), want (ready, 1, true)", value, generation, ok)
	}
}

func TestSnapshotCacheAcceptAndInvalidateControlGeneration(t *testing.T) {
	clock := &snapshotCacheClock{now: time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)}
	cache := newSnapshotCache(time.Minute, clock.Now, func(value string) string { return value })

	cache.Invalidate()
	if value, generation, ok := cache.Latest(); ok || value != "" || generation != 0 {
		t.Fatalf("Latest() after empty Invalidate = (%q, %d, %t), want (empty, 0, false)", value, generation, ok)
	}

	if generation := cache.Accept("first"); generation != 1 {
		t.Fatalf("first Accept() generation = %d, want 1", generation)
	}
	cache.Invalidate()
	if value, generation, ok := cache.Fresh(); ok || value != "" || generation != 0 {
		t.Errorf("Fresh() after Invalidate = (%q, %d, %t), want (empty, 0, false)", value, generation, ok)
	}
	if value, generation, ok := cache.Latest(); !ok || value != "first" || generation != 1 {
		t.Errorf("Latest() after Invalidate = (%q, %d, %t), want (first, 1, true)", value, generation, ok)
	}
	if generation := cache.Accept("second"); generation != 2 {
		t.Errorf("second Accept() generation = %d, want 2", generation)
	}
}

package daemon

import (
	"sync"
	"time"
)

// snapshotCache retains the latest accepted snapshot and exposes it as fresh
// until its expiry. Values cross the cache boundary only through clone, so the
// cache never shares caller-owned mutable storage.
type snapshotCache[T any] struct {
	mu      sync.RWMutex
	clock   func() time.Time
	ttl     time.Duration
	clone   func(T) T
	latest  T
	expires time.Time
	valid   bool
	fresh   bool
	version uint64
}

// newSnapshotCache creates a cache with the supplied lifetime, clock, and
// cloning behavior. A clone function is required to preserve cache ownership.
func newSnapshotCache[T any](ttl time.Duration, clock func() time.Time, clone func(T) T) *snapshotCache[T] {
	if clock == nil {
		clock = time.Now
	}
	if clone == nil {
		panic("daemon: snapshot cache clone is nil")
	}

	return &snapshotCache[T]{
		clock: clock,
		ttl:   ttl,
		clone: clone,
	}
}

// Accept replaces the latest snapshot, starts its freshness lifetime, and
// returns its new monotonic generation.
func (c *snapshotCache[T]) Accept(value T) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.latest = c.clone(value)
	c.expires = c.clock().Add(c.ttl)
	c.valid = true
	c.fresh = true
	c.version++
	return c.version
}

// Fresh returns the current snapshot only before its expiry. A snapshot expires
// exactly at its expiry boundary and remains available through Latest.
func (c *snapshotCache[T]) Fresh() (value T, generation uint64, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.valid || !c.fresh || !c.clock().Before(c.expires) {
		c.fresh = false
		return value, 0, false
	}
	return c.clone(c.latest), c.version, true
}

// Latest returns the most recently accepted snapshot, including expired or
// invalidated snapshots, for stale-result policy owned by the caller.
func (c *snapshotCache[T]) Latest() (value T, generation uint64, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.valid {
		return value, 0, false
	}
	return c.clone(c.latest), c.version, true
}

// Invalidate marks the current snapshot stale without discarding the latest
// accepted value or allocating a generation.
func (c *snapshotCache[T]) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fresh = false
}

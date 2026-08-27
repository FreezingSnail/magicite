// Package state stores short-lived in-memory snapshots.
package state

import (
	"sync"
	"time"
)

// DefaultTTL is the lifetime used by Default.
const DefaultTTL = 5 * time.Second

// Clock supplies the current time. Supplying a clock keeps expiry deterministic
// in tests.
type Clock func() time.Time

// Store holds independently expiring snapshot values. Its methods are safe for
// concurrent use. It performs neither I/O nor refresh work.
type Store struct {
	mu      sync.RWMutex
	ttl     time.Duration
	clock   Clock
	entries map[string]entry

	invalidations chan struct{}
}

type entry struct {
	value     any
	updatedAt time.Time
}

// Default creates a Store with the five-second default lifetime and wall clock.
func Default() *Store {
	return New(DefaultTTL, time.Now)
}

// New creates a Store whose entries remain fresh for ttl. A nil clock uses the
// wall clock. A non-positive ttl uses DefaultTTL.
func New(ttl time.Duration, clock Clock) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if clock == nil {
		clock = time.Now
	}

	return &Store{
		ttl:           ttl,
		clock:         clock,
		entries:       make(map[string]entry),
		invalidations: make(chan struct{}, 1),
	}
}

// Get returns the snapshot stored at key and whether it remains fresh. Missing
// and expired entries return nil and false. Expired entries are retained until
// replaced, invalidated, or reset.
func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[key]
	if !ok || !s.clock().Before(entry.updatedAt.Add(s.ttl)) {
		return nil, false
	}
	return entry.value, true
}

// Put stores value at key and starts its freshness lifetime.
func (s *Store) Put(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = entry{value: value, updatedAt: s.clock()}
}

// Invalidate removes the snapshot at key. It signals a coalesced invalidation
// notification when an entry was removed.
func (s *Store) Invalidate(key string) {
	s.mu.Lock()
	_, ok := s.entries[key]
	if ok {
		delete(s.entries, key)
	}
	s.mu.Unlock()

	if ok {
		s.notify()
	}
}

// Reset removes every snapshot. It signals a coalesced invalidation
// notification when it removed at least one entry.
func (s *Store) Reset() {
	s.mu.Lock()
	hadEntries := len(s.entries) > 0
	if hadEntries {
		s.entries = make(map[string]entry)
	}
	s.mu.Unlock()

	if hadEntries {
		s.notify()
	}
}

// Invalidations returns a coalesced notification channel. The channel is never
// closed; its purpose is to wake readers, not to enumerate invalidated keys.
func (s *Store) Invalidations() <-chan struct{} {
	return s.invalidations
}

func (s *Store) notify() {
	select {
	case s.invalidations <- struct{}{}:
	default:
	}
}

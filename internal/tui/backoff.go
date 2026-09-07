package tui

import "time"

const (
	defaultInitialBackoff = 100 * time.Millisecond
	defaultMaximumBackoff = 5 * time.Second
)

// Backoff returns bounded exponential retry delays. It is intended for one
// retry loop; concurrent callers must synchronize access.
type Backoff struct {
	initial time.Duration
	maximum time.Duration
	next    time.Duration
}

// NewBackoff constructs a bounded exponential retry schedule. Non-positive
// initial values use the TUI default; maximum values below initial are raised
// to initial.
func NewBackoff(initial, maximum time.Duration) *Backoff {
	if initial <= 0 {
		initial = defaultInitialBackoff
	}
	if maximum <= 0 {
		maximum = defaultMaximumBackoff
	}
	if maximum < initial {
		maximum = initial
	}
	return &Backoff{initial: initial, maximum: maximum, next: initial}
}

// Next returns the current delay and advances the schedule without exceeding
// its configured maximum.
func (b *Backoff) Next() time.Duration {
	if b == nil {
		return defaultInitialBackoff
	}
	delay := b.next
	if delay >= b.maximum || delay > b.maximum/2 {
		b.next = b.maximum
	} else {
		b.next = delay * 2
	}
	return delay
}

// Reset makes the next delay the initial configured delay.
func (b *Backoff) Reset() {
	if b != nil {
		b.next = b.initial
	}
}

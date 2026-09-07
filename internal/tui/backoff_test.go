package tui

import (
	"testing"
	"time"
)

func TestBackoffBoundsExponentialDelays(t *testing.T) {
	backoff := NewBackoff(time.Millisecond, 4*time.Millisecond)
	want := []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond, 4 * time.Millisecond}
	for index, delay := range want {
		if got := backoff.Next(); got != delay {
			t.Fatalf("Next() call %d = %s, want %s", index, got, delay)
		}
	}
	backoff.Reset()
	if got := backoff.Next(); got != time.Millisecond {
		t.Fatalf("Next() after Reset() = %s, want %s", got, time.Millisecond)
	}
}

func TestBackoffNormalizesInvalidBounds(t *testing.T) {
	if got := NewBackoff(0, 0).Next(); got != defaultInitialBackoff {
		t.Fatalf("default Next() = %s, want %s", got, defaultInitialBackoff)
	}
	if got := NewBackoff(4*time.Millisecond, time.Millisecond).Next(); got != 4*time.Millisecond {
		t.Fatalf("raised-cap Next() = %s, want %s", got, 4*time.Millisecond)
	}
}

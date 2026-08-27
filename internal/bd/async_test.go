package bd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCoalescerSharesCallsAndCopiesResults(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"ready"}, Stdout: "out", Stderr: "err", DelayMS: 150})
	coalescer := NewCoalescer(client)

	const waiters = 8
	results := make([]Result, waiters)
	errs := make([]error, waiters)
	start := make(chan struct{})
	var group sync.WaitGroup
	for index := range results {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results[index], errs[index] = coalescer.Call(context.Background(), "ready", "--json")
		}()
	}
	close(start)
	group.Wait()

	if got := len(fakeCalls(t, client)); got != 1 {
		t.Fatalf("bd calls = %d, want 1", got)
	}
	for index, err := range errs {
		if err != nil {
			t.Errorf("waiter %d error = %v", index, err)
		}
		if got := results[index]; got.ExitCode != 0 || string(got.Stdout) != "out" || string(got.Stderr) != "err" {
			t.Errorf("waiter %d result = %+v", index, got)
		}
	}
	results[0].Stdout[0] = 'x'
	results[0].Stderr[0] = 'x'
	for index := 1; index < waiters; index++ {
		if string(results[index].Stdout) != "out" || string(results[index].Stderr) != "err" {
			t.Errorf("waiter %d shares output buffers: %+v", index, results[index])
		}
	}
	if got := coalescer.InFlight(); got != 0 {
		t.Errorf("InFlight() = %d, want 0", got)
	}
}

func TestCoalescerRunsDistinctArgv(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"ready"}, DelayMS: 100})
	coalescer := NewCoalescer(client)

	const calls = 4
	var group sync.WaitGroup
	for index := 0; index < calls; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			if _, err := coalescer.Call(context.Background(), "ready", string(rune('a'+index))); err != nil {
				t.Errorf("Call(%d) error = %v", index, err)
			}
		}(index)
	}
	group.Wait()

	if got := len(fakeCalls(t, client)); got != calls {
		t.Errorf("bd calls = %d, want %d", got, calls)
	}
	if got := coalescer.InFlight(); got != 0 {
		t.Errorf("InFlight() = %d, want 0", got)
	}
}

func TestCoalescerWaiterCancellationDoesNotCancelSharedCall(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"ready"}, Stdout: "out", DelayMS: 250})
	coalescer := NewCoalescer(client)
	completed := make(chan struct {
		result Result
		err    error
	}, 1)
	go func() {
		result, err := coalescer.Call(context.Background(), "ready")
		completed <- struct {
			result Result
			err    error
		}{result, err}
	}()
	waitForFakeCalls(t, client, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := coalescer.Call(ctx, "ready")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("canceled Call() error = %v, want context.Canceled", err)
	}
	if result.ExitCode != 0 || result.Stdout != nil || result.Stderr != nil {
		t.Errorf("canceled Call() result = %+v, want zero", result)
	}
	if got := len(fakeCalls(t, client)); got != 1 {
		t.Errorf("bd calls after waiter cancellation = %d, want 1", got)
	}

	got := <-completed
	if got.err != nil || string(got.result.Stdout) != "out" {
		t.Errorf("shared Call() = (%+v, %v), want output and nil", got.result, got.err)
	}
	if got := coalescer.InFlight(); got != 0 {
		t.Errorf("InFlight() = %d, want 0", got)
	}
}

func TestCoalescerCancelAllReleasesWaitersAndRemainsUsable(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"ready"}, Stdout: "out", DelayMS: 250})
	coalescer := NewCoalescer(client)

	const waiters = 3
	errs := make(chan error, waiters)
	for range waiters {
		go func() {
			_, err := coalescer.Call(context.Background(), "ready")
			errs <- err
		}()
	}
	waitForFakeCalls(t, client, 1)
	coalescer.CancelAll()
	for range waiters {
		if err := <-errs; !errors.Is(err, context.Canceled) {
			t.Errorf("Call() error = %v, want context.Canceled", err)
		}
	}
	if got := coalescer.InFlight(); got != 0 {
		t.Fatalf("InFlight() = %d, want 0", got)
	}

	result, err := coalescer.Call(context.Background(), "ready")
	if err != nil || string(result.Stdout) != "out" {
		t.Errorf("Call() after CancelAll = (%+v, %v), want output and nil", result, err)
	}
	if got := len(fakeCalls(t, client)); got != 2 {
		t.Errorf("bd calls = %d, want 2", got)
	}
}

func TestCoalescerDoesNotCacheCompletedCalls(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"ready"}, Stdout: "out"})
	coalescer := NewCoalescer(client)

	for range 2 {
		result, err := coalescer.Call(context.Background(), "ready")
		if err != nil || string(result.Stdout) != "out" {
			t.Fatalf("Call() = (%+v, %v), want output and nil", result, err)
		}
	}
	if got := len(fakeCalls(t, client)); got != 2 {
		t.Errorf("bd calls = %d, want 2", got)
	}
	if got := coalescer.InFlight(); got != 0 {
		t.Errorf("InFlight() = %d, want 0", got)
	}
}

func waitForFakeCalls(t *testing.T, client *Client, want int) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if got := len(fakeCalls(t, client)); got >= want {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("bd calls did not reach %d", want)
		case <-ticker.C:
		}
	}
}

func TestCoalescerClassifiesSharedResult(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"ready"}, Stderr: "Usage: bd ready", Exit: 2})
	coalescer := NewCoalescer(client)

	result, err := coalescer.Call(context.Background(), "ready")
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUsage {
		t.Errorf("Call() error = %v, want usage *Error", err)
	}
	if result.ExitCode != 2 || string(result.Stderr) != "Usage: bd ready" {
		t.Errorf("Call() result = %+v", result)
	}
}

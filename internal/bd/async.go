package bd

import (
	"bytes"
	"context"
	"strings"
	"sync"
)

// Coalescer shares identical in-flight bd calls among their waiters.
type Coalescer struct {
	client *Client

	mu      sync.Mutex
	entries map[string]*coalescedCall
}

type coalescedCall struct {
	done   chan struct{}
	cancel context.CancelFunc
	result Result
	err    error
}

// NewCoalescer creates a coalescer for c.
func NewCoalescer(c *Client) *Coalescer {
	return &Coalescer{
		client:  c,
		entries: make(map[string]*coalescedCall),
	}
}

// Call runs args once for all concurrent callers with the same root and argv.
func (q *Coalescer) Call(ctx context.Context, args ...string) (Result, error) {
	key := callKey(q.client.Root, args)

	q.mu.Lock()
	entry := q.entries[key]
	if entry == nil {
		processCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		entry = &coalescedCall{done: make(chan struct{}), cancel: cancel}
		q.entries[key] = entry
		callArgs := append([]string(nil), args...)
		go q.run(key, entry, processCtx, callArgs)
	}
	q.mu.Unlock()

	select {
	case <-entry.done:
		return cloneResult(entry.result), entry.err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

// InFlight reports the number of distinct calls currently registered.
func (q *Coalescer) InFlight() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}

// CancelAll stops every in-flight process without disabling future calls.
func (q *Coalescer) CancelAll() {
	q.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(q.entries))
	for _, entry := range q.entries {
		cancels = append(cancels, entry.cancel)
	}
	q.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (q *Coalescer) run(key string, entry *coalescedCall, ctx context.Context, args []string) {
	result, err := q.client.Run(ctx, args...)

	q.mu.Lock()
	if q.entries[key] == entry {
		delete(q.entries, key)
	}
	q.mu.Unlock()

	if err == nil {
		op, rest := "", args
		if len(args) > 0 {
			op, rest = args[0], args[1:]
		}
		err = Classify(op, rest, result)
	}
	entry.result = result
	entry.err = err
	close(entry.done)
}

func callKey(root string, args []string) string {
	parts := make([]string, 1, len(args)+1)
	parts[0] = root
	parts = append(parts, args...)
	return strings.Join(parts, "\x00")
}

func cloneResult(result Result) Result {
	result.Stdout = bytes.Clone(result.Stdout)
	result.Stderr = bytes.Clone(result.Stderr)
	return result
}

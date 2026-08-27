package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func newRuntimeAdapter(t *testing.T, name string) *fakeAdapter {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &fakeAdapter{name: name, executable: executable}
}

func TestRuntimeRoutesHandleMethodsAndForgetsDeletedHandles(t *testing.T) {
	adapter := newRuntimeAdapter(t, "test")
	adapter.run = func(context.Context, RunSpec) (Handle, error) { return "run-1", nil }
	adapter.complete = func(context.Context, Handle) (Status, error) { return StatusFailed, nil }
	adapter.diff = func(context.Context, Handle) ([]FileDiff, error) {
		return []FileDiff{{File: "a.go", Additions: 2, Deletions: 1}}, nil
	}
	adapter.output = func(context.Context, Handle) (string, error) { return "output", nil }
	adapter.limited = func(context.Context, Handle) bool { return true }

	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(registry)
	handle, err := runtime.Run(context.Background(), "test", RunSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := runtime.Complete(context.Background(), handle); err != nil || got != StatusFailed {
		t.Errorf("Complete() = (%q, %v), want (%q, nil)", got, err, StatusFailed)
	}
	if got, err := runtime.Diff(context.Background(), handle); err != nil || len(got) != 1 || got[0].File != "a.go" {
		t.Errorf("Diff() = (%v, %v), want a.go diff", got, err)
	}
	if got, err := runtime.Output(context.Background(), handle); err != nil || got != "output" {
		t.Errorf("Output() = (%q, %v), want (output, nil)", got, err)
	}
	if got, err := runtime.UsageLimited(context.Background(), handle); err != nil || !got {
		t.Errorf("UsageLimited() = (%t, %v), want (true, nil)", got, err)
	}
	if err := runtime.Delete(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Output(context.Background(), handle); !errors.Is(err, ErrUnknownHandle) {
		t.Errorf("Output(deleted) error = %v, want ErrUnknownHandle", err)
	}
}

func TestRuntimeRejectsUnavailableBackend(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(&fakeAdapter{name: "missing", executable: "magicite-definitely-missing-executable"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRuntime(registry).Run(context.Background(), "missing", RunSpec{}); !errors.Is(err, ErrExecutableMissing) {
		t.Errorf("Run() error = %v, want ErrExecutableMissing", err)
	}
}

func TestRuntimeNotifiesEachListenerOnce(t *testing.T) {
	adapter := newRuntimeAdapter(t, "test")
	var notify Notifier
	adapter.run = func(_ context.Context, spec RunSpec) (Handle, error) {
		notify = spec.Notify
		return "run-1", nil
	}
	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(registry)

	var registered, caller atomic.Int32
	runtime.OnComplete(func(Handle, Status) { registered.Add(1) })
	handle, err := runtime.Run(context.Background(), "test", RunSpec{
		Notify: func(Handle, Status) { caller.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	for range 8 {
		wait.Go(func() { notify(handle, StatusCompleted) })
	}
	wait.Wait()
	if got := registered.Load(); got != 1 {
		t.Errorf("registered notifications = %d, want 1", got)
	}
	if got := caller.Load(); got != 1 {
		t.Errorf("caller notifications = %d, want 1", got)
	}

	notify("other", StatusFailed)
	if got := registered.Load(); got != 2 {
		t.Errorf("registered notifications after other handle = %d, want 2", got)
	}
	if got := caller.Load(); got != 2 {
		t.Errorf("caller notifications after other handle = %d, want 2", got)
	}
}

func TestRuntimeConcurrentRunCompleteAndDelete(t *testing.T) {
	adapter := newRuntimeAdapter(t, "test")
	var next atomic.Int32
	adapter.run = func(context.Context, RunSpec) (Handle, error) {
		return Handle(fmt.Sprintf("run-%d", next.Add(1))), nil
	}
	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(registry)

	var wait sync.WaitGroup
	for range 32 {
		wait.Go(func() {
			handle, err := runtime.Run(context.Background(), "test", RunSpec{})
			if err != nil {
				t.Errorf("Run() error = %v", err)
				return
			}
			if _, err := runtime.Complete(context.Background(), handle); err != nil {
				t.Errorf("Complete() error = %v", err)
			}
			if err := runtime.Delete(context.Background(), handle); err != nil {
				t.Errorf("Delete() error = %v", err)
			}
		})
	}
	wait.Wait()
}

func TestRuntimeUnknownHandle(t *testing.T) {
	runtime := NewRuntime(NewRegistry())
	ctx := context.Background()
	if _, err := runtime.Complete(ctx, "missing"); !errors.Is(err, ErrUnknownHandle) {
		t.Errorf("Complete() error = %v, want ErrUnknownHandle", err)
	}
	if _, err := runtime.Diff(ctx, "missing"); !errors.Is(err, ErrUnknownHandle) {
		t.Errorf("Diff() error = %v, want ErrUnknownHandle", err)
	}
	if _, err := runtime.Output(ctx, "missing"); !errors.Is(err, ErrUnknownHandle) {
		t.Errorf("Output() error = %v, want ErrUnknownHandle", err)
	}
	if err := runtime.Delete(ctx, "missing"); !errors.Is(err, ErrUnknownHandle) {
		t.Errorf("Delete() error = %v, want ErrUnknownHandle", err)
	}
	if _, err := runtime.UsageLimited(ctx, "missing"); !errors.Is(err, ErrUnknownHandle) {
		t.Errorf("UsageLimited() error = %v, want ErrUnknownHandle", err)
	}
}

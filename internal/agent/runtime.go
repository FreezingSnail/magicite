package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/connorfranc/magicite/internal/logging"
)

// Runtime resolves adapters and owns their run handles.
type Runtime struct {
	registry *Registry

	mu       sync.RWMutex
	handles  map[Handle]Adapter
	complete Notifier
}

// NewRuntime creates a runtime backed by registry.
func NewRuntime(registry *Registry) *Runtime {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Runtime{
		registry: registry,
		handles:  make(map[Handle]Adapter),
	}
}

// Run starts spec through backend and records ownership of its handle.
func (r *Runtime) Run(ctx context.Context, backend string, spec RunSpec) (Handle, error) {
	if err := r.registry.Available(backend); err != nil {
		return "", err
	}
	adapter, err := r.registry.Lookup(backend)
	if err != nil {
		return "", err
	}

	spec.Notify = r.notifier(spec.Notify)
	handle, err := adapter.Run(ctx, spec)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	r.handles[handle] = adapter
	r.mu.Unlock()
	return handle, nil
}

// Complete returns the status supplied by the handle's adapter.
func (r *Runtime) Complete(ctx context.Context, handle Handle) (Status, error) {
	adapter, err := r.adapter(handle)
	if err != nil {
		return "", err
	}
	return adapter.Complete(ctx, handle)
}

// Diff returns the handle's changed files.
func (r *Runtime) Diff(ctx context.Context, handle Handle) ([]FileDiff, error) {
	adapter, err := r.adapter(handle)
	if err != nil {
		return nil, err
	}
	return adapter.Diff(ctx, handle)
}

// Output returns the handle's captured output.
func (r *Runtime) Output(ctx context.Context, handle Handle) (string, error) {
	adapter, err := r.adapter(handle)
	if err != nil {
		return "", err
	}
	return adapter.Output(ctx, handle)
}

// Delete removes the adapter's run and forgets ownership on success.
func (r *Runtime) Delete(ctx context.Context, handle Handle) error {
	adapter, err := r.adapter(handle)
	if err != nil {
		return err
	}
	if err := adapter.Delete(ctx, handle); err != nil {
		return err
	}

	r.mu.Lock()
	delete(r.handles, handle)
	r.mu.Unlock()
	return nil
}

// UsageLimited reports whether the handle's adapter has reached its usage limit.
func (r *Runtime) UsageLimited(ctx context.Context, handle Handle) (bool, error) {
	adapter, err := r.adapter(handle)
	if err != nil {
		return false, err
	}
	return adapter.UsageLimited(ctx, handle), nil
}

// OnComplete installs the listener called once for every adapter notification.
func (r *Runtime) OnComplete(notifier Notifier) {
	r.mu.Lock()
	r.complete = notifier
	r.mu.Unlock()
}

func (r *Runtime) adapter(handle Handle) (Adapter, error) {
	r.mu.RLock()
	adapter, ok := r.handles[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownHandle, handle)
	}
	return adapter, nil
}

func (r *Runtime) notifier(caller Notifier) Notifier {
	var mu sync.Mutex
	notified := make(map[Handle]struct{})
	return func(handle Handle, status Status) {
		mu.Lock()
		if _, ok := notified[handle]; ok {
			mu.Unlock()
			return
		}
		notified[handle] = struct{}{}
		mu.Unlock()

		r.mu.RLock()
		complete := r.complete
		r.mu.RUnlock()

		logging.Event(logging.Info, logging.KindComplete, map[string]any{
			"handle": handle,
			"status": status,
		})
		if complete != nil {
			complete(handle, status)
		}
		if caller != nil {
			caller(handle, status)
		}
	}
}

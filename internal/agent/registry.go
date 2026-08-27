package agent

import (
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"sync"
)

// Registry stores validated adapters by backend name.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

// Register validates and stores adapter. Backend names are unique.
func (r *Registry) Register(adapter Adapter) error {
	if nilAdapter(adapter) || adapter.Name() == "" || adapter.Executable() == "" {
		return ErrInvalidAdapter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[adapter.Name()]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateBackend, adapter.Name())
	}
	r.adapters[adapter.Name()] = adapter
	return nil
}

// Lookup returns the adapter registered under name. An empty name uses the
// default backend.
func (r *Registry) Lookup(name string) (Adapter, error) {
	if name == "" {
		name = DefaultBackend
	}

	r.mu.RLock()
	adapter, ok := r.adapters[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownBackend, name)
	}
	return adapter, nil
}

// Available reports whether the named adapter's executable is discoverable.
func (r *Registry) Available(name string) error {
	adapter, err := r.Lookup(name)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(adapter.Executable()); err != nil {
		return fmt.Errorf("%w: %s", ErrExecutableMissing, adapter.Executable())
	}
	return nil
}

// Names returns registered backend names in lexical order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	r.mu.RUnlock()
	sort.Strings(names)
	return names
}

func nilAdapter(adapter Adapter) bool {
	if adapter == nil {
		return true
	}
	value := reflect.ValueOf(adapter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

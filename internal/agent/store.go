package agent

import (
	"strconv"
	"sync"
)

// Store keeps per-session state behind opaque, process-unique handles.
type Store[T any] struct {
	mu          sync.Mutex
	prefix      string
	next        uint64
	initialized bool
	states      map[Handle]T
	order       []Handle
	aliases     map[string]Handle
}

// NewStore creates an empty handle store using prefix for allocated handles.
func NewStore[T any](prefix string) *Store[T] {
	return &Store[T]{
		prefix:      prefix,
		next:        1,
		initialized: true,
		states:      make(map[Handle]T),
		aliases:     make(map[string]Handle),
	}
}

// Add stores state and returns a new handle and an independent copy of state.
func (s *Store[T]) Add(state T) (Handle, *T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.init()
	var handle Handle
	for {
		if s.next == 0 {
			panic("agent: handle counter exhausted")
		}
		counter := s.next
		s.next++
		handle = Handle(s.prefix + "-" + strconv.FormatUint(counter, 10))
		if _, exists := s.states[handle]; exists {
			continue
		}
		if _, exists := s.aliases[string(handle)]; exists {
			continue
		}
		break
	}

	stored := state
	s.states[handle] = stored
	s.order = append(s.order, handle)
	returned := stored
	return handle, &returned
}

// Get returns a copy of the state stored under handle.
func (s *Store[T]) Get(handle Handle) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[handle]
	return state, ok
}

// Update applies fn to the state stored under handle while holding the lock.
func (s *Store[T]) Update(handle Handle, fn func(*T)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[handle]
	if !ok {
		return false
	}
	fn(&state)
	s.states[handle] = state
	return true
}

// Delete removes handle and its aliases, returning the last stored state.
func (s *Store[T]) Delete(handle Handle) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[handle]
	if !ok {
		var zero T
		return zero, false
	}
	delete(s.states, handle)
	for alias, target := range s.aliases {
		if target == handle {
			delete(s.aliases, alias)
		}
	}
	for i, existing := range s.order {
		if existing == handle {
			copy(s.order[i:], s.order[i+1:])
			s.order = s.order[:len(s.order)-1]
			break
		}
	}
	return state, true
}

// Handles returns active handles in allocation order.
func (s *Store[T]) Handles() []Handle {
	s.mu.Lock()
	defer s.mu.Unlock()

	handles := make([]Handle, len(s.order))
	copy(handles, s.order)
	return handles
}

// Alias binds alias to an existing handle.
func (s *Store[T]) Alias(alias string, handle Handle) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.states[handle]; !ok {
		return false
	}
	if target, ok := s.aliases[alias]; ok {
		return target == handle
	}
	if _, ok := s.states[Handle(alias)]; ok && Handle(alias) != handle {
		return false
	}
	s.aliases[alias] = handle
	return true
}

// Resolve maps a handle or alias to an active handle.
func (s *Store[T]) Resolve(key string) (Handle, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	handle := Handle(key)
	if _, ok := s.states[handle]; ok {
		return handle, true
	}
	handle, ok := s.aliases[key]
	return handle, ok
}

func (s *Store[T]) init() {
	if !s.initialized {
		s.next = 1
		s.initialized = true
	}
	if s.states == nil {
		s.states = make(map[Handle]T)
	}
	if s.aliases == nil {
		s.aliases = make(map[string]Handle)
	}
}

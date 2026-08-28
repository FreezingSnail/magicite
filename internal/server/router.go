package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/wire"
)

// Handler answers one command using its raw JSON parameters.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// RegistrationError reports an invalid Router registration.
type RegistrationError struct {
	Name   string
	Reason string
}

func (e *RegistrationError) Error() string {
	switch e.Reason {
	case "empty name":
		return "router command name is empty"
	case "nil handler":
		return fmt.Sprintf("router handler for %q is nil", e.Name)
	case "duplicate name":
		return fmt.Sprintf("router command %q is already registered", e.Name)
	default:
		return "router registration failed"
	}
}

// Router dispatches decoded wire requests to registered command handlers.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	log      logging.Logger
}

// NewRouter creates an empty Router using log for handler failures.
func NewRouter(log logging.Logger) *Router {
	return &Router{handlers: make(map[string]Handler), log: log}
}

// Register associates name with h. Names may be registered once only.
func (r *Router) Register(name string, h Handler) error {
	if name == "" {
		return &RegistrationError{Reason: "empty name"}
	}
	if h == nil {
		return &RegistrationError{Name: name, Reason: "nil handler"}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.handlers[name]; ok {
		return &RegistrationError{Name: name, Reason: "duplicate name"}
	}
	r.handlers[name] = h
	return nil
}

// Commands returns registered command names in lexical order.
func (r *Router) Commands() []string {
	r.mu.RLock()
	commands := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		commands = append(commands, name)
	}
	r.mu.RUnlock()
	sort.Strings(commands)
	return commands
}

// Handle validates and dispatches req, always returning a wire response.
func (r *Router) Handle(ctx context.Context, req wire.Request) wire.Response {
	response := wire.Response{Schema: wire.Schema, ID: req.ID}
	if req.Schema != wire.Schema {
		response.Err = &wire.Error{Code: wire.CodeSchemaMismatch, Message: fmt.Sprintf("schema %d is unsupported", req.Schema)}
		return response
	}
	if req.Command == "" {
		response.Err = &wire.Error{Code: wire.CodeBadRequest, Message: "command is required"}
		return response
	}

	r.mu.RLock()
	handler, ok := r.handlers[req.Command]
	r.mu.RUnlock()
	if !ok {
		response.Err = &wire.Error{Code: wire.CodeUnknownCommand, Message: fmt.Sprintf("unknown command %q", req.Command)}
		return response
	}

	result, err, panicked := callHandler(ctx, handler, req.Params)
	if panicked {
		r.log.Event(logging.Error, logging.KindError, map[string]any{
			"command": req.Command,
			"error":   err.Error(),
		})
		response.Err = &wire.Error{Code: wire.CodeInternal, Message: "handler panic"}
		return response
	}
	if err != nil {
		response.Err = Classify(err)
		return response
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		response.Err = &wire.Error{Code: wire.CodeInternal, Message: err.Error()}
		return response
	}
	response.Result = encoded
	return response
}

func callHandler(ctx context.Context, handler Handler, params json.RawMessage) (result any, err error, panicked bool) {
	completed := false
	defer func() {
		if !completed {
			panicked = true
			err = fmt.Errorf("handler panic: %v", recover())
		}
	}()
	result, err = handler(ctx, params)
	completed = true
	return result, err, false
}

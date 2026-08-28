package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// ReadRegistrationError reports an invalid read handler registration.
type ReadRegistrationError struct {
	Reason string
}

func (e *ReadRegistrationError) Error() string {
	switch e.Reason {
	case "nil router":
		return "read router is nil"
	case "nil core":
		return "read core is nil"
	default:
		return "read registration failed"
	}
}

// RegisterRead adds status, seats, tasks, and repos handlers to r using c.
func RegisterRead(r *Router, c Core) error {
	if r == nil {
		return &ReadRegistrationError{Reason: "nil router"}
	}
	if nilCore(c) {
		return &ReadRegistrationError{Reason: "nil core"}
	}

	for _, command := range []struct {
		name string
		h    Handler
	}{
		{name: "status", h: statusRead(c)},
		{name: "seats", h: seatsRead(c)},
		{name: "tasks", h: tasksRead(c)},
		{name: "repos", h: reposRead(c)},
	} {
		if err := r.Register(command.name, command.h); err != nil {
			return err
		}
	}
	return nil
}

func statusRead(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := decodeNoReadParams("status", params); err != nil {
			return nil, err
		}
		result, err := c.Status(ctx)
		if err != nil {
			return nil, err
		}
		result.Sessions = nonNil(result.Sessions)
		sort.Slice(result.Sessions, func(i, j int) bool {
			left, right := result.Sessions[i], result.Sessions[j]
			if left.UptimeSeconds != right.UptimeSeconds {
				return left.UptimeSeconds > right.UptimeSeconds
			}
			return left.Handle < right.Handle
		})
		return result, nil
	}
}

func seatsRead(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := decodeNoReadParams("seats", params); err != nil {
			return nil, err
		}
		result, err := c.Seats(ctx)
		if err != nil {
			return nil, err
		}
		result = nonNil(result)
		sort.Slice(result, func(i, j int) bool {
			if result[i].Role != result[j].Role {
				return result[i].Role < result[j].Role
			}
			return result[i].Name < result[j].Name
		})
		return result, nil
	}
}

func tasksRead(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p wire.TasksParams
		if err := decodeControlParams(params, &p); err != nil {
			return nil, err
		}
		result, err := c.Tasks(ctx, p)
		if err != nil {
			return nil, err
		}
		result = nonNil(result)
		sort.Slice(result, func(i, j int) bool {
			left, right := result[i], result[j]
			if left.Repo != right.Repo {
				return left.Repo < right.Repo
			}
			if left.Priority != right.Priority {
				return left.Priority < right.Priority
			}
			return left.ID < right.ID
		})
		return result, nil
	}
}

func reposRead(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := decodeNoReadParams("repos", params); err != nil {
			return nil, err
		}
		result, err := c.Repos(ctx)
		if err != nil {
			return nil, err
		}
		result = nonNil(result)
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
		return result, nil
	}
}

func decodeNoReadParams(command string, params json.RawMessage) error {
	if len(params) == 0 || bytes.Equal(bytes.TrimSpace(params), []byte("null")) || bytes.Equal(bytes.TrimSpace(params), []byte("{}")) {
		return nil
	}
	return fmt.Errorf("%w: %s does not accept parameters", ErrBadRequest, command)
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

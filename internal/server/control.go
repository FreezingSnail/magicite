package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// ControlRegistrationError reports an invalid control handler registration.
type ControlRegistrationError struct {
	Reason string
}

func (e *ControlRegistrationError) Error() string {
	switch e.Reason {
	case "nil router":
		return "control router is nil"
	case "nil core":
		return "control core is nil"
	default:
		return "control registration failed"
	}
}

// RegisterControl adds lifecycle command handlers to r using c.
func RegisterControl(r *Router, c Core) error {
	if r == nil {
		return &ControlRegistrationError{Reason: "nil router"}
	}
	if nilCore(c) {
		return &ControlRegistrationError{Reason: "nil core"}
	}

	for _, command := range []struct {
		name string
		h    Handler
	}{
		{name: "start", h: startControl(c)},
		{name: "stop", h: stopControl(c)},
		{name: "dispatch", h: dispatchControl(c)},
		{name: "review", h: reviewControl(c)},
	} {
		if err := r.Register(command.name, command.h); err != nil {
			return err
		}
	}
	return nil
}

func nilCore(c Core) bool {
	if c == nil {
		return true
	}
	value := reflect.ValueOf(c)
	return value.Kind() == reflect.Ptr && value.IsNil()
}

func startControl(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct{}
		if err := decodeControlParams(params, &p); err != nil {
			return nil, err
		}
		return c.Start(ctx)
	}
}

func stopControl(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p wire.StopParams
		if err := decodeControlParams(params, &p); err != nil {
			return nil, err
		}
		return c.Stop(ctx, p)
	}
}

func dispatchControl(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p wire.DispatchParams
		if err := decodeControlParams(params, &p); err != nil {
			return nil, err
		}
		if p.Task == "" {
			return nil, fmt.Errorf("%w: task is required", ErrBadRequest)
		}
		switch p.Role {
		case "implement", "design", "repair", "review":
		default:
			return nil, fmt.Errorf("%w: role must be one of implement, design, repair, review", ErrBadRequest)
		}
		return c.Dispatch(ctx, p)
	}
}

func reviewControl(c Core) Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p wire.ReviewParams
		if err := decodeControlParams(params, &p); err != nil {
			return nil, err
		}
		if p.Epic == "" {
			return nil, fmt.Errorf("%w: epic is required", ErrBadRequest)
		}
		return c.Review(ctx, p)
	}
}

func decodeControlParams(params json.RawMessage, target any) error {
	if len(params) == 0 {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(params))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: multiple parameter values", ErrBadRequest)
		}
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	return nil
}

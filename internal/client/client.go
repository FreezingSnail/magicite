// Package client provides the CLI transport for magicite's daemon socket.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync/atomic"
	"time"

	"github.com/FreezingSnail/magicite/internal/wire"
)

const defaultTimeout = 10 * time.Second

var requestSequence atomic.Uint64

// Options configures a daemon socket client.
type Options struct {
	Socket  string
	Timeout time.Duration
}

// Client sends independent requests to one daemon socket.
type Client struct {
	socket  string
	timeout time.Duration
}

// New creates a client. An empty socket remains empty so failures name the
// configuration that supplied it.
func New(options Options) *Client {
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	return &Client{socket: options.Socket, timeout: options.Timeout}
}

// Error is a daemon or transport failure with a command exit status.
type Error struct {
	Code    wire.Code
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ExitStatus returns the status associated with e.
func (e *Error) ExitStatus() int {
	if e == nil {
		return 0
	}
	return e.Code.ExitStatus()
}

// ExitStatus converts a returned error into a command exit status.
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}
	var clientErr *Error
	if errors.As(err, &clientErr) {
		return clientErr.ExitStatus()
	}
	return 1
}

// Call sends one command request and decodes its response. Each call owns and
// closes its connection.
func (c *Client) Call(ctx context.Context, command string, params, out any) error {
	ctx = nonNilContext(ctx)
	paramsJSON, err := marshalParams(params)
	if err != nil {
		return internal(err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	conn, err := dial(requestCtx, c.socket)
	if err != nil {
		return callTransportError(ctx, requestCtx, c.socket, err)
	}
	defer conn.Close()

	if deadline, ok := requestCtx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return unavailable(c.socket, err)
		}
	}

	request := wire.Request{
		Schema:  wire.Schema,
		ID:      nextID(),
		Command: command,
		Params:  paramsJSON,
	}
	if err := wire.NewEncoder(conn).Encode(request); err != nil {
		return callTransportError(ctx, requestCtx, c.socket, err)
	}

	frame, err := wire.NewDecoder(conn).Frame()
	if err != nil {
		return callFrameError(ctx, requestCtx, c.socket, err)
	}
	if frame.Response == nil {
		return internal(errors.New("wire response frame is not a response"))
	}
	response := frame.Response
	if response.ID != request.ID {
		return internal(fmt.Errorf("wire response id %q does not match request id %q", response.ID, request.ID))
	}
	if response.Err != nil {
		return &Error{Code: response.Err.Code, Message: response.Err.Message}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(response.Result, out); err != nil {
		return internal(err)
	}
	return nil
}

// Stream subscribes to events after since and delivers each event to fn.
func (c *Client) Stream(ctx context.Context, since uint64, fn func(wire.Event) error, follow ...bool) (uint64, error) {
	ctx = nonNilContext(ctx)
	dialCtx, cancel := context.WithTimeout(ctx, c.timeout)
	conn, err := dial(dialCtx, c.socket)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, unavailable(c.socket, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return 0, unavailable(c.socket, err)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	following := true
	if len(follow) > 0 {
		following = follow[0]
	}
	var requestParams any
	switch {
	case since == math.MaxUint64 && following:
		// No params selects the daemon's current live position.
	case since == math.MaxUint64:
		requestParams = struct {
			Follow *bool `json:"follow"`
		}{Follow: &following}
	default:
		var finite *bool
		if !following {
			finite = &following
		}
		requestParams = wire.SubscribeParams{Since: since, Follow: finite}
	}
	params, err := marshalParams(requestParams)
	if err != nil {
		return 0, internal(err)
	}
	if err := wire.NewEncoder(conn).Encode(wire.Request{
		Schema:  wire.Schema,
		ID:      nextID(),
		Command: "subscribe",
		Params:  params,
	}); err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, unavailable(c.socket, err)
	}

	decoder := wire.NewDecoder(conn)
	var highest uint64
	for {
		frame, err := decoder.Frame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if ctx.Err() != nil {
					return highest, ctx.Err()
				}
				return highest, nil
			}
			if ctx.Err() != nil {
				return highest, ctx.Err()
			}
			if errors.Is(err, wire.ErrSchema) {
				return highest, schemaMismatch(err)
			}
			return highest, unavailable(c.socket, err)
		}
		if frame.Event == nil {
			return highest, unavailable(c.socket, errors.New("wire stream frame is not an event"))
		}
		event := *frame.Event
		if event.Seq <= highest {
			return highest, internal(fmt.Errorf("wire event sequence %d follows %d", event.Seq, highest))
		}
		highest = event.Seq
		if fn != nil {
			if err := fn(event); err != nil {
				return highest, err
			}
		}
	}
}

func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func nextID() string {
	return fmt.Sprintf("request-%d-%d", time.Now().UnixNano(), requestSequence.Add(1))
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func dial(ctx context.Context, socket string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", socket)
}

func callTransportError(parent, request context.Context, socket string, err error) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if request.Err() != nil && errors.Is(request.Err(), context.Canceled) {
		return request.Err()
	}
	return unavailable(socket, err)
}

func callFrameError(parent, request context.Context, socket string, err error) error {
	if errors.Is(err, wire.ErrSchema) {
		return schemaMismatch(err)
	}
	if errors.Is(err, io.EOF) {
		return unavailable(socket, err)
	}
	if parent.Err() != nil {
		return parent.Err()
	}
	if request.Err() != nil {
		return unavailable(socket, request.Err())
	}
	return internal(err)
}

func unavailable(socket string, err error) *Error {
	return &Error{Code: wire.CodeUnavailable, Message: fmt.Sprintf("daemon socket %q: %v", socket, err)}
}

func schemaMismatch(err error) *Error {
	return &Error{Code: wire.CodeSchemaMismatch, Message: err.Error()}
}

func internal(err error) *Error {
	return &Error{Code: wire.CodeInternal, Message: err.Error()}
}

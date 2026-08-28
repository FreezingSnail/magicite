// Package server owns magicite's local daemon protocol.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const shutdownGrace = 250 * time.Millisecond

// Deps supplies the protocol server dependencies.
type Deps struct {
	Router *Router
	Bus    *Bus
	Log    logging.Logger
	Socket string
}

// DepsError identifies a missing server dependency.
type DepsError struct{ Field string }

func (e *DepsError) Error() string { return fmt.Sprintf("server: %s is required", e.Field) }

// Serve accepts wire requests until ctx is cancelled.
func Serve(ctx context.Context, d Deps) error {
	if d.Router == nil {
		return &DepsError{Field: "Router"}
	}
	if d.Bus == nil {
		return &DepsError{Field: "Bus"}
	}
	if d.Socket == "" {
		return &DepsError{Field: "Socket"}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	listener, err := Listen(d.Socket)
	if err != nil {
		return err
	}
	daemon := &daemon{ctx: ctx, deps: d, listener: listener, connections: make(map[net.Conn]struct{}), subscriptions: make(map[*Subscription]struct{})}
	d.Bus.Publish(wire.Event{Kind: wire.KindWarn, Level: "info", Fields: map[string]string{"event": "serve", "socket": d.Socket}})

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				daemon.shutdown()
				_ = os.Remove(d.Socket)
				return nil
			}
			_ = listener.Close()
			_ = os.Remove(d.Socket)
			return fmt.Errorf("server: accept: %w", err)
		}
		daemon.add(conn)
		daemon.group.Add(1)
		go func() {
			defer daemon.group.Done()
			daemon.handle(conn)
		}()
	}
}

type daemon struct {
	ctx      context.Context
	deps     Deps
	listener net.Listener

	mu            sync.Mutex
	connections   map[net.Conn]struct{}
	subscriptions map[*Subscription]struct{}
	group         sync.WaitGroup
}

func (d *daemon) add(conn net.Conn) {
	d.mu.Lock()
	d.connections[conn] = struct{}{}
	d.mu.Unlock()
}

func (d *daemon) remove(conn net.Conn) {
	d.mu.Lock()
	delete(d.connections, conn)
	d.mu.Unlock()
	_ = conn.Close()
}

func (d *daemon) handle(conn net.Conn) {
	defer d.remove(conn)
	decoder := wire.NewDecoder(conn)
	encoder := wire.NewEncoder(conn)
	for {
		request, err := decoder.Request()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, wire.ErrSchema) {
				_ = encoder.Encode(errorResponse("", wire.CodeSchemaMismatch, err.Error()))
				return
			}
			if encoder.Encode(errorResponse("", wire.CodeBadRequest, err.Error())) != nil {
				return
			}
			continue
		}
		if request.Command == "subscribe" {
			if d.stream(conn, decoder, encoder, request) {
				return
			}
			continue
		}
		if encoder.Encode(d.deps.Router.Handle(d.ctx, request)) != nil {
			return
		}
	}
}

// stream owns all writes while a connection has entered subscribe mode. It
// keeps reading requests so conflicts do not interrupt event delivery.
func (d *daemon) stream(conn net.Conn, decoder *wire.Decoder, encoder *wire.Encoder, request wire.Request) bool {
	var params wire.SubscribeParams
	if len(request.Params) != 0 {
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return encoder.Encode(errorResponse(request.ID, wire.CodeBadRequest, err.Error())) != nil
		}
	}
	subscription := d.deps.Bus.Subscribe(params.Since, 64)
	d.track(subscription)
	defer d.untrack(subscription)
	defer subscription.Close()

	requests := make(chan decodedRequest, 1)
	stopped := make(chan struct{})
	defer close(stopped)
	go readRequests(decoder, requests, stopped)
	for {
		select {
		case event, ok := <-subscription.Events():
			if !ok {
				return true
			}
			if encoder.Encode(event) != nil {
				return true
			}
		case next, ok := <-requests:
			if !ok {
				return true
			}
			if next.err != nil {
				if errors.Is(next.err, io.EOF) {
					return true
				}
				code := wire.CodeBadRequest
				if errors.Is(next.err, wire.ErrSchema) {
					code = wire.CodeSchemaMismatch
				}
				if encoder.Encode(errorResponse("", code, next.err.Error())) != nil || code == wire.CodeSchemaMismatch {
					return true
				}
				continue
			}
			if encoder.Encode(errorResponse(next.request.ID, wire.CodeConflict, "connection is streaming")) != nil {
				return true
			}
		case <-d.ctx.Done():
			return true
		}
	}
}

type decodedRequest struct {
	request wire.Request
	err     error
}

func readRequests(decoder *wire.Decoder, requests chan<- decodedRequest, stopped <-chan struct{}) {
	defer close(requests)
	for {
		request, err := decoder.Request()
		select {
		case requests <- decodedRequest{request: request, err: err}:
		case <-stopped:
			return
		}
		if err != nil {
			return
		}
	}
}

func errorResponse(id string, code wire.Code, message string) wire.Response {
	return wire.Response{Schema: wire.Schema, ID: id, Err: &wire.Error{Code: code, Message: message}}
}

func (d *daemon) track(subscription *Subscription) {
	d.mu.Lock()
	d.subscriptions[subscription] = struct{}{}
	d.mu.Unlock()
}

func (d *daemon) untrack(subscription *Subscription) {
	d.mu.Lock()
	delete(d.subscriptions, subscription)
	d.mu.Unlock()
}

func (d *daemon) shutdown() {
	d.mu.Lock()
	for subscription := range d.subscriptions {
		subscription.Close()
	}
	d.mu.Unlock()

	done := make(chan struct{})
	go func() { d.group.Wait(); close(done) }()
	select {
	case <-done:
		return
	case <-time.After(shutdownGrace):
	}

	d.mu.Lock()
	for conn := range d.connections {
		_ = conn.Close()
	}
	d.mu.Unlock()
	<-done
}

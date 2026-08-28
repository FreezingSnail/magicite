package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/wire"
)

var testSocketSequence atomic.Uint64

func testSocket(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".m-%d-%d.sock", os.Getpid(), testSocketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func serve(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	socket := testSocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				handler(conn)
			}()
		}
	}()
	return socket
}

func receiveRequest(conn net.Conn) (wire.Request, error) {
	return wire.NewDecoder(conn).Request()
}

func TestCallSendsRequestDecodesResultAndCloses(t *testing.T) {
	requests := make(chan wire.Request, 2)
	done := make(chan struct{}, 2)
	socket := serve(t, func(conn net.Conn) {
		defer func() { done <- struct{}{} }()
		request, err := receiveRequest(conn)
		if err != nil {
			return
		}
		requests <- request
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{"ok":true}`)})
	})

	client := New(Options{Socket: socket})
	var first struct {
		OK bool `json:"ok"`
	}
	if err := client.Call(context.Background(), "status", map[string]bool{"verbose": true}, &first); err != nil {
		t.Fatal(err)
	}
	if !first.OK {
		t.Fatal("result was not unmarshalled")
	}
	if err := client.Call(context.Background(), "status", nil, nil); err != nil {
		t.Fatal(err)
	}

	var got [2]wire.Request
	for i := range got {
		select {
		case got[i] = <-requests:
		case <-time.After(time.Second):
			t.Fatal("server did not receive request")
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("client did not close connection")
		}
	}
	if got[0].Schema != wire.Schema || got[0].Command != "status" || string(got[0].Params) != `{"verbose":true}` {
		t.Fatalf("first request = %#v", got[0])
	}
	if got[1].ID == got[0].ID || got[1].Params != nil {
		t.Fatalf("request ids or params = %#v", got)
	}
}

func TestCallClassifiesProtocolAndTransportFailures(t *testing.T) {
	tests := []struct {
		name    string
		respond func(wire.Request, net.Conn)
		code    wire.Code
	}{
		{
			name: "daemon error",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Err: &wire.Error{Code: wire.CodeNotFound, Message: "missing"}})
			},
			code: wire.CodeNotFound,
		},
		{
			name: "mismatched id",
			respond: func(_ wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: "other"})
			},
			code: wire.CodeInternal,
		},
		{
			name: "schema mismatch",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema + 1, ID: request.ID})
			},
			code: wire.CodeSchemaMismatch,
		},
		{
			name: "bad result",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{"ok":"not-bool"}`)})
			},
			code: wire.CodeInternal,
		},
		{
			name:    "closed before frame",
			respond: func(wire.Request, net.Conn) {},
			code:    wire.CodeUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			socket := serve(t, func(conn net.Conn) {
				request, err := receiveRequest(conn)
				if err == nil {
					test.respond(request, conn)
				}
			})
			var out struct {
				OK bool `json:"ok"`
			}
			err := New(Options{Socket: socket}).Call(context.Background(), "status", nil, &out)
			var clientErr *Error
			if !errors.As(err, &clientErr) || clientErr.Code != test.code {
				t.Fatalf("Call() error = %#v, want code %q", err, test.code)
			}
			if test.code == wire.CodeSchemaMismatch && ExitStatus(err) != 6 {
				t.Fatalf("ExitStatus() = %d, want 6", ExitStatus(err))
			}
		})
	}
}

func TestCallUnavailableNamesSocketAndHonorsTimeout(t *testing.T) {
	missing := filepath.Join(filepath.Dir(testSocket(t)), "missing.sock")
	err := New(Options{Socket: missing}).Call(context.Background(), "status", nil, nil)
	var clientErr *Error
	if !errors.As(err, &clientErr) || clientErr.Code != wire.CodeUnavailable || !strings.Contains(clientErr.Message, missing) || ExitStatus(err) != 3 {
		t.Fatalf("missing socket error = %#v", err)
	}

	err = New(Options{}).Call(context.Background(), "status", nil, nil)
	if !errors.As(err, &clientErr) || clientErr.Code != wire.CodeUnavailable || !strings.Contains(clientErr.Message, `""`) {
		t.Fatalf("empty socket error = %#v", err)
	}

	socket := serve(t, func(conn net.Conn) { _, _ = receiveRequest(conn) })
	started := time.Now()
	err = New(Options{Socket: socket, Timeout: 25 * time.Millisecond}).Call(context.Background(), "status", nil, nil)
	if !errors.As(err, &clientErr) || clientErr.Code != wire.CodeUnavailable || time.Since(started) > time.Second {
		t.Fatalf("timed call error = %#v after %s", err, time.Since(started))
	}
}

func TestStreamSubscribesDeliversEventsAndReturnsHighestSequence(t *testing.T) {
	request := make(chan wire.Request, 1)
	socket := serve(t, func(conn net.Conn) {
		got, err := receiveRequest(conn)
		if err != nil {
			return
		}
		request <- got
		encoder := wire.NewEncoder(conn)
		_ = encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 4, Kind: wire.KindPickup})
		_ = encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 9, Kind: wire.KindComplete})
	})

	var events []wire.Event
	highest, err := New(Options{Socket: socket}).Stream(context.Background(), 3, func(event wire.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil || highest != 9 || len(events) != 2 || events[0].Seq != 4 || events[1].Seq != 9 {
		t.Fatalf("Stream() = (%d, %v), events = %#v", highest, err, events)
	}
	got := <-request
	var params wire.SubscribeParams
	if got.Schema != wire.Schema || got.Command != "subscribe" || json.Unmarshal(got.Params, &params) != nil || params.Since != 3 {
		t.Fatalf("subscribe request = %#v", got)
	}
}

func TestStreamReturnsCallbackErrorAndCancellationWithHighestSequence(t *testing.T) {
	callbackErr := errors.New("stop")
	socket := serve(t, func(conn net.Conn) {
		if _, err := receiveRequest(conn); err != nil {
			return
		}
		_ = wire.NewEncoder(conn).Encode(wire.Event{Schema: wire.Schema, Seq: 7, Kind: wire.KindWarn})
	})
	highest, err := New(Options{Socket: socket}).Stream(context.Background(), 0, func(wire.Event) error { return callbackErr })
	if highest != 7 || !errors.Is(err, callbackErr) {
		t.Fatalf("callback Stream() = (%d, %v)", highest, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	socket = serve(t, func(conn net.Conn) {
		if _, err := receiveRequest(conn); err != nil {
			return
		}
		encoder := wire.NewEncoder(conn)
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 12, Kind: wire.KindLand}); err != nil {
			return
		}
		<-time.After(time.Second)
	})
	highest, err = New(Options{Socket: socket}).Stream(ctx, 0, func(wire.Event) error {
		cancel()
		return nil
	})
	if highest != 12 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Stream() = (%d, %v)", highest, err)
	}
}

func TestExitStatus(t *testing.T) {
	if got := ExitStatus(nil); got != 0 {
		t.Fatalf("ExitStatus(nil) = %d", got)
	}
	if got := ExitStatus(errors.New("other")); got != 1 {
		t.Fatalf("ExitStatus(other) = %d", got)
	}
	if got := ExitStatus(fmt.Errorf("wrapped: %w", &Error{Code: wire.CodeConflict})); got != 5 {
		t.Fatalf("ExitStatus(wrapped) = %d", got)
	}
}

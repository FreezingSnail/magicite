package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func startProtocolServer(t *testing.T, router *Router, bus *Bus) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	directory := fmt.Sprintf(".magicite-server-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	path := filepath.Join(directory, "magicite.sock")
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Deps{Router: router, Bus: bus, Log: *logging.New(logging.Config{}), Socket: path})
	}()
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return path, cancel, done
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("server did not start")
		}
	}
}

func stopProtocolServer(t *testing.T, path string, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains: %v", err)
	}
}

func TestServeRejectsMissingDependencies(t *testing.T) {
	for _, test := range []struct {
		deps  Deps
		field string
	}{{field: "Router"}, {deps: Deps{Router: NewRouter(logging.Logger{})}, field: "Bus"}, {deps: Deps{Router: NewRouter(logging.Logger{}), Bus: NewBus(1)}, field: "Socket"}} {
		err := Serve(context.Background(), test.deps)
		var dependency *DepsError
		if !errors.As(err, &dependency) || dependency.Field != test.field {
			t.Errorf("Serve(%+v) error = %v, want field %q", test.deps, err, test.field)
		}
	}
}

func TestServeRoutesOrderedRequests(t *testing.T) {
	router := NewRouter(logging.Logger{})
	if err := router.Register("echo", func(_ context.Context, params json.RawMessage) (any, error) { return string(params), nil }); err != nil {
		t.Fatal(err)
	}
	path, cancel, done := startProtocolServer(t, router, NewBus(8))
	defer stopProtocolServer(t, path, cancel, done)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	encoder, decoder := wire.NewEncoder(conn), wire.NewDecoder(conn)
	for _, id := range []string{"one", "two"} {
		if err := encoder.Encode(wire.Request{Schema: wire.Schema, ID: id, Command: "echo", Params: json.RawMessage(`"ok"`)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"one", "two"} {
		frame, err := decoder.Frame()
		if err != nil || frame.Response == nil || frame.Response.ID != id {
			t.Fatalf("response = %#v, %v; want %q", frame, err, id)
		}
	}
}

func TestServeSubscribeStreamsAndConflicts(t *testing.T) {
	router := NewRouter(logging.Logger{})
	bus := NewBus(8)
	path, cancel, done := startProtocolServer(t, router, bus)
	defer stopProtocolServer(t, path, cancel, done)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	encoder, decoder := wire.NewEncoder(conn), wire.NewDecoder(conn)
	if err := encoder.Encode(wire.Request{Schema: wire.Schema, ID: "subscribe", Command: "subscribe", Params: json.RawMessage(`{"since":1}`)}); err != nil {
		t.Fatal(err)
	}
	bus.Publish(wire.Event{Kind: wire.KindPickup, Level: "info"})
	frame, err := decoder.Frame()
	if err != nil || frame.Event == nil || frame.Event.Kind != wire.KindPickup {
		t.Fatalf("event = %#v, %v", frame, err)
	}
	if err := encoder.Encode(wire.Request{Schema: wire.Schema, ID: "next", Command: "status"}); err != nil {
		t.Fatal(err)
	}
	frame, err = decoder.Frame()
	if err != nil || frame.Response == nil || frame.Response.Err == nil || frame.Response.Err.Code != wire.CodeConflict {
		t.Fatalf("conflict = %#v, %v", frame, err)
	}
	bus.Publish(wire.Event{Kind: wire.KindComplete, Level: "info"})
	frame, err = decoder.Frame()
	if err != nil || frame.Event == nil || frame.Event.Kind != wire.KindComplete {
		t.Fatalf("continued event = %#v, %v", frame, err)
	}
}

func TestServeBadRequestStaysOpenAndSchemaCloses(t *testing.T) {
	router := NewRouter(logging.Logger{})
	path, cancel, done := startProtocolServer(t, router, NewBus(8))
	defer stopProtocolServer(t, path, cancel, done)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("not json\n")); err != nil {
		t.Fatal(err)
	}
	decoder := wire.NewDecoder(conn)
	frame, err := decoder.Frame()
	if err != nil || frame.Response == nil || frame.Response.Err.Code != wire.CodeBadRequest {
		t.Fatalf("bad request = %#v, %v", frame, err)
	}
	if err := wire.NewEncoder(conn).Encode(wire.Request{Schema: 99, ID: "old"}); err != nil {
		t.Fatal(err)
	}
	frame, err = decoder.Frame()
	if err != nil || frame.Response == nil || frame.Response.Err.Code != wire.CodeSchemaMismatch {
		t.Fatalf("schema response = %#v, %v", frame, err)
	}
	_, err = decoder.Frame()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("connection remained after schema mismatch: %v", err)
	}
}

func TestServeShutdownDrainsHungConnection(t *testing.T) {
	path, cancel, done := startProtocolServer(t, NewRouter(logging.Logger{}), NewBus(8))
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	started := time.Now()
	stopProtocolServer(t, path, cancel, done)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("shutdown blocked for %s", elapsed)
	}
}

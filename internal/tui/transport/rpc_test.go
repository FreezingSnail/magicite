package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

var socketSequence atomic.Uint64

func TestRPCSnapshotUsesTypedClientAndOwnsEachRequestConnection(t *testing.T) {
	result := wire.SnapshotResult{ModelVersion: 1, Generation: 2, Runtime: wire.StatusResult{Sessions: []wire.SessionResult{}}, Repositories: []wire.RepoResult{{Name: "magicite", Path: "/private/project", Prefix: "magicite", Branch: "main"}}, Seats: []wire.SeatResult{{Name: "ifrit", Worktree: "/private/worktree"}}, Sessions: []wire.SessionResult{}, Beads: []wire.BeadResult{}, StatusCounts: []wire.StatusCount{}, RepositoryErrors: []wire.RepositoryError{}}
	want := Snapshot{ModelVersion: 1, Generation: 2, Runtime: wire.StatusResult{Sessions: []wire.SessionResult{}}, Repositories: []Repository{{Name: "magicite", Prefix: "magicite", Branch: "main"}}, Seats: []Seat{{Name: "ifrit"}}, Sessions: []wire.SessionResult{}, Beads: []wire.BeadResult{}, StatusCounts: []wire.StatusCount{}, RepositoryErrors: []wire.RepositoryError{}}
	accepted := make(chan net.Conn, 2)
	socket := serveSocket(t, func(conn net.Conn) {
		accepted <- conn
		request, err := wire.NewDecoder(conn).Request()
		if err != nil {
			t.Error(err)
			return
		}
		if request.Command != "snapshot" || request.Params != nil {
			t.Errorf("request = %#v, want no-parameter snapshot", request)
			return
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Error(err)
			return
		}
		if err := wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: encoded}); err != nil {
			t.Error(err)
		}
	})
	api := New(client.New(client.Options{Socket: socket, Timeout: time.Second}))

	for range 2 {
		got, err := api.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Snapshot() = %#v, want %#v", got, want)
		}
	}
	first := <-accepted
	second := <-accepted
	if first == second {
		t.Fatal("Snapshot() reused a connection")
	}
}

func TestRPCSnapshotMapsClientErrorWithoutSocketDetail(t *testing.T) {
	socket := serveSocket(t, func(conn net.Conn) {
		request, err := wire.NewDecoder(conn).Request()
		if err != nil {
			t.Error(err)
			return
		}
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Err: &wire.Error{Code: wire.CodeUnavailable, Message: "daemon socket /private/secret.sock unavailable"}})
	})

	_, err := NewRPC(client.New(client.Options{Socket: socket, Timeout: time.Second})).Snapshot(context.Background())
	var transportError *Error
	if !errors.As(err, &transportError) {
		t.Fatalf("Snapshot() error = %T %v, want transport Error", err, err)
	}
	if transportError.Code != ErrorUnavailable || err.Error() != string(ErrorUnavailable) {
		t.Fatalf("Snapshot() error = %#v, want unavailable without socket detail", transportError)
	}
}

func TestRPCSnapshotPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewRPC(client.New(client.Options{Socket: "unused", Timeout: time.Second})).Snapshot(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Snapshot() error = %v, want context cancellation", err)
	}
}

func serveSocket(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	socket := socketPath(t)
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

func socketPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".tui-transport-%d.sock", socketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestRPCSnapshotServerClosesAfterEachResponse(t *testing.T) {
	closed := make(chan struct{}, 1)
	socket := serveSocket(t, func(conn net.Conn) {
		request, err := wire.NewDecoder(conn).Request()
		if err != nil {
			t.Error(err)
			return
		}
		if err := wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{}`)}); err != nil {
			t.Error(err)
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := conn.Read(make([]byte, 1)); errors.Is(err, io.EOF) {
			closed <- struct{}{}
		}
	})

	if _, err := New(client.New(client.Options{Socket: socket, Timeout: time.Second})).Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("snapshot connection remained open")
	}
}

func TestRPCMetricsUsesTypedClientAndMapsUnsupported(t *testing.T) {
	result := wire.MetricsResult{Lifecycle: []wire.MetricsCount{{Key: "pickup", Count: 2}}, Land: []wire.MetricsCount{}, Roles: []wire.RoleDuration{}, Queue: []wire.RepoQueueDepth{}}
	socket := serveSocket(t, func(conn net.Conn) {
		request, err := wire.NewDecoder(conn).Request()
		if err != nil {
			t.Error(err)
			return
		}
		if request.Command != "metrics" || request.Params != nil {
			t.Errorf("request = %#v, want no-parameter metrics", request)
			return
		}
		payload, err := json.Marshal(result)
		if err != nil {
			t.Error(err)
			return
		}
		if err := wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: payload}); err != nil {
			t.Error(err)
		}
	})
	got, err := NewRPC(client.New(client.Options{Socket: socket, Timeout: time.Second})).Metrics(context.Background())
	if err != nil || !reflect.DeepEqual(got, result) {
		t.Fatalf("Metrics() = (%#v, %v), want (%#v, nil)", got, err, result)
	}

	unsupported := serveSocket(t, func(conn net.Conn) {
		request, err := wire.NewDecoder(conn).Request()
		if err != nil {
			t.Error(err)
			return
		}
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Err: &wire.Error{Code: wire.CodeUnknownCommand}})
	})
	_, err = NewRPC(client.New(client.Options{Socket: unsupported, Timeout: time.Second})).Metrics(context.Background())
	var transportError *Error
	if !errors.As(err, &transportError) || transportError.Code != ErrorUnsupported {
		t.Fatalf("Metrics() error = %#v, want unsupported", err)
	}
}

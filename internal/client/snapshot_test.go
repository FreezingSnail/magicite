package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestSnapshotSendsTypedRequestAndReturnsIndependentCollections(t *testing.T) {
	want := snapshotResult()
	requests := make(chan wire.Request, 2)
	socket := serve(t, func(conn net.Conn) {
		request, err := receiveRequest(conn)
		if err != nil {
			return
		}
		requests <- request
		result, err := json.Marshal(want)
		if err != nil {
			t.Error(err)
			return
		}
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: result})
	})

	client := New(Options{Socket: socket})
	first, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", first, want)
	}
	assertSnapshotCollections(t, first)
	first.Repositories[0].Name = "changed"
	first.Runtime.Sessions[0].Handle = "changed"
	first.Beads[0].Dependencies[0].ID = "changed"
	first.Beads[0].Labels[0] = "changed"

	second, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("second Snapshot() = %#v, want %#v", second, want)
	}
	for range 2 {
		request := <-requests
		if request.Schema != wire.Schema || request.Command != "snapshot" || request.Params != nil {
			t.Fatalf("snapshot request = %#v", request)
		}
	}
}

func TestSnapshotNormalizesOmittedCollections(t *testing.T) {
	socket := serve(t, func(conn net.Conn) {
		request, err := receiveRequest(conn)
		if err != nil {
			return
		}
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{}`)})
	})

	result, err := New(Options{Socket: socket}).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertSnapshotCollections(t, result)
}

func TestSnapshotPreservesCallErrorsAndReturnsZeroResult(t *testing.T) {
	tests := []struct {
		name    string
		respond func(wire.Request, net.Conn)
		code    wire.Code
	}{
		{
			name: "unknown command",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Err: &wire.Error{Code: wire.CodeNotFound, Message: "unknown command"}})
			},
			code: wire.CodeNotFound,
		},
		{
			name: "schema mismatch",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema + 1, ID: request.ID})
			},
			code: wire.CodeSchemaMismatch,
		},
		{
			name: "malformed result",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{"model_version":"one"}`)})
			},
			code: wire.CodeInternal,
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
			result, err := New(Options{Socket: socket}).Snapshot(context.Background())
			var clientErr *Error
			if !errors.As(err, &clientErr) || clientErr.Code != test.code {
				t.Fatalf("Snapshot() error = %#v, want code %q", err, test.code)
			}
			if !reflect.DeepEqual(result, wire.SnapshotResult{}) {
				t.Fatalf("Snapshot() result = %#v, want zero", result)
			}
		})
	}

	result, err := New(Options{Socket: testSocket(t)}).Snapshot(context.Background())
	var clientErr *Error
	if !errors.As(err, &clientErr) || clientErr.Code != wire.CodeUnavailable {
		t.Fatalf("unavailable Snapshot() error = %#v", err)
	}
	if !reflect.DeepEqual(result, wire.SnapshotResult{}) {
		t.Fatalf("unavailable Snapshot() result = %#v, want zero", result)
	}
}

func snapshotResult() wire.SnapshotResult {
	return wire.SnapshotResult{
		ModelVersion: 1,
		Generation:   2,
		Cursor:       3,
		Runtime: wire.StatusResult{
			Version:  "v1",
			Sessions: []wire.SessionResult{{Handle: "runtime"}},
		},
		Repositories:     []wire.RepoResult{{Name: "magicite"}},
		Seats:            []wire.SeatResult{{Name: "shiva"}},
		Sessions:         []wire.SessionResult{{Handle: "session"}},
		Beads:            []wire.BeadResult{{ID: "magicite-qik", Dependencies: []wire.DependencyResult{{ID: "dependency"}}, Labels: []string{"staged"}}},
		StatusCounts:     []wire.StatusCount{{Status: "open", Count: 1}},
		RepositoryErrors: []wire.RepositoryError{{Repository: "magicite", Error: "unavailable"}},
	}
}

func assertSnapshotCollections(t *testing.T, result wire.SnapshotResult) {
	t.Helper()
	if result.Repositories == nil || result.Seats == nil || result.Sessions == nil || result.Beads == nil || result.StatusCounts == nil || result.RepositoryErrors == nil || result.Runtime.Sessions == nil {
		t.Fatalf("Snapshot() collections = %#v, want nonnil", result)
	}
	for _, bead := range result.Beads {
		if bead.Dependencies == nil || bead.Labels == nil {
			t.Fatalf("Snapshot() bead collections = %#v, want nonnil", bead)
		}
	}
}

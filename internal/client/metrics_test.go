package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestMetricsSendsTypedRequestAndReturnsIndependentCollections(t *testing.T) {
	want := metricsResult()
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
	first, err := client.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("Metrics() = %#v, want %#v", first, want)
	}
	assertMetricsCollections(t, first)
	first.Lifecycle[0].Key = "changed"
	first.Land[0].Key = "changed"
	first.Roles[0].Role = "changed"
	first.Queue[0].Repo = "changed"

	second, err := client.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("second Metrics() = %#v, want %#v", second, want)
	}
	for range 2 {
		request := <-requests
		if request.Schema != wire.Schema || request.Command != "metrics" || request.Params != nil {
			t.Fatalf("metrics request = %#v", request)
		}
	}
}

func TestMetricsNormalizesOmittedCollections(t *testing.T) {
	socket := serve(t, func(conn net.Conn) {
		request, err := receiveRequest(conn)
		if err != nil {
			return
		}
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{}`)})
	})

	result, err := New(Options{Socket: socket}).Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetricsCollections(t, result)
}

func TestMetricsPreservesCallErrorsAndReturnsZeroResult(t *testing.T) {
	tests := []struct {
		name    string
		respond func(wire.Request, net.Conn)
		code    wire.Code
	}{
		{
			name: "daemon error",
			respond: func(request wire.Request, conn net.Conn) {
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Err: &wire.Error{Code: wire.CodeConflict, Message: "busy"}})
			},
			code: wire.CodeConflict,
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
				_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: json.RawMessage(`{"started_at":"not-a-time"}`)})
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
			result, err := New(Options{Socket: socket}).Metrics(context.Background())
			var clientErr *Error
			if !errors.As(err, &clientErr) || clientErr.Code != test.code {
				t.Fatalf("Metrics() error = %#v, want code %q", err, test.code)
			}
			if !reflect.DeepEqual(result, wire.MetricsResult{}) {
				t.Fatalf("Metrics() result = %#v, want zero", result)
			}
		})
	}

	result, err := New(Options{Socket: testSocket(t)}).Metrics(context.Background())
	var clientErr *Error
	if !errors.As(err, &clientErr) || clientErr.Code != wire.CodeUnavailable {
		t.Fatalf("unavailable Metrics() error = %#v", err)
	}
	if !reflect.DeepEqual(result, wire.MetricsResult{}) {
		t.Fatalf("unavailable Metrics() result = %#v, want zero", result)
	}
}

func TestMetricsHonorsDeadlineAndCancellation(t *testing.T) {
	socket := serve(t, func(conn net.Conn) { _, _ = receiveRequest(conn) })
	started := time.Now()
	result, err := New(Options{Socket: socket, Timeout: 25 * time.Millisecond}).Metrics(context.Background())
	var clientErr *Error
	if !errors.As(err, &clientErr) || clientErr.Code != wire.CodeUnavailable || time.Since(started) > time.Second {
		t.Fatalf("timed Metrics() = (%#v, %v) after %s", result, err, time.Since(started))
	}
	if !reflect.DeepEqual(result, wire.MetricsResult{}) {
		t.Fatalf("timed Metrics() result = %#v, want zero", result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = New(Options{Socket: socket}).Metrics(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Metrics() error = %#v", err)
	}
	if !reflect.DeepEqual(result, wire.MetricsResult{}) {
		t.Fatalf("cancelled Metrics() result = %#v, want zero", result)
	}
}

func metricsResult() wire.MetricsResult {
	startedAt := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC)
	sampledAt := startedAt.Add(time.Minute)
	return wire.MetricsResult{
		StartedAt:     startedAt,
		UptimeSeconds: 60,
		Lifecycle:     []wire.MetricsCount{{Key: "pickup", Count: 1}},
		Land:          []wire.MetricsCount{{Key: "ok", Count: 2}},
		Sessions:      wire.SessionGauges{Active: 3, Peak: 4, Completed: 5, Failed: 6},
		Roles:         []wire.RoleDuration{{Role: "implementer", Sessions: 7, TotalSeconds: 480}},
		Queue:         []wire.RepoQueueDepth{{Repo: "magicite", Depth: 8}},
		QueueTotal:    8,
		QueueSampledAt: &sampledAt,
		Bus:           wire.BusMetrics{Published: 9, Dropped: 10},
	}
}

func assertMetricsCollections(t *testing.T, result wire.MetricsResult) {
	t.Helper()
	if result.Lifecycle == nil || result.Land == nil || result.Roles == nil || result.Queue == nil {
		t.Fatalf("Metrics() collections = %#v, want nonnil", result)
	}
}

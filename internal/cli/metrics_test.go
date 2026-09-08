package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestMetricsRendersDeterministicTextAndRawJSON(t *testing.T) {
	payload := json.RawMessage(`{"started_at":"2026-09-08T04:05:06Z","uptime_seconds":60,"lifecycle":[{"key":"pickup","count":2},{"key":"complete","count":1}],"land":[{"key":"ok","count":3},{"key":"conflict","count":4}],"sessions":{"active":5,"peak":6,"completed":7,"failed":8},"roles":[{"role":"review","sessions":9,"total_seconds":10},{"role":"implementer","sessions":11,"total_seconds":12}],"queue":[{"repo":"zeta","depth":13},{"repo":"alpha","depth":14}],"queue_total":27,"queue_sampled_at":"2026-09-08T04:06:06Z","bus":{"published":15,"dropped":16}}`)
	socket := metricsServer(t, payload, true)

	var out, err bytes.Buffer
	if code := Run(context.Background(), []string{"--socket", socket, "metrics"}, &out, &err); code != 0 {
		t.Fatalf("Run() = %d, stderr = %q", code, err.String())
	}
	const want = "started: 2026-09-08T04:05:06Z  uptime: 60  active: 5  peak: 6  completed: 7  failed: 8\n" +
		"KIND      COUNT\n" +
		"complete  1    \n" +
		"pickup    2    \n" +
		"RESULT    COUNT\n" +
		"conflict  4    \n" +
		"ok        3    \n" +
		"REPO   DEPTH\n" +
		"alpha  14   \n" +
		"zeta   13   \n" +
		"queue total: 27  sampled: 2026-09-08T04:06:06Z\n" +
		"ROLE         SESSIONS  TOTAL_SECONDS\n" +
		"implementer  11        12           \n" +
		"review       9         10           \n" +
		"bus: published: 15  dropped: 16\n"
	if out.String() != want || err.Len() != 0 {
		t.Fatalf("text = %q, stderr = %q", out.String(), err.String())
	}

	out.Reset()
	err.Reset()
	if code := Run(context.Background(), []string{"--socket", socket, "--json", "metrics"}, &out, &err); code != 0 {
		t.Fatalf("JSON Run() = %d, stderr = %q", code, err.String())
	}
	var envelope struct {
		Schema int             `json:"schema"`
		Kind   string          `json:"kind"`
		Data   json.RawMessage `json:"data"`
	}
	if unmarshalErr := json.Unmarshal(out.Bytes(), &envelope); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	if envelope.Schema != wire.Schema || envelope.Kind != "metrics" || string(envelope.Data) != string(payload) || err.Len() != 0 {
		t.Fatalf("JSON envelope = %s, stderr = %q", out.String(), err.String())
	}
}

func TestMetricsRendersEmptyCollectionsAndPartialQueue(t *testing.T) {
	socket := metricsServer(t, json.RawMessage(`{"started_at":"2026-09-08T04:05:06Z","uptime_seconds":0,"lifecycle":[],"land":[],"sessions":{"active":0,"peak":0,"completed":0,"failed":0},"roles":[],"queue":[{"repo":"available","depth":0}],"queue_total":0,"queue_sampled_at":null,"bus":{"published":0,"dropped":0}}`), true)

	var out, err bytes.Buffer
	if code := Run(context.Background(), []string{"--socket", socket, "metrics"}, &out, &err); code != 0 {
		t.Fatalf("Run() = %d, stderr = %q", code, err.String())
	}
	for _, want := range []string{"KIND  COUNT\n", "RESULT  COUNT\n", "REPO       DEPTH\navailable  0    \n", "queue total: 0  sampled: never\n", "ROLE  SESSIONS  TOTAL_SECONDS\n", "bus: published: 0  dropped: 0\n"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("text = %q, missing %q", out.String(), want)
		}
	}
}

func TestMetricsRejectsArgumentsBeforeDialing(t *testing.T) {
	missing := ".magicite-metrics-missing.sock"
	_ = os.Remove(missing)
	for _, args := range [][]string{{"metrics", "extra"}, {"metrics", "--unknown"}} {
		var out, err bytes.Buffer
		args = append([]string{"--socket", missing}, args...)
		if code := Run(context.Background(), args, &out, &err); code != 2 {
			t.Fatalf("Run(%v) = %d, want 2", args, code)
		}
		if out.Len() != 0 || err.String() != "usage: magicite metrics\n" {
			t.Fatalf("Run(%v): stdout = %q, stderr = %q", args, out.String(), err.String())
		}
	}
}

func TestMetricsMapsDaemonFailures(t *testing.T) {
	socket := metricsServer(t, nil, false)
	var out, err bytes.Buffer
	if code := Run(context.Background(), []string{"--socket", socket, "metrics"}, &out, &err); code != 2 {
		t.Fatalf("Run() = %d, want 2; stderr = %q", code, err.String())
	}
	if out.Len() != 0 || err.String() != "magicite: unknown_command: unknown command \"metrics\"\n" {
		t.Fatalf("stdout = %q, stderr = %q", out.String(), err.String())
	}
}

func metricsServer(t *testing.T, payload json.RawMessage, register bool) string {
	t.Helper()
	socket := readSocket(t)
	router := server.NewRouter(logging.Logger{})
	if register {
		if err := router.Register("metrics", func(_ context.Context, params json.RawMessage) (any, error) {
			if params != nil {
				return nil, fmt.Errorf("metrics params = %s", params)
			}
			return payload, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, server.Deps{Router: router, Bus: server.NewBus(8), Socket: socket}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case serveErr := <-done:
			if serveErr != nil {
				t.Error(serveErr)
			}
		case <-time.After(time.Second):
			t.Error("metrics server did not stop")
		}
	})
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
		if time.Now().After(deadline) {
			t.Fatal("metrics server did not start")
		}
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/wire"
)

var readSocketSequence atomic.Uint64

func readSocket(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".magicite-read-%d-%d.sock", os.Getpid(), readSocketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func readServer(t *testing.T) string {
	t.Helper()
	socket := readSocket(t)
	router := server.NewRouter(logging.Logger{})
	responses := map[string]json.RawMessage{
		"status": json.RawMessage(`{"version":"v","schema":1,"running":true,"draining":false,"repos":1,"implementer_cap":2,"sessions":[{"handle":"h","repo":"r","task":"t","role":"implementer","seat":"ifrit","backend":"kiro","model":"m","status":"busy","phase":"run","uptime_seconds":4}]}`),
		"seats":  json.RawMessage(`[{"name":"ifrit","role":"implementer","repo":"r","worktree":"w","task":"t","busy":true}]`),
		"tasks":  json.RawMessage(`[{"id":"r-1","repo":"r","title":"task","status":"open","difficulty":"low","priority":2,"labels":["a","b"]}]`),
		"repos":  json.RawMessage(`[{"name":"r","path":"/r","prefix":"r-","branch":"main"}]`),
	}
	for command, payload := range responses {
		command, payload := command, payload
		if err := router.Register(command, func(_ context.Context, params json.RawMessage) (any, error) {
			if command == "tasks" {
				var got wire.TasksParams
				if err := json.Unmarshal(params, &got); err != nil {
					return nil, err
				}
				if got.Repo != "r" || !got.All {
					return nil, fmt.Errorf("tasks params = %#v", got)
				}
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
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
	}
}

func TestReadCommandsRenderTablesAndPayloads(t *testing.T) {
	socket := readServer(t)
	for _, test := range []struct {
		name    string
		args    []string
		header  string
		payload string
	}{
		{"status", []string{"status"}, "HANDLE", `{"version":"v","schema":1,"running":true,"draining":false,"repos":1,"implementer_cap":2,"sessions":[{"handle":"h","repo":"r","task":"t","role":"implementer","seat":"ifrit","backend":"kiro","model":"m","status":"busy","phase":"run","uptime_seconds":4}]}`},
		{"seats", []string{"seats"}, "NAME", `[{"name":"ifrit","role":"implementer","repo":"r","worktree":"w","task":"t","busy":true}]`},
		{"tasks", []string{"tasks", "--repo", "r", "--all"}, "ID", `[{"id":"r-1","repo":"r","title":"task","status":"open","difficulty":"low","priority":2,"labels":["a","b"]}]`},
		{"repos", []string{"repos"}, "NAME", `[{"name":"r","path":"/r","prefix":"r-","branch":"main"}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, err bytes.Buffer
			args := append([]string{"--socket", socket}, test.args...)
			if code := Run(context.Background(), args, &out, &err); code != 0 {
				t.Fatalf("Run() = %d, stderr = %q", code, err.String())
			}
			if !bytes.Contains(out.Bytes(), []byte(test.header)) {
				t.Fatalf("table = %q, missing %q", out.String(), test.header)
			}

			out.Reset()
			err.Reset()
			args = append([]string{"--socket", socket, "--json"}, test.args...)
			if code := Run(context.Background(), args, &out, &err); code != 0 {
				t.Fatalf("JSON Run() = %d, stderr = %q", code, err.String())
			}
			var envelope struct {
				Schema int             `json:"schema"`
				Kind   string          `json:"kind"`
				Data   json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Schema != wire.Schema || envelope.Kind != test.name || string(envelope.Data) != test.payload {
				t.Fatalf("envelope = %s", out.String())
			}
		})
	}
}

func TestReadCommandsRejectArgumentsAndUnavailableDaemon(t *testing.T) {
	for _, args := range [][]string{{"status", "extra"}, {"seats", "extra"}, {"tasks", "extra"}, {"repos", "extra"}} {
		var out, err bytes.Buffer
		if code := Run(context.Background(), args, &out, &err); code != 2 {
			t.Fatalf("Run(%v) = %d, want 2", args, code)
		}
	}
	var out, err bytes.Buffer
	if code := Run(context.Background(), []string{"--socket", ".missing.sock", "status"}, &out, &err); code != 3 {
		t.Fatalf("unavailable Run() = %d, want 3; stderr = %q", code, err.String())
	}
	if bytes.Count(err.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("unavailable stderr = %q, want one line", err.String())
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/server"
	"github.com/connorfranc/magicite/internal/wire"
)

func TestControlCommandsRenderPlainAndJSON(t *testing.T) {
	socket := controlServer(t)
	tests := []struct {
		name  string
		args  []string
		plain string
		kind  string
		data  string
	}{
		{"start", []string{"start"}, "implementer cap: 3  sessions: 2\n", "start", `{"version":"v","schema":1,"running":true,"implementer_cap":3,"sessions":[{},{}]}`},
		{"stop", []string{"stop", "--hard"}, "mode: hard  sessions: 1\n", "stop", `{"mode":"hard","sessions":1,"draining":false}`},
		{"dispatch", []string{"dispatch", "task-1", "--repo", "repo", "--role", "repair"}, "seat: ifrit  role: repair  handle: h1\n", "dispatch", `{"handle":"h1","repo":"repo","task":"task-1","role":"repair","seat":"ifrit"}`},
		{"review", []string{"review", "epic-1", "--repo", "repo"}, "epic: epic-1  held: true\n", "review", `{"epic":"epic-1","repo":"repo","handle":"h2","held":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out, err bytes.Buffer
			args := append([]string{"--socket", socket}, test.args...)
			if code := Run(context.Background(), args, &out, &err); code != 0 {
				t.Fatalf("plain Run() = %d, stderr = %q", code, err.String())
			}
			if out.String() != test.plain || err.Len() != 0 {
				t.Fatalf("plain output = %q, stderr = %q", out.String(), err.String())
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
			if envelope.Schema != wire.Schema || envelope.Kind != test.kind || string(envelope.Data) != test.data || err.Len() != 0 {
				t.Fatalf("JSON envelope = %s, stderr = %q", out.String(), err.String())
			}
		})
	}
}

func TestControlCommandsRejectArityBeforeDialing(t *testing.T) {
	missing := ".magicite-control-missing.sock"
	_ = os.Remove(missing)
	for _, args := range [][]string{
		{"start", "extra"},
		{"stop", "extra"},
		{"dispatch"},
		{"dispatch", "task-1", "extra"},
		{"review"},
		{"review", "epic-1", "extra"},
	} {
		var out, err bytes.Buffer
		args = append([]string{"--socket", missing}, args...)
		if code := Run(context.Background(), args, &out, &err); code != 2 {
			t.Fatalf("Run(%v) = %d, want 2", args, code)
		}
		if out.Len() != 0 || !bytes.Contains(err.Bytes(), []byte("usage: magicite")) {
			t.Fatalf("Run(%v): stdout = %q, stderr = %q", args, out.String(), err.String())
		}
	}
}

func controlServer(t *testing.T) string {
	t.Helper()
	socket := readSocket(t)
	router := server.NewRouter(logging.Logger{})
	responses := map[string]json.RawMessage{
		"start":    json.RawMessage(`{"version":"v","schema":1,"running":true,"implementer_cap":3,"sessions":[{},{}]}`),
		"stop":     json.RawMessage(`{"mode":"hard","sessions":1,"draining":false}`),
		"dispatch": json.RawMessage(`{"handle":"h1","repo":"repo","task":"task-1","role":"repair","seat":"ifrit"}`),
		"review":   json.RawMessage(`{"epic":"epic-1","repo":"repo","handle":"h2","held":true}`),
	}
	for command, payload := range responses {
		command, payload := command, payload
		if err := router.Register(command, func(_ context.Context, params json.RawMessage) (any, error) {
			switch command {
			case "stop":
				var got wire.StopParams
				if err := json.Unmarshal(params, &got); err != nil || !got.Hard {
					return nil, fmt.Errorf("stop params = %s", params)
				}
			case "dispatch":
				var got wire.DispatchParams
				if err := json.Unmarshal(params, &got); err != nil || got.Task != "task-1" || got.Repo != "repo" || got.Role != "repair" {
					return nil, fmt.Errorf("dispatch params = %s", params)
				}
			case "review":
				var got wire.ReviewParams
				if err := json.Unmarshal(params, &got); err != nil || got.Epic != "epic-1" || got.Repo != "repo" {
					return nil, fmt.Errorf("review params = %s", params)
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
			t.Error("control server did not stop")
		}
	})
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
		if time.Now().After(deadline) {
			t.Fatal("control server did not start")
		}
	}
}

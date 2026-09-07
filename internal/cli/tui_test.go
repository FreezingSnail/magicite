package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/tui"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestTUIRejectsJSONAndArguments(t *testing.T) {
	for _, args := range [][]string{{"--json", "tui"}, {"tui", "extra"}} {
		var out, err bytes.Buffer
		if code := Run(context.Background(), args, &out, &err); code != 2 {
			t.Fatalf("Run(%v) = %d, want 2", args, code)
		}
		if err.Len() == 0 {
			t.Fatalf("Run(%v) did not print usage", args)
		}
	}
}

func TestTUIComposesInheritedSocketClients(t *testing.T) {
	socket := readSocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close(); _ = os.Remove(socket) })
	requests := make(chan string, 2)
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				request, err := wire.NewDecoder(conn).Request()
				if err != nil {
					return
				}
				requests <- request.Command
				switch request.Command {
				case "snapshot":
					payload, _ := json.Marshal(wire.SnapshotResult{ModelVersion: wire.Schema, Generation: 1, Fresh: true, Runtime: wire.StatusResult{Running: true}})
					_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: payload})
				case "subscribe":
					<-serveDone
				}
			}()
		}
	}()

	original := runTUI
	runTUI = func(_ context.Context, program *tui.Program, _ *Env) error {
		command := program.Init()
		message := command()
		program.Update(message)
		program.Stop()
		return nil
	}
	t.Cleanup(func() { runTUI = original })
	var out, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"--socket", socket, "--timeout", "1s", "tui"}, &out, &stderr); code != 0 {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case command := <-requests:
			seen[command] = true
		case <-time.After(time.Second):
			t.Fatalf("requests = %#v", seen)
		}
	}
	if !seen["snapshot"] || !seen["subscribe"] {
		t.Fatalf("requests = %#v", seen)
	}
}

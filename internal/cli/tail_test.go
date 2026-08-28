package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestRenderEventPlain(t *testing.T) {
	at := time.Date(2026, time.August, 28, 20, 0, 0, 123, time.FixedZone("UTC-4", -4*60*60))
	var output bytes.Buffer
	err := RenderEvent(&output, wire.Event{
		Time: at, Level: "info", Kind: wire.KindLand, Repo: "magicite", Task: "magicite-9pt.10", Seat: "ifrit",
		Fields: map[string]string{"zeta": "last", "alpha": "first"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-08-28T20:00:00.000000123-04:00 info land repo=magicite task=magicite-9pt.10 seat=ifrit alpha=first zeta=last\n"
	if output.String() != want {
		t.Fatalf("plain = %q, want %q", output.String(), want)
	}
}

func TestRenderEventJSON(t *testing.T) {
	event := wire.Event{Schema: wire.Schema, Seq: 9, Time: time.Date(2026, time.August, 28, 20, 0, 0, 0, time.UTC), Kind: wire.KindComplete, Level: "info", Fields: map[string]string{"task": "done"}}
	var output bytes.Buffer
	if err := RenderEvent(&output, event, true); err != nil {
		t.Fatal(err)
	}
	var got wire.Event
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != event.Schema || got.Seq != event.Seq || !got.Time.Equal(event.Time) || got.Kind != event.Kind || got.Level != event.Level || got.Fields["task"] != "done" {
		t.Fatalf("JSON event = %#v, want %#v", got, event)
	}
}

func TestTailReplaysFiniteWindowAndStartsLive(t *testing.T) {
	bus := server.NewBus(8)
	bus.Publish(wire.Event{Kind: wire.KindPickup, Level: "info"})
	bus.Publish(wire.Event{Kind: wire.KindComplete, Level: "info"})
	socket := tailServer(t, bus)

	var output, stderr bytes.Buffer
	env := &Env{Client: client.New(client.Options{Socket: socket}), Out: &output, Err: &stderr}
	if code := (Tail{Since: 0, Follow: false, JSON: true}).Run(context.Background(), env); code != 0 {
		t.Fatalf("finite Run() = %d, stderr = %q", code, stderr.String())
	}
	var sequences []uint64
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var event wire.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		sequences = append(sequences, event.Seq)
	}
	if len(sequences) < 2 || sequences[0] != 1 || sequences[1] != 2 {
		t.Fatalf("replayed sequences = %v, want prefix [1 2]", sequences)
	}

	output.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"--socket", socket, "tail", "--follow=false"}, &output, &stderr); code != 0 {
		t.Fatalf("live finite Run() = %d, stderr = %q", code, stderr.String())
	}
	if output.Len() != 0 {
		t.Fatalf("live output = %q, want empty", output.String())
	}
}

func TestTailReconnectBudget(t *testing.T) {
	var output, stderr bytes.Buffer
	env := &Env{Client: client.New(client.Options{Socket: ".missing-tail.sock"}), Out: &output, Err: &stderr}
	if code := (Tail{Follow: true, Reconnect: 0}).Run(context.Background(), env); code != 3 {
		t.Fatalf("Run() = %d, want 3; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "reconnect attempts exhausted") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestTailReportsMissedRange(t *testing.T) {
	bus := server.NewBus(2)
	for range 4 {
		bus.Publish(wire.Event{Kind: wire.KindLand, Level: "info"})
	}
	socket := tailServer(t, bus)
	var output, stderr bytes.Buffer
	env := &Env{Client: client.New(client.Options{Socket: socket}), Out: &output, Err: &stderr}
	if code := (Tail{Since: 1, Follow: false}).Run(context.Background(), env); code != 0 {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(output.String(), "missed_first_seq=2 missed_last_seq=3") {
		t.Fatalf("gap output = %q", output.String())
	}
}

func tailServer(t *testing.T, bus *server.Bus) string {
	t.Helper()
	socket := readSocket(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, server.Deps{Router: server.NewRouter(logging.Logger{}), Bus: bus, Socket: socket})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("tail server did not stop")
		}
	})
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
		if time.Now().After(deadline) {
			t.Fatal(fmt.Errorf("tail server did not start: %s", socket))
		}
	}
}

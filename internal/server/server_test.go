package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/repotest"
	"github.com/FreezingSnail/magicite/internal/state"
)

var socketSequence atomic.Uint64

func testSocket(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".magicite-server-%d-%d.sock", os.Getpid(), socketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func startServer(t *testing.T, lookups ...repo.Lookup) (context.CancelFunc, string, <-chan error) {
	t.Helper()
	lookup := repo.Lookup(repotest.New())
	if len(lookups) > 0 {
		lookup = lookups[0]
	}
	ctx, cancel := context.WithCancel(context.Background())
	socket := testSocket(t)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, socket, config.Default(), state.Default(), lookup) }()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(socket); err == nil {
			return cancel, socket, done
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("server did not create %s", socket)
		}
		time.Sleep(time.Millisecond)
	}
}

func clientRequest(t *testing.T, socket, command string) net.Conn {
	t.Helper()
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(conn).Encode(map[string]string{"command": command}); err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	return conn
}

func stopServer(t *testing.T, cancel context.CancelFunc, done <-chan error) {
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
}

func TestServeStatusAndSocketMode(t *testing.T) {
	cancel, socket, done := startServer(t)
	defer stopServer(t, cancel, done)

	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("socket mode = %o, want 600", mode)
	}

	conn := clientRequest(t, socket, "status")
	defer conn.Close()
	var got Status
	if err := json.NewDecoder(conn).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.State != "running" || got.PID != os.Getpid() {
		t.Fatalf("status = %+v", got)
	}
}

func TestServeBroadcastsEventsAndDropsSlowSubscriber(t *testing.T) {
	cancel, socket, done := startServer(t)
	defer stopServer(t, cancel, done)

	first := clientRequest(t, socket, "tail")
	defer first.Close()
	second := clientRequest(t, socket, "tail")
	defer second.Close()
	firstReader := bufio.NewReader(first)
	secondReader := bufio.NewReader(second)
	for _, reader := range []*bufio.Reader{firstReader, secondReader} {
		line, err := reader.ReadString('\n')
		if err != nil || line != "{\"ready\":true}\n" {
			t.Fatalf("tail ready = %q, %v", line, err)
		}
	}
	logging.Event(logging.Info, "test.broadcast", map[string]any{"value": 1})

	for _, reader := range []*bufio.Reader{firstReader, secondReader} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		var event struct{ Kind string }
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind != "test.broadcast" {
			t.Fatalf("kind = %q", event.Kind)
		}
	}

	slow := clientRequest(t, socket, "tail")
	defer slow.Close()
	slowReader := bufio.NewReader(slow)
	if line, err := slowReader.ReadString('\n'); err != nil || line != "{\"ready\":true}\n" {
		t.Fatalf("slow tail ready = %q, %v", line, err)
	}
	started := time.Now()
	for i := 0; i < 64; i++ {
		logging.Event(logging.Info, "test.slow", map[string]any{"payload": string(make([]byte, 2048))})
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("slow subscriber stalled logging for %s", elapsed)
	}
}

func TestServeRemovesSocketOnCancellation(t *testing.T) {
	cancel, socket, done := startServer(t)
	stopServer(t, cancel, done)
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket remains after shutdown: %v", err)
	}
}

func TestServeReposAndRoute(t *testing.T) {
	first, ok := repo.Make("first", "first", "first", "trunk")
	if !ok {
		t.Fatal("first repository is invalid")
	}
	second, ok := repo.Make("second", "second", "second", "main")
	if !ok {
		t.Fatal("second repository is invalid")
	}
	lookup := repotest.New(first, second)
	cancel, socket, done := startServer(t, lookup)
	defer stopServer(t, cancel, done)

	conn := clientRequest(t, socket, "repos")
	defer conn.Close()
	var listed struct {
		Repos []RepoView `json:"repos"`
	}
	if err := json.NewDecoder(conn).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Repos) != 2 || listed.Repos[0] != (RepoView{Name: first.Name, Root: first.Root, Prefix: first.Prefix, Branch: first.Branch}) || listed.Repos[1] != (RepoView{Name: second.Name, Root: second.Root, Prefix: second.Prefix, Branch: second.Branch}) {
		t.Fatalf("repos = %+v", listed.Repos)
	}
	if got := lookup.Refreshes(); got != 1 {
		t.Fatalf("refreshes = %d, want 1", got)
	}

	route, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()
	if err := json.NewEncoder(route).Encode(request{Command: "route", ID: "second-123"}); err != nil {
		t.Fatal(err)
	}
	var routed struct {
		Repo RepoView `json:"repo"`
	}
	if err := json.NewDecoder(route).Decode(&routed); err != nil {
		t.Fatal(err)
	}
	if routed.Repo != (RepoView{Name: second.Name, Root: second.Root, Prefix: second.Prefix, Branch: second.Branch}) {
		t.Fatalf("route = %+v", routed.Repo)
	}

	missing, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Close()
	if err := json.NewEncoder(missing).Encode(request{Command: "route", ID: "missing-123"}); err != nil {
		t.Fatal(err)
	}
	var notFound map[string]string
	if err := json.NewDecoder(missing).Decode(&notFound); err != nil {
		t.Fatal(err)
	}
	if notFound["error"] != "not found" {
		t.Fatalf("route error = %+v", notFound)
	}
}

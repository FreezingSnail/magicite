package main

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
)

var cliSocketSequence atomic.Uint64

func cliSocket(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".magicite-cli-%d-%d.sock", os.Getpid(), cliSocketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestStatusJSONAndUnreachableExit(t *testing.T) {
	socket := cliSocket(t)
	router := server.NewRouter(logging.Logger{})
	if err := router.Register("status", func(context.Context, json.RawMessage) (any, error) {
		return struct {
			State string `json:"status"`
		}{State: "running"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, server.Deps{Router: router, Bus: server.NewBus(8), Socket: socket}) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	}()
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"status", "--socket", socket, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("status exit = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); !bytes.Contains([]byte(got), []byte(`"status":"running"`)) {
		t.Fatalf("status output = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"status", "--socket", socket + ".missing"}, &stdout, &stderr); code == 0 {
		t.Fatal("unreachable status succeeded")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("daemon unreachable:")) {
		t.Fatalf("unreachable stderr = %q", stderr.String())
	}
}

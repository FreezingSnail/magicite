package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/repotest"
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
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, socket, config.Default(), nil, repotest.New()) }()
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

package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
)

func listenPath(t *testing.T) string {
	t.Helper()
	directory := fmt.Sprintf(".magicite-listen-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	return filepath.Join(directory, "magicite.sock")
}

func TestListenSecuresSocketAndDirectory(t *testing.T) {
	path := listenPath(t)
	listener, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("socket mode = %o, want 600", got)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dir.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory mode = %o, want 700", got)
	}
}

func TestListenRetainsLiveSocketAndReplacesStaleSocket(t *testing.T) {
	path := listenPath(t)
	live, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	if _, err := Listen(path); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("Listen() error = %v, want ErrAlreadyRunning", err)
	}
	if !Probe(path) {
		t.Fatal("live socket not probed")
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	if Probe(path) {
		t.Fatal("stale socket probed live")
	}
	stale, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Close()
	if _, err := net.Dial("unix", path); err != nil {
		t.Fatalf("replacement listener unavailable: %v", err)
	}
}

func TestSocketPathUsesStateDirectory(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, want := SocketPath(config.Config{}), "/state/magicite/magicite.sock"; got != want {
		t.Fatalf("SocketPath() = %q, want %q", got, want)
	}
}

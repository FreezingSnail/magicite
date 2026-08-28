package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/connorfranc/magicite/internal/config"
)

const probeTimeout = 100 * time.Millisecond

// ErrAlreadyRunning reports an active daemon already serving a socket path.
var ErrAlreadyRunning = errors.New("server: daemon already running")

// SocketPath returns the default daemon socket path.
func SocketPath(_ config.Config) string {
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		return filepath.Join(runtime, "magicite", "magicite.sock")
	}
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "magicite", "magicite.sock")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "magicite", "magicite.sock")
	}
	return filepath.Join(home, ".local", "state", "magicite", "magicite.sock")
}

// Listen creates an owner-only Unix socket listener at path.
func Listen(path string) (net.Listener, error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("server: create socket directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("server: secure socket directory: %w", err)
	}

	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("server: inspect socket: %w", err)
	case Probe(path):
		return nil, ErrAlreadyRunning
	case info.Mode()&os.ModeSocket == 0:
		return nil, fmt.Errorf("server: refusing to replace non-socket %q", path)
	default:
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("server: remove stale socket: %w", err)
		}
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("server: listen: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("server: secure socket: %w", err)
	}
	return listener, nil
}

// Probe reports whether a listener accepts connections at path.
func Probe(path string) bool {
	conn, err := net.DialTimeout("unix", path, probeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

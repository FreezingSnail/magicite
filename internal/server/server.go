// Package server owns magicite's headless daemon lifecycle and local socket API.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/state"
)

const statusKey = "magicite.server.status"

// Serve listens on socket until ctx is cancelled. It owns the socket for its
// lifetime, exposes status and tail requests, and broadcasts logging events to
// tail subscribers without allowing a slow subscriber to block event producers.
func Serve(ctx context.Context, socket string, cfg config.Config, store *state.Store) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if socket == "" {
		return errors.New("server socket path is empty")
	}
	if store == nil {
		store = state.Default()
	}
	if err := prepareSocket(socket); err != nil {
		return err
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return fmt.Errorf("listen on %q: %w", socket, err)
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(socket)
		return fmt.Errorf("secure socket %q: %w", socket, err)
	}

	d := &daemon{
		listener: listener,
		store:    store,
		status: Status{
			State:   "running",
			PID:     os.Getpid(),
			Harness: cfg.Harness.Name,
			Version: cfg.Harness.Version,
		},
		connections: make(map[net.Conn]struct{}),
		subscribers: make(map[*subscriber]struct{}),
	}
	store.Put(statusKey, d.status)
	logging.Configure(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: io.MultiWriter(os.Stderr, d)})
	logging.Event(logging.Info, "serve.started", map[string]any{"socket": socket})

	defer func() {
		logging.Event(logging.Info, "serve.stopped", nil)
		d.closeAll()
		_ = listener.Close()
		_ = os.Remove(socket)
		store.Invalidate(statusKey)
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		d.closeAll()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept connection: %w", err)
		}
		d.addConnection(conn)
		go d.handle(conn)
	}
}

// Status is the daemon snapshot returned by a status request.
type Status struct {
	State   string `json:"status"`
	PID     int    `json:"pid"`
	Harness string `json:"harness,omitempty"`
	Version string `json:"version,omitempty"`
}

type request struct {
	Command string `json:"command"`
}

type daemon struct {
	listener *net.UnixListener
	store    *state.Store
	status   Status

	mu          sync.Mutex
	connections map[net.Conn]struct{}
	subscribers map[*subscriber]struct{}
}

type subscriber struct {
	conn   net.Conn
	events chan []byte
	done   chan struct{}
}

func prepareSocket(socket string) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	info, err := os.Lstat(socket)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect socket %q: %w", socket, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace non-socket %q", socket)
	}
	if err := os.Remove(socket); err != nil {
		return fmt.Errorf("remove stale socket %q: %w", socket, err)
	}
	return nil
}

func (d *daemon) addConnection(conn net.Conn) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.connections[conn] = struct{}{}
}

func (d *daemon) handle(conn net.Conn) {
	defer func() {
		d.mu.Lock()
		delete(d.connections, conn)
		d.mu.Unlock()
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var request request
	if err := json.NewDecoder(conn).Decode(&request); err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	switch request.Command {
	case "status":
		_ = json.NewEncoder(conn).Encode(d.currentStatus())
	case "tail":
		subscriber := d.subscribe(conn)
		defer d.drop(subscriber)
		<-subscriber.done
	default:
		_ = json.NewEncoder(conn).Encode(map[string]string{"error": "unknown command"})
	}
}

func (d *daemon) currentStatus() Status {
	if snapshot, ok := d.store.Get(statusKey); ok {
		if status, ok := snapshot.(Status); ok {
			return status
		}
	}
	return d.status
}

func (d *daemon) subscribe(conn net.Conn) *subscriber {
	subscriber := &subscriber{conn: conn, events: make(chan []byte, 16), done: make(chan struct{})}
	d.mu.Lock()
	d.subscribers[subscriber] = struct{}{}
	subscriber.events <- []byte(`{"ready":true}` + "\n")
	d.mu.Unlock()
	go func() {
		for event := range subscriber.events {
			if _, err := subscriber.conn.Write(event); err != nil {
				d.drop(subscriber)
				return
			}
		}
	}()
	return subscriber
}

// Write implements io.Writer for logging. A full subscriber queue is closed
// immediately, so Event callers never wait for socket I/O.
func (d *daemon) Write(event []byte) (int, error) {
	copyEvent := append([]byte(nil), event...)
	d.mu.Lock()
	defer d.mu.Unlock()
	for subscriber := range d.subscribers {
		select {
		case subscriber.events <- copyEvent:
		default:
			d.dropLocked(subscriber)
		}
	}
	return len(event), nil
}

func (d *daemon) drop(subscriber *subscriber) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dropLocked(subscriber)
}

func (d *daemon) dropLocked(subscriber *subscriber) {
	if _, ok := d.subscribers[subscriber]; !ok {
		return
	}
	delete(d.subscribers, subscriber)
	close(subscriber.events)
	close(subscriber.done)
	_ = subscriber.conn.Close()
}

func (d *daemon) closeAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for subscriber := range d.subscribers {
		d.dropLocked(subscriber)
	}
	for conn := range d.connections {
		_ = conn.Close()
	}
}

// Command magicite controls the local magicite daemon.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "serve":
		return serve(ctx, args[1:], stderr)
	case "status":
		return status(args[1:], stdout, stderr)
	case "tail":
		return tail(ctx, args[1:], stdout, stderr)
	default:
		usage(stderr)
		return 2
	}
}

func serve(ctx context.Context, args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	socket := flags.String("socket", defaultSocket(), "Unix socket path")
	configPath := flags.String("config", defaultConfig(), "configuration path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := server.Serve(ctx, *socket, cfg, nil); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}

func status(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	socket := flags.String("socket", defaultSocket(), "Unix socket path")
	asJSON := flags.Bool("json", false, "write JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}

	conn, err := dial(*socket)
	if err != nil {
		return unreachable(stderr, err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(map[string]string{"command": "status"}); err != nil {
		return unreachable(stderr, err)
	}
	var snapshot server.Status
	if err := json.NewDecoder(conn).Decode(&snapshot); err != nil {
		return unreachable(stderr, err)
	}
	if *asJSON {
		encoded, _ := json.Marshal(snapshot)
		_, _ = fmt.Fprintln(stdout, string(encoded))
	} else {
		_, _ = fmt.Fprintln(stdout, snapshot.State)
	}
	return 0
}

func tail(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tail", flag.ContinueOnError)
	flags.SetOutput(stderr)
	socket := flags.String("socket", defaultSocket(), "Unix socket path")
	asJSON := flags.Bool("json", false, "write JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}

	conn, err := dial(*socket)
	if err != nil {
		return unreachable(stderr, err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(map[string]string{"command": "tail"}); err != nil {
		return unreachable(stderr, err)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	reader := bufio.NewScanner(conn)
	for reader.Scan() {
		line := reader.Bytes()
		var ready struct {
			Ready bool `json:"ready"`
		}
		if json.Unmarshal(line, &ready) == nil && ready.Ready {
			continue
		}
		if *asJSON {
			_, _ = stdout.Write(append(append([]byte(nil), line...), '\n'))
			continue
		}
		if _, err := fmt.Fprintln(stdout, renderEvent(line)); err != nil {
			return 1
		}
	}
	if ctx.Err() != nil {
		return 0
	}
	if err := reader.Err(); err != nil {
		return unreachable(stderr, err)
	}
	return 0
}

func dial(socket string) (net.Conn, error) {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func unreachable(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "daemon unreachable: %v\n", err)
	return 1
}

func renderEvent(line []byte) string {
	var event struct {
		Sequence uint64          `json:"sequence"`
		Level    string          `json:"level"`
		Kind     string          `json:"kind"`
		Fields   json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		return string(line)
	}
	return fmt.Sprintf("sequence=%d level=%s kind=%q fields=%s", event.Sequence, event.Level, event.Kind, event.Fields)
}

func defaultSocket() string {
	if socket := os.Getenv("MAGICITE_SOCKET"); socket != "" {
		return socket
	}
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		return filepath.Join(runtime, "magicite.sock")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(".", "magicite.sock")
	}
	return filepath.Join(cache, "magicite", "magicite.sock")
}

func defaultConfig() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "magicite.yaml")
	}
	return filepath.Join(configDir, "magicite", "config.yaml")
}

func usage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "usage: magicite <serve|status|tail> [--socket path] [--json]")
}

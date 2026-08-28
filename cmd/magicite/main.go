// Command magicite controls the local magicite daemon.
package main

import (
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
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/wire"
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
	if _, err := config.Load(*configPath); err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	router := server.NewRouter(logging.Logger{})
	_ = router.Register("status", func(context.Context, json.RawMessage) (any, error) {
		return struct {
			State string `json:"status"`
		}{State: "running"}, nil
	})
	if err := server.Serve(ctx, server.Deps{Router: router, Bus: server.NewBus(1024), Socket: *socket}); err != nil {
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
	response, err := request(*socket, "status", nil)
	if err != nil {
		return unreachable(stderr, err)
	}
	if response.Err != nil {
		return unreachable(stderr, fmt.Errorf("%s", response.Err.Message))
	}
	var snapshot struct {
		State string `json:"status"`
	}
	if err := json.Unmarshal(response.Result, &snapshot); err != nil {
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
	if err := wire.NewEncoder(conn).Encode(wire.Request{Schema: wire.Schema, ID: "tail", Command: "subscribe"}); err != nil {
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
	decoder := wire.NewDecoder(conn)
	for {
		frame, err := decoder.Frame()
		if err != nil {
			if ctx.Err() != nil {
				return 0
			}
			return unreachable(stderr, err)
		}
		if frame.Event == nil {
			continue
		}
		if *asJSON {
			encoded, _ := json.Marshal(frame.Event)
			_, _ = fmt.Fprintln(stdout, string(encoded))
		} else if _, err := fmt.Fprintln(stdout, frame.Event.Kind); err != nil {
			return 1
		}
	}
}

func request(socket, command string, params json.RawMessage) (wire.Response, error) {
	conn, err := dial(socket)
	if err != nil {
		return wire.Response{}, err
	}
	defer conn.Close()
	if err := wire.NewEncoder(conn).Encode(wire.Request{Schema: wire.Schema, ID: "command", Command: command, Params: params}); err != nil {
		return wire.Response{}, err
	}
	frame, err := wire.NewDecoder(conn).Frame()
	if err != nil {
		return wire.Response{}, err
	}
	if frame.Response == nil {
		return wire.Response{}, fmt.Errorf("daemon sent event instead of response")
	}
	return *frame.Response, nil
}

func dial(socket string) (net.Conn, error) {
	return net.DialTimeout("unix", socket, time.Second)
}

func unreachable(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "daemon unreachable: %v\n", err)
	return 1
}

func defaultSocket() string {
	if socket := os.Getenv("MAGICITE_SOCKET"); socket != "" {
		return socket
	}
	return server.SocketPath(config.Config{})
}

func defaultConfig() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "magicite.yaml"
	}
	return filepath.Join(configDir, "magicite", "config.yaml")
}

func usage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "usage: magicite <serve|status|tail> [--socket path] [--json]")
}

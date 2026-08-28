// Package cli implements magicite's command-line interface.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/version"
)

// Env contains the dependencies shared by every command.
type Env struct {
	Client *client.Client
	JSON   bool
	Out    io.Writer
	Err    io.Writer
}

// Command describes one magicite subcommand.
type Command struct {
	Name    string
	Usage   string
	Summary string
	Run     func(ctx context.Context, e *Env, args []string) int
}

var commandTable struct {
	sync.RWMutex
	commands []Command
}

// Register adds c to the command table.
func Register(c Command) {
	commandTable.Lock()
	defer commandTable.Unlock()
	commandTable.commands = append(commandTable.commands, c)
}

// Commands returns a name-sorted copy of the command table.
func Commands() []Command {
	commandTable.RLock()
	commands := append([]Command(nil), commandTable.commands...)
	commandTable.RUnlock()
	sort.Slice(commands, func(i, j int) bool { return commands[i].Name < commands[j].Name })
	return commands
}

// Run parses global options and dispatches one command.
func Run(ctx context.Context, args []string, out, err io.Writer) int {
	if ctx == nil {
		ctx = context.Background()
	}
	flags := flag.NewFlagSet("magicite", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	socket := flags.String("socket", defaultSocket(), "daemon socket path")
	asJSON := flags.Bool("json", false, "write JSON")
	timeout := flags.Duration("timeout", 10*time.Second, "daemon call timeout")
	help := flags.Bool("help", false, "show help")
	showVersion := flags.Bool("version", false, "show version")
	if flags.Parse(args) != nil {
		usage(err)
		return 2
	}
	if *help || flags.NArg() == 0 {
		usage(out)
		return 0
	}
	if *showVersion {
		_, _ = fmt.Fprintf(out, "magicite %s\n", version.Version)
		return 0
	}

	name, commandArgs := flags.Arg(0), flags.Args()[1:]
	for _, command := range Commands() {
		if command.Name != name {
			continue
		}
		env := &Env{Client: client.New(client.Options{Socket: *socket, Timeout: *timeout}), JSON: *asJSON, Out: out, Err: err}
		return command.Run(ctx, env, commandArgs)
	}
	usage(err)
	return 2
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: magicite [--socket path] [--json] [--timeout duration] <command> [args]")
	for _, command := range Commands() {
		_, _ = fmt.Fprintf(w, "  %-12s %s\n", command.Name, command.Summary)
	}
}

func defaultSocket() string {
	if socket := os.Getenv("MAGICITE_SOCKET"); socket != "" {
		return socket
	}
	return server.SocketPath(config.Config{})
}

func init() {
	Register(Command{Name: "serve", Usage: "serve [--socket path] [--config path]", Summary: "run the daemon", Run: serve})
}

func serve(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	socket := flags.String("socket", defaultSocket(), "Unix socket path")
	configPath := flags.String("config", defaultConfig(), "configuration path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return commandUsage(e, "serve")
	}
	if _, err := config.Load(*configPath); err != nil {
		_, _ = fmt.Fprintf(e.Err, "load config: %v\n", err)
		return 1
	}
	router := server.NewRouter(logging.Logger{})
	_ = router.Register("status", func(context.Context, json.RawMessage) (any, error) {
		return struct {
			State string `json:"status"`
		}{State: "running"}, nil
	})
	if err := server.Serve(ctx, server.Deps{Router: router, Bus: server.NewBus(1024), Socket: *socket}); err != nil {
		_, _ = fmt.Fprintf(e.Err, "serve: %v\n", err)
		return 1
	}
	return 0
}

func defaultConfig() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "magicite.yaml"
	}
	return filepath.Join(configDir, "magicite", "config.yaml")
}

func commandUsage(e *Env, name string) int {
	for _, command := range Commands() {
		if command.Name == name {
			_, _ = fmt.Fprintf(e.Err, "usage: magicite %s\n", command.Usage)
			return 2
		}
	}
	return 2
}

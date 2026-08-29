package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/FreezingSnail/magicite/internal/daemon"
)

// RegisterServe adds the daemon serve command to the CLI.
func RegisterServe() {
	Register(Command{Name: "serve", Usage: "serve [--config path]", Summary: "run the daemon", Run: serve})
}

func init() { RegisterServe() }

func serve(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", defaultConfig(), "configuration path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return commandUsage(e, "serve")
	}
	if err := daemon.Run(ctx, *configPath); err != nil {
		_, _ = fmt.Fprintf(e.Err, "serve: %v\n", err)
		return 1
	}
	return 0
}

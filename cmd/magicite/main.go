// Command magicite controls the local magicite daemon.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/FreezingSnail/magicite/internal/cli"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

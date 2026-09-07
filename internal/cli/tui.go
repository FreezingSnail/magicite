package cli

import (
	"context"
	"errors"
	"os"

	"github.com/FreezingSnail/magicite/internal/tui"
	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

func init() {
	Register(Command{Name: "tui", Usage: "tui", Summary: "open read-only fleet dashboard", Run: tuiCommand})
}

var runTUI = func(ctx context.Context, program *tui.Program, e *Env) error {
	return program.Run(ctx, os.Stdin, e.Out)
}

func tuiCommand(ctx context.Context, e *Env, args []string) int {
	if len(args) != 0 || e.JSON {
		return commandUsage(e, "tui")
	}
	if e.Client == nil {
		return Fail(e, errors.New("tui: client is required"))
	}
	program := tui.NewProgram(tui.ProgramOptions{
		API:     transport.NewRPC(e.Client),
		Stream:  transport.NewEventStream(e.Client),
		NoColor: os.Getenv("NO_COLOR") != "",
	})
	if err := runTUI(ctx, program, e); err != nil {
		return Fail(e, err)
	}
	return 0
}

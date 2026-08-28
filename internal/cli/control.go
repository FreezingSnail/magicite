package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"strings"

	"github.com/connorfranc/magicite/internal/wire"
)

// RegisterControl adds lifecycle commands to the CLI.
func RegisterControl() {
	Register(Command{Name: "start", Usage: "start", Summary: "start the daemon", Run: start})
	Register(Command{Name: "stop", Usage: "stop [--hard]", Summary: "stop the daemon", Run: stop})
	Register(Command{Name: "dispatch", Usage: "dispatch TASK [--repo NAME] [--role ROLE]", Summary: "dispatch a task", Run: dispatch})
	Register(Command{Name: "review", Usage: "review EPIC [--repo NAME]", Summary: "review an epic", Run: review})
}

func init() { RegisterControl() }

func start(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return commandUsage(e, "start")
	}

	var raw json.RawMessage
	if err := e.Client.Call(ctx, "start", nil, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "start", raw)
	}
	var result wire.StatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	if err := EmitLine(e.Out, "implementer cap: %d  sessions: %d", result.ImplementerCap, len(result.Sessions)); err != nil {
		return Fail(e, err)
	}
	return 0
}

func stop(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("stop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	hard := flags.Bool("hard", false, "stop immediately")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return commandUsage(e, "stop")
	}

	var raw json.RawMessage
	if err := e.Client.Call(ctx, "stop", wire.StopParams{Hard: *hard}, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "stop", raw)
	}
	var result wire.StopResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	if err := EmitLine(e.Out, "mode: %s  sessions: %d", result.Mode, result.Sessions); err != nil {
		return Fail(e, err)
	}
	return 0
}

func dispatch(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("dispatch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository")
	role := flags.String("role", "", "role")
	flagArgs, positionals, ok := splitControlArgs(args, map[string]bool{"repo": true, "role": true})
	if !ok || flags.Parse(flagArgs) != nil || flags.NArg() != 0 || len(positionals) != 1 {
		return commandUsage(e, "dispatch")
	}

	var raw json.RawMessage
	if err := e.Client.Call(ctx, "dispatch", wire.DispatchParams{Task: positionals[0], Repo: *repo, Role: *role}, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "dispatch", raw)
	}
	var result wire.DispatchResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	if err := EmitLine(e.Out, "seat: %s  role: %s  handle: %s", result.Seat, result.Role, result.Handle); err != nil {
		return Fail(e, err)
	}
	return 0
}

func review(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("review", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository")
	flagArgs, positionals, ok := splitControlArgs(args, map[string]bool{"repo": true})
	if !ok || flags.Parse(flagArgs) != nil || flags.NArg() != 0 || len(positionals) != 1 {
		return commandUsage(e, "review")
	}

	var raw json.RawMessage
	if err := e.Client.Call(ctx, "review", wire.ReviewParams{Epic: positionals[0], Repo: *repo}, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "review", raw)
	}
	var result wire.ReviewResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	if err := EmitLine(e.Out, "epic: %s  held: %t", result.Epic, result.Held); err != nil {
		return Fail(e, err)
	}
	return 0
}

func splitControlArgs(args []string, valueFlags map[string]bool) (flags, positionals []string, ok bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if equal := strings.IndexByte(name, '='); equal >= 0 {
			name = name[:equal]
		}
		if valueFlags[name] && !strings.ContainsRune(arg, '=') {
			if i+1 >= len(args) {
				return nil, nil, false
			}
			i++
			flags = append(flags, args[i])
		}
	}
	return flags, positionals, true
}

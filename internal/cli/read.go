package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func init() {
	Register(Command{Name: "status", Usage: "status", Summary: "show daemon status", Run: status})
	Register(Command{Name: "seats", Usage: "seats", Summary: "list fleet seats", Run: seats})
	Register(Command{Name: "tasks", Usage: "tasks [--repo name] [--all]", Summary: "list dispatchable tasks", Run: tasks})
	Register(Command{Name: "repos", Usage: "repos", Summary: "list configured repositories", Run: repos})
}

func status(ctx context.Context, e *Env, args []string) int {
	if len(args) != 0 {
		return commandUsage(e, "status")
	}
	var raw json.RawMessage
	if err := e.Client.Call(ctx, "status", nil, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "status", raw)
	}
	var result wire.StatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	if err := EmitLine(e.Out, "running: %t  draining: %t  repos: %d  implementer cap: %d", result.Running, result.Draining, result.Repos, result.ImplementerCap); err != nil {
		return Fail(e, err)
	}
	rows := make([][]string, len(result.Sessions))
	for i, session := range result.Sessions {
		rows[i] = []string{session.Handle, session.Repo, session.Task, session.Role, session.Seat, session.Backend, session.Model, session.Status, session.Phase, strconv.FormatInt(session.UptimeSeconds, 10)}
	}
	if err := EmitTable(e.Out, []string{"handle", "repo", "task", "role", "seat", "backend", "model", "status", "phase", "uptime"}, rows); err != nil {
		return Fail(e, err)
	}
	return 0
}

func seats(ctx context.Context, e *Env, args []string) int {
	if len(args) != 0 {
		return commandUsage(e, "seats")
	}
	var raw json.RawMessage
	if err := e.Client.Call(ctx, "seats", nil, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "seats", raw)
	}
	var result []wire.SeatResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	rows := make([][]string, len(result))
	for i, seat := range result {
		rows[i] = []string{seat.Name, seat.Role, seat.Repo, seat.Worktree, seat.Task, strconv.FormatBool(seat.Busy)}
	}
	if err := EmitTable(e.Out, []string{"name", "role", "repo", "worktree", "task", "busy"}, rows); err != nil {
		return Fail(e, err)
	}
	return 0
}

func tasks(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("tasks", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository")
	all := flags.Bool("all", false, "include all tasks")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return commandUsage(e, "tasks")
	}
	var raw json.RawMessage
	if err := e.Client.Call(ctx, "tasks", wire.TasksParams{Repo: *repo, All: *all}, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "tasks", raw)
	}
	var result []wire.TaskResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	rows := make([][]string, len(result))
	for i, task := range result {
		rows[i] = []string{task.ID, task.Repo, task.Title, task.Status, task.Difficulty, strconv.Itoa(task.Priority), strings.Join(task.Labels, ",")}
	}
	if err := EmitTable(e.Out, []string{"id", "repo", "title", "status", "difficulty", "priority", "labels"}, rows); err != nil {
		return Fail(e, err)
	}
	return 0
}

func repos(ctx context.Context, e *Env, args []string) int {
	if len(args) != 0 {
		return commandUsage(e, "repos")
	}
	var raw json.RawMessage
	if err := e.Client.Call(ctx, "repos", nil, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "repos", raw)
	}
	var result []wire.RepoResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	rows := make([][]string, len(result))
	for i, repo := range result {
		rows[i] = []string{repo.Name, repo.Path, repo.Prefix, repo.Branch}
	}
	if err := EmitTable(e.Out, []string{"name", "path", "prefix", "branch"}, rows); err != nil {
		return Fail(e, err)
	}
	return 0
}

func emitPayload(e *Env, kind string, payload json.RawMessage) int {
	if err := EmitJSON(e.Out, kind, payload); err != nil {
		return Fail(e, fmt.Errorf("write %s: %w", kind, err))
	}
	return 0
}

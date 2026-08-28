# magicite

`magicite` is a local agent-runtime daemon for coordinating implementation work across one or more Git repositories. It discovers configured repositories, reads dispatchable `bd` tasks, gives each agent seat an isolated Git worktree, runs an agent backend, and tracks task, session, review, and landing lifecycle events.

## What it can do

- Coordinate named agent seats in roles: concierge, designer, implementer, reviewer, and repairer.
- Run Kiro or OpenCode agent backends with role- or seat-specific model, effort, fallback, and retry settings.
- Discover or explicitly configure repositories; filter them with include/exclude rules.
- Read, claim, release, comment on, and close `bd` tasks; list ready or all tasks.
- Dispatch a task directly to a selected repository and role, or let the daemon schedule available work.
- Create and synchronize an isolated worktree per repository/seat under a configurable workspace path.
- Report daemon status, repository inventory, seats, sessions, and task queues as text or JSON.
- Stream ordered daemon events, resume from an event sequence, and reconnect after transient daemon outages.
- Request review for an epic and coordinate review/repair handoffs.
- Land completed work through a Git landing pipeline with conflict and gate-failure outcomes.

## Prerequisites

- Go toolchain
- Git
- [`bd`](https://github.com/steveyegge/beads) available for each managed repository
- At least one configured agent backend: Kiro or OpenCode

## Build and run

```sh
go build -o ./bin/magicite ./cmd/magicite

# Terminal 1: run the daemon; Ctrl-C stops it.
./bin/magicite serve --config ~/.config/magicite/config.yaml

# Terminal 2: start scheduling and inspect state.
./bin/magicite start
./bin/magicite status
```

The default configuration path is `~/.config/magicite/config.yaml`. A missing configuration file loads built-in defaults; provide a config file before managing real repositories.

## Configuration

Configuration is YAML. This minimal example configures an explicit repository and keeps seat worktrees inside it:

```yaml
repos:
  discover: explicit
  roots:
    - /absolute/path/to/project

workspaces:
  path: harness/workspaces
```

Roles support an agent executable, backend, models, fallback, retry count, polling interval, and named seats. Repository discovery can be controlled with `repos.discover`, `repos.roots`, `repos.include`, and `repos.exclude`. See `internal/config/config.go` for the complete validated schema and built-in role defaults.

## CLI

Global options:

```text
magicite [--socket path] [--json] [--timeout duration] <command> [args]
```

| Command | Purpose |
| --- | --- |
| `serve [--config path]` | Run the daemon and its local socket server. |
| `start` | Start scheduling work. |
| `stop [--hard]` | Drain active work or stop immediately. |
| `status` | Show daemon state and active sessions. |
| `repos` | List configured repositories, roots, prefixes, and branches. |
| `seats` | List configured seats, roles, worktrees, and current assignments. |
| `tasks [--repo name] [--all]` | List dispatchable tasks; `--all` includes non-ready tasks. |
| `dispatch TASK [--repo NAME] [--role ROLE]` | Dispatch one task to a role. |
| `review EPIC [--repo NAME]` | Request review for an epic. |
| `tail [--since SEQ] [--follow] [--reconnect N] [--json]` | Stream daemon events; resume with an event sequence. |

Examples:

```sh
# Inspect configured repositories and ready tasks.
magicite repos
magicite tasks

# Dispatch a task to an implementer in one repository.
magicite dispatch magicite-123 --repo magicite --role implementer

# Follow lifecycle events as JSON.
magicite --json tail --follow

# Stop accepting new work, allowing current sessions to drain.
magicite stop
```

## Operational model

1. Configure repositories, agent roles, and worktree location.
2. Start the daemon, then inspect ready tasks and available seats.
3. Dispatch a task manually or allow scheduler-driven dispatch.
4. Follow sessions and lifecycle events with `status` or `tail`.
5. Review completed epics; review and repair roles coordinate handoffs when enabled.
6. The landing pipeline syncs and evaluates completed seat work before integration.

## Verification

Format tracked Go files and verify formatting:

```sh
make fmt
make fmt-check
```

Run the repository gate:

```sh
make check
```

It verifies formatting, builds every package, runs `go vet`, and executes the test suite with race detection and coverage instrumentation.

## License

[MIT](LICENSE) © 2026 FreezingSnail.

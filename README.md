# magicite

`magicite` is a local agent-runtime daemon and terminal UI for coordinating implementation work across one or more Git repositories. One binary provides the daemon, scheduler controls, read commands, fleet metrics, and Bubble Tea dashboard. The daemon owns repository discovery, `bd` access, isolated seat worktrees, agent sessions, lifecycle events, review, repair, and landing.

## What it can do

- Coordinate named seats in concierge, designer, implementer, reviewer, and repairer roles.
- Run Kiro or OpenCode backends with role- or seat-specific model, effort, fallback, retry, and polling settings.
- Discover explicitly configured repositories or filter discovered repositories with include/exclude rules.
- Read, claim, release, comment on, and close `bd` tasks; dispatch selected work or schedule ready work.
- Create and synchronize one isolated Git worktree per repository/seat under a configurable workspace path.
- Report repository, seat, session, queue, status, and metrics data as text or JSON.
- Stream ordered daemon events with resume and reconnect support.
- Review completed epics, coordinate repair handoffs, and land completed work through the Git gate pipeline.
- Serve a local-socket TUI dashboard with dashboard, beads, seats, repositories, events, and metrics views.

## Prerequisites

- Go toolchain
- Git
- [`bd`](https://github.com/steveyegge/beads) available in each managed repository
- At least one configured agent backend executable: Kiro or OpenCode

## Build and run

Build the single executable:

```sh
go build -o ./bin/magicite ./cmd/magicite
```

Run the daemon, scheduler, and TUI in separate terminals:

```sh
# Terminal 1: long-running daemon and local Unix-socket server.
./bin/magicite serve

# Terminal 2: tell the running daemon to start scheduling.
./bin/magicite start

# Terminal 3: open the dashboard client.
./bin/magicite tui
```

`serve` is the only long-running daemon process. `start` is a client RPC to that daemon; it does not launch a second scheduler process. `tui` is another client of the same socket and requires `serve` to be running. `Ctrl-C` stops `serve`; use `stop` for a coordinated drain.

The `serve --config` default is the OS user config directory. On macOS it is typically `~/Library/Application Support/magicite/config.yaml`:

```sh
./bin/magicite serve --config /path/to/config.yaml
```

A missing or empty config file loads built-in defaults. Configure repository roots and agent backends before managing real repositories.

## Configuration

Configuration is YAML. This minimal example configures an explicit repository and keeps seat worktrees below the repository:

```yaml
repos:
  discover: explicit
  roots:
    - /absolute/path/to/project

workspaces:
  path: harness/workspaces
```

Roles support an agent, backend, models, fallbacks, retry count, polling interval, and named seats. Repository discovery uses `repos.discover`, `repos.roots`, `repos.include`, and `repos.exclude`.

Built-in seat names are:

| Role | Seats |
| --- | --- |
| Concierge | `alexander` |
| Designer | `ramuh` |
| Implementer/fleet | `ifrit`, `shiva`, `titan` |
| Reviewer | `odin` |
| Repairer | `phoenix` |

The built-in harness identity is `maduin`; `maduin` is not itself a worker seat. See `internal/config/config.go` for the complete schema and defaults.

## CLI

Global options:

```text
magicite [--socket path] [--json] [--timeout duration] <command> [args]
```

The default socket can be overridden with `--socket` or `MAGICITE_SOCKET`. `--json` applies to non-interactive commands; `tui` is interactive and does not accept JSON output.

| Command | Purpose |
| --- | --- |
| `serve [--config path]` | Assemble and run the daemon and local socket server. |
| `start` | Start scheduler activity in the running daemon. |
| `stop [--hard]` | Drain active work, or immediately stop and release workers with `--hard`. |
| `status` | Show daemon lifecycle state and active sessions. |
| `seats` | List configured seats, roles, worktrees, assignments, and busy state. |
| `tasks [--repo NAME] [--all]` | List dispatchable tasks; `--all` includes non-ready tasks. |
| `repos` | List configured repositories, paths, prefixes, and branches. |
| `dispatch TASK [--repo NAME] [--role ROLE]` | Dispatch one task directly. |
| `review EPIC [--repo NAME]` | Request review for an epic. |
| `metrics` | Show process-lifetime lifecycle, session, queue, land, role, and bus counters. |
| `tail [--since SEQ] [--follow] [--reconnect N] [--json]` | Stream ordered daemon events. |
| `tui` | Open the interactive fleet dashboard. |

Examples:

```sh
# Inspect the daemon and scheduler queues.
./bin/magicite status
./bin/magicite seats
./bin/magicite tasks
./bin/magicite metrics

# Dispatch one task to a selected role.
./bin/magicite dispatch magicite-123 --repo magicite --role implementer

# Follow lifecycle events as JSON.
./bin/magicite --json tail --follow

# Drain or immediately stop scheduling.
./bin/magicite stop
./bin/magicite stop --hard
```

## TUI controls

The TUI is compiled into the `magicite` binary and communicates through typed local-socket APIs. It does not read repositories, invoke `bd`, or run Git itself.

| Key | Action |
| --- | --- |
| `s` | Send the daemon `start` command and begin scheduling. |
| `d` | Send a graceful drain/stop command. |
| `h`, then `c` | Confirm hard stop; immediately terminate workers and release claims. |
| `r` | Open review for the selected review-eligible item. |

The TUI owns presentation, refresh/reconnect state, bounded event history, and stale/unavailable display. The daemon remains the authority for repository, task, seat, session, and metrics data.

## Operational model

1. Configure repository roots, workspaces, roles, seats, and backend credentials.
2. Run `serve`; it assembles the daemon and listens on the local Unix socket.
3. Run `start`, press TUI `s`, or use another control client to start scheduling.
4. The scheduler claims ready non-epic work for available implementer seats. `bd` dependencies, deferred tasks, and parent gates still control eligibility.
5. Inspect `status`, `seats`, `tasks`, `metrics`, `tail`, or the TUI while sessions run.
6. Completed work is tested, reviewed, repaired when needed, and landed through the Git pipeline.

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

The gate formats/builds packages, runs `go vet`, and executes the race-enabled test suite with coverage instrumentation.

## License

[MIT](LICENSE) © 2026 FreezingSnail.

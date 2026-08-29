# Agent Instructions

Issue tracking is [`bd`](https://github.com/steveyegge/beads). Find work with
`bd ready`, read it with `bd show <id>`, claim it with `bd update <id> --claim`,
and close it with `bd close <id> -r "..."`. The commit message is the record of
the task; write no summary or report files.

## Verification: run the smallest suite that covers your change

**Do not run the full suite unless you are told to.** Run the packages your
change affects, and only those. A full `make check` is for the end of a task, a
gate you were explicitly asked to run, or a change whose blast radius genuinely
spans the tree.

```sh
go test ./internal/<package>/                 # the package you changed
go test ./internal/<package>/ -run TestName   # narrower still, while iterating
go build ./... && go vet ./internal/<package>/
```

Reach for the wider forms deliberately:

```sh
make check     # formatting, build, vet, and the full race + coverage suite
make parity    # the parity packages only
```

Why this is a rule and not a preference: these suites spawn processes, build
fixture git repositories, and run under `-race` with coverage instrumentation.
A full run costs real time, heat, and disk on a laptop, and repeating it after
every edit has previously written gigabytes to no purpose. Iterate narrow,
verify wide once.

When you do need the wide run, prefer one run at the end over several during
development, and say in the commit message what you ran.

## Changing test infrastructure

`internal/testenv` publishes the fake `bd` and agent CLIs by hardlinking the
already-compiled test binary and re-executing it, so a suite performs **zero**
compilations of its own. If you touch that machinery, do not reintroduce a
`go build`, a per-test `GOCACHE`, or anything that recompiles per test
environment. `TestParityBudget` bounds replay cost per catalog invariant and
will fail if that regresses.

## Conventions

- Every subprocess goes through `internal/exec` with argv only, never a shell
  string, always context-bounded.
- Tests are permanent files using the standard library `testing` package. No
  disposable tests, no shell or Python drivers, no network, no live `bd`, `kiro`
  or `opencode`.
- Do not weaken an existing assertion to make a new one pass. If a behavior has
  no honest counterpart, record it in `internal/parity/testdata/divergences.tsv`
  with a reason and an owner bead rather than asserting less.
- Golden files are regenerated only through the documented `UPDATE=1` path, and a
  regenerated golden is reviewed as a behavior change.
- `gofmt` is gated. Run `make fmt` before committing.
- Shell commands must be non-interactive: `rm -f`, `cp -f`, `mv -f`.

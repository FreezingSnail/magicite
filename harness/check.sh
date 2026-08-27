#!/bin/sh
# Repository verification gate.
#
# Maduin's land pipeline invokes <worktree>/harness/check.sh and refuses to land
# on any non-zero exit, so this script is the single entry point that decides
# whether work may reach the integration branch. It delegates to `make check`
# rather than duplicating the build, vet, and test invocations: the Makefile
# stays the one place the gate is defined.
set -eu

# Resolve the repository root from this script's location so the gate behaves
# identically whether it is run from the root, a seat worktree, or any subdirectory.
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd -- "$root"

exec make check

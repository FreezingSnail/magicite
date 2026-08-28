#!/bin/sh
# External daemon acceptance client. Arguments: magicite socket task repo.
set -eu

magicite=$1
socket=$2
task=$3
repo=$4

"$magicite" --socket "$socket" --json start
"$magicite" --socket "$socket" --json dispatch "$task" --repo "$repo" --role implement
events=$("$magicite" --socket "$socket" --json tail --follow=false)
case "$events" in
  *'"kind":"land"'*'"kind":"close"'*) ;;
  *) exit 1 ;;
esac
status=$("$magicite" --socket "$socket" --json status)
case "$status" in
  *'"sessions":[]'*) ;;
  *) exit 1 ;;
esac
"$magicite" --socket "$socket" --json stop

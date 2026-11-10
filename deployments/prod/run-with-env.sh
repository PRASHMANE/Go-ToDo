#!/bin/sh
# Lives at: /usr/local/bin/go-todo-run
# Usage: go-todo-run <env-file> <program> [args...]
#
# Loads KEY=value lines from <env-file> into the environment, then REPLACES
# itself with <program> (exec). Because of exec, supervisord's SIGTERM goes
# straight to the Go binary → our graceful shutdown runs.
set -eu
env_file=$1
shift
set -a        # export every variable defined while sourcing
. "$env_file"
set +a
exec "$@"

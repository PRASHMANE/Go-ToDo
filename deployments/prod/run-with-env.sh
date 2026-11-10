#!/bin/sh
# Usage: run-with-env.sh <env-file> <program> [args...]
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

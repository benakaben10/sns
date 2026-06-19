#!/bin/sh
# Routes to the appropriate entrypoint script based on the first argument.
set -e

ENTRYPOINTS_DIR="${WORK_DIR:-/opt/app}/deploy/entrypoints"

if [ $# -eq 0 ]; then
    exec "${ENTRYPOINTS_DIR}/docker-entrypoint.sh"
else
    COMPONENT="$1"
    shift
    exec "${ENTRYPOINTS_DIR}/docker-entrypoint-${COMPONENT}.sh" "$@"
fi

#!/bin/sh

# Modify the command below if extra flags are needed for the binary.
#   Default: runs the web server + email worker in a single process.
MAIN_PROCESS_SCRIPT="${WORK_DIR:-/opt/app}/notification-service"

exec $MAIN_PROCESS_SCRIPT

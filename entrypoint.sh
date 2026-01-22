#!/bin/bash
set -e

CLAUDE_USER="claude"

# If running as root, switch to claude user
if [ "$(id -u)" = "0" ]; then
    exec gosu ${CLAUDE_USER} "$@"
fi

# If not root, just execute the command
exec "$@"

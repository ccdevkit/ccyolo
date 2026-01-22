#!/bin/bash
set -e

CLAUDE_USER="claude"

# If running as root, switch to claude user
if [ "$(id -u)" = "0" ]; then
    # Set flag so setup runs after switching to claude user
    export CCYOLO_NEEDS_SETUP=1
    exec gosu ${CLAUDE_USER} "$0" "$@"
fi

# Run ccproxy setup to create hijacker scripts if config exists
# Only run once after switching from root to claude user
if [ -n "$CCYOLO_NEEDS_SETUP" ] && [ -f /tmp/ccyolo-proxy.json ]; then
    ccdebug "Running ccproxy --setup"
    ccproxy --setup
    ccdebug "ccproxy setup complete"
fi

ccdebug "Starting: $@"

# Execute the command
exec "$@"

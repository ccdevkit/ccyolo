#!/bin/bash
set -e

CLAUDE_USER="claude"

# If running as root, switch to claude user
if [ "$(id -u)" = "0" ]; then
    # Create X11 socket directory as root (required by Xvfb)
    mkdir -p /tmp/.X11-unix
    chmod 1777 /tmp/.X11-unix

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

# Start Xvfb (virtual X server) for clipboard support
if [ -n "$CCYOLO_CLIP_PORT" ]; then
    ccdebug "Starting Xvfb on display :99"
    Xvfb :99 -screen 0 1024x768x24 2>&1 | ccdebug --prefix xvfb &
    export DISPLAY=:99

    # Start clipboard daemon
    ccdebug "Starting ccclipd on port $CCYOLO_CLIP_PORT"
    ccclipd 2>&1 | ccdebug --prefix ccclipd &
fi

# Execute the command
exec "$@"

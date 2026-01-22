#!/bin/bash
set -e

CLAUDE_USER="claude"

# If running as root, fix permissions and switch to claude user
if [ "$(id -u)" = "0" ]; then
    # Ensure claude user owns necessary directories
    chown -R ${CLAUDE_USER}:${CLAUDE_USER} /home/${CLAUDE_USER}/.claude 2>/dev/null || true

    # Fix workspace ownership
    if [ -d "/workspace" ]; then
        chown ${CLAUDE_USER}:${CLAUDE_USER} /workspace 2>/dev/null || true
    fi

    # Execute command as claude user
    exec gosu ${CLAUDE_USER} "$@"
fi

# If not root, just execute the command
exec "$@"

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccyolo/internal/constants"
)

// GetUniqueCommands extracts unique base commands from the runOnHost patterns
// For example: ["git", "npx ccstatusline"] -> ["git", "npx"]
func GetUniqueCommands(patterns []string) []string {
	seen := make(map[string]bool)
	var commands []string

	for _, pattern := range patterns {
		// Get the first word (base command)
		parts := strings.Fields(pattern)
		if len(parts) == 0 {
			continue
		}
		baseCmd := parts[0]
		if !seen[baseCmd] {
			seen[baseCmd] = true
			commands = append(commands, baseCmd)
		}
	}

	return commands
}

// CreateHijackers creates hijacker scripts for each unique command
func CreateHijackers(commands []string) error {
	// Ensure the hijacker directory exists
	if err := os.MkdirAll(constants.HijackerDir, 0755); err != nil {
		return fmt.Errorf("failed to create hijacker directory: %w", err)
	}

	for _, cmd := range commands {
		if err := createHijacker(cmd); err != nil {
			return fmt.Errorf("failed to create hijacker for %s: %w", cmd, err)
		}
	}

	return nil
}

// createHijacker creates a single hijacker script
func createHijacker(cmd string) error {
	scriptPath := filepath.Join(constants.HijackerDir, cmd)

	// The hijacker script calls ccproxy --exec with the full command
	// It passes through all arguments, properly quoted to preserve spaces/special chars
	script := fmt.Sprintf(`#!/bin/bash
# ccyolo hijacker for %s
# Passes through to ccproxy which decides whether to proxy to host or run locally

# Build command string with properly quoted arguments
args="%s"
for arg in "$@"; do
    # Escape single quotes and wrap each arg in single quotes
    escaped=$(printf "%%s" "$arg" | sed "s/'/'\\\\''/g")
    args="$args '$escaped'"
done
exec ccproxy --exec "$args"
`, cmd, cmd)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	return nil
}

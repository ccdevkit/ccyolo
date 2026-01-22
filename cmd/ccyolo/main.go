package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"ccyolo/internal/claude"
	"ccyolo/internal/docker"
	"ccyolo/internal/hostexec"
	"ccyolo/internal/session"
)

var verbose bool

func debug(format string, args ...any) {
	if verbose {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func main() {
	flag.BoolVar(&verbose, "v", false, "Enable verbose debug logging")
	flag.Parse()

	debug("ccyolo starting")

	// Create session for this run
	sess, err := session.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cleanup()
	debug("Session created: %s", sess.ID())
	debug("Session temp dir: %s", sess.TempDir())

	// Get home and working directories for the hostexec server
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Start the TCP server for host-side command execution
	server, err := hostexec.Start(homeDir, cwd, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting hostexec server: %v\n", err)
		os.Exit(1)
	}
	defer server.Stop()
	debug("TCP server started on port %d", server.Port())

	// Write the settings file for the container with the server port
	if err := sess.WriteSettings(server.Port()); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing settings: %v\n", err)
		os.Exit(1)
	}
	debug("Settings written to: %s", sess.SettingsPath())

	token, err := claude.CaptureToken(debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	spec, err := claude.GetContainerSpec(token, sess.ID(), sess.SettingsPath(), homeDir, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating container spec: %v\n", err)
		os.Exit(1)
	}

	if err := docker.RunSpec(spec, debug); err != nil {
		if exitErr, ok := err.(*docker.ExitError); ok {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

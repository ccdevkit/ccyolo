package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"ccyolo/internal/args"
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

// extractCcyoloFlags separates --y:* and -y:* args from claude args
// Returns (ccyolo args with prefix stripped, claude args)
func extractCcyoloFlags(args []string) (ccyoloArgs []string, claudeArgs []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--y:") {
			ccyoloArgs = append(ccyoloArgs, "--"+strings.TrimPrefix(arg, "--y:"))
		} else if strings.HasPrefix(arg, "-y:") {
			ccyoloArgs = append(ccyoloArgs, "-"+strings.TrimPrefix(arg, "-y:"))
		} else {
			claudeArgs = append(claudeArgs, arg)
		}
	}
	return
}

// parseCcyoloFlags parses ccyolo-specific flags using the flag package
func parseCcyoloFlags(args []string) {
	fs := flag.NewFlagSet("ccyolo", flag.ExitOnError)
	fs.BoolVar(&verbose, "v", false, "Enable verbose debug logging")
	fs.BoolVar(&verbose, "verbose", false, "Enable verbose debug logging")
	fs.Parse(args)
}

func main() {
	// Extract --ccy:* flags for ccyolo, pass the rest to claude
	ccyoloArgs, remainingArgs := extractCcyoloFlags(os.Args[1:])
	parseCcyoloFlags(ccyoloArgs)

	debug("ccyolo starting")
	debug("Remaining args: %v", remainingArgs)

	// Process args: extract session-id, find paths to bind
	processed, err := args.Process(remainingArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing arguments: %v\n", err)
		os.Exit(1)
	}
	debug("Processed args: SessionID=%s, PassArgs=%v, ExtraMounts=%d", processed.SessionID, processed.PassArgs, len(processed.ExtraMounts))

	// Use provided session ID or create new session
	var sess *session.Session
	if processed.SessionID != "" {
		debug("Using provided session ID: %s", processed.SessionID)
		sess, err = session.WithID(processed.SessionID)
	} else {
		sess, err = session.New()
	}
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

	spec, err := claude.GetContainerSpec(token, sess.ID(), sess.SettingsPath(), homeDir, cwd, processed.PassArgs, processed.ExtraMounts)
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

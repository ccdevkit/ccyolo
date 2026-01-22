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
var logFile string
var passthrough []string

// stringSliceFlag allows a flag to be specified multiple times
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var logger *log.Logger

func initLogger() (*os.File, error) {
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger = log.New(f, "", log.LstdFlags)
		return f, nil
	}
	logger = log.New(os.Stderr, "", log.LstdFlags)
	return nil, nil
}

func debug(format string, args ...any) {
	if verbose {
		logger.Printf("[DEBUG] "+format, args...)
	}
}

// ccyoloFlagsWithValues lists ccyolo flags that take a value argument
var ccyoloFlagsWithValues = map[string]bool{
	"pt":          true,
	"passthrough": true,
	"log":         true,
}

// extractCcyoloFlags separates --y:* and -y:* args from claude args
// Returns (ccyolo args with prefix stripped, claude args)
func extractCcyoloFlags(args []string) (ccyoloArgs []string, claudeArgs []string) {
	expectValue := false
	for _, arg := range args {
		if expectValue {
			// This arg is a value for a previous ccyolo flag
			ccyoloArgs = append(ccyoloArgs, arg)
			expectValue = false
			continue
		}

		if strings.HasPrefix(arg, "--y:") {
			stripped := "--" + strings.TrimPrefix(arg, "--y:")
			ccyoloArgs = append(ccyoloArgs, stripped)
			// Check if this flag expects a value and doesn't have = in it
			if !strings.Contains(stripped, "=") {
				flagName := strings.TrimPrefix(stripped, "--")
				if ccyoloFlagsWithValues[flagName] {
					expectValue = true
				}
			}
		} else if strings.HasPrefix(arg, "-y:") {
			stripped := "-" + strings.TrimPrefix(arg, "-y:")
			ccyoloArgs = append(ccyoloArgs, stripped)
			// Check if this flag expects a value and doesn't have = in it
			if !strings.Contains(stripped, "=") {
				flagName := strings.TrimPrefix(stripped, "-")
				if ccyoloFlagsWithValues[flagName] {
					expectValue = true
				}
			}
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
	fs.StringVar(&logFile, "log", "", "Path to log file (when set with -v, logs go to file instead of stdout)")
	fs.Var((*stringSliceFlag)(&passthrough), "pt", "Command prefix to pass through to host (can be repeated)")
	fs.Var((*stringSliceFlag)(&passthrough), "passthrough", "Command prefix to pass through to host (can be repeated)")
	fs.Parse(args)
}

func main() {
	// Extract --ccy:* flags for ccyolo, pass the rest to claude
	ccyoloArgs, remainingArgs := extractCcyoloFlags(os.Args[1:])
	parseCcyoloFlags(ccyoloArgs)

	// Initialize logger
	logFileHandle, err := initLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}

	debug("ccyolo starting")
	debug("Remaining args: %v", remainingArgs)
	debug("Passthrough: %v", passthrough)
	if logFile != "" {
		debug("Logging to: %s", logFile)
	}

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

	// Write proxy config and system prompt if we have passthrough patterns
	proxyConfigPath := ""
	systemPromptPath := ""
	if len(passthrough) > 0 {
		if err := sess.WriteProxyConfig(server.Port(), passthrough, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing proxy config: %v\n", err)
			os.Exit(1)
		}
		proxyConfigPath = sess.ProxyConfigPath()
		debug("Proxy config written to: %s", proxyConfigPath)

		if err := sess.WriteSystemPrompt(passthrough); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing system prompt: %v\n", err)
			os.Exit(1)
		}
		systemPromptPath = sess.SystemPromptPath()
		debug("System prompt written to: %s", systemPromptPath)
	}

	token, err := claude.CaptureToken(debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	spec, err := claude.GetContainerSpec(token, sess.ID(), sess.SettingsPath(), proxyConfigPath, systemPromptPath, homeDir, cwd, processed.PassArgs, processed.ExtraMounts)
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

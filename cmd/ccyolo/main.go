package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"ccyolo/internal/args"
	"ccyolo/internal/claude"
	"ccyolo/internal/clipboard"
	"ccyolo/internal/docker"
	"ccyolo/internal/hostexec"
	"ccyolo/internal/session"
	"ccyolo/internal/stdin"
)

// Version is set at build time via ldflags
var Version = "dev"

var verbose bool
var logFile string
var passthrough []string

const (
	clipboardPort          = "9999"
	containerBridgeDir     = "/home/claude/.ccyolo-bridge"
)

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
		logger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
		return f, nil
	}
	logger = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	return nil, nil
}

func printHelp() {
	fmt.Print(`ccyolo - Run Claude Code in a Docker container

Usage:
  ccyolo [ccyolo-flags] -- [claude-args]
  ccyolo [claude-args]

If "--" is present, arguments before it are for ccyolo, after are for claude.
If "--" is absent, all arguments are passed to claude.

To see claude's help: ccyolo -- --help

ccyolo flags:
  -v, --verbose         Enable debug logging to stderr
  --log <path>          Write debug logs to file (implies -v)
  --passthrough <cmd>   Run commands matching prefix on host (repeatable)
  -pt <cmd>             Short for --passthrough
  --version             Print ccyolo version

Examples:
  ccyolo                          Start claude interactively
  ccyolo -p "hello"               Pass prompt to claude
  ccyolo -v -- -p "hello"         Debug mode with prompt
  ccyolo --log /tmp/debug.log --  Log to file
  ccyolo --pt git -- -p "status"  Run git commands on host
`)
}

// splitArgs splits arguments at "--" separator.
// Returns (ccyoloArgs, claudeArgs).
// If "--" is not present, all args go to claude.
func splitArgs(args []string) (ccyoloArgs []string, claudeArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	// No "--" found, all args go to claude
	return nil, args
}

func debug(format string, args ...any) {
	if verbose {
		logger.Printf("[DEBUG] "+format, args...)
	}
}

// parseCcyoloFlags parses ccyolo-specific flags using the flag package.
// Returns true if the program should exit (e.g., --help was shown).
func parseCcyoloFlags(args []string) bool {
	fs := flag.NewFlagSet("ccyolo", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage, we handle --help ourselves

	var showHelp bool
	var showVersion bool
	fs.BoolVar(&showHelp, "help", false, "")
	fs.BoolVar(&showHelp, "h", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&verbose, "v", false, "Enable verbose debug logging")
	fs.BoolVar(&verbose, "verbose", false, "Enable verbose debug logging")
	fs.StringVar(&logFile, "log", "", "Path to log file (when set with -v, logs go to file instead of stdout)")
	fs.Var((*stringSliceFlag)(&passthrough), "pt", "Command prefix to pass through to host (can be repeated)")
	fs.Var((*stringSliceFlag)(&passthrough), "passthrough", "Command prefix to pass through to host (can be repeated)")

	if err := fs.Parse(args); err != nil {
		// Unknown flag - print help and exit
		printHelp()
		os.Exit(1)
	}

	if showHelp {
		printHelp()
		return true
	}

	if showVersion {
		fmt.Printf("ccyolo %s\n", Version)
		return true
	}

	// If --log is set, enable verbose mode
	if logFile != "" {
		verbose = true
	}

	return false
}

func main() {
	// Set version in claude package
	claude.Version = Version

	// Check for --help before splitting (special case: no -- required)
	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-h" {
			printHelp()
			os.Exit(0)
		}
		// Stop at -- to avoid catching claude's --help
		if arg == "--" {
			break
		}
	}

	// Split args at "--": before goes to ccyolo, after goes to claude
	ccyoloArgs, remainingArgs := splitArgs(os.Args[1:])
	if parseCcyoloFlags(ccyoloArgs) {
		os.Exit(0)
	}

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
	debug("Processed args: PassArgs=%v, ExtraMounts=%d", processed.PassArgs, len(processed.ExtraMounts))

	// Get cwd early
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Create our temp session (always with a new UUID for our files)
	// Note: We never pass --session-id to Claude - it manages its own sessions.
	// Our temp dir UUID is just for organizing ccyolo's ephemeral files.
	sess, err := session.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cleanup()
	debug("Temp session created: %s", sess.ID())
	debug("Session temp dir: %s", sess.TempDir())

	// Get home directory for the hostexec server
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
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

	// Always write proxy config (needed for ccdebug even without passthrough)
	if err := sess.WriteProxyConfig(server.Port(), passthrough, verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing proxy config: %v\n", err)
		os.Exit(1)
	}
	proxyConfigPath := sess.ProxyConfigPath()
	debug("Proxy config written to: %s", proxyConfigPath)

	// Write system prompt only if we have passthrough patterns
	systemPromptPath := ""
	if len(passthrough) > 0 {
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

	// Set up clipboard and image drag-drop support
	var stdinReader io.Reader // nil means use os.Stdin directly in RunSpec
	bridgeDir := ""
	clipboardEnabled := false

	// Set clipboard debug function before init
	clipboard.SetDebug(debug)

	// Initialize clipboard library
	if err := clipboard.Init(); err != nil {
		debug("Warning: Failed to initialize clipboard: %v", err)
	} else {
		clipboardEnabled = true
		debug("Clipboard initialized: %s", clipboard.PlatformInfo())
	}

	// Create bridge directory for file drag-drop
	bridgeDir = filepath.Join(homeDir, ".ccyolo-bridge")
	if err := os.MkdirAll(bridgeDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create bridge directory: %v\n", err)
	}
	debug("Bridge directory: %s", bridgeDir)

	// Create clipboard syncer (will connect to container daemon) - only if clipboard works
	var clipboardSyncer stdin.ClipboardSyncer
	if clipboardEnabled {
		clipboardSyncer = stdin.NewTCPClipboardSyncer("localhost:"+clipboardPort, debug)
	}

	// Create stdin interceptor (always enabled for file drag-drop, clipboard optional)
	stdinReader = stdin.NewInterceptor(os.Stdin, clipboardSyncer, bridgeDir, containerBridgeDir, debug)
	debug("Stdin interceptor created")

	spec, err := claude.GetContainerSpec(token, sess.SettingsPath(), proxyConfigPath, systemPromptPath, homeDir, cwd, processed.PassArgs, processed.ExtraMounts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating container spec: %v\n", err)
		os.Exit(1)
	}

	// Add clipboard and bridge configuration
	// Add clipboard port env var
	spec.Env = append(spec.Env, docker.EnvVar{Name: "CCYOLO_CLIP_PORT", Value: clipboardPort})
	// Add DISPLAY for xclip
	spec.Env = append(spec.Env, docker.EnvVar{Name: "DISPLAY", Value: ":99"})
	// Add port mapping for clipboard daemon
	spec.Ports = append(spec.Ports, docker.PortMapping{
		Host:      clipboardPort,
		Container: clipboardPort,
	})
	// Add bridge directory mount
	if bridgeDir != "" {
		spec.Mounts = append(spec.Mounts, docker.Mount{
			Host:      bridgeDir,
			Container: containerBridgeDir,
			ReadOnly:  false,
			CreateDir: true,
		})
	}
	debug("Added clipboard environment, port mapping, and mounts")

	if err := docker.RunSpec(spec, stdinReader, debug); err != nil {
		if exitErr, ok := err.(*docker.ExitError); ok {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

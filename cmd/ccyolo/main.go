package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ccyolo/internal/args"
	"ccyolo/internal/claude"
	"ccyolo/internal/clipboard"
	"ccyolo/internal/constants"
	"ccyolo/internal/docker"
	"ccyolo/internal/hostexec"
	"ccyolo/internal/image"
	"ccyolo/internal/session"
	"ccyolo/internal/settings"
	"ccyolo/internal/stdin"
)

// Version is set at build time via ldflags
var Version = "dev"

// Config holds ccyolo configuration parsed from command-line arguments
type Config struct {
	Verbose      bool
	LogFile      string
	ClaudePath   string
	ClaudeVersion string
	Passthrough  []string
	ClaudeArgs   []string
}

var logger *log.Logger

func initLogger(cfg *Config) (*os.File, error) {
	if !cfg.Verbose {
		// No logging if not in verbose mode
		return nil, nil
	}
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
  -v, --verbose           Enable debug logging to stderr
  --log <path>            Write debug logs to file (implies -v)
  -c, --claudePath <path> Path to claude CLI (default: claude in PATH)
  --use <version>         Use specific Claude Code version (e.g., 2.1.16)
  -pt:<cmd>               Run commands matching prefix on host (repeatable)
  --passthrough:<cmd>     Long form of -pt:<cmd>
  --version               Print ccyolo version

Settings files (.ccdevkit/ccyolo/settings.{json,yaml,yml}) are loaded from
cwd up to root, with closer files taking precedence. CLI flags override file
settings. Passthrough arrays are merged (file + CLI).

Examples:
  ccyolo                              Start claude interactively
  ccyolo -p "hello"                   Pass prompt to claude
  ccyolo -v -- -p "hello"             Debug mode with prompt
  ccyolo --log /tmp/debug.log --      Log to file
  ccyolo -pt:git -- -p "status"       Run git commands on host
  ccyolo -pt:git -pt:docker -- -c     Multiple passthroughs
  ccyolo --use 2.1.16 --              Use specific Claude Code version
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
	if logger != nil {
		logger.Printf("[DEBUG] "+format, args...)
	}
}

// extractPassthroughArgs extracts -pt:<cmd> and --passthrough:<cmd> args,
// returning the passthrough commands and remaining args for the flag package.
func extractPassthroughArgs(args []string) (passthrough []string, remaining []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-pt:") {
			cmd := strings.TrimPrefix(arg, "-pt:")
			if cmd != "" {
				passthrough = append(passthrough, cmd)
			}
		} else if strings.HasPrefix(arg, "--passthrough:") {
			cmd := strings.TrimPrefix(arg, "--passthrough:")
			if cmd != "" {
				passthrough = append(passthrough, cmd)
			}
		} else {
			remaining = append(remaining, arg)
		}
	}
	return passthrough, remaining
}

// ParseConfig parses command-line arguments and returns a Config.
// Returns (nil, nil) if program should exit normally (e.g., --help shown).
// Returns (nil, error) if parsing failed.
func ParseConfig(osArgs []string) (*Config, error) {
	// Check for --help before splitting (special case: no -- required)
	for _, arg := range osArgs {
		if arg == "--help" || arg == "-h" {
			printHelp()
			return nil, nil
		}
		// Stop at -- to avoid catching claude's --help
		if arg == "--" {
			break
		}
	}

	// Load settings from filesystem first (lowest precedence)
	fileSettings, err := settings.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	// Split args at "--": before goes to ccyolo, after goes to claude
	ccyoloArgs, claudeArgs := splitArgs(osArgs)

	// Extract passthrough args (colon syntax not supported by flag package)
	cliPassthrough, ccyoloArgs := extractPassthroughArgs(ccyoloArgs)

	cfg := &Config{
		ClaudeArgs: claudeArgs,
		// Start with file settings passthrough, CLI will be appended later
		Passthrough: fileSettings.Passthrough,
	}

	fs := flag.NewFlagSet("ccyolo", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage, we handle --help ourselves

	var showHelp bool
	var showVersion bool
	fs.BoolVar(&showHelp, "help", false, "")
	fs.BoolVar(&showHelp, "h", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&cfg.Verbose, "v", false, "Enable verbose debug logging")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose debug logging")
	fs.StringVar(&cfg.LogFile, "log", "", "Path to log file (when set with -v, logs go to file instead of stdout)")
	fs.StringVar(&cfg.ClaudePath, "c", "", "Path to claude CLI")
	fs.StringVar(&cfg.ClaudePath, "claudePath", "", "Path to claude CLI")
	fs.StringVar(&cfg.ClaudeVersion, "use", "", "Use specific Claude Code version")

	if err := fs.Parse(ccyoloArgs); err != nil {
		printHelp()
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	if showHelp {
		printHelp()
		return nil, nil
	}

	if showVersion {
		fmt.Printf("ccyolo %s\n", Version)
		return nil, nil
	}

	// If --log is set, enable verbose mode
	if cfg.LogFile != "" {
		cfg.Verbose = true
	}

	// Append CLI passthrough to file passthrough
	cfg.Passthrough = append(cfg.Passthrough, cliPassthrough...)

	// Apply defaults: CLI flag > file setting > hardcoded default
	if cfg.ClaudePath == "" {
		cfg.ClaudePath = fileSettings.ClaudePath
	}
	if cfg.ClaudePath == "" {
		cfg.ClaudePath = "claude"
	}

	return cfg, nil
}

// findFreePort finds an available TCP port by binding to port 0
func findFreePort() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("%d", port), nil
}

// setupClipboard initializes clipboard support and returns the stdin reader,
// bridge directory, clipboard port, and any errors.
func setupClipboard(homeDir string) (stdinReader io.Reader, bridgeDir string, clipboardPort string, err error) {
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
	bridgeDir = filepath.Join(homeDir, constants.BridgeDirName)
	if err := os.MkdirAll(bridgeDir, 0755); err != nil {
		return nil, "", "", fmt.Errorf("failed to create bridge directory: %w", err)
	}
	debug("Bridge directory: %s", bridgeDir)

	// Find a free port for clipboard daemon
	clipboardPort, err = findFreePort()
	if err != nil {
		debug("Warning: Failed to find free port for clipboard: %v", err)
		clipboardEnabled = false
	} else {
		debug("Clipboard port: %s", clipboardPort)
	}

	// Create clipboard syncer (will connect to container daemon) - only if clipboard works
	var clipboardSyncer stdin.ClipboardSyncer
	if clipboardEnabled {
		clipboardSyncer = stdin.NewTCPClipboardSyncer("localhost:"+clipboardPort, debug)
	}

	// Create stdin interceptor (always enabled for file drag-drop, clipboard optional)
	stdinReader = stdin.NewInterceptor(os.Stdin, clipboardSyncer, bridgeDir, constants.ContainerBridgeDir, debug)
	debug("Stdin interceptor created")

	return stdinReader, bridgeDir, clipboardPort, nil
}

// writeSessionFiles writes settings, proxy config, and system prompt files.
// Returns the proxy config path and system prompt path.
func writeSessionFiles(sess *session.Session, serverPort int, passthrough []string, verbose bool) (proxyConfigPath, systemPromptPath string, err error) {
	// Write the settings file for the container with the server port
	if err := sess.WriteSettings(serverPort); err != nil {
		return "", "", fmt.Errorf("failed to write settings: %w", err)
	}
	debug("Settings written to: %s", sess.SettingsPath())

	// Always write proxy config (needed for ccdebug even without passthrough)
	if err := sess.WriteProxyConfig(serverPort, passthrough, verbose); err != nil {
		return "", "", fmt.Errorf("failed to write proxy config: %w", err)
	}
	proxyConfigPath = sess.ProxyConfigPath()
	debug("Proxy config written to: %s", proxyConfigPath)

	// Write system prompt only if we have passthrough patterns
	if len(passthrough) > 0 {
		if err := sess.WriteSystemPrompt(passthrough); err != nil {
			return "", "", fmt.Errorf("failed to write system prompt: %w", err)
		}
		systemPromptPath = sess.SystemPromptPath()
		debug("System prompt written to: %s", systemPromptPath)
	}

	return proxyConfigPath, systemPromptPath, nil
}

// buildContainerSpec creates the Docker container specification.
func buildContainerSpec(token, settingsPath, proxyConfigPath, systemPromptPath, homeDir, cwd string,
	passArgs []string, extraMounts []docker.Mount, bridgeDir, clipboardPort string) (docker.ContainerSpec, error) {

	spec, err := claude.GetContainerSpec(token, settingsPath, proxyConfigPath, systemPromptPath, homeDir, cwd, passArgs, extraMounts)
	if err != nil {
		return docker.ContainerSpec{}, fmt.Errorf("failed to create container spec: %w", err)
	}

	// Forward terminal env vars for proper color support
	if term := os.Getenv(constants.EnvTerm); term != "" {
		spec.Env = append(spec.Env, docker.EnvVar{Name: constants.EnvTerm, Value: term})
	}
	if colorterm := os.Getenv(constants.EnvColorTerm); colorterm != "" {
		spec.Env = append(spec.Env, docker.EnvVar{Name: constants.EnvColorTerm, Value: colorterm})
	}

	// Add clipboard and bridge configuration
	spec.Env = append(spec.Env, docker.EnvVar{Name: constants.EnvClipboardPort, Value: clipboardPort})
	spec.Env = append(spec.Env, docker.EnvVar{Name: constants.EnvDisplay, Value: constants.DefaultXDisplay})

	// Add port mapping for clipboard daemon
	spec.Ports = append(spec.Ports, docker.PortMapping{
		Host:      clipboardPort,
		Container: clipboardPort,
	})

	// Add bridge directory mount
	if bridgeDir != "" {
		spec.Mounts = append(spec.Mounts, docker.Mount{
			Host:      bridgeDir,
			Container: constants.ContainerBridgeDir,
			ReadOnly:  false,
			CreateDir: true,
		})
	}
	debug("Added clipboard environment, port mapping, and mounts")

	return spec, nil
}

// runUpdate handles the "update" command by running it on the host,
// then rebuilding the local image if needed.
func runUpdate(cfg *Config) error {
	debug("Intercepted update command, running on host")

	// Run claude update on host with passthrough stdin/stdout/stderr
	cmd := exec.Command(cfg.ClaudePath, "update")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &docker.ExitError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("failed to run claude update: %w", err)
	}

	// Get the (potentially new) claude version
	claudeVersion, err := image.GetClaudeVersion(cfg.ClaudePath, debug)
	if err != nil {
		return fmt.Errorf("failed to get claude version: %w", err)
	}
	debug("Claude version after update: %s", claudeVersion)

	// Ensure local image exists for this version (build if needed)
	localImageName, err := image.EnsureLocalImage(Version, claudeVersion, debug)
	if err != nil {
		return fmt.Errorf("failed to ensure local image: %w", err)
	}
	debug("Local image ready: %s", localImageName)

	return nil
}

func run() error {
	cfg, err := ParseConfig(os.Args[1:])
	if err != nil {
		return err
	}
	if cfg == nil {
		// Normal exit (--help or --version shown)
		return nil
	}

	// Set version in claude package
	claude.Version = Version

	// Initialize logger
	logFileHandle, err := initLogger(cfg)
	if err != nil {
		return err
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}

	debug("ccyolo starting")
	debug("Claude args: %v", cfg.ClaudeArgs)
	debug("Passthrough: %q", cfg.Passthrough)
	if cfg.LogFile != "" {
		debug("Logging to: %s", cfg.LogFile)
	}

	// Intercept "update" command - run on host instead of container
	if len(cfg.ClaudeArgs) > 0 && cfg.ClaudeArgs[0] == "update" {
		return runUpdate(cfg)
	}

	// Process args: extract session-id, find paths to bind
	processed, err := args.Process(cfg.ClaudeArgs)
	if err != nil {
		return fmt.Errorf("failed to process arguments: %w", err)
	}
	debug("Processed args: PassArgs=%v, ExtraMounts=%d", processed.PassArgs, len(processed.ExtraMounts))

	// Get cwd and home directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create temp session for ccyolo's ephemeral files
	sess, err := session.New()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer sess.Cleanup()
	debug("Temp session created: %s", sess.ID())
	debug("Session temp dir: %s", sess.TempDir())

	// Start the TCP server for host-side command execution
	server, err := hostexec.Start(homeDir, cwd, debug)
	if err != nil {
		return fmt.Errorf("failed to start hostexec server: %w", err)
	}
	defer server.Stop()
	debug("TCP server started on port %d", server.Port())

	// Write session files (settings, proxy config, system prompt)
	proxyConfigPath, systemPromptPath, err := writeSessionFiles(sess, server.Port(), cfg.Passthrough, cfg.Verbose)
	if err != nil {
		return err
	}

	// Capture OAuth token (always needed)
	// Detect Claude version in parallel unless --use was specified
	type tokenResult struct {
		token string
		err   error
	}
	type versionResult struct {
		version string
		err     error
	}
	tokenCh := make(chan tokenResult, 1)
	versionCh := make(chan versionResult, 1)

	go func() {
		token, err := claude.CaptureToken(cfg.ClaudePath, debug)
		tokenCh <- tokenResult{token, err}
	}()

	// If --use was specified, use that version; otherwise detect from host
	if cfg.ClaudeVersion != "" {
		debug("Using specified Claude version: %s", cfg.ClaudeVersion)
		versionCh <- versionResult{cfg.ClaudeVersion, nil}
	} else {
		go func() {
			version, err := image.GetClaudeVersion(cfg.ClaudePath, debug)
			versionCh <- versionResult{version, err}
		}()
	}

	// Wait for both to complete
	tokenRes := <-tokenCh
	versionRes := <-versionCh

	if tokenRes.err != nil {
		return tokenRes.err
	}
	token := tokenRes.token

	if versionRes.err != nil {
		return versionRes.err
	}
	claudeVersion := versionRes.version
	debug("Claude version: %s, Base version: %s", claudeVersion, Version)

	// Ensure local image exists (build if necessary)
	localImageName, err := image.EnsureLocalImage(Version, claudeVersion, debug)
	if err != nil {
		return fmt.Errorf("failed to ensure local image: %w", err)
	}
	claude.LocalImageName = localImageName
	debug("Using local image: %s", localImageName)

	// Set up clipboard and stdin interceptor
	stdinReader, bridgeDir, clipboardPort, err := setupClipboard(homeDir)
	if err != nil {
		return err
	}

	// Build container spec
	spec, err := buildContainerSpec(token, sess.SettingsPath(), proxyConfigPath, systemPromptPath,
		homeDir, cwd, processed.PassArgs, processed.ExtraMounts, bridgeDir, clipboardPort)
	if err != nil {
		return err
	}

	return docker.RunSpec(spec, stdinReader, debug)
}

func main() {
	if err := run(); err != nil {
		if exitErr, ok := err.(*docker.ExitError); ok {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

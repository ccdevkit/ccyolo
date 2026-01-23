package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ccyolo/cmd/ccproxy/matcher"
	"ccyolo/internal/constants"
)

func main() {
	setupMode := flag.Bool("setup", false, "Setup hijacker scripts based on config")
	execCmd := flag.String("exec", "", "Execute a command (proxy or local)")
	flag.Parse()

	if *setupMode {
		if err := runSetup(); err != nil {
			fmt.Fprintf(os.Stderr, "ccproxy setup failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *execCmd != "" {
		exitCode := runExec(*execCmd)
		os.Exit(exitCode)
	}

	fmt.Fprintln(os.Stderr, "ccproxy: use --setup or --exec")
	os.Exit(1)
}

// runSetup reads config and creates hijacker scripts
func runSetup() error {
	config, err := LoadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			// No config file means no proxying needed
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger for setup phase
	InitLogger(config.HostAddress, config.Verbose)

	if len(config.Passthrough) == 0 {
		// No commands to proxy
		Debug("No passthrough commands configured")
		return nil
	}

	// Get unique base commands to hijack
	commands := GetUniqueCommands(config.Passthrough)
	Debug("Setting up hijackers for commands: %s", strings.Join(commands, ", "))

	// Create hijacker scripts
	if err := CreateHijackers(commands); err != nil {
		return err
	}

	Debug("Hijacker setup complete")
	return nil
}

// runExec executes a command, either proxying to host or running locally
func runExec(fullCommand string) int {
	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: failed to load config: %v\n", err)
		return 1
	}

	// Initialize logger
	InitLogger(config.HostAddress, config.Verbose)

	// Check if this command should be proxied
	shouldProxy := false
	matchedPattern := ""
	for _, pattern := range config.Passthrough {
		m := matcher.NewPrefixMatcher(pattern)
		if m.Matches(fullCommand) {
			shouldProxy = true
			matchedPattern = pattern
			break
		}
	}

	if shouldProxy {
		Debug("Command matches pattern '%s', proxying to host: %s", matchedPattern, fullCommand)
		return ProxyToHost(config.HostAddress, fullCommand)
	}

	Debug("Command does not match any passthrough pattern, running locally: %s", fullCommand)
	// Run locally - find the real binary (skipping our hijacker)
	return runLocal(fullCommand)
}

// runLocal executes a command locally, finding the real binary
func runLocal(fullCommand string) int {
	parts := strings.Fields(fullCommand)
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "ccproxy: empty command")
		return 1
	}

	cmdName := parts[0]
	args := parts[1:]

	// Find the real binary by searching PATH, skipping our hijacker directory
	realPath := findRealBinary(cmdName)
	if realPath == "" {
		fmt.Fprintf(os.Stderr, "ccproxy: command not found: %s\n", cmdName)
		return 127
	}

	cmd := exec.Command(realPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "ccproxy: failed to run %s: %v\n", cmdName, err)
		return 1
	}

	return 0
}

// findRealBinary searches PATH for the binary, skipping our hijacker directory
func findRealBinary(name string) string {
	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)

	for _, dir := range paths {
		// Skip our hijacker directory
		if dir == constants.HijackerDir {
			continue
		}

		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			// Check if executable
			if info.Mode()&0111 != 0 {
				return candidate
			}
		}
	}

	return ""
}

package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// DebugFunc is a function for debug logging
type DebugFunc func(format string, args ...any)

// Mount represents a Docker bind mount
type Mount struct {
	Host      string
	Container string
	ReadOnly  bool
	CreateDir bool // If true, create Host as a directory if it doesn't exist
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name   string
	Value  string
	Secret bool // If true, value is redacted in logs
}

// PortMapping represents a port mapping from host to container
type PortMapping struct {
	Host      string // Host port (or host:port for binding to specific interface)
	Container string // Container port
}

// ContainerSpec defines everything needed to run a container
type ContainerSpec struct {
	ImageName string
	Mounts    []Mount
	Env       []EnvVar
	Ports     []PortMapping
	Args      []string // args to the container command
	Command   string   // e.g., "claude"
	WorkDir   string
}

// ExitError represents an exit with a specific code
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// EnsureDirsExist creates host directories for mounts that have CreateDir set
func EnsureDirsExist(mounts []Mount) error {
	for _, m := range mounts {
		if !m.CreateDir {
			continue
		}
		if err := os.MkdirAll(m.Host, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", m.Host, err)
		}
	}
	return nil
}

// RunSpec executes a container based on the provided ContainerSpec.
// If stdinInterceptor is provided, it will be used instead of os.Stdin.
func RunSpec(spec ContainerSpec, stdinInterceptor io.Reader, debug DebugFunc) error {
	debug("Starting docker container")
	debug("Image: %s", spec.ImageName)
	debug("WorkDir: %s", spec.WorkDir)
	debug("Command: %s", spec.Command)

	// Ensure directories exist for rw mounts
	if err := EnsureDirsExist(spec.Mounts); err != nil {
		return err
	}

	// Check if stdin is a terminal
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	debug("stdin is TTY: %v", isTTY)

	var args, logArgs []string
	if isTTY {
		args = []string{"run", "-it", "--rm"}
		logArgs = []string{"run", "-it", "--rm"}
	} else {
		args = []string{"run", "-i", "--rm"}
		logArgs = []string{"run", "-i", "--rm"}
	}

	// Add environment variables
	for _, env := range spec.Env {
		args = append(args, "-e", env.Name+"="+env.Value)
		if env.Secret {
			logArgs = append(logArgs, "-e", env.Name+"=***")
		} else {
			logArgs = append(logArgs, "-e", env.Name+"="+env.Value)
		}
	}

	// Add port mappings
	for _, p := range spec.Ports {
		portArg := p.Host + ":" + p.Container
		args = append(args, "-p", portArg)
		logArgs = append(logArgs, "-p", portArg)
		debug("Port: %s -> %s", p.Host, p.Container)
	}

	// Add mounts
	for _, m := range spec.Mounts {
		mountArg := m.Host + ":" + m.Container
		if m.ReadOnly {
			mountArg += ":ro"
		}
		args = append(args, "-v", mountArg)
		logArgs = append(logArgs, "-v", mountArg)
		debug("Mount: %s -> %s (ro=%v)", m.Host, m.Container, m.ReadOnly)
	}

	// Add working directory
	if spec.WorkDir != "" {
		args = append(args, "-w", spec.WorkDir)
		logArgs = append(logArgs, "-w", spec.WorkDir)
	}

	// Add image and command
	args = append(args, spec.ImageName)
	logArgs = append(logArgs, spec.ImageName)
	if spec.Command != "" {
		args = append(args, spec.Command)
		logArgs = append(logArgs, spec.Command)
	}

	// Add command args
	args = append(args, spec.Args...)
	logArgs = append(logArgs, spec.Args...)

	cmd := exec.Command("docker", args...)
	debug("Running docker command: %v", append([]string{"docker"}, logArgs...))

	// If we have an interceptor AND stdin is a TTY, use PTY to preserve terminal
	if stdinInterceptor != nil && isTTY {
		debug("Using PTY for stdin interception with TTY preservation")
		return runWithPTY(cmd, stdinInterceptor, debug)
	}

	// Simple case: no interception or not a TTY
	if stdinInterceptor != nil {
		cmd.Stdin = stdinInterceptor
	} else {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	debug("Docker command finished")

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("failed to run docker: %w", err)
	}

	return nil
}

// runWithPTY runs the command in a PTY, allowing stdin interception while preserving TTY
func runWithPTY(cmd *exec.Cmd, stdinInterceptor io.Reader, debug DebugFunc) error {
	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}
	defer ptmx.Close()
	debug("Started command with PTY")

	// Handle pty size changes
	cleanup := handleResize(ptmx, debug)
	defer cleanup()

	// Set stdin to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		debug("Warning: failed to set raw mode: %v", err)
	} else {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// Copy intercepted stdin to pty master
	go func() {
		_, err := io.Copy(ptmx, stdinInterceptor)
		if err != nil {
			debug("stdin copy error: %v", err)
		}
	}()

	// Copy pty master output to stdout
	_, _ = io.Copy(os.Stdout, ptmx)

	// Wait for command to finish
	err = cmd.Wait()
	debug("Docker command finished")

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("failed to run docker: %w", err)
	}

	return nil
}

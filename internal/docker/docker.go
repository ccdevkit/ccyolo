package docker

import (
	"fmt"
	"os"
	"os/exec"
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
	Name  string
	Value string
}

// ContainerSpec defines everything needed to run a container
type ContainerSpec struct {
	ImageName string
	Mounts    []Mount
	Env       []EnvVar
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

// RunSpec executes a container based on the provided ContainerSpec
func RunSpec(spec ContainerSpec, debug DebugFunc) error {
	debug("Starting docker container")
	debug("Image: %s", spec.ImageName)
	debug("WorkDir: %s", spec.WorkDir)
	debug("Command: %s", spec.Command)

	// Ensure directories exist for rw mounts
	if err := EnsureDirsExist(spec.Mounts); err != nil {
		return err
	}

	args := []string{"run", "-it", "--rm"}

	// Add environment variables
	for _, env := range spec.Env {
		args = append(args, "-e", env.Name+"="+env.Value)
	}

	// Add mounts
	for _, m := range spec.Mounts {
		mountArg := m.Host + ":" + m.Container
		if m.ReadOnly {
			mountArg += ":ro"
		}
		args = append(args, "-v", mountArg)
		debug("Mount: %s -> %s (ro=%v)", m.Host, m.Container, m.ReadOnly)
	}

	// Add working directory
	if spec.WorkDir != "" {
		args = append(args, "-w", spec.WorkDir)
	}

	// Add image and command
	args = append(args, spec.ImageName)
	if spec.Command != "" {
		args = append(args, spec.Command)
	}

	// Add command args
	args = append(args, spec.Args...)

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	debug("Running docker command: %v", cmd.Args)
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

// Package image handles local Docker image building for ccyolo.
// The base image (ghcr.io/ccdevkit/ccyolo-base) doesn't include Claude CLI.
// This package builds a local image with the specific Claude version installed.
package image

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ccyolo/internal/constants"
)

// DebugFunc is a function for debug logging
type DebugFunc func(format string, args ...any)

// progress prints a user-facing status message to stderr
func progress(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// GetClaudeVersion runs `claude --version` and parses the version string.
// Example: "2.1.16 (Claude Code)" -> "2.1.16"
func GetClaudeVersion(claudePath string, debug DebugFunc) (string, error) {
	debug("Getting claude version from: %s", claudePath)

	cmd := exec.Command(claudePath, "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run claude --version: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	debug("claude --version output: %q", output)

	// Parse "2.1.16 (Claude Code)" -> "2.1.16"
	version := strings.Split(output, " ")[0]
	if version == "" {
		return "", fmt.Errorf("failed to parse version from: %q", output)
	}

	debug("Parsed claude version: %s", version)
	return version, nil
}

// LocalImageName returns the name of the local image for the given base and claude versions.
// Format: ccyolo-local:{baseVersion}-{claudeVersion}
func LocalImageName(baseVersion, claudeVersion string) string {
	return fmt.Sprintf("%s:%s-%s", constants.DockerLocalImageName, baseVersion, claudeVersion)
}

// BaseImageName returns the name of the base image for the given version.
// Format: ghcr.io/ccdevkit/ccyolo-base:{version}
func BaseImageName(baseVersion string) string {
	return fmt.Sprintf("%s:%s", constants.DockerBaseImageRegistry, baseVersion)
}

// ImageExists checks if a Docker image exists locally.
func ImageExists(imageName string, debug DebugFunc) (bool, error) {
	debug("Checking if image exists: %s", imageName)

	cmd := exec.Command("docker", "image", "inspect", imageName)
	err := cmd.Run()

	if err != nil {
		// Exit code 1 means image doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			debug("Image does not exist: %s", imageName)
			return false, nil
		}
		return false, fmt.Errorf("failed to check image: %w", err)
	}

	debug("Image exists: %s", imageName)
	return true, nil
}

// BuildLocalImage builds a local image with Claude installed from the base image.
func BuildLocalImage(baseVersion, claudeVersion string, debug DebugFunc) error {
	localImage := LocalImageName(baseVersion, claudeVersion)
	baseImage := BaseImageName(baseVersion)

	debug("Building local image: %s from base: %s", localImage, baseImage)

	// Generate Dockerfile content
	dockerfile := fmt.Sprintf(`FROM %s
USER root
RUN curl -fsSL https://claude.ai/install.sh | bash -s -- %s
RUN cp /root/.local/bin/claude /home/claude/.local/bin/claude \
    && chown claude:claude /home/claude/.local/bin/claude
USER claude
`, baseImage, claudeVersion)

	debug("Generated Dockerfile:\n%s", dockerfile)

	// Build using stdin Dockerfile
	cmd := exec.Command("docker", "build", "-t", localImage, "-")
	cmd.Stdin = strings.NewReader(dockerfile)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build image: %w\nstderr: %s", err, stderr.String())
	}

	debug("Successfully built image: %s", localImage)
	return nil
}

// EnsureLocalImage ensures the local image exists, building it if necessary.
// Returns the local image name.
func EnsureLocalImage(baseVersion, claudeVersion string, debug DebugFunc) (string, error) {
	localImage := LocalImageName(baseVersion, claudeVersion)

	exists, err := ImageExists(localImage, debug)
	if err != nil {
		return "", err
	}

	if exists {
		debug("Local image already exists: %s", localImage)
		return localImage, nil
	}

	// Only show progress when we need to build
	progress("Building local image with Claude %s...", claudeVersion)
	debug("Local image not found, building: %s", localImage)
	if err := BuildLocalImage(baseVersion, claudeVersion, debug); err != nil {
		return "", err
	}

	progress("Local image ready: %s", localImage)
	return localImage, nil
}

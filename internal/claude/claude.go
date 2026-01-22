package claude

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ccyolo/internal/docker"
)

// DebugFunc is a function for debug logging
type DebugFunc func(format string, args ...any)

// CaptureToken starts a local HTTP server, runs claude with ANTHROPIC_BASE_URL
// pointing to it, and captures the OAuth token from the Authorization header.
func CaptureToken(debug DebugFunc) (string, error) {
	debug("Starting token capture")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to start listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	debug("Listening on %s", baseURL)

	tokenCaptured := make(chan string, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			debug("Received request: %s %s", r.Method, r.URL.Path)
			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			debug("Captured token: %s...", token[:min(20, len(token))])
			tokenCaptured <- token
		}),
	}

	go func() {
		debug("Starting HTTP server")
		server.Serve(listener)
	}()

	debug("Starting claude process")
	cmd := exec.Command("claude")
	cmd.Env = append(os.Environ(), "ANTHROPIC_BASE_URL="+baseURL)

	if err := cmd.Start(); err != nil {
		listener.Close()
		return "", fmt.Errorf("failed to start claude: %w", err)
	}
	debug("Claude process started with PID %d", cmd.Process.Pid)

	debug("Waiting for token...")
	token := <-tokenCaptured
	debug("Token received")

	debug("Killing claude process")
	cmd.Process.Kill()
	cmd.Wait()
	debug("Claude process terminated")

	server.Close()
	listener.Close()
	debug("Server and listener closed")

	return token, nil
}

// GetContainerSpec returns a ContainerSpec configured for running Claude in a container
func GetContainerSpec(token string, sessionID string, settingsPath string, homeDir string, cwd string, extraArgs []string, extraMounts []docker.Mount) (docker.ContainerSpec, error) {
	cckitDir := filepath.Join(homeDir, ".cckit", "ccyolo")
	if err := os.MkdirAll(cckitDir, 0755); err != nil {
		return docker.ContainerSpec{}, fmt.Errorf("failed to create cckit directory: %w", err)
	}

	claudeJsonPath, err := ensureClaudeJson(homeDir, cckitDir)
	if err != nil {
		return docker.ContainerSpec{}, err
	}

	claudeDir := filepath.Join(homeDir, ".claude")

	mounts := []docker.Mount{
		// .claude.json for configuration (file created by ensureClaudeJson)
		{Host: claudeJsonPath, Container: "/root/.claude.json", ReadOnly: false},
		// Mount entire ~/.claude as read-write
		{Host: claudeDir, Container: "/root/.claude", ReadOnly: false},
		// Mount cwd
		{Host: cwd, Container: cwd, ReadOnly: false},
	}

	// Add settings file mount if provided
	if settingsPath != "" {
		mounts = append(mounts, docker.Mount{
			Host:      settingsPath,
			Container: "/tmp/ccyolo-settings.json",
			ReadOnly:  true,
		})
	}

	// Add extra mounts from path arguments
	mounts = append(mounts, extraMounts...)

	env := []docker.EnvVar{
		{Name: "CLAUDE_CODE_OAUTH_TOKEN", Value: token},
	}

	var args []string
	if settingsPath != "" {
		args = append(args, "--settings", "/tmp/ccyolo-settings.json")
	}
	if sessionID != "" {
		args = append(args, "--session-id", sessionID)
	}

	// Append extra args after our hardcoded args
	args = append(args, extraArgs...)

	return docker.ContainerSpec{
		ImageName: "ccyolo",
		Mounts:    mounts,
		Env:       env,
		Args:      args,
		Command:   "claude",
		WorkDir:   cwd,
	}, nil
}

// cwdToProjectPath converts a working directory path to Claude's project directory name
// e.g., "/Users/brad/Development/foo" -> "-Users-brad-Development-foo"
// e.g., "/Users/brad/.config" -> "-Users-brad--config"
func cwdToProjectPath(cwd string) string {
	result := strings.ReplaceAll(cwd, "/", "-")
	result = strings.ReplaceAll(result, ".", "-")
	return result
}

// ensureClaudeJson ensures the .claude.json file exists in the cckit directory.
// It copies the user's existing ~/.claude.json if it exists, otherwise creates an empty one.
func ensureClaudeJson(homeDir, cckitDir string) (string, error) {
	claudeJsonPath := filepath.Join(cckitDir, ".claude.json")

	if _, err := os.Stat(claudeJsonPath); os.IsNotExist(err) {
		userClaudeJson := filepath.Join(homeDir, ".claude.json")
		if data, err := os.ReadFile(userClaudeJson); err == nil {
			if err := os.WriteFile(claudeJsonPath, data, 0644); err != nil {
				return "", fmt.Errorf("failed to copy .claude.json: %w", err)
			}
		} else {
			if err := os.WriteFile(claudeJsonPath, []byte("{}"), 0644); err != nil {
				return "", fmt.Errorf("failed to create .claude.json: %w", err)
			}
		}
	}

	return claudeJsonPath, nil
}

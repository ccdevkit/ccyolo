package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ccyolo/internal/constants"
	"github.com/google/uuid"
)

// ProxyConfig is the config file for ccproxy in the container
type ProxyConfig struct {
	HostAddress string   `json:"hostAddress"`
	Passthrough []string `json:"passthrough"`
	Verbose     bool     `json:"verbose"`
}

// Session represents a ccyolo session with a unique ID and directories
type Session struct {
	id      string
	tempDir string // Ephemeral session files (settings.json)
}

// New creates a new session with a unique UUID and creates the temp directory
func New() (*Session, error) {
	return WithID(uuid.New().String())
}

// WithID creates a session with a specific ID and creates the temp directory
func WithID(id string) (*Session, error) {
	tempDir := filepath.Join(os.TempDir(), id)

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &Session{
		id:      id,
		tempDir: tempDir,
	}, nil
}

// ID returns the session UUID
func (s *Session) ID() string {
	return s.id
}

// TempDir returns the ephemeral temp directory for this session
func (s *Session) TempDir() string {
	return s.tempDir
}

// SettingsPath returns the path to the generated settings file for this session
func (s *Session) SettingsPath() string {
	return filepath.Join(s.tempDir, "settings.json")
}

// ProxyConfigPath returns the path to the proxy config file for this session
func (s *Session) ProxyConfigPath() string {
	return filepath.Join(s.tempDir, "ccyolo-proxy.json")
}

// SystemPromptPath returns the path to the system prompt file for this session
func (s *Session) SystemPromptPath() string {
	return filepath.Join(s.tempDir, "system-prompt.md")
}

// WriteSettings generates the settings.json file with the statusline command
// configured to communicate with the host via TCP using host.docker.internal,
// and sets bypassPermissions mode (the whole point of ccyolo)
func (s *Session) WriteSettings(port int) error {
	// Pipe stdin directly to the host server via nc
	// -N shuts down the network socket after EOF on stdin
	content := fmt.Sprintf(`{
  "permissions": {
    "defaultMode": "bypassPermissions"
  },
  "statusLine": {
    "type": "command",
    "command": "nc -N %s %d"
  }
}
`, constants.DockerHostDNS, port)
	return os.WriteFile(s.SettingsPath(), []byte(content), 0644)
}

// WriteProxyConfig generates the ccyolo-proxy.json config file for the container
func (s *Session) WriteProxyConfig(port int, passthrough []string, verbose bool) error {
	config := ProxyConfig{
		HostAddress: fmt.Sprintf("%s:%d", constants.DockerHostDNS, port),
		Passthrough: passthrough,
		Verbose:     verbose,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	return os.WriteFile(s.ProxyConfigPath(), data, 0644)
}

// WriteSystemPrompt generates the system prompt file explaining the container/host setup
func (s *Session) WriteSystemPrompt(passthrough []string) error {
	var passthroughList string
	if len(passthrough) > 0 {
		passthroughList = "\n\nThe following command patterns are configured to run on the host:\n"
		for _, p := range passthrough {
			passthroughList += fmt.Sprintf("- `%s`\n", p)
		}
	}

	content := fmt.Sprintf(`# Container Execution Environment

You are running inside a Docker container, not directly on the host machine.

## Key Points

- **Your environment is isolated**: You're in a container with its own filesystem, installed packages, and environment variables.
- **Some commands run on the host**: Certain commands are configured to be executed on the host machine instead of inside the container. When this happens, the output will be prefixed with "[NOTE: This command was run on the host machine]".
- **Potential inconsistencies**: Because some commands run in the container and others on the host, you may see inconsistencies. For example:
  - File paths that exist on the host may not exist in the container (and vice versa)
  - Environment variables may differ between host and container
  - Installed tools and their versions may differ
  - User identity and permissions may differ
%s
## What This Means for You

- If you see "[NOTE: This command was run on the host machine]" in command output, that command executed on the host, not in your container.
- If a command shows files or paths that don't match what you see with other commands, consider whether one ran on host vs container.
- The working directory is bind-mounted, so files in the project directory are shared between host and container.
`, passthroughList)

	return os.WriteFile(s.SystemPromptPath(), []byte(content), 0644)
}

// Cleanup removes the temp directory and all its contents
func (s *Session) Cleanup() error {
	return os.RemoveAll(s.tempDir)
}

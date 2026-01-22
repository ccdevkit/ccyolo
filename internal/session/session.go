package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

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

// WriteSettings generates the settings.json file with the statusline command
// configured to communicate with the host via TCP using host.docker.internal
func (s *Session) WriteSettings(port int) error {
	// Pipe stdin directly to the host server via nc
	// -N shuts down the network socket after EOF on stdin
	content := fmt.Sprintf(`{
  "statusLine": {
    "type": "command",
    "command": "nc -N host.docker.internal %d"
  }
}
`, port)
	return os.WriteFile(s.SettingsPath(), []byte(content), 0644)
}

// Cleanup removes the temp directory and all its contents
func (s *Session) Cleanup() error {
	return os.RemoveAll(s.tempDir)
}

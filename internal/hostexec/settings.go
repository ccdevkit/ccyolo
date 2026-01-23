package hostexec

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ccyolo/internal/constants"
)

// StatusLine represents the statusLine configuration
type StatusLine struct {
	Command string `json:"command"`
}

// Settings represents the Claude settings.json structure (partial)
type Settings struct {
	StatusLine *StatusLine `json:"statusLine,omitempty"`
}

// GetMergedSettings reads and merges settings from ~/.claude/settings.json
// and {cwd}/.claude/settings.json, with cwd settings taking precedence.
func GetMergedSettings(homeDir, cwd string) (*Settings, error) {
	result := &Settings{}

	// Read global settings first
	globalPath := filepath.Join(homeDir, constants.ClaudeDirName, constants.SettingsJson)
	if global, err := readSettings(globalPath); err == nil {
		mergeSettings(result, global)
	}

	// Read and merge local settings (takes precedence)
	localPath := filepath.Join(cwd, constants.ClaudeDirName, constants.SettingsJson)
	if local, err := readSettings(localPath); err == nil {
		mergeSettings(result, local)
	}

	return result, nil
}

// readSettings reads a settings.json file and returns the parsed Settings
func readSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// mergeSettings merges src into dst, with src taking precedence for non-nil fields
func mergeSettings(dst, src *Settings) {
	if src.StatusLine != nil {
		dst.StatusLine = src.StatusLine
	}
}

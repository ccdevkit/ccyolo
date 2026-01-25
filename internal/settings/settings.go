// Package settings handles loading ccyolo configuration from filesystem.
package settings

import (
	"github.com/ccdevkit/common/settings"
)

// Settings represents ccyolo configuration loaded from filesystem.
// Settings are loaded from .ccdevkit/ccyolo/settings.{json,yaml,yml} files
// discovered by walking up from cwd to root.
type Settings struct {
	// ClaudePath is the path to the claude CLI executable.
	ClaudePath string `yaml:"claudePath" json:"claudePath"`

	// Passthrough is a list of command prefixes to run on the host.
	Passthrough []string `yaml:"passthrough" json:"passthrough"`
}

// settingsPath is the relative path for settings file discovery.
const settingsPath = ".ccdevkit/ccyolo/settings"

// Load discovers and loads settings from the filesystem.
// Returns an empty Settings if no files are found.
func Load() (*Settings, error) {
	cfg := &Settings{}
	if err := settings.Load(settingsPath, cfg, nil); err != nil {
		return nil, err
	}
	return cfg, nil
}

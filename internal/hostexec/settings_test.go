package hostexec

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadSettings tests reading settings from a file
func TestReadSettings(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantCommand string
		wantErr     bool
	}{
		{
			name:        "valid settings with statusLine",
			content:     `{"statusLine": {"command": "echo test"}}`,
			wantCommand: "echo test",
			wantErr:     false,
		},
		{
			name:        "empty object",
			content:     `{}`,
			wantCommand: "",
			wantErr:     false,
		},
		{
			name:        "null statusLine",
			content:     `{"statusLine": null}`,
			wantCommand: "",
			wantErr:     false,
		},
		{
			name:        "invalid JSON",
			content:     `{invalid}`,
			wantCommand: "",
			wantErr:     true,
		},
		{
			name:        "extra fields ignored",
			content:     `{"statusLine": {"command": "test"}, "otherField": "ignored"}`,
			wantCommand: "test",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			settingsPath := filepath.Join(tmpDir, "settings.json")
			os.WriteFile(settingsPath, []byte(tt.content), 0644)

			settings, err := readSettings(settingsPath)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var gotCommand string
			if settings.StatusLine != nil {
				gotCommand = settings.StatusLine.Command
			}

			if gotCommand != tt.wantCommand {
				t.Errorf("StatusLine.Command = %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}

// TestReadSettingsFileNotFound tests reading non-existent file
func TestReadSettingsFileNotFound(t *testing.T) {
	_, err := readSettings("/nonexistent/path/settings.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// TestMergeSettings tests the settings merge logic
func TestMergeSettings(t *testing.T) {
	tests := []struct {
		name        string
		dst         *Settings
		src         *Settings
		wantCommand string
	}{
		{
			name:        "src overwrites dst",
			dst:         &Settings{StatusLine: &StatusLine{Command: "global"}},
			src:         &Settings{StatusLine: &StatusLine{Command: "local"}},
			wantCommand: "local",
		},
		{
			name:        "src nil statusLine keeps dst",
			dst:         &Settings{StatusLine: &StatusLine{Command: "global"}},
			src:         &Settings{StatusLine: nil},
			wantCommand: "global",
		},
		{
			name:        "dst nil statusLine gets src",
			dst:         &Settings{StatusLine: nil},
			src:         &Settings{StatusLine: &StatusLine{Command: "local"}},
			wantCommand: "local",
		},
		{
			name:        "both nil",
			dst:         &Settings{StatusLine: nil},
			src:         &Settings{StatusLine: nil},
			wantCommand: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeSettings(tt.dst, tt.src)

			var gotCommand string
			if tt.dst.StatusLine != nil {
				gotCommand = tt.dst.StatusLine.Command
			}

			if gotCommand != tt.wantCommand {
				t.Errorf("After merge, StatusLine.Command = %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}

// TestGetMergedSettings tests the full settings merge from files
func TestGetMergedSettings(t *testing.T) {
	tests := []struct {
		name          string
		globalContent string
		localContent  string
		wantCommand   string
	}{
		{
			name:          "local overrides global",
			globalContent: `{"statusLine": {"command": "global-cmd"}}`,
			localContent:  `{"statusLine": {"command": "local-cmd"}}`,
			wantCommand:   "local-cmd",
		},
		{
			name:          "only global exists",
			globalContent: `{"statusLine": {"command": "global-only"}}`,
			localContent:  "",
			wantCommand:   "global-only",
		},
		{
			name:          "only local exists",
			globalContent: "",
			localContent:  `{"statusLine": {"command": "local-only"}}`,
			wantCommand:   "local-only",
		},
		{
			name:          "neither exists",
			globalContent: "",
			localContent:  "",
			wantCommand:   "",
		},
		{
			name:          "global has command local is empty object",
			globalContent: `{"statusLine": {"command": "global-cmd"}}`,
			localContent:  `{}`,
			wantCommand:   "global-cmd",
		},
		{
			name:          "local has command global is empty object",
			globalContent: `{}`,
			localContent:  `{"statusLine": {"command": "local-cmd"}}`,
			wantCommand:   "local-cmd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directories
			homeDir := t.TempDir()
			cwdDir := t.TempDir()

			// Create global settings if content provided
			if tt.globalContent != "" {
				globalDir := filepath.Join(homeDir, ".claude")
				os.MkdirAll(globalDir, 0755)
				os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte(tt.globalContent), 0644)
			}

			// Create local settings if content provided
			if tt.localContent != "" {
				localDir := filepath.Join(cwdDir, ".claude")
				os.MkdirAll(localDir, 0755)
				os.WriteFile(filepath.Join(localDir, "settings.json"), []byte(tt.localContent), 0644)
			}

			settings, err := GetMergedSettings(homeDir, cwdDir)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var gotCommand string
			if settings.StatusLine != nil {
				gotCommand = settings.StatusLine.Command
			}

			if gotCommand != tt.wantCommand {
				t.Errorf("GetMergedSettings() command = %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}

// TestGetMergedSettingsIgnoresInvalidJSON tests that invalid JSON files are ignored
func TestGetMergedSettingsIgnoresInvalidJSON(t *testing.T) {
	homeDir := t.TempDir()
	cwdDir := t.TempDir()

	// Create invalid global settings
	globalDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(globalDir, 0755)
	os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte(`{invalid json}`), 0644)

	// Create valid local settings
	localDir := filepath.Join(cwdDir, ".claude")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "settings.json"), []byte(`{"statusLine": {"command": "valid"}}`), 0644)

	settings, err := GetMergedSettings(homeDir, cwdDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if settings.StatusLine == nil || settings.StatusLine.Command != "valid" {
		t.Errorf("Expected valid local settings, got %+v", settings)
	}
}

package args

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLooksLikePath(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   bool
		skipOn string // "windows" or "unix" to skip on that platform
	}{
		// Unix absolute paths
		{"unix root", "/", true, ""},
		{"unix absolute", "/usr/local/bin", true, ""},
		{"unix home path", "/home/user/file.txt", true, ""},
		{"unix var path", "/var/log/syslog", true, ""},

		// Relative paths with explicit prefix
		{"relative current dir", "./config.yaml", true, ""},
		{"relative parent dir", "../Makefile", true, ""},
		{"relative nested", "./src/main.go", true, ""},

		// Tilde paths
		{"tilde home", "~/Documents", true, ""},
		{"tilde with file", "~/file.txt", true, ""},

		// Files with common extensions (detected via regex)
		{"json file", "config.json", true, ""},
		{"yaml file", "settings.yaml", true, ""},
		{"go file", "main.go", true, ""},
		{"txt file", "readme.txt", true, ""},
		{"md file", "README.md", true, ""},
		{"rs file", "lib.rs", true, ""},

		// Path with directory separators
		{"subdirectory path", "src/main.go", true, ""},
		{"deep path", "a/b/c/d.txt", true, ""},
		{"path without extension", "foo/bar", true, ""},

		// Flags - should NOT match
		{"short flag", "-v", false, ""},
		{"long flag", "--verbose", false, ""},
		{"flag with value", "--config=file.yaml", false, ""},
		{"flag with dash value", "-o=output", false, ""},
		{"double dash", "--", false, ""},
		{"flag with number", "-p8080", false, ""},

		// Plain words - should NOT match
		{"plain word", "hello", false, ""},
		{"command name", "status", false, ""},
		{"number", "12345", false, ""},
		{"word with number", "test123", false, ""},
		{"word with underscore", "my_var", false, ""},
		{"word with dash", "my-command", false, ""},

		// Edge cases
		{"empty string", "", false, ""},
		{"single dot", ".", false, ""},
		{"double dot", "..", false, ""},
		{"just tilde", "~", false, ""},
		{"hidden file", ".gitignore", true, ""}, // has extension pattern

		// Windows paths (skip on non-Windows since they're only detected on Windows)
		{"windows drive C", "C:\\Users\\file.txt", true, "unix"},
		{"windows drive D", "D:\\Projects\\code", true, "unix"},
		{"windows relative", ".\\config.yaml", true, "unix"},
		{"windows parent", "..\\parent", true, "unix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOn == "windows" && runtime.GOOS == "windows" {
				t.Skip("Skipping on Windows")
			}
			if tt.skipOn == "unix" && runtime.GOOS != "windows" {
				t.Skip("Skipping on Unix")
			}

			got := looksLikePath(tt.input)
			if got != tt.want {
				t.Errorf("looksLikePath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcess(t *testing.T) {
	// Create a temporary directory and file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create temp subdir: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		wantPassArgs   []string
		wantMountCount int
	}{
		{
			name:           "empty args",
			args:           []string{},
			wantPassArgs:   []string{},
			wantMountCount: 0,
		},
		{
			name:           "flags only",
			args:           []string{"-v", "--verbose", "--config=foo"},
			wantPassArgs:   []string{"-v", "--verbose", "--config=foo"},
			wantMountCount: 0,
		},
		{
			name:           "plain words",
			args:           []string{"status", "hello", "world"},
			wantPassArgs:   []string{"status", "hello", "world"},
			wantMountCount: 0,
		},
		{
			name:           "existing file path",
			args:           []string{tmpFile},
			wantPassArgs:   []string{tmpFile},
			wantMountCount: 1,
		},
		{
			name:           "existing directory path",
			args:           []string{subDir},
			wantPassArgs:   []string{subDir},
			wantMountCount: 1,
		},
		{
			name:           "non-existing path",
			args:           []string{"/nonexistent/path/file.txt"},
			wantPassArgs:   []string{"/nonexistent/path/file.txt"},
			wantMountCount: 0,
		},
		{
			name:           "mixed args with existing path",
			args:           []string{"-v", tmpFile, "--verbose"},
			wantPassArgs:   []string{"-v", tmpFile, "--verbose"},
			wantMountCount: 1,
		},
		{
			name:           "multiple existing paths",
			args:           []string{tmpFile, subDir},
			wantPassArgs:   []string{tmpFile, subDir},
			wantMountCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Process(tt.args)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			// Check PassArgs
			if len(got.PassArgs) != len(tt.wantPassArgs) {
				t.Errorf("Process() PassArgs length = %d, want %d", len(got.PassArgs), len(tt.wantPassArgs))
			}
			for i, arg := range got.PassArgs {
				if i < len(tt.wantPassArgs) && arg != tt.wantPassArgs[i] {
					t.Errorf("Process() PassArgs[%d] = %q, want %q", i, arg, tt.wantPassArgs[i])
				}
			}

			// Check mount count
			if len(got.ExtraMounts) != tt.wantMountCount {
				t.Errorf("Process() ExtraMounts count = %d, want %d", len(got.ExtraMounts), tt.wantMountCount)
			}
		})
	}
}

func TestProcessMountDetails(t *testing.T) {
	// Create a temporary file for testing mount details
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	got, err := Process([]string{tmpFile})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(got.ExtraMounts) != 1 {
		t.Fatalf("Expected 1 mount, got %d", len(got.ExtraMounts))
	}

	mount := got.ExtraMounts[0]

	// Host path should be the absolute path
	absPath, _ := filepath.Abs(tmpFile)
	if mount.Host != absPath {
		t.Errorf("Mount.Host = %q, want %q", mount.Host, absPath)
	}

	// Container path should match host path
	if mount.Container != absPath {
		t.Errorf("Mount.Container = %q, want %q", mount.Container, absPath)
	}

	// Should not be read-only
	if mount.ReadOnly {
		t.Errorf("Mount.ReadOnly = true, want false")
	}
}

func TestProcessTildeExpansion(t *testing.T) {
	// This test checks that tilde expansion works
	// We can't easily test with actual home directory files,
	// so we just verify the function doesn't error on tilde paths
	got, err := Process([]string{"~/nonexistent_test_file_12345.txt"})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// The tilde path looks like a path
	if len(got.PassArgs) != 1 || got.PassArgs[0] != "~/nonexistent_test_file_12345.txt" {
		t.Errorf("PassArgs not preserved correctly")
	}

	// But since it doesn't exist, no mount should be created
	if len(got.ExtraMounts) != 0 {
		t.Errorf("Expected 0 mounts for non-existent tilde path, got %d", len(got.ExtraMounts))
	}
}

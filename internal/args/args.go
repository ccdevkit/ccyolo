package args

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"ccyolo/internal/docker"
)

// ProcessedArgs holds the result of parsing CLI arguments
type ProcessedArgs struct {
	PassArgs                  []string       // args to pass through to claude
	ExtraMounts               []docker.Mount // mounts for path arguments that exist
	HasAppendSystemPrompt     bool           // true if user provided --append-system-prompt
	HasAppendSystemPromptFile bool           // true if user provided --append-system-prompt-file
}

// Process parses arguments and identifies paths to bind
func Process(args []string) (ProcessedArgs, error) {
	result := ProcessedArgs{
		PassArgs:    []string{},
		ExtraMounts: []docker.Mount{},
	}

	// Track which paths we've already mounted to avoid duplicates
	mountedPaths := make(map[string]bool)

	// Process args and detect system prompt flags
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check if this is --append-system-prompt or --append-system-prompt-file
		if arg == "--append-system-prompt" {
			result.HasAppendSystemPrompt = true
		} else if arg == "--append-system-prompt-file" {
			result.HasAppendSystemPromptFile = true

			// Mount the file if it's the next arg and looks like a path
			if i+1 < len(args) {
				filePath := args[i+1]

				// Expand ~ to home directory
				expandedPath := filePath
				if strings.HasPrefix(filePath, "~/") {
					home, err := os.UserHomeDir()
					if err == nil {
						expandedPath = filepath.Join(home, filePath[2:])
					}
				}

				// Convert to absolute path
				absPath, err := filepath.Abs(expandedPath)
				if err == nil {
					// Add mount for the file if it exists
					if _, err := os.Stat(absPath); err == nil {
						result.ExtraMounts = append(result.ExtraMounts, docker.Mount{
							Host:      absPath,
							Container: absPath,
							ReadOnly:  true,
						})
						mountedPaths[absPath] = true
					}
				}
			}
		}

		// Add this arg to passArgs
		result.PassArgs = append(result.PassArgs, arg)

		// Check if it looks like a path for mounting
		if looksLikePath(arg) {
			// Expand ~ to home directory
			expandedPath := arg
			if strings.HasPrefix(arg, "~/") {
				home, err := os.UserHomeDir()
				if err == nil {
					expandedPath = filepath.Join(home, arg[2:])
				}
			}

			// Convert to absolute path
			absPath, err := filepath.Abs(expandedPath)
			if err != nil {
				continue
			}

			// Check if path exists and not already mounted
			if _, err := os.Stat(absPath); err == nil {
				if !mountedPaths[absPath] {
					result.ExtraMounts = append(result.ExtraMounts, docker.Mount{
						Host:      absPath,
						Container: absPath,
						ReadOnly:  false,
					})
					mountedPaths[absPath] = true
				}
			}
		}
	}

	return result, nil
}

// looksLikePath returns true if the string could be a filesystem path
func looksLikePath(s string) bool {
	// Skip flags
	if strings.HasPrefix(s, "-") {
		return false
	}

	// Unix-style path indicators
	if strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "~/") {
		return true
	}

	// Windows-style paths (only check on Windows)
	if runtime.GOOS == "windows" {
		// Drive letter paths: C:\, D:\, etc.
		if len(s) >= 2 && s[1] == ':' {
			return true
		}
		// Relative paths with backslash: .\, ..\
		if strings.HasPrefix(s, ".\\") || strings.HasPrefix(s, "..\\") {
			return true
		}
	}

	// Contains path separator
	if strings.Contains(s, "/") {
		return true
	}

	// Has file extension pattern (e.g., "foo.json", "config.yaml")
	if matched, _ := regexp.MatchString(`\.\w+$`, s); matched {
		return true
	}

	return false
}

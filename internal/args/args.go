package args

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ccyolo/internal/docker"
)

// ProcessedArgs holds the result of parsing CLI arguments
type ProcessedArgs struct {
	PassArgs    []string       // args to pass through to claude
	ExtraMounts []docker.Mount // mounts for path arguments that exist
}

// Process parses arguments and identifies paths to bind
func Process(args []string) (ProcessedArgs, error) {
	result := ProcessedArgs{
		PassArgs:    args,
		ExtraMounts: []docker.Mount{},
	}

	// Find paths to bind mount
	for _, arg := range result.PassArgs {
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

			// Check if path exists
			if _, err := os.Stat(absPath); err == nil {
				result.ExtraMounts = append(result.ExtraMounts, docker.Mount{
					Host:      absPath,
					Container: absPath,
					ReadOnly:  false,
				})
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

	// Starts with path indicators
	if strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "~/") {
		return true
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

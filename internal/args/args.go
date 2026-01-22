package args

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ccyolo/internal/docker"
)

// ProcessedArgs holds the result of parsing CLI arguments
type ProcessedArgs struct {
	SessionID   string         // extracted --session-id value (empty if not provided)
	PassArgs    []string       // args to pass through to claude (with --session-id removed)
	ExtraMounts []docker.Mount // mounts for path arguments that exist
}

// Process parses arguments, extracts session ID, and identifies paths to bind
func Process(args []string) (ProcessedArgs, error) {
	result := ProcessedArgs{
		PassArgs:    []string{},
		ExtraMounts: []docker.Mount{},
	}

	// First pass: extract --session-id
	i := 0
	for i < len(args) {
		arg := args[i]

		// Handle --session-id=value
		if strings.HasPrefix(arg, "--session-id=") {
			value := strings.TrimPrefix(arg, "--session-id=")
			if !isUUID(value) {
				return result, fmt.Errorf("invalid session ID format: %s", value)
			}
			result.SessionID = value
			i++
			continue
		}

		// Handle --session-id value
		if arg == "--session-id" {
			if i+1 >= len(args) {
				return result, fmt.Errorf("--session-id requires a value")
			}
			value := args[i+1]
			if !isUUID(value) {
				return result, fmt.Errorf("invalid session ID format: %s", value)
			}
			result.SessionID = value
			i += 2
			continue
		}

		result.PassArgs = append(result.PassArgs, arg)
		i++
	}

	// Second pass: find paths to bind mount
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

// isUUID validates UUID format for session IDs
func isUUID(s string) bool {
	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	pattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}

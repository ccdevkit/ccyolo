package matcher

import "strings"

// PrefixMatcher matches commands that start with a given prefix
type PrefixMatcher struct {
	prefix string
}

// NewPrefixMatcher creates a new prefix matcher
func NewPrefixMatcher(prefix string) *PrefixMatcher {
	return &PrefixMatcher{prefix: prefix}
}

// Matches returns true if the command starts with the prefix
// For example, prefix "git" matches "git", "git status", "git commit -m 'foo'"
// But not "gitk" or "gitsomething"
// Also handles quoted arguments from the hijacker (e.g., "go 'build'" matches "go build")
func (m *PrefixMatcher) Matches(fullCommand string) bool {
	// Normalize the command by removing single quotes around simple arguments
	normalized := normalizeCommand(fullCommand)

	// Exact match
	if normalized == m.prefix {
		return true
	}
	// Prefix followed by space (command with arguments)
	if strings.HasPrefix(normalized, m.prefix+" ") {
		return true
	}
	return false
}

// normalizeCommand removes single quotes that wrap individual arguments
// e.g., "go 'build'" -> "go build", "git 'commit' '-m' 'message'" -> "git commit -m message"
func normalizeCommand(cmd string) string {
	var result strings.Builder
	inQuote := false
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if c == '\'' {
			inQuote = !inQuote
			// Skip the quote character
			continue
		}
		result.WriteByte(c)
	}
	return result.String()
}

// Prefix returns the prefix this matcher uses
func (m *PrefixMatcher) Prefix() string {
	return m.prefix
}

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
func (m *PrefixMatcher) Matches(fullCommand string) bool {
	// Exact match
	if fullCommand == m.prefix {
		return true
	}
	// Prefix followed by space (command with arguments)
	if strings.HasPrefix(fullCommand, m.prefix+" ") {
		return true
	}
	return false
}

// Prefix returns the prefix this matcher uses
func (m *PrefixMatcher) Prefix() string {
	return m.prefix
}

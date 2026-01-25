package matcher

import "testing"

func TestPrefixMatcher_Matches(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		fullCommand string
		want        bool
	}{
		// Exact matches
		{"exact match single word", "git", "git", true},
		{"exact match longer", "docker", "docker", true},

		// Prefix with arguments
		{"prefix with single arg", "git", "git status", true},
		{"prefix with multiple args", "git", "git commit -m 'message'", true},
		{"docker with args", "docker", "docker run -it ubuntu", true},

		// Should NOT match - prefix is substring of command word
		{"prefix is substring no space", "git", "gitk", false},
		{"prefix is substring github", "git", "github", false},

		// Different commands entirely
		{"different command", "git", "docker", false},
		{"no common prefix", "npm", "yarn install", false},

		// Empty cases
		{"empty command", "git", "", false},
		{"empty prefix empty command", "", "", true},

		// Edge cases with spaces
		{"command with leading space", "git", " git status", false},
		{"prefix with trailing space in command", "git", "git ", true},

		// Case sensitivity
		{"case sensitive no match", "git", "Git status", false},
		{"case sensitive no match upper", "Git", "git status", false},

		// Quoted arguments (from hijacker scripts)
		{"quoted single arg", "go build", "go 'build'", true},
		{"quoted multiple args", "git commit", "git 'commit' '-m' 'message'", true},
		{"quoted exact match", "git", "git", true},
		{"quoted with spaces in arg", "git commit", "git 'commit' '-m' 'hello world'", true},
		{"quoted no match", "go run", "go 'build'", false},
		{"mixed quoted unquoted", "pnpm run", "pnpm 'run' dev", true},
		{"quoted commit message with spaces", "git commit", "git 'commit' '-m' 'fix: handle multi-word commit messages'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewPrefixMatcher(tt.prefix)
			got := m.Matches(tt.fullCommand)
			if got != tt.want {
				t.Errorf("PrefixMatcher(%q).Matches(%q) = %v, want %v",
					tt.prefix, tt.fullCommand, got, tt.want)
			}
		})
	}
}

func TestPrefixMatcher_Prefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"simple prefix", "git"},
		{"longer prefix", "docker-compose"},
		{"empty prefix", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewPrefixMatcher(tt.prefix)
			if got := m.Prefix(); got != tt.prefix {
				t.Errorf("Prefix() = %q, want %q", got, tt.prefix)
			}
		})
	}
}

func TestNewPrefixMatcher(t *testing.T) {
	m := NewPrefixMatcher("test")
	if m == nil {
		t.Error("NewPrefixMatcher returned nil")
	}

	// Verify it implements Matcher interface
	var _ Matcher = m
}

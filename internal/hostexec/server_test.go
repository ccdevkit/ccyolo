package hostexec

import (
	"encoding/json"
	"testing"
)

// noopDebug is a no-op debug function for tests
func noopDebug(format string, args ...any) {}

// TestRewriteContainerPaths tests the path rewriting functionality
func TestRewriteContainerPaths(t *testing.T) {
	s := &Server{
		homeDir: "/Users/testuser",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path rewrite",
			input:    `{"transcriptPath": "/home/claude/.claude/transcripts/abc.json"}`,
			expected: `{"transcriptPath":"/Users/testuser/.claude/transcripts/abc.json"}`,
		},
		{
			name:     "multiple paths",
			input:    `{"path1": "/home/claude/file1.txt", "path2": "/home/claude/file2.txt"}`,
			expected: `{"path1":"/Users/testuser/file1.txt","path2":"/Users/testuser/file2.txt"}`,
		},
		{
			name:     "nested object with path",
			input:    `{"outer": {"inner": "/home/claude/nested.txt"}}`,
			expected: `{"outer":{"inner":"/Users/testuser/nested.txt"}}`,
		},
		{
			name:     "array with paths",
			input:    `{"paths": ["/home/claude/a.txt", "/home/claude/b.txt"]}`,
			expected: `{"paths":["/Users/testuser/a.txt","/Users/testuser/b.txt"]}`,
		},
		{
			name:     "mixed content - some paths some not",
			input:    `{"path": "/home/claude/file.txt", "name": "not a path", "count": 42}`,
			expected: `{"count":42,"name":"not a path","path":"/Users/testuser/file.txt"}`,
		},
		{
			name:     "no container paths",
			input:    `{"path": "/usr/local/bin/app", "name": "test"}`,
			expected: `{"name":"test","path":"/usr/local/bin/app"}`,
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: `{}`,
		},
		{
			name:     "invalid JSON returns as-is",
			input:    `{invalid json}`,
			expected: `{invalid json}`,
		},
		{
			name:     "path in array of objects",
			input:    `{"items": [{"file": "/home/claude/x.txt"}, {"file": "/home/claude/y.txt"}]}`,
			expected: `{"items":[{"file":"/Users/testuser/x.txt"},{"file":"/Users/testuser/y.txt"}]}`,
		},
		{
			name:     "deeply nested path",
			input:    `{"a": {"b": {"c": {"d": "/home/claude/deep.txt"}}}}`,
			expected: `{"a":{"b":{"c":{"d":"/Users/testuser/deep.txt"}}}}`,
		},
		{
			name:     "partial match does rewrite (HasPrefix behavior)",
			input:    `{"path": "/home/claudette/file.txt"}`,
			expected: `{"path":"/Users/testusertte/file.txt"}`,
		},
		{
			name:     "exact container home",
			input:    `{"home": "/home/claude"}`,
			expected: `{"home":"/Users/testuser"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.rewriteContainerPaths([]byte(tt.input))

			// For invalid JSON, compare directly
			if tt.name == "invalid JSON returns as-is" {
				if string(result) != tt.expected {
					t.Errorf("rewriteContainerPaths(%q) = %q, want %q", tt.input, string(result), tt.expected)
				}
				return
			}

			// For valid JSON, parse and compare to handle key ordering differences
			var gotObj, wantObj map[string]any
			if err := json.Unmarshal(result, &gotObj); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &wantObj); err != nil {
				t.Fatalf("Failed to parse expected JSON: %v", err)
			}

			gotJSON, _ := json.Marshal(gotObj)
			wantJSON, _ := json.Marshal(wantObj)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("rewriteContainerPaths(%q) = %s, want %s", tt.input, gotJSON, wantJSON)
			}
		})
	}
}

// Note: Integration tests for TCP server (TestServerStartStop, TestServerExecRequest,
// TestServerLogRequest, TestServerMultipleConnections, TestServerExecWithCwd) require
// network access and are excluded from unit tests. They can be run manually in an
// environment that allows TCP binding on localhost.

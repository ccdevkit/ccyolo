package stdin

import (
	"bytes"
	"io"
	"testing"
)

func TestLooksLikeImagePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid image paths with path prefix
		{"absolute png", "/path/to/image.png", true},
		{"absolute jpg", "/path/to/photo.jpg", true},
		{"absolute jpeg", "/path/to/photo.jpeg", true},
		{"absolute gif", "/path/to/animation.gif", true},
		{"absolute webp", "/path/to/image.webp", true},
		{"relative png", "./screenshot.png", true},
		{"parent dir png", "../images/logo.png", true},
		{"tilde png", "~/Pictures/photo.jpg", true},

		// Case insensitivity
		{"uppercase PNG", "/path/to/IMAGE.PNG", true},
		{"mixed case Jpg", "/path/to/Photo.Jpg", true},
		{"uppercase JPEG", "/path/to/PHOTO.JPEG", true},

		// Invalid - not image extensions
		{"text file", "/path/to/file.txt", false},
		{"go file", "/path/to/main.go", false},
		{"no extension", "/path/to/image", false},
		{"pdf file", "/path/to/document.pdf", false},
		{"svg file", "/path/to/vector.svg", false},

		// Invalid - no path prefix
		{"just filename", "image.png", false},
		{"no path", "photo.jpg", false},
		{"http url", "http://example.com/image.png", false},
		{"https url", "https://example.com/image.png", false},

		// Edge cases
		{"empty string", "", false},
		{"just slash", "/", false},
		{"just extension", ".png", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeImagePath(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeImagePath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestUnescapeShellPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no escaping needed", "/path/to/file.png", "/path/to/file.png"},
		{"escaped space", "/path/to/file\\ name.png", "/path/to/file name.png"},
		{"escaped parentheses", "/path/to/file\\(1\\).png", "/path/to/file(1).png"},
		{"multiple escapes", "/path/my\\ file\\ \\(copy\\).png", "/path/my file (copy).png"},
		{"escaped single quote", "/path/to/\\'quoted\\'.png", "/path/to/'quoted'.png"},
		{"escaped double quote", "/path/to/\\\"quoted\\\".png", "/path/to/\"quoted\".png"},
		{"escaped backslash", "/path/to/file\\\\.png", "/path/to/file\\.png"},
		{"empty string", "", ""},
		{"complex path", "/Users/test/My\\ Documents/Photos\\ \\(2024\\)/image.png", "/Users/test/My Documents/Photos (2024)/image.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unescapeShellPath(tt.input)
			if got != tt.expected {
				t.Errorf("unescapeShellPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEscapeShellPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no escaping needed", "/path/to/file.png", "/path/to/file.png"},
		{"space in path", "/path/to/file name.png", "/path/to/file\\ name.png"},
		{"parentheses", "/path/to/file(1).png", "/path/to/file\\(1\\).png"},
		{"single quote", "/path/to/'quoted'.png", "/path/to/\\'quoted\\'.png"},
		{"double quote", "/path/to/\"quoted\".png", "/path/to/\\\"quoted\\\".png"},
		{"backslash", "/path/to/file\\.png", "/path/to/file\\\\.png"},
		{"empty string", "", ""},
		{"complex path", "/Users/test/My Documents/Photos (2024)/image.png", "/Users/test/My\\ Documents/Photos\\ \\(2024\\)/image.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeShellPath(tt.input)
			if got != tt.expected {
				t.Errorf("escapeShellPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEscapeUnescapeRoundTrip(t *testing.T) {
	// Test that escape and unescape are inverses
	paths := []string{
		"/path/to/file.png",
		"/path/to/file name.png",
		"/path/to/file(1).png",
		"/path/with spaces and (parens).png",
		"/path/to/'quoted'.png",
		"/path/to/\"double quoted\".png",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			escaped := escapeShellPath(path)
			unescaped := unescapeShellPath(escaped)
			if unescaped != path {
				t.Errorf("roundtrip failed: %q -> %q -> %q", path, escaped, unescaped)
			}
		})
	}
}

func TestSplitShellPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single path", "/path/to/file.png", []string{"/path/to/file.png"}},
		{"two paths", "/path/one.png /path/two.png", []string{"/path/one.png", "/path/two.png"}},
		{"path with escaped space", "/path/to/my\\ file.png", []string{"/path/to/my\\ file.png"}},
		{"mixed paths", "/a.png /path/with\\ space.png /b.png", []string{"/a.png", "/path/with\\ space.png", "/b.png"}},
		{"empty string", "", nil},
		{"multiple spaces between", "/a.png   /b.png", []string{"/a.png", "/b.png"}},
		{"trailing space", "/path/to/file.png ", []string{"/path/to/file.png"}},
		{"leading space", " /path/to/file.png", []string{"/path/to/file.png"}},
		{"escaped backslash before space", "/path/to/file\\\\ name.png", []string{"/path/to/file\\\\", "name.png"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitShellPaths(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("splitShellPaths(%q) returned %d items %v, want %d items %v", tt.input, len(got), got, len(tt.expected), tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("splitShellPaths(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// Mock ClipboardSyncer for testing
type mockClipboardSyncer struct {
	syncCalled bool
	syncError  error
}

func (m *mockClipboardSyncer) Sync() error {
	m.syncCalled = true
	return m.syncError
}

func TestNewInterceptor(t *testing.T) {
	source := bytes.NewReader([]byte("test"))
	mock := &mockClipboardSyncer{}

	interceptor := NewInterceptor(source, mock, "/bridge", "/container-bridge", nil)

	if interceptor == nil {
		t.Fatal("NewInterceptor returned nil")
	}
	if interceptor.source != source {
		t.Error("source not set correctly")
	}
	if interceptor.clipSync != mock {
		t.Error("clipSync not set correctly")
	}
	if interceptor.bridgeDir != "/bridge" {
		t.Errorf("bridgeDir = %q, want /bridge", interceptor.bridgeDir)
	}
	if interceptor.containerBridgeDir != "/container-bridge" {
		t.Errorf("containerBridgeDir = %q, want /container-bridge", interceptor.containerBridgeDir)
	}
}

func TestInterceptor_CtrlVTriggersSync(t *testing.T) {
	mock := &mockClipboardSyncer{}
	source := bytes.NewReader([]byte{0x16}) // Ctrl+V

	interceptor := NewInterceptor(source, mock, "", "", nil)

	buf := make([]byte, 10)
	n, err := interceptor.Read(buf)

	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	if !mock.syncCalled {
		t.Error("Expected Sync() to be called on Ctrl+V")
	}

	if n < 1 || buf[0] != 0x16 {
		t.Errorf("Expected Ctrl+V byte to pass through, got n=%d, buf[0]=%d", n, buf[0])
	}
}

func TestInterceptor_CtrlVWithNilSyncer(t *testing.T) {
	source := bytes.NewReader([]byte{0x16}) // Ctrl+V

	interceptor := NewInterceptor(source, nil, "", "", nil)

	buf := make([]byte, 10)
	n, err := interceptor.Read(buf)

	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	if n < 1 || buf[0] != 0x16 {
		t.Errorf("Expected Ctrl+V byte to pass through, got n=%d, buf[0]=%d", n, buf[0])
	}
}

func TestInterceptor_RegularDataPassthrough(t *testing.T) {
	testData := []byte("hello world")
	source := bytes.NewReader(testData)
	mock := &mockClipboardSyncer{}

	interceptor := NewInterceptor(source, mock, "", "", nil)

	buf := make([]byte, 100)
	n, err := interceptor.Read(buf)

	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	if !bytes.Equal(buf[:n], testData) {
		t.Errorf("Data not passed through correctly: got %q, want %q", buf[:n], testData)
	}

	if mock.syncCalled {
		t.Error("Sync() should not be called for regular data")
	}
}

func TestInterceptor_MultipleCtrlV(t *testing.T) {
	// Multiple Ctrl+V bytes in sequence
	source := bytes.NewReader([]byte{0x16, 'a', 0x16, 'b'})
	mock := &mockClipboardSyncer{}

	interceptor := NewInterceptor(source, mock, "", "", nil)

	buf := make([]byte, 100)
	n, err := interceptor.Read(buf)

	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []byte{0x16, 'a', 0x16, 'b'}
	if !bytes.Equal(buf[:n], expected) {
		t.Errorf("Data not passed through correctly: got %v, want %v", buf[:n], expected)
	}

	if !mock.syncCalled {
		t.Error("Expected Sync() to be called")
	}
}

func TestInterceptor_BracketedPasteSequence(t *testing.T) {
	// Test bracketed paste without image paths - should pass through unchanged
	pasteContent := "regular text paste"
	input := bracketedStart + pasteContent + bracketedEnd

	source := bytes.NewReader([]byte(input))
	mock := &mockClipboardSyncer{}

	interceptor := NewInterceptor(source, mock, "", "", nil)

	buf := make([]byte, 200)
	n, err := interceptor.Read(buf)

	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	result := string(buf[:n])
	if result != input {
		t.Errorf("Bracketed paste not handled correctly: got %q, want %q", result, input)
	}
}

func TestInterceptor_DebugFunc(t *testing.T) {
	debugCalled := false
	debugFunc := func(format string, args ...any) {
		debugCalled = true
	}

	source := bytes.NewReader([]byte{0x16})
	mock := &mockClipboardSyncer{}

	interceptor := NewInterceptor(source, mock, "", "", debugFunc)

	buf := make([]byte, 10)
	_, _ = interceptor.Read(buf)

	if !debugCalled {
		t.Error("Debug function should have been called")
	}
}

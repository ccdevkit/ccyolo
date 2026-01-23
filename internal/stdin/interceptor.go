package stdin

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const (
	ctrlV          = byte(0x16)
	bracketedStart = "\x1B[200~"
	bracketedEnd   = "\x1B[201~"
)

// imageExtRegex matches image file extensions (same as Claude Code)
var imageExtRegex = regexp.MustCompile(`(?i)\.(png|jpe?g|gif|webp)$`)

// DebugFunc is a function for debug logging
type DebugFunc func(format string, args ...any)

// ClipboardSyncer syncs clipboard images to the container
type ClipboardSyncer interface {
	// Sync reads the host clipboard and sends any image to the container.
	// Returns nil if no image in clipboard or if sync successful.
	Sync() error
}

// Interceptor wraps stdin to intercept Ctrl+V and file paths in pastes
type Interceptor struct {
	source             io.Reader
	clipSync           ClipboardSyncer
	bridgeDir          string // Host-side bridge directory
	containerBridgeDir string // Container-side bridge directory
	debug              DebugFunc

	mu        sync.Mutex
	buffer    bytes.Buffer
	inPaste   bool
	pasteData bytes.Buffer
}

// NewInterceptor creates a new stdin interceptor.
// bridgeDir is the host-side directory for file bridging.
// containerBridgeDir is the path inside the container (e.g., /home/claude/.ccyolo-bridge)
// debug is an optional debug logging function (can be nil)
func NewInterceptor(source io.Reader, clipSync ClipboardSyncer, bridgeDir, containerBridgeDir string, debug DebugFunc) *Interceptor {
	if debug == nil {
		debug = func(format string, args ...any) {} // no-op
	}
	return &Interceptor{
		source:             source,
		clipSync:           clipSync,
		bridgeDir:          bridgeDir,
		containerBridgeDir: containerBridgeDir,
		debug:              debug,
	}
}

// Read implements io.Reader
func (i *Interceptor) Read(p []byte) (n int, err error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// If we have buffered data from processing, return that first
	if i.buffer.Len() > 0 {
		return i.buffer.Read(p)
	}

	// Read from source
	n, err = i.source.Read(p)
	if n == 0 {
		return n, err
	}

	// Process the data
	processed := i.process(p[:n])

	// Return processed data
	copy(p, processed)
	if len(processed) > len(p) {
		// Buffer overflow data for next read
		i.buffer.Write(processed[len(p):])
		return len(p), err
	}
	return len(processed), err
}

// process handles Ctrl+V detection and bracketed paste processing
func (i *Interceptor) process(data []byte) []byte {
	var result bytes.Buffer

	// Log raw bytes for debugging
	i.debug("Interceptor processing %d bytes: %v", len(data), data)

	for idx := 0; idx < len(data); idx++ {
		b := data[idx]

		// Check for bracketed paste sequences
		remaining := string(data[idx:])

		if strings.HasPrefix(remaining, bracketedStart) {
			i.debug("Detected bracketed paste start")
			i.inPaste = true
			i.pasteData.Reset()
			result.WriteString(bracketedStart)
			idx += len(bracketedStart) - 1
			continue
		}

		if strings.HasPrefix(remaining, bracketedEnd) {
			i.debug("Detected bracketed paste end, content length: %d", i.pasteData.Len())
			i.inPaste = false
			// Process the pasted content for file paths
			processed := i.processPastedContent(i.pasteData.Bytes())
			result.Write(processed)
			result.WriteString(bracketedEnd)
			idx += len(bracketedEnd) - 1
			continue
		}

		if i.inPaste {
			i.pasteData.WriteByte(b)
			continue
		}

		// Check for Ctrl+V outside of bracketed paste
		if b == ctrlV {
			i.debug("Detected Ctrl+V (0x16), triggering clipboard sync")
			// Sync clipboard before forwarding the keystroke
			if i.clipSync != nil {
				if err := i.clipSync.Sync(); err != nil {
					i.debug("Clipboard sync error: %v", err)
				} else {
					i.debug("Clipboard sync completed successfully")
				}
			} else {
				i.debug("No clipboard syncer configured")
			}
		}

		result.WriteByte(b)
	}

	// If still in paste mode, we need to buffer and wait for more data
	// For simplicity, just write what we have (paste may span multiple reads)
	if i.inPaste {
		result.Write(i.pasteData.Bytes())
		i.pasteData.Reset()
	}

	return result.Bytes()
}

// processPastedContent checks for file paths and bridges them
func (i *Interceptor) processPastedContent(data []byte) []byte {
	content := string(data)
	i.debug("Processing pasted content: %q", content)

	// The pasted content may be a single path with escaped spaces (e.g., /path/to/file\ name.png)
	// or multiple space-separated paths. We need to handle both cases.

	// First, try to parse the entire content as a single shell-escaped path
	// Trim trailing whitespace (terminals often add a trailing space)
	content = strings.TrimRight(content, " \t\n\r")
	unescaped := unescapeShellPath(content)
	i.debug("Unescaped path: %q", unescaped)

	// Check if it looks like an image path
	if looksLikeImagePath(unescaped) {
		i.debug("Looks like image path, checking if exists: %q", unescaped)
		if _, err := os.Stat(unescaped); err == nil {
			i.debug("File exists, bridging: %q", unescaped)
			bridgedPath, err := i.bridgeFile(unescaped)
			if err == nil {
				i.debug("Bridged to: %q", bridgedPath)
				// Return the bridged path, re-escaped for shell
				return []byte(escapeShellPath(bridgedPath))
			}
			i.debug("Bridge error: %v", err)
		} else {
			i.debug("File does not exist: %v", err)
		}
	}

	// If single-path approach didn't work, try splitting on unescaped spaces
	// This handles cases like: /path/one.png /path/two.png
	parts := splitShellPaths(content)
	if len(parts) <= 1 {
		return data // Nothing to process
	}

	modified := false
	for idx, part := range parts {
		unescaped := unescapeShellPath(part)
		if !looksLikeImagePath(unescaped) {
			continue
		}
		if _, err := os.Stat(unescaped); err != nil {
			continue
		}
		bridgedPath, err := i.bridgeFile(unescaped)
		if err != nil {
			continue
		}
		parts[idx] = escapeShellPath(bridgedPath)
		modified = true
	}

	if !modified {
		return data
	}

	return []byte(strings.Join(parts, " "))
}

// bridgeFile copies a file to the bridge directory and returns the container path
func (i *Interceptor) bridgeFile(hostPath string) (string, error) {
	if i.bridgeDir == "" || i.containerBridgeDir == "" {
		return hostPath, nil // No bridging configured
	}

	// Ensure bridge directory exists
	if err := os.MkdirAll(i.bridgeDir, 0755); err != nil {
		return hostPath, err
	}

	// Get the filename
	filename := filepath.Base(hostPath)

	// Copy file to bridge directory
	srcData, err := os.ReadFile(hostPath)
	if err != nil {
		return hostPath, err
	}

	destPath := filepath.Join(i.bridgeDir, filename)
	if err := os.WriteFile(destPath, srcData, 0644); err != nil {
		return hostPath, err
	}

	// Return the container-side path
	return filepath.Join(i.containerBridgeDir, filename), nil
}

// looksLikeImagePath returns true if the string looks like an image file path
// that Claude Code would recognize
func looksLikeImagePath(s string) bool {
	// Must start with a path prefix
	if !strings.HasPrefix(s, "/") &&
		!strings.HasPrefix(s, "./") &&
		!strings.HasPrefix(s, "../") &&
		!strings.HasPrefix(s, "~/") {
		return false
	}
	// Must have an image extension (same as Claude Code)
	return imageExtRegex.MatchString(s)
}

// unescapeShellPath converts shell-escaped path to regular path
// e.g., "/path/to/file\ name.png" -> "/path/to/file name.png"
func unescapeShellPath(s string) string {
	// Handle backslash-escaped characters
	result := strings.ReplaceAll(s, "\\ ", " ")
	result = strings.ReplaceAll(result, "\\(", "(")
	result = strings.ReplaceAll(result, "\\)", ")")
	result = strings.ReplaceAll(result, "\\'", "'")
	result = strings.ReplaceAll(result, "\\\"", "\"")
	result = strings.ReplaceAll(result, "\\\\", "\\")
	return result
}

// escapeShellPath escapes a path for shell use
func escapeShellPath(s string) string {
	// Escape spaces and special characters
	result := strings.ReplaceAll(s, "\\", "\\\\")
	result = strings.ReplaceAll(result, " ", "\\ ")
	result = strings.ReplaceAll(result, "(", "\\(")
	result = strings.ReplaceAll(result, ")", "\\)")
	result = strings.ReplaceAll(result, "'", "\\'")
	result = strings.ReplaceAll(result, "\"", "\\\"")
	return result
}

// splitShellPaths splits a string on unescaped spaces
// This handles paths like: /path/one.png /path/two.png
// but keeps /path/to/file\ name.png as a single path
func splitShellPaths(s string) []string {
	var parts []string
	var current strings.Builder
	escaped := false

	for _, r := range s {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if r == ' ' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

package clipboard

import (
	"fmt"
	"runtime"

	"golang.design/x/clipboard"
)

// DebugFunc is a function for debug logging
type DebugFunc func(format string, args ...any)

var debugFn DebugFunc = func(format string, args ...any) {} // no-op default

// SetDebug sets the debug logging function
func SetDebug(fn DebugFunc) {
	if fn != nil {
		debugFn = fn
	}
}

// Init initializes the clipboard library. Must be called before ReadImage.
// Returns error if initialization fails, including when built without CGO.
func Init() (err error) {
	debugFn("clipboard.Init: initializing clipboard library")

	// The clipboard library panics when built with CGO_ENABLED=0
	// Recover from this panic and return an error instead
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("clipboard not available: %v", r)
			debugFn("clipboard.Init: panic recovered: %v", r)
		}
	}()

	err = clipboard.Init()
	if err != nil {
		debugFn("clipboard.Init: failed: %v", err)
	} else {
		debugFn("clipboard.Init: success")
	}
	return err
}

// ReadImage reads image data from the system clipboard.
// Returns PNG bytes if an image is present, nil if no image.
func ReadImage() ([]byte, error) {
	debugFn("clipboard.ReadImage: reading image from clipboard")
	data := clipboard.Read(clipboard.FmtImage)
	if len(data) == 0 {
		debugFn("clipboard.ReadImage: no image data (len=0)")
		return nil, nil
	}
	debugFn("clipboard.ReadImage: got %d bytes, first 16: %v", len(data), firstN(data, 16))
	return data, nil
}

// HasImage returns true if the clipboard contains an image.
func HasImage() bool {
	data := clipboard.Read(clipboard.FmtImage)
	return len(data) > 0
}

// firstN returns the first n bytes of data (for logging)
func firstN(data []byte, n int) []byte {
	if len(data) <= n {
		return data
	}
	return data[:n]
}

// PlatformInfo returns information about clipboard support on this platform.
func PlatformInfo() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS: using NSPasteboard"
	case "linux":
		return "Linux: requires X11 (xclip/xsel) or Wayland (wl-paste)"
	case "windows":
		return "Windows: using Win32 clipboard API"
	default:
		return fmt.Sprintf("Unknown platform: %s", runtime.GOOS)
	}
}

//go:build windows

package docker

import "os"

func handleResize(ptmx *os.File, debug DebugFunc) func() {
	// Windows: resize handling not supported with PTY
	// The creack/pty package has limited Windows support
	return func() {}
}

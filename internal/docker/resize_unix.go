//go:build !windows

package docker

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
)

func handleResize(p PTY, debug DebugFunc) func() {
	// Type assert to get access to the underlying file for pty.InheritSize
	up, ok := p.(*unixPTY)
	if !ok {
		debug("Warning: PTY is not a unixPTY, resize not supported")
		return func() {}
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, up.File()); err != nil {
				debug("Error resizing pty: %v", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize
	return func() { signal.Stop(ch); close(ch) }
}

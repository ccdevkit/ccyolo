//go:build !windows

package docker

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
)

func handleResize(ptmx *os.File, debug DebugFunc) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				debug("Error resizing pty: %v", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize
	return func() { signal.Stop(ch); close(ch) }
}

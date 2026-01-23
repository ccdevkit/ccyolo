//go:build windows

package docker

import (
	"os"
	"time"

	"golang.org/x/term"
)

func handleResize(p PTY, debug DebugFunc) func() {
	// Get initial terminal size and apply it
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err == nil {
		p.Resize(uint16(rows), uint16(cols))
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		lastRows, lastCols := rows, cols
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				newCols, newRows, err := term.GetSize(int(os.Stdin.Fd()))
				if err != nil {
					continue
				}
				if newRows != lastRows || newCols != lastCols {
					if debug != nil {
						debug("resize", "rows=%d cols=%d", newRows, newCols)
					}
					p.Resize(uint16(newRows), uint16(newCols))
					lastRows, lastCols = newRows, newCols
				}
			}
		}
	}()
	return func() { close(done) }
}

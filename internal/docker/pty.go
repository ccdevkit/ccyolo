package docker

import "io"

// PTY represents a running pseudo-terminal with an attached command
type PTY interface {
	io.ReadWriteCloser
	Resize(rows, cols uint16) error
	Wait() error
}

// newPTY is implemented per-platform in pty_unix.go and pty_windows.go
// func newPTY(cmd *exec.Cmd) (PTY, error)

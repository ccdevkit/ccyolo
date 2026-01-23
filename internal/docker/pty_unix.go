//go:build !windows

package docker

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixPTY struct {
	ptmx *os.File
	cmd  *exec.Cmd
}

func newPTY(cmd *exec.Cmd) (PTY, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &unixPTY{ptmx: ptmx, cmd: cmd}, nil
}

func (p *unixPTY) Read(b []byte) (int, error)  { return p.ptmx.Read(b) }
func (p *unixPTY) Write(b []byte) (int, error) { return p.ptmx.Write(b) }
func (p *unixPTY) Close() error                { return p.ptmx.Close() }
func (p *unixPTY) Wait() error                 { return p.cmd.Wait() }

func (p *unixPTY) Resize(rows, cols uint16) error {
	return pty.Setsize(p.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// File returns the underlying file for use with pty.InheritSize
func (p *unixPTY) File() *os.File { return p.ptmx }

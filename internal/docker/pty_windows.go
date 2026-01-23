//go:build windows

package docker

import (
	"os/exec"

	gopty "github.com/aymanbagabas/go-pty"
)

type windowsPTY struct {
	pty gopty.Pty
	cmd *gopty.Cmd
}

func newPTY(cmd *exec.Cmd) (PTY, error) {
	p, err := gopty.New()
	if err != nil {
		return nil, err
	}
	ptyCmd := p.Command(cmd.Path, cmd.Args[1:]...)
	ptyCmd.Env = cmd.Env
	ptyCmd.Dir = cmd.Dir
	if err := ptyCmd.Start(); err != nil {
		p.Close()
		return nil, err
	}
	return &windowsPTY{pty: p, cmd: ptyCmd}, nil
}

func (p *windowsPTY) Read(b []byte) (int, error)  { return p.pty.Read(b) }
func (p *windowsPTY) Write(b []byte) (int, error) { return p.pty.Write(b) }
func (p *windowsPTY) Close() error                { return p.pty.Close() }
func (p *windowsPTY) Wait() error                 { return p.cmd.Wait() }

func (p *windowsPTY) Resize(rows, cols uint16) error {
	// go-pty uses (width, height) = (cols, rows), our interface uses (rows, cols)
	return p.pty.Resize(int(cols), int(rows))
}

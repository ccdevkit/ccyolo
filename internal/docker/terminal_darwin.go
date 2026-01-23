//go:build darwin

package docker

import "syscall"

const (
	ioctlReadTermios  = uintptr(syscall.TIOCGETA)
	ioctlWriteTermios = uintptr(syscall.TIOCSETA)
)

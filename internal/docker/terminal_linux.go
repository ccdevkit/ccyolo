//go:build linux

package docker

import "golang.org/x/sys/unix"

const (
	ioctlReadTermios  = uintptr(unix.TCGETS)
	ioctlWriteTermios = uintptr(unix.TCSETS)
)

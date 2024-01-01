//go:build windows

package water

import (
	"syscall"
)

func platformSpecificFd(fd uintptr) syscall.Handle {
	return syscall.Handle(fd)
}

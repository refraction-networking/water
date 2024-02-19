//go:build wasi || wasip1

package net

import (
	"syscall"
	"unsafe"
)

func syscallSetNonblock(fd uintptr, nonblocking bool) error {
	flags, err := fd_fdstat_get_flags(int32(fd))
	if err != nil {
		return err
	}
	if nonblocking {
		flags |= FDFLAG_NONBLOCK
	} else {
		flags &^= FDFLAG_NONBLOCK
	}
	errno := fd_fdstat_set_flags(int32(fd), flags)
	return syscall.Errno(errno)
}

func fd_fdstat_get_flags(fd int32) (uint32, error) {
	var stat fdstat
	errno := fd_fdstat_get(fd, unsafe.Pointer(&stat))
	if errno != 0 {
		return 0, syscall.Errno(errno)
	}
	return uint32(stat.fdflags), nil
}

type fdstat struct {
	filetype         uint8
	fdflags          uint16
	rightsBase       uint64
	rightsInheriting uint64
}

const (
	FDFLAG_APPEND   = 0x0001
	FDFLAG_DSYNC    = 0x0002
	FDFLAG_NONBLOCK = 0x0004
	FDFLAG_RSYNC    = 0x0008
	FDFLAG_SYNC     = 0x0010
)

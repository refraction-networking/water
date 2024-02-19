//go:build windows

package net

import (
	"syscall"
	"unsafe"
)

const (
	_FIONBIO = 0x8004667e
)

var (
	// modws2_32 is WinSock.
	modws2_32 = syscall.NewLazyDLL("ws2_32.dll")
	// procioctlsocket exposes ioctlsocket from WinSock.
	procioctlsocket = modws2_32.NewProc("ioctlsocket")
)

func syscallSetNonblock(fd uintptr, nonblocking bool) (err error) {
	opt := uint64(0)
	if nonblocking {
		opt = 1
	}
	// ioctlsocket(fd, FIONBIO, &opt)
	_, _, errno := syscall.SyscallN(
		procioctlsocket.Addr(),
		uintptr(fd),
		uintptr(_FIONBIO),
		uintptr(unsafe.Pointer(&opt)))
	return errno
}

//go:build unix && !wasi && !wasip1

package net

import "syscall"

func syscallSetNonblock(fd uintptr, nonblocking bool) (err error) {
	return syscall.SetNonblock(int(fd), nonblocking)
}

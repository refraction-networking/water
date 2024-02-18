package net

import (
	"fmt"
	"syscall"
)

func syscallControlFd(rawConn syscall.RawConn, f func(fd uintptr) error) (err error) {
	if controlErr := rawConn.Control(func(fd uintptr) {
		err = f(fd)
	}); controlErr != nil {
		panic(fmt.Sprintf("controlErr = %v", controlErr))
		return controlErr
	}
	return err
}

func syscallFnFd(rawConn syscall.RawConn, f func(fd uintptr) (int, error)) (n int, err error) {
	if controlErr := rawConn.Control(func(fd uintptr) {
		n, err = f(fd)
	}); controlErr != nil {
		return 0, controlErr
	}
	return n, err
}

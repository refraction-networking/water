//go:build wasip1 || wasi

package main

import "unsafe"

//go:wasmimport wasi_snapshot_preview1 poll_oneoff
//go:noescape
func poll_oneoff(in, out unsafe.Pointer, nsubscriptions size, nevents unsafe.Pointer) errno

func main() {}

//export poll
func poll(fd1, fd2, fd3 int32) int32 {
	n, err := _poll([]pollFd{
		{fd: uintptr(fd1), events: EventFdRead},
		{fd: uintptr(fd2), events: EventFdRead},
		{fd: uintptr(fd3), events: EventFdWrite},
	}, -1)
	if err != nil {
		panic(err)
	}

	return n
}

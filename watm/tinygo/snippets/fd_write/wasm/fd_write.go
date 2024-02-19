//go:build wasip1 || wasi

package main

import "syscall"

// Write "Hello, World!" to the given file descriptor and return the number of
// bytes written.
//
//export hello
func hello(fd int) int {
	n, err := syscall.Write(fd, []byte("Hello, World!\n"))
	if err != nil {
		panic(err)
	}

	return n
}

// Read from the given file descriptor and write to stdout.
//
//export world
func world(fd int) int {
	buf := make([]byte, 64)
	n, err := syscall.Read(fd, buf)
	if err != nil {
		panic(err)
	}

	buf = append([]byte("host: "), buf[:n]...)
	nWr, err := syscall.Write(syscall.Stdout, buf[:n+6])
	if err != nil {
		panic(err)
	}

	if nWr != n+6 {
		panic("nWr != n+6")
	}

	return n
}

// main is required for the `wasi` target, even if it isn't used.
func main() {}

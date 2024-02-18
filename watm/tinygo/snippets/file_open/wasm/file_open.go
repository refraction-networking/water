//go:build wasip1 || wasi

package main

import (
	"fmt"
	"os"
)

// Open the file with name defined by os.Args[0] and write "Hello, World!" to it.
//
//export open_exist_file
func openExistFile() {
	file, err := os.OpenFile(os.Args[0], os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}

	n, err := file.Write([]byte("Hello, World!\n"))
	if err != nil {
		panic(err)
	}

	if n != 14 {
		panic("n != 14")
	}

	if err := file.Close(); err != nil {
		panic(err)
	}
}

// Open the file with name os.Args[1] and write "Hello, Void!" to it.
//
//export open_new_file
func openNewFile() {
	file, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}

	n, err := file.Write([]byte("Hello, Void!\n"))
	if err != nil {
		panic(err)
	}

	if n != 13 {
		panic("n != 13")
	}

	if err := file.Close(); err != nil {
		panic(err)
	}
}

// Wrap the file descriptor fd into a file object, and write
// "Hello, FD!" to it.
//
//export open_fd
func openFd(fd uintptr) {
	file := os.NewFile(fd, "fd")
	if file == nil {
		panic("file is nil")
	}

	n, err := file.Write([]byte("Hello, FD!\n"))
	if err != nil {
		panic(err)
	}

	if n != 11 {
		panic("n != 11")
	}

	if err := file.Close(); err != nil {
		panic(err)
	}
}

// Wrap the bad file descriptor fd into a file object, and write
// "Not you, bad FD!" to it.
//
// This function will panic if the write succeeds.
//
//export open_bad_fd
func openBadFd(fd uintptr) {
	file := os.NewFile(fd, "bad fd")
	if file == nil {
		fmt.Println("file is nil, cannot open bad fd")
		return
	}

	var writeMsg = []byte("Not you, bad FD!\n")
	n, err := file.Write(writeMsg)
	// n, err := fmt.Fprint(file, string(writeMsg))
	if err != nil {
		fmt.Println("file opened but write failed:", err)
		return
	}

	if n != len(writeMsg) {
		fmt.Printf("file opened but write failed: n == %d != %d", n, len(writeMsg))
		return
	}

	panic("writing to a bad fd succeeded")
}

// main is required for the `wasi` target, even if it isn't used.
func main() {}

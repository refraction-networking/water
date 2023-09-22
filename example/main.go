package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"sync"
)

func main() {
	// get temp dir
	tempDir := os.TempDir()
	// append OS-specific path separator
	tempDir += string(os.PathSeparator)
	// randomize a socket name
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		panic(fmt.Errorf("water: rand.Read returned error: %w", err))
	}
	tempDir += fmt.Sprintf("%x", randBytes)

	// create a one-time use UnixListener
	ul, err := net.Listen("unix", tempDir)
	if err != nil {
		panic(fmt.Errorf("water: net.Listen returned error: %w", err))
	}
	defer ul.Close()

	var waConn net.Conn
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		waConn, err = ul.Accept()
		if err != nil {
			panic(fmt.Errorf("water: ul.Accept returned error: %w", err))
		}
	}()

	// dial the one-time use UnixListener
	uc, err := net.Dial("unix", ul.Addr().String())
	if err != nil {
		panic(fmt.Errorf("water: net.Dial returned error: %w", err))
	}
	defer uc.Close()
	wg.Wait()

	// write to uc, read from waConn
	uc.Write([]byte("hello"))
	buf := make([]byte, 128)
	n, err := waConn.Read(buf)
	if err != nil {
		panic(fmt.Errorf("water: waConn.Read returned error: %w", err))
	}
	fmt.Printf("read %d bytes from waConn: %s\n", n, string(buf[:n]))

	// write to waConn, read from uc
	waConn.Write([]byte("world"))
	n, err = uc.Read(buf)
	if err != nil {
		panic(fmt.Errorf("water: uc.Read returned error: %w", err))
	}
	fmt.Printf("read %d bytes from uc: %s\n", n, string(buf[:n]))
}

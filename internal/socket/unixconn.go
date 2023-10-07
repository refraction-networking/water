package socket

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// UnixConnWrap wraps an io.Reader/io.Writer/io.ReadWriteCloser
// interface into a UnixConn.
//
// This function spins up either one or two goroutines to copy
// data between the ReadWriteCloser and the UnixConn. Anything
// written to the UnixConn by caller will be written to the
// wrapped object if the object implements io.Writer, and if
// the object implements io.Reader, anything read by goroutine
// from the wrapped object will be readable from the UnixConn
// by caller.
//
// Once this function is invoked, the caller should not perform I/O
// operations on the ReadWriteCloser anymore.
func UnixConnWrap(obj any) (*net.UnixConn, error) {
	// randomize the name of the socket
	var randName []byte = make([]byte, 8) // 8-byte so 16-char hex string, 64-bit randomness is good enough
	if _, err := rand.Read(randName); err != nil {
		return nil, err
	}
	socketName := hex.EncodeToString(randName)

	// listen on the socket
	unixAddr, err := net.ResolveUnixAddr("unix", os.TempDir()+"/"+string(socketName))
	if err != nil {
		return nil, err
	}
	unixListener, err := net.ListenUnix("unix", unixAddr)
	if err != nil {
		return nil, err
	}
	defer unixListener.Close() // we will no longer need this listener since the name is not recorded anywhere

	// spin up a goroutine to wait for listening
	var unixConn *net.UnixConn
	var acceptErr error
	acceptWg := &sync.WaitGroup{}
	acceptWg.Add(1)
	go func() {
		defer acceptWg.Done()
		unixConn, acceptErr = unixListener.AcceptUnix() // so caller will have the accepted connection
		if acceptErr != nil {
			return
		}
	}()

	// reverseUnixConn is used to access the unixConn's read/write buffer:
	// - writing to reverseUnixConn = save to unixConn's read buffer
	// - reading from reverseUnixConn = read from unixConn's write buffer
	reverseUnixConn, err := net.DialUnix("unix", nil, unixAddr)
	if err != nil {
		return nil, err
	}
	acceptWg.Wait() // wait for the goroutine to accept the connection
	if acceptErr != nil {
		return nil, acceptErr
	}

	// if the object implements io.Reader: read from the object and write to the reverseUnixConn
	if reader, ok := obj.(io.Reader); ok {
		go func() {
			io.Copy(reverseUnixConn, reader)
			// when the src is closed, we will close the dst
			time.Sleep(1 * time.Millisecond)
			reverseUnixConn.Close()
		}()
	}

	// if the object implements io.Writer: read from the reverseUnixConn and write to the object
	if writer, ok := obj.(io.Writer); ok {
		go func() {
			io.Copy(writer, reverseUnixConn)
			// when the src is closed, we will close the dst
			if closer, ok := obj.(io.Closer); ok {
				time.Sleep(1 * time.Millisecond)
				closer.Close()
			}
		}()
	}

	return unixConn, nil
}

func UnixConnPair(path string) (c1, c2 net.Conn, err error) {
	unixPath := path
	if path == "" {
		// randomize a socket name
		randBytes := make([]byte, 16)
		if _, err := rand.Read(randBytes); err != nil {
			return nil, nil, fmt.Errorf("crypto/rand.Read returned error: %w", err)
		}
		unixPath = os.TempDir() + string(os.PathSeparator) + fmt.Sprintf("%x", randBytes)
	}

	// create a one-time use UnixListener
	ul, err := net.Listen("unix", unixPath)
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}
	defer ul.Close()

	var wg *sync.WaitGroup = new(sync.WaitGroup)
	var goroutineErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		c2, goroutineErr = ul.Accept()
	}()

	// dial the one-time use UnixListener
	c1, err = net.Dial("unix", ul.Addr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("net.Dial returned error: %w", err)
	}
	wg.Wait()

	if goroutineErr != nil {
		return nil, nil, fmt.Errorf("ul.Accept returned error: %w", goroutineErr)
	}

	if c1 == nil || c2 == nil {
		return nil, nil, fmt.Errorf("c1 or c2 is nil")
	}

	return c1, c2, nil
}

func TCPConnPair(address string) (c1, c2 net.Conn, err error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}
	defer l.Close()

	var wg *sync.WaitGroup = new(sync.WaitGroup)
	var goroutineErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		c2, goroutineErr = l.Accept()
	}()

	c1, err = net.Dial("tcp", l.Addr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("net.Dial returned error: %w", err)
	}
	wg.Wait()

	if goroutineErr != nil {
		return nil, nil, fmt.Errorf("l.Accept returned error: %w", goroutineErr)
	}

	if c1 == nil || c2 == nil {
		return nil, nil, fmt.Errorf("c1 or c2 is nil")
	}

	return c1, c2, nil
}

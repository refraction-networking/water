package socket

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gaukas/water/internal/log"
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
	// get a pair of connected UnixConn
	unixConn, reverseUnixConn, err := UnixConnPair()
	if err != nil && (unixConn == nil || reverseUnixConn == nil) {
		return nil, err
	}

	// if the object implements io.Reader: read from the object and write to the reverseUnixConn
	if reader, ok := obj.(io.Reader); ok {
		go func() {
			_, _ = io.Copy(reverseUnixConn, reader)

			// when the src is closed, we will close the dst
			time.Sleep(1 * time.Millisecond)
			log.Debugf("closing reverseUnixConn and unixConn")
			_ = reverseUnixConn.Close()
			_ = unixConn.Close()
		}()
	}

	// if the object implements io.Writer: read from the reverseUnixConn and write to the object
	if writer, ok := obj.(io.Writer); ok {
		go func() {
			_, _ = io.Copy(writer, reverseUnixConn)
			// when the src is closed, we will close the dst
			if closer, ok := obj.(io.Closer); ok {
				time.Sleep(1 * time.Millisecond)
				log.Debugf("closing obj")
				_ = closer.Close()
			}
		}()
	}

	return unixConn, nil
}

// UnixConnFileWrap wraps an object into a *os.File from an
// underlying net.UnixConn. The object must implement io.Reader
// and/or io.Writer.
//
// If the object implements io.Reader, upon completing copying
// the object to the returned *os.File, the callback functions
// will be called.
//
// It is caller's responsibility to close the returned *os.File.
func UnixConnFileWrap(obj any, callbacks ...func()) (*os.File, error) {
	// get a pair of connected UnixConn
	unixConn, reverseUnixConn, err := UnixConnPair()
	if err != nil && (unixConn == nil || reverseUnixConn == nil) {
		return nil, err
	}

	unixConnFile, err := unixConn.File()
	if err != nil {
		return nil, err
	}

	// if the object implements io.Reader: read from the object and write to the reverseUnixConn
	if reader, ok := obj.(io.Reader); ok {
		go func() {
			_, _ = io.Copy(reverseUnixConn, reader)
			// when the src is closed, we will close the dst
			time.Sleep(1 * time.Millisecond)
			log.Debugf("closing reverseUnixConn and unixConn")
			for _, f := range callbacks {
				f()
			}
			_ = reverseUnixConn.Close()
			_ = unixConn.Close()
		}()
	}

	// if the object implements io.Writer: read from the reverseUnixConn and write to the object
	if writer, ok := obj.(io.Writer); ok {
		go func() {
			_, _ = io.Copy(writer, reverseUnixConn)
			// when the src is closed, we will close the dst
			if closer, ok := obj.(io.Closer); ok {
				time.Sleep(1 * time.Millisecond)
				log.Debugf("closing obj")
				_ = closer.Close()
			}
		}()
	}

	return unixConnFile, nil
}

// UnixConnPair returns a pair of connected net.UnixConn.
func UnixConnPair(path ...string) (*net.UnixConn, *net.UnixConn, error) {
	var c1, c2 net.Conn

	unixPath := ""
	if len(path) == 0 || path[0] == "" {
		// randomize a socket name
		randBytes := make([]byte, 16)
		if _, err := rand.Read(randBytes); err != nil {
			return nil, nil, fmt.Errorf("crypto/rand.Read returned error: %w", err)
		}
		unixPath = os.TempDir() + string(os.PathSeparator) + fmt.Sprintf("%x", randBytes)
	} else {
		unixPath = path[0]
	}

	// create a one-time use UnixListener
	ul, err := net.Listen("unix", unixPath)
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}

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

	// type assertion
	if uc1, ok := c1.(*net.UnixConn); ok {
		if uc2, ok := c2.(*net.UnixConn); ok {
			return uc1, uc2, ul.Close()
		} else {
			return nil, nil, fmt.Errorf("c2 is not *net.UnixConn")
		}
	} else {
		return nil, nil, fmt.Errorf("c1 is not *net.UnixConn")
	}
}

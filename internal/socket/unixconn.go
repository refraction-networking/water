package socket

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/refraction-networking/water/internal/log"
)

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

// UnixConnWrap wraps an io.Reader/io.Writer/io.Closer
// interface into a UnixConn.
//
// This function spins up goroutine(s) to copy data between the
// ReadWrite(Close)r and the UnixConn. Anything written to the
// UnixConn by caller will be written to the wrapped object if
// the object implements io.Writer, and if the object implements
// io.Reader, anything read by goroutine from the wrapped object
// will be readable from the UnixConn by caller.
//
// Once this function is invoked, the caller should not perform I/O
// operations on the wrapped connection anymore.
//
// The returned context.Context can be used to check if the connection
// is still alive. If the connection is closed, the context will be
// canceled.
func UnixConnWrap(wrapped any) (wrapperConn *net.UnixConn, ctxCancel context.Context, err error) {
	// get a pair of connected UnixConn
	unixConn, reverseUnixConn, err := UnixConnPair()
	if err != nil && (unixConn == nil || reverseUnixConn == nil) {
		return nil, nil, err
	}

	var cancel context.CancelFunc
	ctxCancel, cancel = context.WithCancel(context.Background())

	reader, readerOk := wrapped.(io.Reader)
	writer, writerOk := wrapped.(io.Writer)
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	if !readerOk && !writerOk {
		cancel()
		return nil, nil, fmt.Errorf("wrapped does not implement io.Reader nor io.Writer")
	} else if readerOk && !writerOk {
		// only reader is implemented
		log.Debugf("wrapped does not implement io.Writer, skipping copy from wrapped to wrapper")

		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = io.Copy(reverseUnixConn, reader) // unsafe: error is ignored
			_ = reverseUnixConn.Close()             // unsafe: error is ignored
			_ = unixConn.Close()                    // unsafe: error is ignored
		}(wg)
	} else if !readerOk && writerOk {
		// only writer is implemented
		log.Debugf("wrapped does not implement io.Reader, skipping copy from wrapper to wrapped")

		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = io.Copy(writer, reverseUnixConn) // unsafe: error is ignored
			// when the src is closed, we will close the dst (if implements io.Closer)
			if closer, ok := wrapped.(io.Closer); ok {
				_ = closer.Close() // unsafe: error is ignored
			}
		}(wg)
	} else {
		// both reader and writer are implemented
		wg.Add(2)

		// copy from wrapped to wrapper
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = io.Copy(reverseUnixConn, reader) // unsafe: error is ignored
			_ = reverseUnixConn.Close()             // unsafe: error is ignored
			_ = unixConn.Close()                    // unsafe: error is ignored
		}(wg)

		// copy from wrapper to wrapped
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = io.Copy(writer, reverseUnixConn) // unsafe: error is ignored
			// when the src is closed, we will close the dst (if implements io.Closer)
			if closer, ok := wrapped.(io.Closer); ok {
				_ = closer.Close() // unsafe: error is ignored
			}
		}(wg)
	}

	// spawn a goroutine to wait for all copying to finish
	go func(wg *sync.WaitGroup) {
		wg.Wait()
		cancel()

		// close again to make sure we don't forget to close anything
		// if io.Reader or io.Writer is not implemented.

		// close the reverseTCPConn
		_ = reverseUnixConn.Close() // unsafe: error is ignored

		// close the tcpConn
		_ = unixConn.Close() // unsafe: error is ignored

		// close the wrapped
		if closer, ok := wrapped.(io.Closer); ok {
			_ = closer.Close() // unsafe: error is ignored
		}
	}(wg)

	return unixConn, ctxCancel, nil
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
func UnixConnFileWrap(wrapped any) (wrapperFile *os.File, ctxCancel context.Context, err error) {
	unixWrapperConn, ctxCancel, err := UnixConnWrap(wrapped)
	if err != nil {
		return nil, nil, err
	}

	unixWrapperFile, err := unixWrapperConn.File()
	if err != nil {
		return nil, nil, fmt.Errorf("(*net.UnixConn).File returned error: %w", err)
	}

	return unixWrapperFile, ctxCancel, nil
}

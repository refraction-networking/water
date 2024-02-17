package socket

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/refraction-networking/water/internal/log"
)

// TCPConnPair returns a pair of connected net.TCPConn.
func TCPConnPair(address ...string) (c1, c2 *net.TCPConn, err error) {
	var addr string = "localhost:0" // use a localhost TCP connection by default
	if len(address) > 0 && address[0] != "" {
		addr = address[0]
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("net.ResolveTCPAddr returned error: %w", err)
	}

	l, err := net.ListenTCP("tcp", tcpAddr) // skipcq: GSC-G102
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen returned error: %w", err)
	}

	var wg *sync.WaitGroup = new(sync.WaitGroup)
	var goroutineErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		c2, goroutineErr = l.AcceptTCP()
	}()

	c1, err = net.DialTCP("tcp", nil, l.Addr().(*net.TCPAddr))
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

	return c1, c2, l.Close()
}

// TCPConnWrap wraps an io.Reader/io.Writer/io.Closer
// interface into a TCPConn.
//
// This function spins up goroutine(s) to copy data between the
// ReadWrite(Close)r and the TCPConn. Anything written to the
// TCPConn by caller will be written to the wrapped object if
// the object implements io.Writer, and if the object implements
// io.Reader, anything read by goroutine from the wrapped object
// will be readable from the TCPConn by caller.
//
// Once this function is invoked, the caller should not perform I/O
// operations on the wrapped connection anymore.
//
// The returned context.Context can be used to check if the connection
// is still alive. If the connection is closed, the context will be
// canceled.
func TCPConnWrap(wrapped any) (wrapperConn *net.TCPConn, ctxCancel context.Context, err error) {
	// get a pair of connected TCPConn
	tcpConn, reverseTCPConn, err := TCPConnPair()
	if err != nil && (tcpConn == nil || reverseTCPConn == nil) { // ignore error caused by closing TCP Listener
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
			_, _ = io.Copy(reverseTCPConn, reader) // unsafe: error is ignored
			_ = reverseTCPConn.Close()             // unsafe: error is ignored
			_ = tcpConn.Close()                    // unsafe: error is ignored
		}(wg)
	} else if !readerOk && writerOk {
		// only writer is implemented
		log.Debugf("wrapped does not implement io.Reader, skipping copy from wrapper to wrapped")

		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = io.Copy(writer, reverseTCPConn) // unsafe: error is ignored
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
			_, _ = io.Copy(reverseTCPConn, reader) // unsafe: error is ignored
			_ = reverseTCPConn.Close()             // unsafe: error is ignored
			_ = tcpConn.Close()                    // unsafe: error is ignored
		}(wg)

		// copy from wrapper to wrapped
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = io.Copy(writer, reverseTCPConn) // unsafe: error is ignored
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
		_ = reverseTCPConn.Close() // unsafe: error is ignored

		// close the tcpConn
		_ = tcpConn.Close() // unsafe: error is ignored

		// close the wrapped
		if closer, ok := wrapped.(io.Closer); ok {
			_ = closer.Close() // unsafe: error is ignored
		}
	}(wg)

	return tcpConn, ctxCancel, nil
}

// TCPConnFileWrap wraps an object into a *os.File from an
// underlying net.TCPConn. The object must implement io.Reader
// and/or io.Writer.
//
// If the object implements io.Reader, upon completing copying
// the object to the returned *os.File, the callback functions
// will be called.
//
// It is caller's responsibility to close the returned *os.File.
func TCPConnFileWrap(wrapped any) (wrapperFile *os.File, ctxCancel context.Context, err error) {
	tcpWrapperConn, ctxCancel, err := TCPConnWrap(wrapped)
	if err != nil {
		return nil, nil, err
	}

	tcpWrapperFile, err := tcpWrapperConn.File()
	if err != nil {
		return nil, nil, fmt.Errorf("(*net.TCPConn).File returned error: %w", err)
	}

	return tcpWrapperFile, ctxCancel, nil
}

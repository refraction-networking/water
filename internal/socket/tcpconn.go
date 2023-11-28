package socket

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gaukas/water/internal/log"
)

// TCPConnFileWrap wraps an object into a *os.File from an
// underlying net.TCPConn. The object must implement io.Reader
// and/or io.Writer.
//
// If the object implements io.Reader, upon completing copying
// the object to the returned *os.File, the callback functions
// will be called.
//
// It is caller's responsibility to close the returned *os.File.
func TCPConnFileWrap(obj any, callbacks ...func()) (*os.File, error) {
	// get a pair of connected UnixConn
	tcpConn, reverseTCPConn, err := TCPConnPair()
	if err != nil && (tcpConn == nil || reverseTCPConn == nil) {
		return nil, err
	}

	tcpConnFile, err := tcpConn.File()
	if err != nil {
		return nil, err
	}

	// if the object implements io.Reader: read from the object and write to the reverseUnixConn
	if reader, ok := obj.(io.Reader); ok {
		go func() {
			_, _ = io.Copy(reverseTCPConn, reader)
			// when the src is closed, we will close the dst
			time.Sleep(1 * time.Millisecond)
			log.Debugf("closing reverseTCPConn and tcpConn")
			for _, f := range callbacks {
				f()
			}
			_ = reverseTCPConn.Close()
			_ = tcpConn.Close()
		}()
	}

	// if the object implements io.Writer: read from the reverseUnixConn and write to the object
	if writer, ok := obj.(io.Writer); ok {
		go func() {
			_, _ = io.Copy(writer, reverseTCPConn)
			// when the src is closed, we will close the dst
			if closer, ok := obj.(io.Closer); ok {
				time.Sleep(1 * time.Millisecond)
				log.Debugf("closing obj")
				_ = closer.Close()
			}
		}()
	}

	return tcpConnFile, nil
}

// TCPConnPair returns a pair of connected net.TCPConn.
func TCPConnPair(address ...string) (c1, c2 *net.TCPConn, err error) {
	var addr string = ":0"
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

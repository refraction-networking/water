package net

import (
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

// Conn is the interface for a generic stream-oriented network connection.
type Conn interface {
	net.Conn
	syscall.Conn
	SetNonBlock(nonblocking bool) error
	Fd() int32
}

// type guard: *TCPConn must implement Conn
var _ Conn = (*TCPConn)(nil)

// TCPConn is a wrapper around a file descriptor that implements the [net.Conn].
//
// Despite the name, this type is not specific to TCP connections. It can be used
// for any file descriptor that is connection-oriented.
type TCPConn struct {
	rawConn *rawTCPConn

	readDeadline  time.Time
	writeDeadline time.Time
}

// RebuildTCPConn recovers a [TCPConn] from a file descriptor.
func RebuildTCPConn(fd int32) *TCPConn {
	return &TCPConn{
		rawConn: &rawTCPConn{
			fd: fd,
		},
	}
}

// Read implements [net.Conn.Read].
func (c *TCPConn) Read(b []byte) (n int, err error) {
	if rdl := c.readDeadline; rdl.IsZero() {
		// if no deadline set, behavior depends on blocking mode of the
		// underlying file descriptor.
		return syscallFnFd(c.rawConn, func(fd uintptr) (int, error) {
			n, err := syscall.Read(int(fd), b)
			if n == 0 && err == nil {
				err = io.EOF
			}
			if n < 0 && err != nil {
				n = 0
			}
			return n, err
		})
	} else {
		// readDeadline is set, if EAGAIN/EWOULDBLOCK is returned,
		// we retry until the deadline is reached.
		for {
			if n, err = syscallFnFd(c.rawConn, func(fd uintptr) (int, error) {
				n, err := syscall.Read(int(fd), b)
				if n == 0 && err == nil {
					err = io.EOF
				}
				if n < 0 && err != nil {
					n = 0
				}
				return n, err
			}); errors.Is(err, syscall.EAGAIN) {
				if time.Now().Before(rdl) {
					continue
				}
			}
			return n, err
		}
	}
}

// Write implements [net.Conn.Write].
func (c *TCPConn) Write(b []byte) (n int, writeErr error) {
	if wdl := c.writeDeadline; wdl.IsZero() {
		// if no deadline set, behavior depends on blocking mode of the
		// underlying file descriptor.
		return syscallFnFd(c.rawConn, func(fd uintptr) (int, error) {
			return syscall.Write(int(fd), b)
		})
	} else {
		// writeDeadline is set, if EAGAIN/EWOULDBLOCK is returned,
		// we retry until the deadline is reached.
		if err := c.rawConn.Write(func(fd uintptr) (done bool) {
			n, writeErr = syscall.Write(int(fd), b)
			if errors.Is(writeErr, syscall.EAGAIN) {
				if time.Now().Before(wdl) {
					return false
				}
				writeErr = os.ErrDeadlineExceeded
			}
			return true
		}); err != nil {
			return 0, err
		}
		return
	}
}

// Close implements [net.Conn.Close].
func (c *TCPConn) Close() error {
	// if shutdownErr := syscallControlFd(c.rawConn, func(fd uintptr) error {
	// 	return syscall.Shutdown(int(fd), syscall.SHUT_RDWR)
	// }); shutdownErr != nil {
	// 	return shutdownErr
	// } else {
	// 	return syscallControlFd(c.rawConn, func(fd uintptr) error {
	// 		return syscall.Close(int(fd))
	// 	})
	// }
	return syscallControlFd(c.rawConn, func(fd uintptr) error {
		return syscall.Close(int(fd))
	})
}

// LocalAddr implements [net.Conn.LocalAddr].
//
// This function is currently not implemented, but may be fleshed out in the
// future should there be better support for getting the local address of a
// socket managed by the Go runtime.
func (c *TCPConn) LocalAddr() net.Addr {
	return nil
}

// RemoteAddr implements [net.Conn.RemoteAddr].
//
// This function is currently not implemented, but may be fleshed out in the
// future should there be better support for getting the remote address of a
// socket managed by the Go runtime.
func (c *TCPConn) RemoteAddr() net.Addr {
	return nil
}

// SetDeadline implements [net.Conn.SetDeadline].
func (c *TCPConn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t

	// set deadline will enable non-blocking mode
	return c.SetNonBlock(true)
}

// SetReadDeadline implements [net.Conn.SetReadDeadline].
func (c *TCPConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t

	// set deadline will enable non-blocking mode
	return c.SetNonBlock(true)
}

// SetWriteDeadline implements [net.Conn.SetWriteDeadline].
func (c *TCPConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t

	return nil
}

// SyscallConn implements [syscall.Conn].
func (c *TCPConn) SyscallConn() (syscall.RawConn, error) {
	return c.rawConn, nil
}

// SetNonBlock sets the socket to blocking or non-blocking mode.
func (c *TCPConn) SetNonBlock(nonblocking bool) error {
	return syscallControlFd(c.rawConn, func(fd uintptr) error {
		if errno := syscallSetNonblock(fd, nonblocking); errno != nil && !errors.Is(errno, syscall.Errno(0)) {
			return errno
		} else {
			return nil
		}
	})
}

// Fd returns the file descriptor of the socket.
func (c *TCPConn) Fd() int32 {
	return c.rawConn.fd
}

// type guard: *rawTCPConn must implement [syscall.RawConn].
var _ syscall.RawConn = (*rawTCPConn)(nil)

// rawTCPConn is a wrapper around a file descriptor that implements the
// [syscall.RawConn] interface for some syscalls.
type rawTCPConn struct {
	fd int32
}

// Control implements [syscall.RawConn.Control].
func (rt *rawTCPConn) Control(f func(fd uintptr)) error {
	if rt.fd == 0 {
		return syscall.EBADF
	}

	f(uintptr(rt.fd))
	return nil
}

// Read implements [syscall.RawConn.Read].
//
// Deprecated: Use [net.Conn.Read] instead.
func (rt *rawTCPConn) Read(f func(fd uintptr) (done bool)) error {
	if rt.fd == 0 {
		return syscall.EBADF
	}

	for {
		if f(uintptr(rt.fd)) {
			return nil
		}
	}
}

// Write implements [syscall.RawConn.Write].
//
// Deprecated: Use [net.Conn.Write] instead.
func (rt *rawTCPConn) Write(f func(fd uintptr) (done bool)) error {
	if rt.fd == 0 {
		return syscall.EBADF
	}

	for {
		if f(uintptr(rt.fd)) {
			return nil
		}
	}
}

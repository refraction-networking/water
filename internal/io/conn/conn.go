package conn

import (
	"context"
	"io"
	"net"
	"time"
)

// Conn is an interface that represents a connection.
type Conn interface {
	io.ReadWriteCloser // Conn embeds io.ReadWriteCloser
}

// NetworkConn is an interface that represents a network connection.
type NetworkConn interface {
	Conn // NetworkConn embeds Conn

	// LocalAddr returns the local network address.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr
}

type DeadlineConn interface {
	Conn // embeds Conn

	// SetDeadline sets the read and write deadlines associated with the
	// connection.
	SetDeadline(time.Time) error

	// SetReadDeadline sets the read deadline associated with the
	// connection.
	SetReadDeadline(time.Time) error

	// SetWriteDeadline sets the write deadline associated with the
	// connection.
	SetWriteDeadline(time.Time) error
}

type NonblockingConn interface {
	Conn // embeds Conn

	// IsNonblock returns true if the connection is in non-blocking mode.
	IsNonblock() bool

	// SetNonblock updates the non-blocking mode of the connection if
	// applicable.
	//
	// It should return true if the update was successful, and false
	// otherwise. Caller to this function is not expected to retry
	// even if the operation failed.
	SetNonblock(bool) bool
}

// PollConn is an interface that represents a connection that can be polled.
//
// The methods, such as PollR, PollW, and PollRW, returns true with nil error
// if the connection became readable, writable, or both before the timeout.
// If the method returns false, the error must not be nil.
// For example, [io.EOF] is expected if the connection is closed, and
// ctx.Err() is expected if the context is canceled.
type PollConn interface {
	NonblockingConn // PollConn embeds NonblockingConn, given that polling on a blocking connection might not be able to respect the deadline.

	// PollR polls the connection for readability.
	PollR(ctx context.Context) (bool, error)

	// PollW polls the connection for writability.
	PollW(ctx context.Context) (bool, error)
}

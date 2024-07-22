package conn

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type ChannelConn struct {
	chanRX <-chan []byte // read from this channel, owned by the writing-side to this channel
	chanTX chan<- []byte // write to this channel, owned by this struct

	chanClose chan struct{} // notify close event to unblock blocking read/write operations
	closed    atomic.Bool   // true if the channel is closed

	readBuf      []byte // protected by readBufMutex, accessed only from readLocked and readLockedFromBuffer
	readBufMutex sync.Mutex

	pendingWrite atomic.Bool // indicates if there is an outstanding writer blocking, protecting TX channel

	nonblocking atomic.Bool
}

func NewChannelConn(rx <-chan []byte, tx chan<- []byte) *ChannelConn {
	return &ChannelConn{
		chanRX:    rx,
		chanTX:    tx,
		chanClose: make(chan struct{}),
	}
}

// ChannelConn implements [Conn].
var _ Conn = (*ChannelConn)(nil)

// Read reads data from the channel. Implements [net.Conn].
func (c *ChannelConn) Read(b []byte) (n int, err error) {
	if c.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	c.readBufMutex.Lock()
	defer c.readBufMutex.Unlock()

	return c.readLocked(b)
}

// read blocks until some data is available, or the channel is closed.
func (c *ChannelConn) readLocked(b []byte) (int, error) {
	if len(c.readBuf) != 0 { // need to resume reading from the buffer
		return c.readLockedFromBuffer(b)
	}

	if c.nonblocking.Load() {
		for {
			select {
			case <-c.chanClose:
				return 0, io.ErrClosedPipe
			case c.readBuf = <-c.chanRX:
				if len(c.readBuf) != 0 { // buffer is empty, read from the channel OK
					return c.readLockedFromBuffer(b)
				} else { // empty read from channel
					if c.readBuf == nil { // closed channel
						return 0, io.EOF
					} else { // channel open, but empty read: other end testing if write will block
						continue
					}
				}
			default:
				return 0, syscall.EAGAIN
			}
		}
	} else {
		for {
			select {
			case <-c.chanClose:
				return 0, io.ErrClosedPipe
			case c.readBuf = <-c.chanRX:
				if len(c.readBuf) != 0 { // buffer is empty, read from the channel OK
					return c.readLockedFromBuffer(b)
				} else { // empty read from channel
					if c.readBuf == nil { // closed channel
						return 0, io.EOF
					} else { // channel open, but empty read: other end testing if write will block
						continue
					}
				}
			}
		}
	}
}

// readLockedFromBuffer reads from the buffer. Assumes the buffer is non-empty.
func (c *ChannelConn) readLockedFromBuffer(b []byte) (n int, err error) {
	n = copy(b, c.readBuf)
	c.readBuf = c.readBuf[n:]
	return
}

// Write writes data to the channel. Implements [net.Conn].
func (c *ChannelConn) Write(b []byte) (n int, err error) {
	if c.nonblocking.Load() {
		if c.pendingWrite.CompareAndSwap(false, true) {
			defer c.pendingWrite.Store(false)
			n, err = c.writeFlagAcquired(b)
		} else {
			return 0, syscall.EAGAIN
		}
	} else {
		// retry until acquired the pending write flag
		for !c.pendingWrite.CompareAndSwap(false, true) {
			runtime.Gosched()
		}
		defer c.pendingWrite.Store(false)
		n, err = c.writeFlagAcquired(b)
	}

	return
}

// writeFlagAcquired writes data to the channel. Caller must
// acquire the pending write flag before calling this function.
func (c *ChannelConn) writeFlagAcquired(b []byte) (n int, err error) {
	if c.closed.Load() { // check if the channel is closed only after acquiring the pending write flag to prevent racing condition
		return 0, io.ErrClosedPipe
	}

	expectedLen := len(b)

	bCopy := make([]byte, expectedLen)
	if copy(bCopy, b) != expectedLen {
		return 0, io.ErrUnexpectedEOF
	}

	if c.nonblocking.Load() {
		select {
		case <-c.chanClose:
			return 0, io.ErrClosedPipe
		case c.chanTX <- bCopy:
			return expectedLen, nil
		default:
			return 0, syscall.EAGAIN
		}
	} else {
		select {
		case <-c.chanClose:
			return 0, io.ErrClosedPipe
		case c.chanTX <- bCopy:
			return expectedLen, nil
		}
	}
}

func (c *ChannelConn) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		close(c.chanClose)

		// acquire the pending write flag before closing the TX channel
		for !c.pendingWrite.CompareAndSwap(false, true) {
			runtime.Gosched()
		}
		close(c.chanTX)
		c.pendingWrite.Store(false)

		return nil
	}

	return io.ErrClosedPipe // double close
}

type channelAddr struct{}

func (channelAddr) Network() string { return "channel" }
func (channelAddr) String() string  { return "channel" }

// ChannelConn does not implement [NetworkConn].
var _ NetworkConn = (*ChannelConn)(nil)

// LocalAddr returns the local network address. Implements [net.Conn].
func (*ChannelConn) LocalAddr() net.Addr { return channelAddr{} }

// RemoteAddr returns the remote network address. Implements [net.Conn].
func (*ChannelConn) RemoteAddr() net.Addr { return channelAddr{} }

// ChannelConn does not implement [DeadlineConn]. However, fake implementation
// is provided such that it can be used as [net.Conn] in some cases when
// deadlines are not used.
//
// TODO: properly implement [DeadlineConn].
var _ DeadlineConn = (*ChannelConn)(nil)

// SetDeadline is not supported by ChannelConn. It will always return
// [os.ErrNoDeadline].
//
// TODO: properly implement the support for deadlines.
func (*ChannelConn) SetDeadline(time.Time) error {
	return os.ErrNoDeadline
}

// SetReadDeadline is not supported by ChannelConn. It will always return
// [os.ErrNoDeadline].
//
// TODO: properly implement the support for read deadline.
func (*ChannelConn) SetReadDeadline(time.Time) error {
	return os.ErrNoDeadline
}

// SetWriteDeadline is not supported by ChannelConn. It will always return
// [os.ErrNoDeadline].
//
// TODO: properly implement the support for write deadline.
func (*ChannelConn) SetWriteDeadline(time.Time) error {
	return os.ErrNoDeadline
}

// ChannelConn implements [NonblockingConn].
var _ NonblockingConn = (*ChannelConn)(nil)

// IsNonblock returns true if the connection is in non-blocking mode.
func (c *ChannelConn) IsNonblock() bool {
	return c.nonblocking.Load()
}

// SetNonblock updates the non-blocking mode of the connection if applicable.
func (c *ChannelConn) SetNonblock(nonblocking bool) (ok bool) {
	c.nonblocking.Store(nonblocking)
	return true
}

// ChannelConn implements [PollConn].
var _ PollConn = (*ChannelConn)(nil)

func (c *ChannelConn) PollR(ctx context.Context) (bool, error) {
	if !c.nonblocking.Load() {
		return false, errors.New("polling is not supported in blocking mode")
	}

	if c.closed.Load() {
		return false, io.EOF
	}

	for !c.readBufMutex.TryLock() && ctx.Err() == nil {
		runtime.Gosched()
	}

	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	defer c.readBufMutex.Unlock()

	if len(c.readBuf) != 0 {
		return true, nil
	}

	// We cannot check cap(c.chanRX) vs. len(c.chanRX) here because it is
	// possible that messages in the buffer being empty probes sent by the
	// other end to check if the write will block. Instead the universal
	// reading strategy below is used.

	for {
		select {
		case <-c.chanClose:
			return false, io.EOF
		case c.readBuf = <-c.chanRX:
			if len(c.readBuf) != 0 {
				return true, nil
			} else {
				if c.readBuf == nil {
					return false, io.EOF
				} else {
					continue
				}
			}
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

func (c *ChannelConn) PollW(ctx context.Context) (bool, error) {
	if !c.nonblocking.Load() {
		return false, errors.New("polling is not supported in blocking mode")
	}

	// aquire the pending write flag before writing to the TX channel
	for !c.pendingWrite.CompareAndSwap(false, true) && ctx.Err() == nil {
		runtime.Gosched()
	}

	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	defer c.pendingWrite.Store(false)

	if c.closed.Load() {
		return false, io.EOF
	}

	// Buffered channel:
	if cap(c.chanTX) > 0 {
		for ctx.Err() == nil && len(c.chanTX) >= cap(c.chanTX) {
			runtime.Gosched()
		}
		return len(c.chanTX) < cap(c.chanTX), ctx.Err()
	}

	// Unbuffered channel:
	select {
	case <-c.chanClose:
		return false, io.EOF
	case c.chanTX <- []byte{}:
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

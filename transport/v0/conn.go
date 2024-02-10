package v0

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gaukas/water"
	"github.com/gaukas/water/internal/log"
	"github.com/gaukas/water/internal/socket"
)

// Conn is the first experimental version of Conn implementation.
type Conn struct {
	// callerConn is used by DialV0() and AcceptV0(). It is used to talk to
	// the caller of water API by allowing the caller to Read() and Write() to it.
	callerConn net.Conn // the connection from the caller, usually a *net.TCPConnPair

	// srcConn is used by AcceptV0() and RelayV0(). It is used
	// to talk to a remote source by accepting a connection from it.
	srcConn net.Conn // the connection from the remote source, usually a *net.TCPConn

	// dstConn is used by DialV0() and RelayV0(). It is used
	// to talk to a remote destination by actively dialing to it.
	dstConn net.Conn // the connection to the remote destination, usually a *net.TCPConn

	tm      *TransportModule
	tmMutex sync.Mutex

	closeOnce sync.Once
	closed    atomic.Bool

	water.UnimplementedConn // embedded to ensure forward compatibility
}

// dial dials the network address using through the WASM module
// while using the dialerFunc specified in core.config.
func dial(core water.Core, network, address string) (c water.Conn, err error) {
	tm := UpgradeCore(core)
	conn := &Conn{
		tm: tm,
	}

	dialer := NewManagedDialer(network, address, core.Config().NetworkDialerFuncOrDefault())

	if err = conn.tm.LinkNetworkInterface(dialer, nil); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	reverseCallerConn, callerConn, err := socket.TCPConnPair()
	// wasmCallerConn, conn.uoConn, err = socket.TCPConnPair()
	if err != nil {
		if reverseCallerConn == nil || callerConn == nil {
			return nil, fmt.Errorf("water: socket.TCPConnPair returned error: %w", err)
		} else { // likely due to Close() call errored
			log.LErrorf(core.Logger(), "water: socket.TCPConnPair returned error: %v", err)
		}
	}
	conn.callerConn = callerConn

	conn.dstConn, err = conn.tm.DialFrom(reverseCallerConn)
	if err != nil {
		return nil, err
	}

	// safety: we need to watch for the blocking worker thread's status.
	// If it returns, no further data can be processed by the WASM module
	// and we need to close this connection in that case.
	go conn.closeOnWorkerError()

	if err := conn.tm.Worker(); err != nil {
		return nil, err
	}

	return conn, nil
}

// accept accepts the network connection using through the WASM module
// while using the net.Listener specified in core.config.
func accept(core water.Core) (c water.Conn, err error) {
	tm := UpgradeCore(core)
	conn := &Conn{
		tm: tm,
	}

	if err = conn.tm.LinkNetworkInterface(nil, core.Config().NetworkListenerOrPanic()); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	reverseCallerConn, callerConn, err := socket.TCPConnPair()
	if err != nil {
		if reverseCallerConn == nil || callerConn == nil {
			return nil, fmt.Errorf("water: socket.TCPConnPair returned error: %w", err)
		} else { // likely due to Close() call errored
			log.LErrorf(core.Logger(), "water: socket.TCPConnPair returned error: %v", err)
		}
	} else if reverseCallerConn == nil || callerConn == nil {
		return nil, errors.New("water: socket.TCPConnPair returned nil")
	}

	conn.callerConn = callerConn

	conn.srcConn, err = conn.tm.AcceptFor(reverseCallerConn)
	if err != nil {
		return nil, err
	}

	// safety: we need to watch for the blocking worker thread's status.
	// If it returns, no further data can be processed by the WASM module
	// and we need to close this connection in that case.
	go conn.closeOnWorkerError()

	if err := conn.tm.Worker(); err != nil {
		return nil, err
	}

	return conn, nil
}

func relay(core water.Core, network, address string) (c water.Conn, err error) {
	tm := UpgradeCore(core)
	conn := &Conn{
		tm: tm,
	}

	dialer := NewManagedDialer(network, address, core.Config().NetworkDialerFuncOrDefault())

	if err = conn.tm.LinkNetworkInterface(dialer, core.Config().NetworkListenerOrPanic()); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	if err := conn.tm.Associate(); err != nil {
		return nil, err
	}

	// safety: we need to watch for the blocking worker thread's status.
	// If it returns, no further data can be processed by the WASM module
	// and we need to close this connection in that case.
	go conn.closeOnWorkerError()

	if err := conn.tm.Worker(); err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *Conn) closeOnWorkerError() {
	var tm *TransportModule
	var core water.Core

	c.tmMutex.Lock()
	if c.tm != nil {
		tm = c.tm
		core = tm.Core()
	}
	c.tmMutex.Unlock()

	<-tm.WorkerErrored()
	log.LDebugf(core.Logger(), "water: v0.accept: worker thread returned")
	c.Close()
}

// Read implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's [net.Conn.Read] method.
func (c *Conn) Read(b []byte) (n int, err error) {
	if c.callerConn == nil {
		return 0, errors.New("water: cannot read, (*RuntimeConn).uoConn is nil")
	}

	return c.callerConn.Read(b)
}

// Write implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's [net.Conn.Write] method.
func (c *Conn) Write(b []byte) (n int, err error) {
	if c.callerConn == nil {
		return 0, errors.New("water: cannot write, (*RuntimeConn).uoConn is nil")
	}

	n, err = c.callerConn.Write(b)
	if err != nil {
		return n, fmt.Errorf("uoConn.Write: %w", err)
	}

	if n == len(b) {
		return n, nil
	} else if n < len(b) {
		return n, io.ErrShortWrite
	} else {
		return n, errors.New("invalid write result") // io.errInvalidWrite
	}
}

// Close implements the net.Conn interface.
//
// It will close both the network connection AND the WASM module, then
// the user-facing net.Conn will be closed.
func (c *Conn) Close() (err error) {
	if !c.closed.CompareAndSwap(false, true) {
		err = errors.New("water: already closed")
		return err
	}

	c.closeOnce.Do(func() {
		c.tmMutex.Lock()
		if c.tm != nil {
			err = c.tm.Close()
			c.tm = nil
		}
		c.tmMutex.Unlock()
	})

	return err
}

// LocalAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's [net.Conn.LocalAddr] method.
// For Listener and Relay, the network connection of interest is the srcConn.
// And for Dialer, the network connection of interest is the dstConn.
func (c *Conn) LocalAddr() net.Addr {
	// for Listener and Relay, the srcConn is of interest
	if c.srcConn != nil {
		return c.srcConn.LocalAddr()
	}
	return c.dstConn.LocalAddr() // for dialer
}

// RemoteAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's [net.Conn.RemoteAddr] method.
// For Listener and Relay, the network connection of interest is the srcConn.
// And for Dialer, the network connection of interest is the dstConn.
func (c *Conn) RemoteAddr() net.Addr {
	// for Listener and Relay, the srcConn is of interest
	if c.srcConn != nil {
		return c.srcConn.RemoteAddr()
	}
	return c.dstConn.RemoteAddr() // for dialer
}

// SetDeadline implements the net.Conn interface.
//
// It calls to the underlying connections' [net.Conn.SetDeadline] method.
func (c *Conn) SetDeadline(t time.Time) (err error) {
	// SetDeadline is only available to Dialer/Listener. But not Relay.
	if c.callerConn == nil {
		return errors.New("water: cannot set deadline, (*RuntimeConn).callerConn is nil")
	}

	// note: the deadline needs to be set on the actually pushed connection,
	// which is not necessarily the networkConn. (there would be middleware conns)

	if c.dstConn != nil {
		err = c.dstConn.SetDeadline(t)
		if err != nil {
			return err
		}
	}

	if c.srcConn != nil {
		err = c.srcConn.SetDeadline(t)
		if err != nil {
			return err
		}
	}

	return c.callerConn.SetDeadline(t)
}

// SetReadDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's [net.Conn.SetReadDeadline] method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	// SetReadDeadline is only available to Dialer/Listener. But not Relay.
	if c.callerConn == nil {
		return errors.New("water: cannot set deadline, (*RuntimeConn).callerConn is nil")
	}

	return c.callerConn.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's [net.Conn.SetWriteDeadline] method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	// SetWriteDeadline is only available to Dialer/Listener. But not Relay.
	if c.callerConn == nil {
		return errors.New("water: cannot set deadline, (*RuntimeConn).callerConn is nil")
	}

	return c.callerConn.SetWriteDeadline(t)
}

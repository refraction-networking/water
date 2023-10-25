//go:build !exclude_v0

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
	v0 "github.com/gaukas/water/internal/v0"
)

// Conn is the first experimental version of Conn implementation.
type Conn struct {
	// callerConn is used by DialV0() and AcceptV0(). It is used to talk to
	// the caller of water API by allowing the caller to Read() and Write() to it.
	callerConn net.Conn // the connection from the caller, usually a *net.UnixConn

	// srcConn is used by AcceptV0() and RelayV0(). It is used
	// to talk to a remote source by accepting a connection from it.
	srcConn net.Conn // the connection from the remote source, usually a *net.TCPConn

	// dstConn is used by DialV0() and RelayV0(). It is used
	// to talk to a remote destination by actively dialing to it.
	dstConn net.Conn // the connection to the remote destination, usually a *net.TCPConn

	tm *v0.TransportModule

	closeOnce *sync.Once
	closed    atomic.Bool

	water.UnimplementedConn // embedded to ensure forward compatibility
}

// dial dials the network address using through the WASM module
// while using the dialerFunc specified in core.config.
func dial(core water.Core, network, address string) (c water.Conn, err error) {
	tm := v0.Core2TransportModule(core)
	conn := &Conn{
		tm:        tm,
		closeOnce: &sync.Once{},
	}

	dialer := v0.NewManagedDialer(network, address, core.Config().NetworkDialerFuncOrDefault())

	if err = conn.tm.LinkNetworkInterface(dialer, nil); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	reverseCallerConn, callerConn, err := socket.UnixConnPair()
	// wasmCallerConn, conn.uoConn, err = socket.TCPConnPair()
	if err != nil {
		if reverseCallerConn == nil || callerConn == nil {
			return nil, fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
		} else { // likely due to Close() call errored
			log.Errorf("water: socket.UnixConnPair returned error: %v", err)
		}
	}
	conn.callerConn = callerConn

	conn.dstConn, err = conn.tm.DialFrom(reverseCallerConn)
	if err != nil {
		return nil, err
	}

	if err := conn.tm.Worker(); err != nil {
		return nil, err
	}

	log.Debugf("water: DialV0: conn.tm.Worker() returned")

	// safety: we need to watch for the blocking worker thread's status.
	// If it returns, no further data can be processed by the WASM module
	// and we need to close this connection in that case.
	go func() {
		<-conn.tm.WorkerErrored()
		log.Debugf("water: DialV0: worker thread returned")
		conn.Close()
	}()

	return conn, nil
}

// accept accepts the network connection using through the WASM module
// while using the net.Listener specified in core.config.
func accept(core water.Core) (c water.Conn, err error) {
	tm := v0.Core2TransportModule(core)
	conn := &Conn{
		tm:        tm,
		closeOnce: &sync.Once{},
	}

	if err = conn.tm.LinkNetworkInterface(nil, core.Config().NetworkListenerOrPanic()); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	reverseCallerConn, callerConn, err := socket.UnixConnPair()
	if err != nil {
		if reverseCallerConn == nil || callerConn == nil {
			return nil, fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
		} else { // likely due to Close() call errored
			log.Errorf("water: socket.UnixConnPair returned error: %v", err)
		}
	} else if reverseCallerConn == nil || callerConn == nil {
		return nil, errors.New("water: socket.UnixConnPair returned nil")
	}

	conn.callerConn = callerConn

	conn.srcConn, err = conn.tm.AcceptFor(reverseCallerConn)
	if err != nil {
		return nil, err
	}

	if err := conn.tm.Worker(); err != nil {
		return nil, err
	}

	// safety: we need to watch for the blocking worker thread's status.
	// If it returns, no further data can be processed by the WASM module
	// and we need to close this connection in that case.
	go func() {
		<-conn.tm.WorkerErrored()
		conn.Close()
	}()

	return conn, nil
}

func relay(core water.Core, network, address string) (c water.Conn, err error) {
	tm := v0.Core2TransportModule(core)
	conn := &Conn{
		tm:        tm,
		closeOnce: &sync.Once{},
	}

	dialer := v0.NewManagedDialer(network, address, core.Config().NetworkDialerFuncOrDefault())

	if err = conn.tm.LinkNetworkInterface(dialer, core.Config().NetworkListenerOrPanic()); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	if err := conn.tm.Associate(); err != nil {
		return nil, err
	}

	if err := conn.tm.Worker(); err != nil {
		return nil, err
	}

	// safety: we need to watch for the blocking worker thread's status.
	// If it returns, no further data can be processed by the WASM module
	// and we need to close this connection in that case.
	go func() {
		<-conn.tm.WorkerErrored()
		conn.Close()
	}()

	return conn, nil
}

// Read implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Read() method.
func (c *Conn) Read(b []byte) (n int, err error) {
	if c.callerConn == nil {
		return 0, errors.New("water: cannot read, (*RuntimeConn).uoConn is nil")
	}

	return c.callerConn.Read(b)
}

// Write implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Write() method.
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
		log.Debugf("Defering TM")
		c.tm.DeferAll()
		log.Debugf("Canceling TM")
		err = c.tm.Cancel()
		log.Debugf("Cleaning TM")
		c.tm.Cleanup()
		log.Debugf("TM canceled")
	})

	return err
}

// LocalAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's LocalAddr() method.
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
// It calls to the underlying network connection's RemoteAddr() method.
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
// It calls to the underlying user-oriented connection's SetDeadline() method.
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
// It calls to the underlying user-oriented connection's SetReadDeadline() method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	// SetReadDeadline is only available to Dialer/Listener. But not Relay.
	if c.callerConn == nil {
		return errors.New("water: cannot set deadline, (*RuntimeConn).callerConn is nil")
	}

	return c.callerConn.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetWriteDeadline() method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	// SetWriteDeadline is only available to Dialer/Listener. But not Relay.
	if c.callerConn == nil {
		return errors.New("water: cannot set deadline, (*RuntimeConn).callerConn is nil")
	}

	return c.callerConn.SetWriteDeadline(t)
}

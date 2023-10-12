//go:build !nov0

package water

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gaukas/water/interfaces"
	"github.com/gaukas/water/internal/log"
	"github.com/gaukas/water/internal/socket"
	v0 "github.com/gaukas/water/internal/v0"
)

func init() {
	RegisterDial("_v0", DialV0)     // Registering the dialer function to WATER library
	RegisterAccept("_v0", AcceptV0) // Registering the accept function to WATER library
}

// ConnV0 is the first version of RuntimeConn.
type ConnV0 struct {
	networkConn net.Conn // network-facing net.Conn, data written to this connection will be sent on the wire
	uoConn      net.Conn // user-oriented net.Conn, user Read()/Write() to this connection

	tm *v0.TransportModule

	interfaces.UnimplementedConn // embedded to ensure forward compatibility
}

// DialV0 dials the network address using through the WASM module
// while using the dialerFunc specified in core.config.
func DialV0(core interfaces.Core, network, address string) (c interfaces.Conn, err error) {
	tm := v0.Core2TransportModule(core)
	conn := &ConnV0{
		tm: tm,
	}

	dialer := v0.ManagedDialer(network, address, core.Config().DialerFuncOrDefault())

	if err = conn.tm.LinkNetworkInterface(dialer, nil); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	var wasmCallerConn net.Conn
	wasmCallerConn, conn.uoConn, err = socket.UnixConnPair()
	if err != nil {
		if wasmCallerConn == nil || conn.uoConn == nil {
			return nil, fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
		} else { // likely due to Close() call errored
			log.Errorf("water: socket.UnixConnPair returned error: %v", err)
		}
	}

	conn.networkConn, err = conn.tm.DialFrom(wasmCallerConn)
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

// AcceptV0 accepts the network connection using through the WASM module
// while using the net.Listener specified in core.config.
func AcceptV0(core interfaces.Core) (c interfaces.Conn, err error) {
	tm := v0.Core2TransportModule(core)
	conn := &ConnV0{
		tm: tm,
	}

	if err = conn.tm.LinkNetworkInterface(nil, core.Config().NetworkListenerOrPanic()); err != nil {
		return nil, err
	}

	if err = conn.tm.Initialize(); err != nil {
		return nil, err
	}

	var wasmCallerConn net.Conn
	wasmCallerConn, conn.uoConn, err = socket.UnixConnPair()
	if err != nil {
		if wasmCallerConn == nil || conn.uoConn == nil {
			return nil, fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
		} else { // likely due to Close() call errored
			log.Errorf("water: socket.UnixConnPair returned error: %v", err)
		}
	}

	conn.networkConn, err = conn.tm.AcceptFor(wasmCallerConn)
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

func RelayV0(core interfaces.Core, network, address string) (c interfaces.Conn, err error) {
	tm := v0.Core2TransportModule(core)
	conn := &ConnV0{
		tm: tm,
	}

	dialer := v0.ManagedDialer(network, address, core.Config().DialerFuncOrDefault())

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
func (c *ConnV0) Read(b []byte) (n int, err error) {
	if c.uoConn == nil {
		return 0, errors.New("water: cannot read, (*RuntimeConnV0).uoConn is nil")
	}

	return c.uoConn.Read(b)
}

// Write implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Write() method.
func (c *ConnV0) Write(b []byte) (n int, err error) {
	if c.uoConn == nil {
		return 0, errors.New("water: cannot write, (*RuntimeConnV0).uoConn is nil")
	}

	n, err = c.uoConn.Write(b)
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
func (c *ConnV0) Close() error {
	if c.networkConn == nil {
		if err := c.networkConn.Close(); err != nil {
			return fmt.Errorf("water: (*RuntimeConnV0).netConn.Close returned error: %w", err)
		}
	}

	c.tm.Cancel()
	c.tm.DeferAll()
	c.tm.Cleanup()

	if c.uoConn != nil {
		return c.uoConn.Close()
	}

	return nil
}

// LocalAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's LocalAddr() method.
func (c *ConnV0) LocalAddr() net.Addr {
	return c.networkConn.LocalAddr()
}

// RemoteAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's RemoteAddr() method.
func (c *ConnV0) RemoteAddr() net.Addr {
	return c.networkConn.RemoteAddr()
}

// SetDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetDeadline() method.
func (c *ConnV0) SetDeadline(t time.Time) error {
	return c.uoConn.SetDeadline(t)
}

// SetReadDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetReadDeadline() method.
//
// Note: in practice this method should actively be used by the caller. Otherwise
// it is possible for a silently failed network connection to cause the WASM module
// to hang forever on Read().
func (c *ConnV0) SetReadDeadline(t time.Time) error {
	return c.uoConn.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetWriteDeadline() method.
func (c *ConnV0) SetWriteDeadline(t time.Time) error {
	return c.uoConn.SetWriteDeadline(t)
}

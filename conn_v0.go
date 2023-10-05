//go:build !nov0

package water

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gaukas/water/internal/socket"
	v0 "github.com/gaukas/water/internal/v0"
	"github.com/gaukas/water/internal/wasm"
)

func init() {
	RegisterDial("_v0", DialV0)
	RegisterAccept("_v0", AcceptV0)
}

const (
	RUNTIME_VERSION_ZERO int32 = 0
)

// ConnV0 is the first version of RuntimeConn.
type ConnV0 struct {
	networkConn net.Conn // network-facing net.Conn, data written to this connection will be sent on the wire
	uoConn      net.Conn // user-oriented net.Conn, user Read()/Write() to this connection

	wasm *WASMv0

	UnimplementedConn // embedded to ensure forward compatibility
}

// DialV0 dials the network address using through the WASM module
// while using the dialerFunc specified in core.config.
func DialV0(core *core, network, address string) (c Conn, err error) {
	wasm := NewWASMv0(core)
	conn := &ConnV0{
		wasm: wasm,
	}

	dialer := v0.MakeWASIDialer(network, address, core.Config().DialerFuncOrDefault())

	if err = conn.wasm.LinkNetworkInterface(dialer, nil); err != nil {
		return nil, err
	}

	if err = conn.wasm.Initialize(); err != nil {
		return nil, err
	}

	// Initialize WASM module as ReadWriter
	if err = conn.wasm.InitializeReadWriter(); err != nil {
		return nil, err
	}

	var wasmCallerConn net.Conn
	wasmCallerConn, conn.uoConn, err = socket.UnixConnPair("")
	if err != nil {
		return nil, fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
	}

	wasmNetworkConn, err := conn.wasm.DialFrom(wasmCallerConn)
	if err != nil {
		return nil, err
	}

	conn.networkConn = wasmNetworkConn

	return conn, nil
}

// AcceptV0 accepts the network connection using through the WASM module
// while using the net.Listener specified in core.config.
func AcceptV0(core *core) (c Conn, err error) {
	wasm := NewWASMv0(core)
	conn := &ConnV0{
		wasm: wasm,
	}

	listener := v0.MakeWASIListener(core.Config().NetworkListenerOrPanic())

	if err = conn.wasm.LinkNetworkInterface(nil, listener); err != nil {
		return nil, err
	}

	if err = conn.wasm.Initialize(); err != nil {
		return nil, err
	}

	// Initialize WASM module as ReadWriter
	if err = conn.wasm.InitializeReadWriter(); err != nil {
		return nil, err
	}

	var wasmCallerConn net.Conn
	wasmCallerConn, conn.uoConn, err = socket.UnixConnPair("")
	if err != nil {
		return nil, fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
	}

	wasmNetworkConn, err := conn.wasm.AcceptFor(wasmCallerConn)
	if err != nil {
		return nil, err
	}

	conn.networkConn = wasmNetworkConn

	return conn, nil
}

// Read implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Read() method.
func (rcv *ConnV0) Read(b []byte) (n int, err error) {
	if rcv.uoConn == nil {
		return 0, errors.New("water: cannot read, (*RuntimeConnV0).uoConn is nil")
	}

	// call _read
	ret, err := rcv.wasm._read.Call(rcv.wasm.Store())
	if err != nil {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned error: %w", err)
	}

	if ret32, ok := ret.(int32); !ok {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned non-int32 value")
	} else {
		if ret32 != 0 {
			return 0, wasm.WASMErr(ret32)
		}
	}

	return rcv.uoConn.Read(b)
}

// Write implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Write() method.
func (rcv *ConnV0) Write(b []byte) (n int, err error) {
	if rcv.uoConn == nil {
		return 0, errors.New("water: cannot write, (*RuntimeConnV0).uoConn is nil")
	}

	n, err = rcv.uoConn.Write(b)
	if err != nil {
		return n, fmt.Errorf("uoConn.Write: %w", err)
	}

	if n < len(b) {
		return n, io.ErrShortWrite
	}

	if n > len(b) {
		return n, errors.New("invalid write result") // io.errInvalidWrite
	}

	// call _write to notify WASM
	ret, err := rcv.wasm._write.Call(rcv.wasm.Store())
	if err != nil {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned error: %w", err)
	}

	if ret32, ok := ret.(int32); !ok {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned non-int32 value")
	} else {
		return n, wasm.WASMErr(ret32)
	}
}

// Close implements the net.Conn interface.
//
// It will close both the network connection AND the WASM module, then
// the user-facing net.Conn will be closed.
func (rcv *ConnV0) Close() error {
	err := rcv.networkConn.Close()
	if err != nil {
		return fmt.Errorf("water: (*RuntimeConnV0).netConn.Close returned error: %w", err)
	}

	_, err = rcv.wasm._close.Call(rcv.wasm.Store())
	if err != nil {
		return fmt.Errorf("water: (*RuntimeConnV0)._close.Call returned error: %w", err)
	}

	rcv.wasm.DeferAll()
	rcv.wasm.Cleanup()

	if rcv.uoConn != nil {
		rcv.uoConn.Close()
	}

	return nil
}

// LocalAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's LocalAddr() method.
func (rcv *ConnV0) LocalAddr() net.Addr {
	return rcv.networkConn.LocalAddr()
}

// RemoteAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's RemoteAddr() method.
func (rcv *ConnV0) RemoteAddr() net.Addr {
	return rcv.networkConn.RemoteAddr()
}

// SetDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetDeadline() method.
func (rcv *ConnV0) SetDeadline(t time.Time) error {
	return rcv.uoConn.SetDeadline(t)
}

// SetReadDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetReadDeadline() method.
//
// Note: in practice this method should actively be used by the caller. Otherwise
// it is possible for a silently failed network connection to cause the WASM module
// to hang forever on Read().
func (rcv *ConnV0) SetReadDeadline(t time.Time) error {
	return rcv.uoConn.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetWriteDeadline() method.
func (rcv *ConnV0) SetWriteDeadline(t time.Time) error {
	return rcv.uoConn.SetWriteDeadline(t)
}

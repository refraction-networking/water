package water

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v12"
	"github.com/gaukas/water/socket"
)

const (
	RUNTIME_VERSION_ZERO int32 = iota
)

// RuntimeConnWithVersion spins up a RuntimeConn of the corresponding version with the
// given core and (implicitly) initializes it.
func RuntimeConnWithVersion(core *runtimeCore, version int32) (RuntimeConn, error) {
	switch version {
	case RUNTIME_VERSION_ZERO:
		return NewRuntimeConnV0(core)
	default:
		return nil, fmt.Errorf("water: unsupported runtime version: %d", version)
	}
}

// RuntimeConnV0 is the first version of RuntimeConn.
type RuntimeConnV0 struct {
	networkConn net.Conn // network-facing net.Conn, data written to this connection will be sent on the wire
	uoConn      net.Conn // user-oriented net.Conn, user Read()/Write() to this connection
	waConn      net.Conn // WASM-facing net.Conn, data written to this connection will be readable from user's end

	core *runtimeCore

	// wasi exports

	// `_dial` is a WASM-exported function, in which WASM calls to env.dial()
	// to request a network connection.
	//
	// `env.dial` is defined as a WasiDialerFunc (see wasi.go):
	//  dial(network i32) (netFd i32)
	// where network is a WasiNetwork (in wasi.go), fd refers to the file descriptor
	// where the network connection can be found in the WASM's world.
	//
	// `_dial` is defined as:
	//   _dial(userConnFd i32) (netFd i32)
	// where userConnFd is the file descriptor of the user-facing net.Conn for WASM to associate
	// with the network connection, and netFd is the return value of `env.dial` returned as-is.
	_dial  *wasmtime.Func
	_close *wasmtime.Func // on _close, WASM ceases to process data and clean up resources
	_write *wasmtime.Func // _write(expected int32) (actual int32), WASM will prepare a buffer at least `expected` bytes long and read from user-facing net.Conn
	_read  *wasmtime.Func // _read()
}

func NewRuntimeConnV0(core *runtimeCore) (*RuntimeConnV0, error) {
	rcv0 := &RuntimeConnV0{core: core}
	err := rcv0.initialize()
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConnV0).initialize returned error: %w", err)
	}

	return rcv0, nil
}

// Read implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Read() method.
func (rcv *RuntimeConnV0) Read(b []byte) (n int, err error) {
	// call _read
	_, err = rcv._read.Call(rcv.core.store)
	if err != nil {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned error: %w", err)
	}

	return rcv.uoConn.Read(b)
}

// Write implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Write() method.
func (rcv *RuntimeConnV0) Write(b []byte) (n int, err error) {
	n, err = rcv.uoConn.Write(b)
	if err != nil {
		return n, err
	}

	if n < len(b) {
		return n, io.ErrShortWrite
	}

	if n > len(b) {
		return n, errors.New("invalid write result") // io.errInvalidWrite
	}

	// call _write to notify WASM
	ret, err := rcv._write.Call(rcv.core.store, int32(len(b)))
	if err != nil {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned error: %w", err)
	}

	if actualWritten, ok := ret.(int32); !ok {
		return 0, fmt.Errorf("water: (*wasmtime.Func).Call returned non-int32 value")
	} else {
		if actualWritten < int32(n) {
			return int(actualWritten), io.ErrShortWrite
		} else if actualWritten > int32(n) {
			return int(actualWritten), errors.New("invalid write result") // io.errInvalidWrite
		}
		return int(actualWritten), nil
	}
}

// Close implements the net.Conn interface.
//
// It will close both the network connection AND the WASM module, then
// the user-facing net.Conn will be closed.
func (rcv *RuntimeConnV0) Close() error {
	err := rcv.networkConn.Close()
	if err != nil {
		return fmt.Errorf("water: (*RuntimeConnV0).netConn.Close returned error: %w", err)
	}

	_, err = rcv._close.Call(rcv.core.store)
	if err != nil {
		return fmt.Errorf("water: (*RuntimeConnV0)._close.Call returned error: %w", err)
	}

	err = rcv.waConn.Close()
	if err != nil {
		return fmt.Errorf("water: (*RuntimeConnV0).waConn.Close returned error: %w", err)
	}

	return rcv.uoConn.Close()
}

// LocalAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's LocalAddr() method.
func (rcv *RuntimeConnV0) LocalAddr() net.Addr {
	return rcv.networkConn.LocalAddr()
}

// RemoteAddr implements the net.Conn interface.
//
// It calls to the underlying network connection's RemoteAddr() method.
func (rcv *RuntimeConnV0) RemoteAddr() net.Addr {
	return rcv.networkConn.RemoteAddr()
}

// SetDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetDeadline() method.
func (rcv *RuntimeConnV0) SetDeadline(t time.Time) error {
	return rcv.uoConn.SetDeadline(t)
}

// SetReadDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetReadDeadline() method.
func (rcv *RuntimeConnV0) SetReadDeadline(t time.Time) error {
	return rcv.uoConn.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetWriteDeadline() method.
func (rcv *RuntimeConnV0) SetWriteDeadline(t time.Time) error {
	return rcv.uoConn.SetWriteDeadline(t)
}

func (rcv *RuntimeConnV0) initialize() error {
	rcv._dial = rcv.core.instance.GetFunc(rcv.core.store, "_dial")
	if rcv._dial == nil {
		return fmt.Errorf("water: WASM module missing required function _dial for V0")
	}
	rcv._close = rcv.core.instance.GetFunc(rcv.core.store, "_close")
	if rcv._close == nil {
		return fmt.Errorf("water: WASM module missing required function _close for V0")
	}

	var err error
	// create a UnixConn pair
	rcv.uoConn, rcv.waConn, err = socket.UnixConnPair("")
	if err != nil {
		return fmt.Errorf("water: socket.UnixConnPair returned error: %w", err)
	}
	// push waConn to WASM
	waConnFile, err := socket.AsFile(rcv.waConn)
	if err != nil {
		return fmt.Errorf("water: socket.AsFile returned error: %w", err)
	}

	// push waConnFile to WASM
	wasmFd, err := rcv.core.store.PushFile(waConnFile, wasmtime.READ_WRITE)
	if err != nil {
		return fmt.Errorf("water: (*wasmtime.Store).PushFile returned error: %w", err)
	}

	// call _dial
	netFd, err := rcv._dial.Call(rcv.core.store, int32(wasmFd))
	if err != nil {
		return fmt.Errorf("water: (*wasmtime.Func).Call returned error: %w", err)
	}
	// type assertion
	if netFdInt32, ok := netFd.(int32); !ok {
		return fmt.Errorf("water: (*wasmtime.Func).Call returned non-int32 value")
	} else {
		// get the net.Conn from the WASM's world
		rcv.networkConn = rcv.core.wd.GetConnByFd(netFdInt32)
		if rcv.networkConn == nil {
			return fmt.Errorf("water: (*wasmtime.Func).Call returned invalid net.Conn")
		}
	}

	return nil
}

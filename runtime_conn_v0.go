// //go:build v0
// // +build v0
// TODO: uncomment the above lines

package water

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"log/slog"

	"github.com/bytecodealliance/wasmtime-go/v12"
	"github.com/gaukas/water/socket"
)

func init() {
	RegisterOutboundRuntimeConnWithVersion(RUNTIME_VERSION_ZERO, NewOutboundRuntimeConnV0)
	RegisterInboundRuntimeConnWithVersion(RUNTIME_VERSION_ZERO, NewInboundRuntimeConnV0)
}

const (
	RUNTIME_VERSION_ZERO int32 = iota
)

// RuntimeConnV0 is the first version of RuntimeConn.
type RuntimeConnV0 struct {
	networkConn net.Conn // network-facing net.Conn, data written to this connection will be sent on the wire
	uoConn      net.Conn // user-oriented net.Conn, user Read()/Write() to this connection
	waConn      net.Conn // WASM-facing net.Conn, data written to this connection will be readable from user's end

	core *runtimeCore

	// wasi exports

	// `_config` is a WASM-exported function, in which WASM reads the config file
	// at the file descriptor specified by the first parameter.
	_config *wasmtime.Func

	// `_dial` is a WASM-exported function, in which WASM calls to env.dial()
	// to request a network connection.
	//
	// `env.dial` is defined as a WASIDialerFunc (see wasi.go):
	//  dial() (netFd i32) // TODO: dial(apw i32) (netFd i32)
	// where network is a WASINetwork (in wasi.go), fd refers to the file descriptor
	// where the network connection can be found in the WASM's world.
	//
	// `_dial` is defined as:
	//   _dial(userConnFd i32) (netConnFd i32)
	// where userConnFd is the file descriptor of the user-facing net.Conn for WASM to associate
	// with the network connection, and netFd is the return value of `env.dial` returned as-is.
	_dial *wasmtime.Func

	_preaccept *wasmtime.Func // TODO: _preaccept() (apw i32)
	_accept    *wasmtime.Func // TODO: _accept(userConnFd, netConnFd i32)
	ibc        net.Conn       // inbound net.Conn, data written to this connection will be sent on the wire

	_close *wasmtime.Func // on _close, WASM ceases to process data and clean up resources
	_write *wasmtime.Func // _write(expected int32) (actual int32), WASM will prepare a buffer at least `expected` bytes long and read from user-facing net.Conn
	_read  *wasmtime.Func // _read()
}

func NewOutboundRuntimeConnV0(core *runtimeCore) (RuntimeConn, error) {
	rc := &RuntimeConnV0{core: core}
	err := rc.initializeOutboundConn()
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConnV0).initialize returned error: %w", err)
	}

	return rc, nil
}

func NewInboundRuntimeConnV0(core *runtimeCore, ibc net.Conn) (RuntimeConn, error) {
	rc := &RuntimeConnV0{core: core, ibc: ibc}
	err := rc.initializeInboundConn()
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConnV0).initialize returned error: %w", err)
	}

	return rc, nil
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
//
// Note: in practice this method should actively be used by the caller. Otherwise
// it is possible for a silently failed network connection to cause the WASM module
// to hang forever on Read().
func (rcv *RuntimeConnV0) SetReadDeadline(t time.Time) error {
	return rcv.uoConn.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
//
// It calls to the underlying user-oriented connection's SetWriteDeadline() method.
func (rcv *RuntimeConnV0) SetWriteDeadline(t time.Time) error {
	return rcv.uoConn.SetWriteDeadline(t)
}

func (rcv *RuntimeConnV0) initializeOutboundConn() error {
	rcv._dial = rcv.core.instance.GetFunc(rcv.core.store, "_dial")
	if rcv._dial == nil {
		return fmt.Errorf("water: WASM module missing required function _dial for V0")
	}
	rcv._read = rcv.core.instance.GetFunc(rcv.core.store, "_read")
	if rcv._read == nil {
		return fmt.Errorf("water: WASM module missing required function _read for V0")
	}
	rcv._write = rcv.core.instance.GetFunc(rcv.core.store, "_write")
	if rcv._write == nil {
		return fmt.Errorf("water: WASM module missing required function _write for V0")
	}
	rcv._close = rcv.core.instance.GetFunc(rcv.core.store, "_close")
	if rcv._close == nil {
		return fmt.Errorf("water: WASM module missing required function _close for V0")
	}
	rcv._config = rcv.core.instance.GetFunc(rcv.core.store, "_config")
	if rcv._config == nil {
		return fmt.Errorf("water: WASM module missing required function _config for V0")
	}

	// TODO: call _config to pass the config file to WASM

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

func (rcv *RuntimeConnV0) initializeInboundConn() error {
	rcv._preaccept = rcv.core.instance.GetFunc(rcv.core.store, "_preaccept")
	if rcv._preaccept == nil {
		return fmt.Errorf("water: WASM module missing required function _preaccept for V0")
	}
	rcv._accept = rcv.core.instance.GetFunc(rcv.core.store, "_accept")
	if rcv._dial == nil {
		return fmt.Errorf("water: WASM module missing required function _accept for V0")
	}
	rcv._read = rcv.core.instance.GetFunc(rcv.core.store, "_read")
	if rcv._read == nil {
		return fmt.Errorf("water: WASM module missing required function _read for V0")
	}
	rcv._write = rcv.core.instance.GetFunc(rcv.core.store, "_write")
	if rcv._write == nil {
		return fmt.Errorf("water: WASM module missing required function _write for V0")
	}
	rcv._close = rcv.core.instance.GetFunc(rcv.core.store, "_close")
	if rcv._close == nil {
		return fmt.Errorf("water: WASM module missing required function _close for V0")
	}
	rcv._config = rcv.core.instance.GetFunc(rcv.core.store, "_config")
	if rcv._config == nil {
		return fmt.Errorf("water: WASM module missing required function _config for V0")
	}

	// TODO: call _config to pass the config file to WASM

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

	// call _preaccept
	apw, err := rcv._preaccept.Call(rcv.core.store)
	if err != nil {
		return fmt.Errorf("water: (*wasmtime.Func).Call returned error: %w", err)
	}
	// type assertion
	if apwInt32, ok := apw.(int32); !ok {
		return fmt.Errorf("water: (*wasmtime.Func).Call returned non-int32 value")
	} else {
		// wrap the inbound net.Conn with the application protocol
		// rcv.core.config...
		slog.Default().Debug(fmt.Sprintf("TODO: wrap inbound net.Conn with application protocol %d", apwInt32))

		// push ibc to WASM
		ibcFile, err := socket.AsFile(rcv.ibc)
		if err != nil {
			return fmt.Errorf("water: socket.AsFile returned error: %w", err)
		}

		// push ibcFile to WASM
		netconnFd, err := rcv.core.store.PushFile(ibcFile, wasmtime.READ_WRITE)
		if err != nil {
			return fmt.Errorf("water: (*wasmtime.Store).PushFile returned error: %w", err)
		}

		// call _accept
		_, err = rcv._accept.Call(rcv.core.store, int32(netconnFd), int32(wasmFd))
	}

	return nil
}

//go:build !nov0

package water

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/socket"
)

func init() {
	RegisterOutboundRuntimeConnWithVersion(RUNTIME_VERSION_ZERO, NewOutboundRuntimeConnV0)
	RegisterInboundRuntimeConnWithVersion(RUNTIME_VERSION_ZERO, NewInboundRuntimeConnV0)
	RegisterRelayingRuntimeConnWithVersion(RUNTIME_VERSION_ZERO, NewRelayingRuntimeConnV0)
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
	_config *wasmtime.Func // _config(fd i32)

	// _dial:
	//  - Calls to `env.dial(apw) -> fd i32` to dial a network connection (wrapped with the
	//  application protocol) and bind it to one of its file descriptors, record the fd as
	//  `remoteConnFd`. This will be the fd it used to read/write data from/to the remote
	//  destination.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `remoteConnFd` to the caller to be kept track of.
	_dial *wasmtime.Func // _dial(callerConnFd i32) (remoteConnFd i32)

	// _accept:
	//  - Calls to `env.accept(apw) -> fd i32` to accept a network connection (wrapped with the
	//  application protocol) and bind it to one of its file descriptors, record the fd as
	//  `sourceConnFd`. This will be the fd it used to read/write data from/to the source
	//  address.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `sourceConnFd` to the caller to be kept track of.
	_accept *wasmtime.Func // _accept(callerConnFd i32) (sourceConnFd i32)

	// _assoc:
	//  - Calls to `env.accept(apw) -> fd i32` to accept a network connection (wrapped with the
	//  application protocol) and bind it to one of its file descriptors, record the fd as
	//  `sourceConnFd`. This will be the fd it used to read/write data from/to the source
	//  address.
	//  - Calls to `env.dial(apw) -> fd i32` to dial a network connection (wrapped with the
	//  application protocol) and bind it to one of its file descriptors, record the fd as
	//  `remoteConnFd`. This will be the fd it used to read/write data from/to the remote
	//  destination.
	//  - Calling `_assoc()` DOES NOT automatically start relaying data between the two
	//  connections. How to relay data is up to the WASM module per version spec.
	_assoc *wasmtime.Func // _assoc()

	// _close:
	//  - Closes the all the file descriptors it owns.
	//  - Cleans up any other resouce it allocated within the WASM module.
	//  - Calls back to runtime by calling `env.defer` for the runtime to self-clean.
	_close *wasmtime.Func

	// _write:
	//  - if both `sourceConnFd` and `remoteConnFd` are valid, this will be a no-op.
	//  - if `callerConnFd` is invalid, this will return an error.
	//  - if `sourceConnFd` is valid, this will read from `callerConnFd` and write to `sourceConnFd`.
	//  - if `remoteConnFd` is valid, this will read from `callerConnFd` and write to `remoteConnFd`.
	// WASM will prepare a buffer at least `expected` bytes long before reading from `callerConnFd`.
	_write *wasmtime.Func // _write(expected int32) (actual int32)

	// _read:
	//  - if both `sourceConnFd` and `remoteConnFd` are valid, this will be a no-op.
	//  - if `callerConnFd` is invalid, this will return an error.
	//  - if `sourceConnFd` is valid, this will read from `sourceConnFd` and write to `callerConnFd`.
	//  - if `remoteConnFd` is valid, this will read from `remoteConnFd` and write to `callerConnFd`.
	_read *wasmtime.Func // _read()
}

func NewOutboundRuntimeConnV0(core *runtimeCore) (RuntimeConn, error) {
	rc := &RuntimeConnV0{core: core}
	err := rc.initializeOutboundConn()
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConnV0).initialize returned error: %w", err)
	}

	return rc, nil
}

func NewInboundRuntimeConnV0(core *runtimeCore) (RuntimeConn, error) {
	rc := &RuntimeConnV0{core: core}
	err := rc.initializeInboundConn()
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConnV0).initialize returned error: %w", err)
	}

	return rc, nil
}

func NewRelayingRuntimeConnV0(core *runtimeCore) (RuntimeConn, error) {
	rc := &RuntimeConnV0{core: core}
	err := rc.initializeRelayingConn()
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConnV0).initialize returned error: %w", err)
	}

	return rc, nil
}

// Read implements the net.Conn interface.
//
// It calls to the underlying user-oriented net.Conn's Read() method.
func (rcv *RuntimeConnV0) Read(b []byte) (n int, err error) {
	if rcv.uoConn == nil {
		return 0, errors.New("water: cannot read, (*RuntimeConnV0).uoConn is nil")
	}

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
	if rcv.uoConn == nil {
		return 0, errors.New("water: cannot write, (*RuntimeConnV0).uoConn is nil")
	}

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

	rcv.core.wd.CloseAllConn()

	if rcv.waConn != nil {
		rcv.waConn.Close()
	}

	if rcv.uoConn != nil {
		rcv.uoConn.Close()
	}

	return nil
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

// Generic initialization for connections that could be used to
// read or write.
func (rcv *RuntimeConnV0) initializeRWConn() error {
	rcv._read = rcv.core.instance.GetFunc(rcv.core.store, "_read")
	if rcv._read == nil {
		return fmt.Errorf("WASM module missing required function _read for V0")
	}
	rcv._write = rcv.core.instance.GetFunc(rcv.core.store, "_write")
	if rcv._write == nil {
		return fmt.Errorf("WASM module missing required function _write for V0")
	}

	// create a UnixConn pair
	var err error
	rcv.uoConn, rcv.waConn, err = socket.UnixConnPair("")
	if err != nil {
		return fmt.Errorf("socket.UnixConnPair returned error: %w", err)
	}

	return nil
}

func (rcv *RuntimeConnV0) initializeConnCloser() error {
	rcv._close = rcv.core.instance.GetFunc(rcv.core.store, "_close")
	if rcv._close == nil {
		return fmt.Errorf("WASM module missing required function _close for V0")
	}

	return nil
}

func (rcv *RuntimeConnV0) pushConfig() error {
	rcv._config = rcv.core.instance.GetFunc(rcv.core.store, "_config")
	if rcv._config == nil {
		return fmt.Errorf("WASM module missing required function _config for V0")
	}

	// push WAConfig
	configFd, err := rcv.core.store.PushFile(rcv.core.config.WAConfig.File(), wasmtime.READ_ONLY)
	if err != nil {
		return fmt.Errorf("(*wasmtime.Store).PushFile returned error: %w", err)
	}

	ret, err := rcv._config.Call(rcv.core.store, int32(configFd))
	if err != nil {
		return fmt.Errorf("_config returned error: %w", err)
	}
	return WASMErr(ret.(int32))
}

func (rcv *RuntimeConnV0) initializeOutboundConn() error {
	err := rcv.initializeRWConn()
	if err != nil {
		return err
	}

	err = rcv.initializeConnCloser()
	if err != nil {
		return err
	}

	err = rcv.pushConfig()
	if err != nil {
		return err
	}

	rcv._dial = rcv.core.instance.GetFunc(rcv.core.store, "_dial")
	if rcv._dial == nil {
		return fmt.Errorf("WASM module missing required function _dial for V0")
	}

	// push waConn to WASM
	waConnFile, err := socket.AsFile(rcv.waConn)
	if err != nil {
		return fmt.Errorf("socket.AsFile returned error: %w", err)
	}

	// push waConnFile to WASM
	wasmFd, err := rcv.core.store.PushFile(waConnFile, wasmtime.READ_WRITE)
	if err != nil {
		return fmt.Errorf("(*wasmtime.Store).PushFile returned error: %w", err)
	}

	// call _dial
	netFd, err := rcv._dial.Call(rcv.core.store, int32(wasmFd))
	if err != nil {
		return fmt.Errorf("_dial returned error: %w", err)
	}
	// type assertion
	if netFdInt32, ok := netFd.(int32); !ok {
		return fmt.Errorf("_dial returned non-int32 value")
	} else {
		// get the net.Conn from the WASM's world
		rcv.networkConn = rcv.core.wd.GetConnByFd(netFdInt32)
		if rcv.networkConn == nil {
			return fmt.Errorf("_dial returned invalid net.Conn")
		}
	}

	return nil
}

func (rcv *RuntimeConnV0) initializeInboundConn() error {
	err := rcv.initializeRWConn()
	if err != nil {
		return err
	}

	err = rcv.initializeConnCloser()
	if err != nil {
		return err
	}

	err = rcv.pushConfig()
	if err != nil {
		return err
	}

	rcv._accept = rcv.core.instance.GetFunc(rcv.core.store, "_accept")
	if rcv._dial == nil {
		return fmt.Errorf("WASM module missing required function _accept for V0")
	}

	// push waConn to WASM
	waConnFile, err := socket.AsFile(rcv.waConn)
	if err != nil {
		return fmt.Errorf("socket.AsFile returned error: %w", err)
	}

	// push waConnFile to WASM
	wasmFd, err := rcv.core.store.PushFile(waConnFile, wasmtime.READ_WRITE)
	if err != nil {
		return fmt.Errorf("(*wasmtime.Store).PushFile returned error: %w", err)
	}

	// call _accept
	netFd, err := rcv._accept.Call(rcv.core.store, int32(wasmFd))
	if err != nil {
		return fmt.Errorf("_accept returned error: %w", err)
	}
	// type assertion
	if netFdInt32, ok := netFd.(int32); !ok {
		return fmt.Errorf("_accept returned non-int32 value")
	} else {
		// get the net.Conn from the WASM's world
		rcv.networkConn = rcv.core.wd.GetConnByFd(netFdInt32)
		if rcv.networkConn == nil {
			return fmt.Errorf("_accept returned invalid net.Conn")
		}
	}

	return nil
}

func (rcv *RuntimeConnV0) initializeRelayingConn() error {
	err := rcv.initializeConnCloser()
	if err != nil {
		return err
	}

	err = rcv.pushConfig()
	if err != nil {
		return err
	}

	rcv._assoc = rcv.core.instance.GetFunc(rcv.core.store, "_assoc")
	if rcv._assoc == nil {
		return fmt.Errorf("WASM module missing required function _assoc for V0")
	}

	// call _assoc
	_, err = rcv._assoc.Call(rcv.core.store)
	if err != nil {
		return fmt.Errorf("_assoc returned error: %w", err)
	}

	return nil
}

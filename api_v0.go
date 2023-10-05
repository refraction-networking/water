//go:build !nov0

package water

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/internal/socket"
	v0 "github.com/gaukas/water/internal/v0"
	"github.com/gaukas/water/internal/wasm"
)

// WASMv0
type WASMv0 struct {
	*core

	_init *wasmtime.Func // _init()

	// `_config` is a WASM-exported function, in which WASM reads the config file
	// at the file descriptor specified by the first parameter.
	_config *wasmtime.Func // _config(fd i32)

	// _dial:
	//  - Calls to `env.dialh(apw) -> fd i32` to dial a network connection (wrapped with the
	//  application protocol) and bind it to one of its file descriptors, record the fd as
	//  `remoteConnFd`. This will be the fd it used to read/write data from/to the remote
	//  destination.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `remoteConnFd` to the caller to be kept track of.
	_dial *wasmtime.Func // _dial(callerConnFd i32) (remoteConnFd i32)

	// _accept:
	//  - Calls to `env.accepth(apw) -> fd i32` to accept a network connection (wrapped with the
	//  application protocol) and bind it to one of its file descriptors, record the fd as
	//  `sourceConnFd`. This will be the fd it used to read/write data from/to the source
	//  address.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `sourceConnFd` to the caller to be kept track of.
	_accept *wasmtime.Func // _accept(callerConnFd i32) (sourceConnFd i32)

	// _read:
	//  - if both `sourceConnFd` and `remoteConnFd` are valid, this will be a no-op.
	//  - if `callerConnFd` is invalid, this will return an error.
	//  - if `sourceConnFd` is valid, this will read from `sourceConnFd` and write to `callerConnFd`.
	//  - if `remoteConnFd` is valid, this will read from `remoteConnFd` and write to `callerConnFd`.
	_read *wasmtime.Func // _read() (err int32)

	// _write:
	//  - if both `sourceConnFd` and `remoteConnFd` are valid, this will be a no-op.
	//  - if `callerConnFd` is invalid, this will return an error.
	//  - if `sourceConnFd` is valid, this will read from `callerConnFd` and write to `sourceConnFd`.
	//  - if `remoteConnFd` is valid, this will read from `callerConnFd` and write to `remoteConnFd`.
	_write *wasmtime.Func // _write() (err int32)

	// _close:
	//  - Closes the all the file descriptors it owns.
	//  - Cleans up any other resouce it allocated within the WASM module.
	//  - Calls back to runtime by calling `env.defer` for the runtime to self-clean.
	_close *wasmtime.Func

	dialer   *v0.WASIDialer
	listener *v0.WASIListener

	gcfixOnce  *sync.Once
	pushedConn map[int32]*struct {
		conn net.Conn
		file *os.File
	}

	deferOnce     *sync.Once
	deferredFuncs []func()
}

func NewWASMv0(core *core) *WASMv0 {
	wasm := &WASMv0{
		core:      core,
		gcfixOnce: new(sync.Once),
		pushedConn: make(map[int32]*struct {
			conn net.Conn
			file *os.File
		}),
		deferOnce:     new(sync.Once),
		deferredFuncs: make([]func(), 0),
	}

	runtime.SetFinalizer(wasm, func(w *WASMv0) {
		w.DeferAll()
		w.Cleanup()
	})

	return wasm
}

func (w *WASMv0) LinkNetworkInterface(dialer *v0.WASIDialer, listener *v0.WASIListener) error {
	if w.linker == nil {
		return fmt.Errorf("water: linker not set, is Core initialized?")
	}

	if dialer != nil {
		if err := w.linker.FuncNew("env", "dialh", v0.WASIConnectFuncType, dialer.WrappedDial()); err != nil {
			return fmt.Errorf("water: linking WASI dialer, (*wasmtime.Linker).FuncNew: %w", err)
		}
	} else {
		if err := w.linker.FuncNew("env", "dialh", v0.WASIConnectFuncType, v0.WrappedNopWASIConnectFunc()); err != nil {
			return fmt.Errorf("water: linking NOP dialer, (*wasmtime.Linker).FuncNew: %w", err)
		}
	}
	w.dialer = dialer

	if listener != nil {
		if err := w.linker.FuncNew("env", "accepth", v0.WASIConnectFuncType, listener.WrappedAccept()); err != nil {
			return fmt.Errorf("water: linking WASI listener, (*wasmtime.Linker).FuncNew: %w", err)
		}
	} else {
		if err := w.linker.FuncNew("env", "accepth", v0.WASIConnectFuncType, v0.WrappedNopWASIConnectFunc()); err != nil {
			return fmt.Errorf("water: linking NOP listener, (*wasmtime.Linker).FuncNew: %w", err)
		}
	}
	w.listener = listener

	return nil
}

// Initialize initializes the WASMv0 runtime by getting all the exported functions from
// the WASM module.
//
// All imports must be set before calling this function.
func (w *WASMv0) Initialize() error {
	if w.core == nil {
		return fmt.Errorf("water: no core loaded")
	}

	var err error
	// import deferh function
	if err = w.linker.FuncWrap("env", "deferh", func() {
		w.DeferAll()
	}); err != nil {
		return fmt.Errorf("water: linking deferh function, (*wasmtime.Linker).FuncWrap: %w", err)
	}

	// instantiate the WASM module
	if err = w.Instantiate(); err != nil {
		return err
	}

	// _init
	w._init = w.Instance().GetFunc(w.Store(), "_init")
	if w._init == nil {
		return fmt.Errorf("water: WASM module does not export _init")
	}

	// _config
	w._config = w.Instance().GetFunc(w.Store(), "_config")
	if w._config == nil {
		return fmt.Errorf("water: WASM module does not export _config")
	}

	// _dial
	w._dial = w.Instance().GetFunc(w.Store(), "_dial")
	if w._dial == nil {
		return fmt.Errorf("water: WASM module does not export _dial")
	}

	// _accept
	w._accept = w.Instance().GetFunc(w.Store(), "_accept")
	if w._accept == nil {
		return fmt.Errorf("water: WASM module does not export _accept")
	}

	// _close
	w._close = w.Instance().GetFunc(w.Store(), "_close")
	if w._close == nil {
		return fmt.Errorf("water: WASM module does not export _close")
	}

	// push file to WASM
	configFd, err := w.Store().PushFile(w.Config().WAConfig.File(), wasmtime.READ_ONLY)
	if err != nil {
		return fmt.Errorf("water: pushing config file to store failed: %w", err)
	}

	// config WASM instance
	ret, err := w._config.Call(w.Store(), int32(configFd))
	if err != nil {
		return fmt.Errorf("water: calling _config function returned error: %w", err)
	}
	return wasm.WASMErr(ret.(int32))
}

// Caller need to make sure anything caller writes to the WASM module is
// readable on the callerConn.
func (w *WASMv0) InitializeReadWriter() error {
	// _read
	w._read = w.Instance().GetFunc(w.Store(), "_read")
	if w._read == nil {
		return fmt.Errorf("water: WASM module does not export _read")
	}

	// _write
	w._write = w.Instance().GetFunc(w.Store(), "_write")
	if w._write == nil {
		return fmt.Errorf("water: WASM module does not export _write")
	}

	return nil
}

func (w *WASMv0) DialFrom(callerConn net.Conn) (destConn net.Conn, err error) {
	callerFd, err := w.PushConn(callerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn to store failed: %w", err)
	}

	ret, err := w._dial.Call(w.Store(), callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _dial function returned error: %w", err)
	}

	if remoteFd, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _dial function signature")
	} else {
		if remoteFd < 0 {
			return nil, wasm.WASMErr(remoteFd)
		} else {
			destConn := w.dialer.GetConnByFd(remoteFd)
			if destConn == nil {
				return nil, fmt.Errorf("water: failed to look up network connection by fd")
			}
			return destConn, nil
		}
	}
}

func (w *WASMv0) AcceptFor(callerConn net.Conn) (sourceConn net.Conn, err error) {
	callerFd, err := w.PushConn(callerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn to store failed: %w", err)
	}

	ret, err := w._accept.Call(w.Store(), callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _accept function returned error: %w", err)
	}

	if sourceFd, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _accept function signature")
	} else {
		if sourceFd < 0 {
			return nil, wasm.WASMErr(sourceFd)
		} else {
			sourceConn := w.listener.GetConnByFd(sourceFd)
			if sourceConn == nil {
				return nil, fmt.Errorf("water: failed to look up network connection by fd")
			}
			return sourceConn, nil
		}
	}
}

func (w *WASMv0) PushConn(conn net.Conn) (fd int32, err error) {
	w.gcfixOnce.Do(func() {
		if GCFIX {
			// create temp file
			var f *os.File
			f, err = os.CreateTemp("", "water-gcfix")
			if err != nil {
				return
			}

			// push dummy file
			fd, err := w.Store().PushFile(f, wasmtime.READ_ONLY)
			if err != nil {
				return
			}

			// save dummy file to map
			w.pushedConn[int32(fd)] = &struct {
				conn net.Conn
				file *os.File
			}{
				conn: nil,
				file: f,
			}
		}
	})

	if err != nil {
		return 0, fmt.Errorf("water: creating temp file for GC fix: %w", err)
	}

	connFile, err := socket.AsFile(conn)
	if err != nil {
		return 0, fmt.Errorf("water: converting conn to file failed: %w", err)
	}

	fdu32, err := w.store.PushFile(connFile, wasmtime.READ_WRITE)
	if err != nil {
		return 0, fmt.Errorf("water: pushing conn file to store failed: %w", err)
	}
	fd = int32(fdu32)

	w.pushedConn[fd] = &struct {
		conn net.Conn
		file *os.File
	}{
		conn: conn,
		file: connFile,
	}

	return fd, nil
}

func (w *WASMv0) DeferAll() {
	w.deferOnce.Do(func() { // execute all deferred functions if not yet executed
		for _, f := range w.deferredFuncs {
			f()
		}
	})
}

func (w *WASMv0) Defer(f func()) {
	w.deferredFuncs = append(w.deferredFuncs, f)
}

func (w *WASMv0) Cleanup() {
	// clean up pushed files
	var keyList []int32
	for k, v := range w.pushedConn {
		if v != nil {
			if v.file != nil {
				v.file.Close()
				v.file = nil
			}
			if v.conn != nil {
				v.conn.Close()
				v.conn = nil
			}
		}
		keyList = append(keyList, k)
	}
	for _, k := range keyList {
		delete(w.pushedConn, k)
	}

	// clean up deferred functions
	w.deferredFuncs = nil

	w.dialer.CloseAllConn()
	w.listener.CloseAllConn()
}

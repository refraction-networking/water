package v0

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water"
	"github.com/gaukas/water/internal/wasm"
)

// TransportModule acts like a "managed core". It was build to provide WebAssembly
// Transport Module API-facing functions and utilities that are exclusive to
// version 0.
type TransportModule struct {
	core water.Core

	_init *wasmtime.Func // _init() -> i32

	// _dial:
	//  - Calls to `env.host_dial() -> fd: i32` to dial a network connection and bind it to one
	//  of its file descriptors, record the fd for `remoteConnFd`. This will be the fd it used
	//  to read/write data from/to the remote destination.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `remoteConnFd` to the caller.
	_dial *wasmtime.Func // _dial(callerConnFd i32) -> (remoteConnFd i32)

	// _accept:
	//  - Calls to `env.host_accept() -> fd: i32` to accept a network connection and bind it
	//  to one of its file descriptors, record the fd for `sourceConnFd`. This will be the fd
	//  it used to read/write data from/to the source address.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `sourceConnFd` to the caller.
	_accept *wasmtime.Func // _accept(callerConnFd i32) -> (sourceConnFd i32)

	// _associate:
	//  - Calls to `env.host_accept() -> fd: i32` to accept a network connection and bind it
	//  to one of its file descriptors, record the fd for `sourceConnFd`. This will be the fd
	//  it used to read/write data from/to the source address.
	//  - Calls to `env.host_dial() -> fd: i32` to dial a network connection and bind it to one
	//  of its file descriptors, record the fd for `remoteConnFd`. This will be the fd it used
	//  to read/write data from/to the remote destination.
	//  - Returns 0 to the caller or an error code if any of the above steps failed.
	_associate *wasmtime.Func // _associate() -> (err i32)

	// backgroundWorker is used to replace the deprecated read-write-close model.
	// We put it in a inlined struct for better code styling.
	backgroundWorker *struct {
		// _cancel_with:
		//  - Provides a socket to the WASM module for it to listen to cancellation events.
		//  - on Cancel() call, the pipe will be written to by the host (caller).
		//  - WebAssembly instance should select on the socket and handle cancellation ASAP.
		//
		// This is a workaround for not being able to call another WASM function until the
		// current one returns. And apparently this function needs to be called BEFORE the
		// blocking _worker() function.
		_cancel_with *wasmtime.Func // _cancel_with(fd int32) (err int32)

		// _worker provides a blocking function for the WASM module to run a worker thread.
		// In the worker thread, WASM module should select on all previously pushed sockets
		// (typically, two among callerConnFd, remoteConnFd, and sourceConnFd) and handle
		// data bi-directionally. The exact behavior is up to the WebAssembly module and
		// overall it drives data flow below based on the identity of the module:
		//  - Dialer: callerConn <==> remoteConn
		//  - Listener: callerConn <==> sourceConn
		//  - Relay: sourceConn <==> remoteConn
		//
		// The worker thread should exit and return when the cancellation pipe is available
		// for reading. For the current design, the content read from the pipe does not
		// include meaningful data.
		_worker *wasmtime.Func // _worker() (err int32)

		// a channel used to send errors from the worker thread to the host in a non-blocking
		// manner. When the worker thread exits, this channel will be closed.
		//
		// Read-only to the caller. Write-only to the worker thread.
		chanWorkerErr chan error

		// a socket used to cancel the worker thread. When the host calls Cancel(), it should
		// write to this socket.
		cancelSocket net.Conn
	}

	gcfixOnce  *sync.Once
	pushedConn map[int32]*struct {
		groundTruthConn net.Conn // the conn we want to keep alive
		pushedFile      *os.File // the file we actually pushed to WebAssembly world also needs to be kept alive
	}
	pushedConnMutex *sync.RWMutex

	deferOnce     *sync.Once
	deferredFuncs []func()
}

func Core2TransportModule(core water.Core) *TransportModule {
	wasm := &TransportModule{
		core:      core,
		gcfixOnce: new(sync.Once),
		pushedConn: make(map[int32]*struct {
			groundTruthConn net.Conn
			pushedFile      *os.File
		}),
		pushedConnMutex: new(sync.RWMutex),
		deferOnce:       new(sync.Once),
		deferredFuncs:   make([]func(), 0),
	}

	// SetFinalizer, so Go GC automatically cleans up the WASM runtime
	// and all opened file descriptors (if any) associated with it
	// when the TransportModule is garbage collected.
	runtime.SetFinalizer(wasm, func(tm *TransportModule) {
		_ = tm.Cancel() // tm cannot be nil here as we just set it above
		tm.DeferAll()
		tm.Cleanup()
	})

	return wasm
}

func (tm *TransportModule) LinkNetworkInterface(dialer *ManagedDialer, listener net.Listener) error {
	if tm.core.Linker() == nil {
		return fmt.Errorf("water: linker not set, is Core initialized?")
	}

	// import host_dial
	if dialer != nil {
		if err := tm.core.Linker().FuncNew(
			"env", "host_dial", WASIConnectFuncType,
			WrapConnectFunc(
				func(caller *wasmtime.Caller) (fd int32, err error) {
					conn, err := dialer.Dial()
					if err != nil {
						return wasm.GENERAL_ERROR, err
					}

					return tm.PushConn(conn, caller)
				},
			),
		); err != nil {
			return fmt.Errorf("water: linking WASI dialer, (*wasmtime.Linker).FuncNew: %w", err)
		}
	} else {
		if err := tm.core.Linker().FuncNew(
			"env", "host_dial", WASIConnectFuncType,
			WrappedUnimplementedWASIConnectFunc(),
		); err != nil {
			return fmt.Errorf("water: linking NOP dialer, (*wasmtime.Linker).FuncNew: %w", err)
		}
	}

	// import host_accept
	if listener != nil {
		if err := tm.core.Linker().FuncNew(
			"env", "host_accept", WASIConnectFuncType,
			WrapConnectFunc(
				func(caller *wasmtime.Caller) (fd int32, err error) {
					conn, err := listener.Accept()
					if err != nil {
						return wasm.GENERAL_ERROR, err
					}

					return tm.PushConn(conn, caller)
				},
			),
		); err != nil {
			return fmt.Errorf("water: linking WASI listener, (*wasmtime.Linker).FuncNew: %w", err)
		}
	} else {
		if err := tm.core.Linker().FuncNew(
			"env", "host_accept", WASIConnectFuncType,
			WrappedUnimplementedWASIConnectFunc(),
		); err != nil {
			return fmt.Errorf("water: linking NOP listener, (*wasmtime.Linker).FuncNew: %w", err)
		}
	}

	return nil
}

// Initialize initializes the WASMv0 runtime by getting all the exported functions from
// the WASM module.
//
// All imports must be set before calling this function.
func (tm *TransportModule) Initialize() error {
	if tm.core == nil {
		return fmt.Errorf("water: no core loaded")
	}

	var err error
	// import host_defer function
	if err = tm.core.Linker().FuncWrap("env", "host_defer", func() {
		tm.DeferAll()
	}); err != nil {
		return fmt.Errorf("water: linking deferh function, (*wasmtime.Linker).FuncWrap: %w", err)
	}

	// import pull_config function (it is called pushConfig here in the host)
	if err := tm.core.Linker().FuncNew("env", "pull_config", WASIConnectFuncType, WrapConnectFunc(tm.pushConfig)); err != nil {
		return fmt.Errorf("water: linking pull_config function, (*wasmtime.Linker).FuncNew: %w", err)
	}

	// instantiate the WASM module
	if err = tm.core.Instantiate(); err != nil {
		return err
	}

	// _init
	tm._init = tm.core.Instance().GetFunc(tm.core.Store(), "_water_init")
	if tm._init == nil {
		return fmt.Errorf("water: WASM module does not export _water_init")
	}

	// _dial
	tm._dial = tm.core.Instance().GetFunc(tm.core.Store(), "_water_dial")
	// if tm._dial == nil {
	// 	return fmt.Errorf("water: WASM module does not export _dial")
	// }

	// _accept
	tm._accept = tm.core.Instance().GetFunc(tm.core.Store(), "_water_accept")
	// if tm._accept == nil {
	// 	return fmt.Errorf("water: WASM module does not export _accept")
	// }

	// _associate
	tm._associate = tm.core.Instance().GetFunc(tm.core.Store(), "_water_associate")
	// if tm._associate == nil {
	// 	return fmt.Errorf("water: WASM module does not export _associate")
	// }

	// _cancel_with
	_cancel_with := tm.core.Instance().GetFunc(tm.core.Store(), "_water_cancel_with")
	if _cancel_with == nil {
		return fmt.Errorf("water: WASM module does not export _water_cancel_with")
	}

	// _worker
	_worker := tm.core.Instance().GetFunc(tm.core.Store(), "_water_worker")
	if _worker == nil {
		return fmt.Errorf("water: WASM module does not export _water_worker")
	}

	// wrap _cancel_with and _worker
	tm.backgroundWorker = &struct {
		_cancel_with  *wasmtime.Func
		_worker       *wasmtime.Func
		chanWorkerErr chan error
		cancelSocket  net.Conn
	}{
		_cancel_with:  _cancel_with,
		_worker:       _worker,
		chanWorkerErr: make(chan error, 8), // at max 1 error would occur, but we will write multiple copies
		// cancelSocket:  nil,
	}

	// call _init
	ret, err := tm._init.Call(tm.core.Store())
	if err != nil {
		return fmt.Errorf("water: calling _water_init function returned error: %w", err)
	}

	return wasm.WASMErr(ret.(int32))
}

// DialFrom is used to make the Transport Module act as a dialer and
// dial a network connection.
//
// Takes the reverse caller connection as an argument, which is used
// to communicate with the caller.
func (tm *TransportModule) DialFrom(reverseCallerConn net.Conn) (destConn net.Conn, err error) {
	// check if _dial is exported
	if tm._dial == nil {
		return nil, fmt.Errorf("water: WASM module does not export _water_dial")
	}

	callerFd, err := tm.PushConn(reverseCallerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn to store failed: %w", err)
	}

	ret, err := tm._dial.Call(tm.core.Store(), callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _dial function returned error: %w", err)
	}

	if remoteFd, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _dial function signature")
	} else {
		if remoteFd < 0 {
			return nil, wasm.WASMErr(remoteFd)
		} else {
			destConn := tm.GetPushedConn(remoteFd)
			if destConn == nil {
				return nil, fmt.Errorf("water: failed to look up network connection by fd")
			}
			return destConn, nil
		}
	}
}

// AcceptFor is used to make the Transport Module act as a listener and
// accept a network connection.
func (tm *TransportModule) AcceptFor(reverseCallerConn net.Conn) (sourceConn net.Conn, err error) {
	// check if _accept is exported
	if tm._accept == nil {
		return nil, fmt.Errorf("water: WASM module does not export _water_accept")
	}

	callerFd, err := tm.PushConn(reverseCallerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn to store failed: %w", err)
	}

	ret, err := tm._accept.Call(tm.core.Store(), callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _accept function returned error: %w", err)
	}

	if sourceFd, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _accept function signature")
	} else {
		if sourceFd < 0 {
			return nil, wasm.WASMErr(sourceFd)
		} else {
			sourceConn := tm.GetPushedConn(sourceFd)
			if sourceConn == nil {
				return nil, fmt.Errorf("water: failed to look up network connection by fd")
			}
			return sourceConn, nil
		}
	}
}

// Associate is used to make the Transport Module act as a relay and
// associate two network connections, where one is from a source via
// a listener, and the other is to a destination via a dialer.
func (tm *TransportModule) Associate() error {
	// check if _associate is exported
	if tm._associate == nil {
		return fmt.Errorf("water: WASM module does not export _water_associate")
	}

	ret, err := tm._associate.Call(tm.core.Store())
	if err != nil {
		return fmt.Errorf("water: calling _associate function returned error: %w", err)
	}

	return wasm.WASMErr(ret.(int32))
}

// Worker spins up a worker thread for the WASM module to run a blocking function.
//
// This function is non-blocking UNLESS the error occurred before entering the worker
// thread. In that case, the error will be returned immediately.
//
// Worker's implementation diverges by opting to use TCPConn or UnixConn.
// See transport_module_tcpconn.go and transport_module_unixconn.go for details.
//
// func (tm *TransportModule) Worker() error

// Cancel cancels the worker thread if it is running and returns
// the error returned by the worker thread. This call is designed
// to block until the worker thread exits.
func (tm *TransportModule) Cancel() error {
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized")
	}

	if tm.backgroundWorker.cancelSocket == nil {
		return fmt.Errorf("water: Transport Module is cancelled")
	}

	select {
	case err := <-tm.backgroundWorker.chanWorkerErr: // if already returned, we don't need to cancel
		if err != nil {
			return fmt.Errorf("water: worker thread returned error: %w", err)
		}
		return nil
	default: // otherwise we will need to cancel the worker thread
		break
	}

	// write to the cancel pipe
	if _, err := tm.backgroundWorker.cancelSocket.Write([]byte{0}); err != nil {
		return fmt.Errorf("water: writing to cancel pipe failed: %w", err)
	}

	// wait for the worker thread to exit
	if err := <-tm.backgroundWorker.chanWorkerErr; err != nil {
		return fmt.Errorf("water: worker thread returned error: %w", err)
	}

	if err := tm.backgroundWorker.cancelSocket.Close(); err != nil {
		return fmt.Errorf("water: closing cancel pipe failed: %w", err)
	}

	tm.backgroundWorker.cancelSocket = nil

	return nil
}

// WorkerErrored returns a channel that will be closed when the worker thread exits.
func (tm *TransportModule) WorkerErrored() <-chan error {
	if tm.backgroundWorker == nil {
		return nil
	}
	return tm.backgroundWorker.chanWorkerErr
}

// wasiCtx is an interface used to push files to WASI store.
//
// In (the patched) package wasmtime, WasiCtx, Caller, and Store
// all implement this interface.
type wasiCtx interface {
	PushFile(file *os.File, accessMode wasmtime.WasiFileAccessMode) (uint32, error)
}

// PushConn's implementation diverges by opting to use TCPConn or UnixConn.
// See transport_module_tcpconn.go and transport_module_unixconn.go for details.
//
// func (tm *TransportModule) PushConn(conn net.Conn, wasiCtxOverride ...wasiCtx) (fd int32, err error)

func (tm *TransportModule) DeferAll() {
	tm.deferOnce.Do(func() { // execute all deferred functions if not yet executed
		for _, f := range tm.deferredFuncs {
			f()
		}
	})
}

func (tm *TransportModule) Defer(f func()) {
	tm.deferredFuncs = append(tm.deferredFuncs, f)
}

func (tm *TransportModule) Cleanup() {
	// clean up pushed files
	var keyList []int32
	tm.pushedConnMutex.Lock()
	for k, v := range tm.pushedConn {
		if v != nil {
			if v.pushedFile != nil {
				v.pushedFile.Close()
				v.pushedFile = nil
			}
			if v.groundTruthConn != nil {
				v.groundTruthConn.Close()
				v.groundTruthConn = nil
			}
		}
		keyList = append(keyList, k)
	}
	for _, k := range keyList {
		delete(tm.pushedConn, k)
	}
	tm.pushedConnMutex.Unlock()

	// clean up deferred functions
	tm.deferredFuncs = nil
}

func (tm *TransportModule) pushConfig(caller *wasmtime.Caller) (int32, error) {
	// get config file
	configFile := tm.core.Config().TMConfig.File()
	if configFile == nil {
		return wasm.INVALID_FD, nil // we don't return error here so no trap is triggered
	}

	// push file to WASM
	configFd, err := caller.PushFile(configFile, wasmtime.READ_ONLY)
	if err != nil {
		return wasm.INVALID_FD, err
	}

	return int32(configFd), nil
}

func (tm *TransportModule) GetPushedConn(fd int32) net.Conn {
	tm.pushedConnMutex.RLock()
	defer tm.pushedConnMutex.RUnlock()
	if tm.pushedConn == nil {
		return nil
	}
	if v, ok := tm.pushedConn[fd]; ok {
		return v.groundTruthConn
	}
	return nil
}

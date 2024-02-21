package v0

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync"
	"syscall"

	"github.com/refraction-networking/water"
	"github.com/refraction-networking/water/internal/log"
	"github.com/refraction-networking/water/internal/socket"
	"github.com/refraction-networking/water/internal/wasip1"
	"github.com/tetratelabs/wazero/api"
)

// TransportModule acts like a "managed core". It was build to provide WebAssembly
// Transport Module API-facing functions and utilities that are exclusive to
// version 0.
type TransportModule struct {
	core      water.Core
	coreMutex sync.RWMutex

	_init func() (int32, error) // _init() -> (err i32)

	// _dial:
	//  - Calls to `env.host_dial() -> fd: i32` to dial a network connection and bind it to one
	//  of its file descriptors, record the fd for `remoteConnFd`. This will be the fd it used
	//  to read/write data from/to the remote destination.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `remoteConnFd` to the caller.
	_dial func(int32) (int32, error) // _water_dial(callerConnFd i32) -> (remoteConnFd i32)

	// _accept:
	//  - Calls to `env.host_accept() -> fd: i32` to accept a network connection and bind it
	//  to one of its file descriptors, record the fd for `sourceConnFd`. This will be the fd
	//  it used to read/write data from/to the source address.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `sourceConnFd` to the caller.
	_accept func(int32) (int32, error) // _water_accept(callerConnFd i32) -> (sourceConnFd i32)

	// _associate:
	//  - Calls to `env.host_accept() -> fd: i32` to accept a network connection and bind it
	//  to one of its file descriptors, record the fd for `sourceConnFd`. This will be the fd
	//  it used to read/write data from/to the source address.
	//  - Calls to `env.host_dial() -> fd: i32` to dial a network connection and bind it to one
	//  of its file descriptors, record the fd for `remoteConnFd`. This will be the fd it used
	//  to read/write data from/to the remote destination.
	//  - Returns 0 to the caller or an error code if any of the above steps failed.
	_associate func() (int32, error) // _water_associate() -> (err i32)

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
		_cancel_with func(int32) (int32, error) // _water_cancel_with(fd i32) -> (err i32)

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
		_worker func() (int32, error) // _water_worker() (err int32)

		// a channel used to send errors from the worker thread to the host in a non-blocking
		// manner. When the worker thread exits, this channel will be closed.
		//
		// Read-only to the caller. Write-only to the worker thread.
		chanWorkerErr chan error

		// a socket used to cancel the worker thread. When the host calls Cancel(), it should
		// write to this socket.
		cancelSocket net.Conn
	}

	// gcfixOnce  sync.Once
	pushedConn      map[int32]net.Conn // the conn we want to keep alive
	pushedConnMutex sync.RWMutex

	deferOnce     sync.Once
	deferredFuncs []func()

	closeOnce sync.Once
}

// UpgradeCore upgrades a water.Core to a v0 TransportModule.
func UpgradeCore(core water.Core) *TransportModule {
	wasm := &TransportModule{
		core:          core,
		pushedConn:    make(map[int32]net.Conn),
		deferredFuncs: make([]func(), 0),
	}

	err := core.WASIPreview1()
	if err != nil {
		log.LErrorf(core.Logger(), "water: WASI preview 1 is not supported: %v", err)
		return nil
	}

	// SetFinalizer, so Go GC automatically cleans up the WASM runtime
	// and all opened file descriptors (if any) associated with it
	// when the TransportModule is garbage collected.
	runtime.SetFinalizer(wasm, func(tm *TransportModule) {
		tm.Close()
	})

	return wasm
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
		return nil, fmt.Errorf("water: pushing caller conn failed: %w", err)
	}

	sourceFd, err := tm._accept(callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _accept: %w", err)
	} else {
		sourceConn := tm.GetPushedConn(sourceFd)
		if sourceConn == nil {
			return nil, fmt.Errorf("water: failed to look up network connection by fd")
		}
		return sourceConn, nil
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

	_, err := tm._associate()
	if err != nil {
		return fmt.Errorf("water: calling _associate function returned error: %w", err)
	}
	return nil
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

// Clean up the Transport Module by closing all connections pushed into the Transport Module.
func (tm *TransportModule) Cleanup() {
	// clean up pushed files
	var keyList []int32
	tm.pushedConnMutex.Lock()
	for k, v := range tm.pushedConn {
		if v != nil {
			if err := v.Close(); err != nil {
				log.LErrorf(tm.Core().Logger(), "water: closing pushed connection failed: %v", err)
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

	// clean up all saved functions
	tm._init = nil
	tm._dial = nil
	tm._accept = nil
	tm._associate = nil
	tm.backgroundWorker._cancel_with = nil
	tm.backgroundWorker._worker = nil
}

func (tm *TransportModule) Close() error {
	var err error

	tm.closeOnce.Do(func() {
		tm.DeferAll()
		err = tm.Cancel()
		tm.Cleanup()
		tm.coreMutex.Lock()
		if tm.core != nil {
			tm.core.Close()
			tm.core = nil
		}
		tm.coreMutex.Unlock()
	})

	return err
}

func (tm *TransportModule) Core() water.Core {
	tm.coreMutex.RLock()
	core := tm.core
	defer tm.coreMutex.RUnlock()
	return core
}

func (tm *TransportModule) Defer(f func()) {
	tm.deferredFuncs = append(tm.deferredFuncs, f)
}

func (tm *TransportModule) DeferAll() {
	tm.deferOnce.Do(func() { // execute all deferred functions ONLY IF not yet executed
		for _, f := range tm.deferredFuncs {
			f()
		}
	})
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
		return nil, fmt.Errorf("water: pushing caller conn failed: %w", err)
	}

	remoteFd, err := tm._dial(callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _dial: %w", err)
	} else {
		destConn := tm.GetPushedConn(remoteFd)
		if destConn == nil {
			return nil, fmt.Errorf("water: failed to look up network connection by fd")
		}
		return destConn, nil
	}
}

func (tm *TransportModule) LinkNetworkInterface(dialer *ManagedDialer, listener net.Listener) error {
	var dialerFunc func() (fd int32)
	if dialer != nil {
		dialerFunc = func() (fd int32) {
			conn, err := dialer.Dial()
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: dialer.Dial: %v", err)
				return wasip1.EncodeWATERError(syscall.ENOTCONN) // not connected
			}
			fd, err = tm.PushConn(conn)
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: PushConn: %v", err)
			}
			return fd
		}
	} else {
		dialerFunc = func() (fd int32) {
			return wasip1.EncodeWATERError(syscall.ENODEV) // no such device
		}
	}

	if err := tm.Core().ImportFunction("env", "host_dial", dialerFunc); err != nil {
		if dialer != nil || err != water.ErrFuncNotImported {
			return fmt.Errorf("water: linking dialer function, (*water.Core).ImportFunction: %w", err)
		}
	}

	var acceptFunc func() (fd int32)
	if listener != nil {
		acceptFunc = func() (fd int32) {
			conn, err := listener.Accept()
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: listener.Accept: %v", err)
				return wasip1.EncodeWATERError(syscall.ENOTCONN) // not connected
			}
			fd, err = tm.PushConn(conn)
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: PushConn: %v", err)
			}
			return fd
		}
	} else {
		acceptFunc = func() (fd int32) {
			return wasip1.EncodeWATERError(syscall.ENODEV) // no such device
		}
	}

	if err := tm.Core().ImportFunction("env", "host_accept", acceptFunc); err != nil {
		if listener != nil || err != water.ErrFuncNotImported {
			return fmt.Errorf("water: linking listener function, (*water.Core).ImportFunction: %w", err)
		}
	}

	return nil
}

// Initialize initializes the WASMv0 runtime by getting all the exported functions from
// the WASM module.
//
// All imports must be set before calling this function.
func (tm *TransportModule) Initialize() error {
	if tm.Core() == nil {
		return fmt.Errorf("water: core is not initialized")
	}

	var err error
	// import host_defer function
	if err = tm.Core().ImportFunction("env", "host_defer", func() {
		// tm.DeferAll() // Deprecated: do nothing here.
		log.LWarnf(tm.Core().Logger(), "water: host_defer is invoked by WATM, since it is deprecated, it does nothing")
	}); err != nil {
		if err == water.ErrFuncNotImported {
			// Deprecated: do nothing here.
			// log.LWarnf(tm.Core().Logger(), "water: host_defer function is not imported by WATM, "+
			// 	"deferred functions will not be executed when WATM exits")
		} else {
			return fmt.Errorf("water: (*water.Core).ImportFunction returned error "+
				"when importing host_defer function: %w", err)
		}
	} else {
		log.LWarnf(tm.Core().Logger(), "water: host_defer function is imported by WATM, "+
			"it is deprecated and will NOT be executed when WATM exits")
	}

	// import pull_config function (it is called pushConfig here in the host)
	if err = tm.Core().ImportFunction("env", "pull_config", tm.pushConfig); err != nil {
		if err == water.ErrFuncNotImported {
			// If a config is provided, we will warn the user that the config WILL NOT be
			// pushed to the WASM module.
			if tm.Core().Config().TransportModuleConfig != nil {
				f, err := tm.Core().Config().TransportModuleConfig.AsFile()
				if f != nil && err == nil {
					// there is a config file provided, must warn
					log.LWarnf(tm.Core().Logger(), "water: pull_config function is not imported by WATM, "+
						"config file will not be pushed to the WASM module")
				}
			}
		} else {
			return fmt.Errorf("water: (*water.Core).ImportFunction returned error "+
				"when importing pull_config function: %w", err)
		}
	}

	// instantiate the WASM module
	if err = tm.Core().Instantiate(); err != nil {
		return err
	}

	coreCtx := tm.Core().Context()

	// _init
	init := tm.Core().ExportedFunction("_water_init")
	if init == nil {
		return fmt.Errorf("water: WASM module does not export _water_init")
	} else {
		// check signature:
		//  _water_init() -> (err i32)
		if len(init.Definition().ParamTypes()) != 0 {
			return fmt.Errorf("water: _water_init function expects 0 argument, got %d", len(init.Definition().ParamTypes()))
		}

		if len(init.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: _water_init function expects 1 result, got %d", len(init.Definition().ResultTypes()))
		} else if init.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_init function expects result type i32, got %s", api.ValueTypeName(init.Definition().ResultTypes()[0]))
		}

		tm._init = func() (int32, error) {
			ret, err := init.Call(coreCtx)
			if err != nil {
				return 0, fmt.Errorf("water: calling _water_init function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _dial
	dial := tm.Core().ExportedFunction("_water_dial")
	if dial == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export _water_dial, water.Dialer will not work.")
		tm._dial = func(callerFd int32) (int32, error) {
			return 0, water.ErrUnimplementedDialer
		}
	} else {
		// check signature:
		//  _water_dial(callerFd i32) -> (remoteFd i32)
		if len(dial.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: _water_dial function expects 1 argument, got %d", len(dial.Definition().ParamTypes()))
		} else if dial.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_dial function expects argument type i32, got %s", api.ValueTypeName(dial.Definition().ParamTypes()[0]))
		}

		if len(dial.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: _water_dial function expects 1 result, got %d", len(dial.Definition().ResultTypes()))
		} else if dial.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_dial function expects result type i32, got %s", api.ValueTypeName(dial.Definition().ResultTypes()[0]))
		}

		tm._dial = func(callerFd int32) (int32, error) {
			ret, err := dial.Call(coreCtx, api.EncodeI32(callerFd))
			if err != nil {
				return 0, fmt.Errorf("water: calling _water_dial function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _accept
	accept := tm.Core().ExportedFunction("_water_accept")
	if accept == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export _water_accept, water.Listener will not work.")
		tm._accept = func(callerFd int32) (int32, error) {
			return 0, water.ErrUnimplementedListener
		}
	} else {
		// check signature:
		//  _water_accept(callerFd i32) -> (sourceFd i32)
		if len(accept.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: _water_accept function expects 1 argument, got %d", len(accept.Definition().ParamTypes()))
		} else if accept.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_accept function expects argument type i32, got %s", api.ValueTypeName(accept.Definition().ParamTypes()[0]))
		}

		if len(accept.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: _water_accept function expects 1 result, got %d", len(accept.Definition().ResultTypes()))
		} else if accept.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_accept function expects result type i32, got %s", api.ValueTypeName(accept.Definition().ParamTypes()[0]))
		}

		tm._accept = func(callerFd int32) (int32, error) {
			ret, err := accept.Call(coreCtx, api.EncodeI32(callerFd))
			if err != nil {
				return 0, fmt.Errorf("water: calling _water_accept function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _associate
	associate := tm.Core().ExportedFunction("_water_associate")
	if associate == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export _water_associate, water.Relay will not work.")
		tm._associate = func() (int32, error) {
			return 0, water.ErrUnimplementedRelay
		}
	} else {
		// check signature:
		//  _water_associate() -> (err i32)
		if len(associate.Definition().ParamTypes()) != 0 {
			return fmt.Errorf("water: _water_associate function expects 0 argument, got %d", len(associate.Definition().ParamTypes()))
		}

		if len(associate.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: _water_associate function expects 1 result, got %d", len(associate.Definition().ResultTypes()))
		} else if associate.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_associate function expects result type i32, got %s", api.ValueTypeName(associate.Definition().ResultTypes()[0]))
		}

		tm._associate = func() (int32, error) {
			ret, err := associate.Call(coreCtx)
			if err != nil {
				return 0, fmt.Errorf("water: calling _water_associate function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _cancel_with
	cancelWith := tm.Core().ExportedFunction("_water_cancel_with")
	if cancelWith == nil {
		return fmt.Errorf("water: WASM module does not export _water_cancel_with")
	} else {
		// check signature:
		//  _water_cancel_with(fd i32) -> (err i32)
		if len(cancelWith.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: _water_cancel_with function expects 1 argument, got %d", len(cancelWith.Definition().ParamTypes()))
		} else if cancelWith.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_cancel_with function expects argument type i32, got %s", api.ValueTypeName(cancelWith.Definition().ParamTypes()[0]))
		}

		if len(cancelWith.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: _water_cancel_with function expects 1 result, got %d", len(cancelWith.Definition().ResultTypes()))
		} else if cancelWith.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_cancel_with function expects result type i32, got %s", api.ValueTypeName(cancelWith.Definition().ResultTypes()[0]))
		}
	}

	// _worker
	worker := tm.Core().ExportedFunction("_water_worker")
	if worker == nil {
		return fmt.Errorf("water: WASM module does not export _water_worker")
	} else {
		// check signature:
		//  _water_worker() -> (err i32)
		if len(worker.Definition().ParamTypes()) != 0 {
			return fmt.Errorf("water: _water_worker function expects 0 argument, got %d", len(worker.Definition().ParamTypes()))
		}

		if len(worker.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: _water_worker function expects 1 result, got %d", len(worker.Definition().ResultTypes()))
		} else if worker.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: _water_worker function expects result type i32, got %s", api.ValueTypeName(worker.Definition().ResultTypes()[0]))
		}
	}

	// wrap _cancel_with and _worker
	tm.backgroundWorker = &struct {
		_cancel_with  func(int32) (int32, error)
		_worker       func() (int32, error)
		chanWorkerErr chan error
		cancelSocket  net.Conn
	}{
		_cancel_with: func(fd int32) (int32, error) {
			ret, err := cancelWith.Call(coreCtx, api.EncodeI32(fd))
			if err != nil {
				return 0, fmt.Errorf("water: calling _water_cancel_with function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		},
		_worker: func() (int32, error) {
			ret, err := worker.Call(coreCtx)
			if err != nil {
				return 0, fmt.Errorf("water: calling _water_worker function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		},
		chanWorkerErr: make(chan error, 4), // at max 1 error would occur, but we can buffer more copies
		// cancelSocket:  nil,
	}

	// call _init
	if errno, err := tm._init(); err != nil {
		return fmt.Errorf("water: calling _water_init function returned error: %w", err)
	} else {
		_, err := wasip1.DecodeWATERError(errno)
		return err
	}
}

// Worker spins up a worker thread for the WATM to run a blocking function, which is
// expected to be the mainloop.
//
// This function is non-blocking UNLESS the error occurred before entering the worker
// thread. In that case, the error will be returned immediately.
func (tm *TransportModule) Worker() error {
	// check if _worker is exported
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized properly for background worker")
	}

	// create cancel pipe
	cancelConnR, cancelConnW, err := socket.TCPConnPair()
	if err != nil {
		return fmt.Errorf("water: creating cancel pipe failed: %w", err)
	}
	tm.backgroundWorker.cancelSocket = cancelConnW // host will Write to this pipe to cancel the worker

	// push cancel pipe
	cancelPipeFd, err := tm.PushConn(cancelConnR)
	if err != nil {
		return fmt.Errorf("water: pushing cancel pipe failed: %w", err)
	}

	// pass the fd to the WASM module
	_, err = tm.backgroundWorker._cancel_with(cancelPipeFd)
	if err != nil {
		return fmt.Errorf("water: calling _water_cancel_with: %w", err)
	}

	log.LDebugf(tm.Core().Logger(), "water: starting worker thread")

	// in a goroutine, call _worker
	go func() {
		defer close(tm.backgroundWorker.chanWorkerErr)
		_, err := tm.backgroundWorker._worker()
		if err != nil && !errors.Is(err, syscall.ECANCELED) {
			// multiple copies in case of multiple receivers on the channel
			tm.backgroundWorker.chanWorkerErr <- err
			tm.backgroundWorker.chanWorkerErr <- err
			tm.backgroundWorker.chanWorkerErr <- err
			tm.backgroundWorker.chanWorkerErr <- err
			return
		} else {
			log.LDebugf(tm.Core().Logger(), "water: worker thread exited normally")
		}
	}()

	log.LDebugf(tm.Core().Logger(), "water: worker thread started")

	// last sanity check if the worker thread crashed immediately even before we return
	select {
	case err := <-tm.backgroundWorker.chanWorkerErr: // if already returned, basically it failed to start
		return fmt.Errorf("water: worker thread returned error: %w", err)
	default:
		log.LDebugf(tm.Core().Logger(), "water: Worker (func, not the worker thread) returning")
		return nil
	}
}

// WorkerErrored returns a channel that will be closed when the worker thread exits.
func (tm *TransportModule) WorkerErrored() <-chan error {
	if tm.backgroundWorker == nil {
		return nil
	}
	return tm.backgroundWorker.chanWorkerErr
}

func (tm *TransportModule) pushConfig() int32 {
	// get config file
	if tm.Core().Config().TransportModuleConfig == nil {
		log.LWarnf(tm.Core().Logger(), "water: WATM tried to pull config, but none is provided")
		return wasip1.EncodeWATERError(syscall.EACCES) // No config file provided
	}

	configFile, err := tm.Core().Config().TransportModuleConfig.AsFile()
	if err != nil {
		log.LErrorf(tm.Core().Logger(), "water: getting config file failed: %v", err)
		return wasip1.EncodeWATERError(syscall.EBADF) // Cannot read a provided config file
	}

	// push config file into WebAssembly instance
	configFd, err := tm.Core().InsertFile(configFile)
	if err != nil {
		log.LErrorf(tm.Core().Logger(), "water: pushing config file failed: %v", err)
		return wasip1.EncodeWATERError(syscall.EBADF) // Cannot push a config file
	}

	return int32(configFd)
}

// PushConn pushes a net.Conn into the Transport Module.
func (tm *TransportModule) PushConn(conn net.Conn) (fd int32, err error) {
	fd, err = tm.Core().InsertConn(conn)
	if err != nil {
		return wasip1.EncodeWATERError(syscall.EBADF), err // Cannot push a connection
	}

	tm.pushedConnMutex.Lock()
	tm.pushedConn[fd] = conn
	tm.pushedConnMutex.Unlock()

	return fd, nil
}

func (tm *TransportModule) GetPushedConn(fd int32) net.Conn {
	tm.pushedConnMutex.RLock()
	defer tm.pushedConnMutex.RUnlock()
	if tm.pushedConn == nil {
		return nil
	}
	if v, ok := tm.pushedConn[fd]; ok {
		return v
	}

	return nil
}

package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/refraction-networking/water"
	"github.com/refraction-networking/water/internal/log"
	"github.com/refraction-networking/water/internal/socket"
	"github.com/refraction-networking/water/internal/wasip1"
	"github.com/tetratelabs/wazero/api"
)

const (
	defaultCancelTimeout = 5 * time.Second
)

// TransportModule acts like a "managed core". It was build to provide WebAssembly
// Transport Module API-facing functions and utilities that are exclusive to
// version 0.
type TransportModule struct {
	core      water.Core // the underlying WASM runtime
	coreMutex sync.RWMutex

	_init func() (int32, error) // watm_init_v1() -> (err i32)

	// _dial_fixed
	_dial_fixed func(context.Context, int32) (int32, error) // watm_dial_fixed_v1(callerConnFd i32) -> (remoteConnFd i32)

	// _dial:
	//  - Calls to `env.host_dial() -> fd: i32` to dial a network connection and bind it to one
	//  of its file descriptors, record the fd for `remoteConnFd`. This will be the fd it used
	//  to read/write data from/to the remote destination.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `remoteConnFd` to the caller.
	_dial func(context.Context, int32) (int32, error) // watm_dial_v1(callerConnFd i32) -> (remoteConnFd i32)

	// _accept:
	//  - Calls to `env.host_accept() -> fd: i32` to accept a network connection and bind it
	//  to one of its file descriptors, record the fd for `sourceConnFd`. This will be the fd
	//  it used to read/write data from/to the source address.
	//  - Records the `callerConnFd`. This will be the fd it used to read/write data from/to
	//  the caller.
	//  - Returns `sourceConnFd` to the caller.
	_accept func(context.Context, int32) (int32, error) // watm_accept_v1(callerConnFd i32) -> (sourceConnFd i32)

	// _associate:
	//  - Calls to `env.host_accept() -> fd: i32` to accept a network connection and bind it
	//  to one of its file descriptors, record the fd for `sourceConnFd`. This will be the fd
	//  it used to read/write data from/to the source address.
	//  - Calls to `env.host_dial() -> fd: i32` to dial a network connection and bind it to one
	//  of its file descriptors, record the fd for `remoteConnFd`. This will be the fd it used
	//  to read/write data from/to the remote destination.
	//  - Returns 0 to the caller or an error code if any of the above steps failed.
	_associate func(context.Context) (int32, error) // watm_associate_v1() -> (err i32)

	// backgroundWorker is used to replace the deprecated read-write-close model.
	// We put it in a inlined struct for better code styling.
	backgroundWorker *struct {
		// _ctrlpipe:
		//  - Provides a socket to the WASM module for it to listen to cancellation events.
		//  - on Cancel() call, the pipe will be written to by the host (caller).
		//  - WebAssembly instance should select on the socket and handle cancellation ASAP.
		//
		// This is a workaround for not being able to call another WASM function until the
		// current one returns. And apparently this function needs to be called BEFORE the
		// blocking _start() function.
		_ctrlpipe func(context.Context, int32) (int32, error) // watm_ctrlpipe_v1(fd i32) -> (err i32)

		// _start provides a blocking function for the WASM module to run a worker thread.
		// In the worker thread, WASM module should select on all previously pushed sockets
		// (typically, two among callerConnFd, remoteConnFd, and sourceConnFd) and handle
		// data bi-directionally. The exact behavior is up to the WebAssembly module and
		// overall it drives data flow below based on the identity of the module:
		//  - Dialer: callerConn + remoteConn
		//  - Listener: callerConn + sourceConn
		//  - Relay: sourceConn + remoteConn
		//
		// The worker thread should exit and return when the cancellation pipe is available
		// for reading. For the current design, the content read from the pipe does not
		// include meaningful data.
		_start func() (int32, error) // watm_start_v1() (err int32)

		// When the worker thread exits, this channel will be closed after the error
		// is stored in exitedWith if any.
		//
		// Read-only to the caller. Write-only to the worker thread.
		exited     chan bool
		exitedWith atomic.Value // error

		// a socket used to cancel the worker thread. When the host calls Cancel(), it should
		// write to this socket.
		controlPipe *CtrlPipe
	}

	managedConns      map[int32]net.Conn // the conn we want to keep alive
	managedConnsMutex sync.RWMutex

	deferOnce     sync.Once
	deferredFuncs []func()

	closeOnce sync.Once
}

// UpgradeCore upgrades a water.Core to a v0 TransportModule.
func UpgradeCore(core water.Core) *TransportModule {
	watm := &TransportModule{
		core:          core,
		managedConns:  make(map[int32]net.Conn),
		deferredFuncs: make([]func(), 0),
	}

	err := core.WASIPreview1()
	if err != nil {
		log.LErrorf(core.Logger(), "water: unable to import WASI Preview 1: %v", err)
		return nil
	}

	// SetFinalizer, so Go GC automatically cleans up the WASM runtime
	// and all opened file descriptors (if any) associated with it
	// when the TransportModule is garbage collected.
	runtime.SetFinalizer(watm, func(tm *TransportModule) {
		tm.Close()
	})

	return watm
}

// AcceptFor is used to make the Transport Module act as a listener and
// accept a network connection.
func (tm *TransportModule) AcceptFor(ctx context.Context, reverseCallerConn net.Conn) (sourceConn net.Conn, err error) {
	// check if _accept is exported
	if tm._accept == nil {
		return nil, fmt.Errorf("water: WASM module does not export watm_accept_v1")
	}

	callerFd, err := tm.PushConn(reverseCallerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn failed: %w", err)
	}

	sourceFd, err := tm._accept(ctx, callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _accept: %w", err)
	} else {
		sourceConn := tm.GetManagedConns(sourceFd)
		if sourceConn == nil {
			return nil, fmt.Errorf("water: failed to look up network connection by fd")
		}
		return sourceConn, nil
	}
}

// Associate is used to make the Transport Module act as a relay and
// associate two network connections, where one is from a source via
// a listener, and the other is to a destination via a dialer.
func (tm *TransportModule) Associate(ctx context.Context) error {
	// check if _associate is exported
	if tm._associate == nil {
		return fmt.Errorf("water: WASM module does not export watm_associate_v1")
	}

	_, err := tm._associate(ctx)
	if err != nil {
		return fmt.Errorf("water: calling _associate function returned error: %w", err)
	}
	return nil
}

// Cancel cancels the worker thread if it is running and returns
// the error returned by the worker thread. This call is designed
// to block until the worker thread exits.
//
// If a timeout is set, this function will close the underlying
// runtime core to force the WebAssembly execution to terminate
// if the worker thread does not exit within the timeout. There's
// a default timeout of 5 seconds if no timeout is set.
func (tm *TransportModule) Cancel(timeout time.Duration) error {
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized")
	}

	if tm.backgroundWorker.controlPipe == nil {
		return fmt.Errorf("water: Transport Module is cancelled")
	}

	// Sanity check: if the worker thread has already exited, we don't need to cancel
	select {
	case <-tm.backgroundWorker.exited: // already exited
		if err := tm.ExitedWith(); err != nil {
			return fmt.Errorf("water: worker thread returned error: %w", err)
		}
		return nil
	default: // still running
		break
	}

	// write to the cancel pipe
	if err := tm.backgroundWorker.controlPipe.WriteExit(); err != nil {
		return fmt.Errorf("water: writing to cancel pipe failed: %w", err)
	}

	if timeout == 0 {
		timeout = defaultCancelTimeout
	}

	select {
	case <-time.After(timeout):
		log.LDebugf(tm.Core().Logger(), "water: force cancelling worker thread since it did not exit within %v", timeout)
		if err := tm.core.Close(); err != nil { // if not exited before timeout, close the runtime core to force the WebAssembly execution to terminate
			log.LDebugf(tm.Core().Logger(), "water: closing runtime core failed: %v", err)
		}
		// wait for the worker thread to exit, may take forever
		if err := tm.WaitWorker(); err != nil {
			return fmt.Errorf("water: worker thread returned error: %w", err)
		}
		return nil
	case <-tm.backgroundWorker.exited:
		log.LDebugf(tm.Core().Logger(), "water: worker thread exited within %v", timeout)
		if err := tm.ExitedWith(); err != nil {
			return fmt.Errorf("water: worker thread returned error: %w", err)
		}
		return nil
	}

	// We don't need to handle ControlPipe here. Since it is also pushed into the Transport Module
	// via PushConn, it will be closed when the Transport Module is cleaned up.
}

// Clean up the Transport Module by closing all connections pushed into the Transport Module.
func (tm *TransportModule) Cleanup() {
	// clean up pushed files
	var keyList []int32
	tm.managedConnsMutex.Lock()
	for k, v := range tm.managedConns {
		if v != nil {
			if err := v.Close(); err != nil {
				log.LErrorf(tm.Core().Logger(), "water: closing pushed connection failed: %v", err)
			}
		}
		keyList = append(keyList, k)
	}
	for _, k := range keyList {
		delete(tm.managedConns, k)
	}
	tm.managedConnsMutex.Unlock()

	// clean up deferred functions
	tm.deferredFuncs = nil

	// clean up all saved functions
	tm._init = nil
	tm._dial = nil
	tm._accept = nil
	tm._associate = nil
	tm.backgroundWorker._ctrlpipe = nil
	tm.backgroundWorker._start = nil
	tm.backgroundWorker.controlPipe = nil
}

func (tm *TransportModule) Close() error {
	var err error

	tm.closeOnce.Do(func() {
		err = tm.Cancel(defaultCancelTimeout)
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

func (tm *TransportModule) DialFixedFrom(ctx context.Context, reverseCallerConn net.Conn) (destConn net.Conn, err error) {
	// check if _connect is exported
	if tm._dial_fixed == nil {
		return nil, fmt.Errorf("water: WASM module does not export watm_dial_fixed_v1")
	}

	callerFd, err := tm.PushConn(reverseCallerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn failed: %w", err)
	}

	remoteFd, err := tm._dial_fixed(ctx, callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _dial_fixed: %w", err)
	} else {
		destConn := tm.GetManagedConns(remoteFd)
		if destConn == nil {
			return nil, fmt.Errorf("water: failed to look up network connection by fd")
		}
		return destConn, nil
	}
}

// DialFrom is used to make the Transport Module act as a dialer and
// dial a network connection.
//
// Takes the reverse caller connection as an argument, which is used
// to communicate with the caller.
func (tm *TransportModule) DialFrom(ctx context.Context, reverseCallerConn net.Conn) (destConn net.Conn, err error) {
	// check if _dial is exported
	if tm._dial == nil {
		return nil, fmt.Errorf("water: WASM module does not export watm_dial_v1")
	}

	callerFd, err := tm.PushConn(reverseCallerConn)
	if err != nil {
		return nil, fmt.Errorf("water: pushing caller conn failed: %w", err)
	}

	remoteFd, err := tm._dial(ctx, callerFd)
	if err != nil {
		return nil, fmt.Errorf("water: calling _dial: %w", err)
	} else {
		destConn := tm.GetManagedConns(remoteFd)
		if destConn == nil {
			return nil, fmt.Errorf("water: failed to look up network connection by fd")
		}
		return destConn, nil
	}
}

// ExitedWith returns the error that the worker thread exited with.
//
// It is recommended to use [TransportModule.WaitWorker] to wait for the worker
// thread to exit and get the error. This function does not check if the worker
// thread has exited yet and will return always return nil before the worker
// thread exits.
func (tm *TransportModule) ExitedWith() error {
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized")
	}

	maybeErr := tm.backgroundWorker.exitedWith.Load()

	if maybeErr == nil {
		return nil
	}

	return maybeErr.(error)
}

// GetManagedConns returns the net.Conn associated with the file descriptor.
func (tm *TransportModule) GetManagedConns(fd int32) net.Conn {
	tm.managedConnsMutex.RLock()
	defer tm.managedConnsMutex.RUnlock()
	if tm.managedConns == nil {
		return nil
	}
	if v, ok := tm.managedConns[fd]; ok {
		return v
	}

	return nil
}

// Initialize verifies the WebAssembly Transport Module loaded meets the
// WATM v1 specification and create bindings to all exported functions.
// It also calls the _init function to initialize the WATM.
//
// TODO: remove _init in WATM v1.
//
// All imports must be set before calling this function.
func (tm *TransportModule) Initialize() error {
	if tm.Core() == nil {
		return fmt.Errorf("water: core is not initialized")
	}

	var err error

	// v0 API removed:
	// - host_defer: deprecated in v0, removed in v1
	// - pull_config: WATM now probes /conf/watm.cfg for configuration

	// instantiate the WASM module
	if err = tm.Core().Instantiate(); err != nil {
		return err
	}

	// _init
	init := tm.Core().ExportedFunction("watm_init_v1")
	if init == nil {
		return fmt.Errorf("water: WASM module does not export watm_init_v1")
	} else {
		// check signature:
		//  watm_init_v1() -> (err i32)
		if len(init.Definition().ParamTypes()) != 0 {
			return fmt.Errorf("water: watm_init_v1 function expects 0 argument, got %d", len(init.Definition().ParamTypes()))
		}

		if len(init.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_init_v1 function expects 1 result, got %d", len(init.Definition().ResultTypes()))
		} else if init.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_init_v1 function expects result type i32, got %s", api.ValueTypeName(init.Definition().ResultTypes()[0]))
		}

		tm._init = func() (int32, error) {
			ret, err := init.Call(tm.Core().Context())
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_init_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// watm_ctrlpipe_v1: set up the control pipe
	ctrlPipe := tm.Core().ExportedFunction("watm_ctrlpipe_v1")
	if ctrlPipe == nil {
		return fmt.Errorf("water: WASM module does not export watm_ctrlpipe_v1")
	} else {
		// check signature:
		//  watm_ctrlpipe_v1(fd i32) -> (err i32)
		if len(ctrlPipe.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: watm_ctrlpipe_v1 function expects 1 argument, got %d", len(ctrlPipe.Definition().ParamTypes()))
		} else if ctrlPipe.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_ctrlpipe_v1 function expects argument type i32, got %s", api.ValueTypeName(ctrlPipe.Definition().ParamTypes()[0]))
		}

		if len(ctrlPipe.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_ctrlpipe_v1 function expects 1 result, got %d", len(ctrlPipe.Definition().ResultTypes()))
		} else if ctrlPipe.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_ctrlpipe_v1 function expects result type i32, got %s", api.ValueTypeName(ctrlPipe.Definition().ResultTypes()[0]))
		}
	}

	// watm_start_v1: the mainloop entry point
	start := tm.Core().ExportedFunction("watm_start_v1")
	if start == nil {
		return fmt.Errorf("water: WASM module does not export watm_start_v1")
	} else {
		// check signature:
		//  watm_start_v1() -> (err i32)
		if len(start.Definition().ParamTypes()) != 0 {
			return fmt.Errorf("water: watm_start_v1 function expects 0 argument, got %d", len(start.Definition().ParamTypes()))
		}

		if len(start.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_start_v1 function expects 1 result, got %d", len(start.Definition().ResultTypes()))
		} else if start.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_start_v1 function expects result type i32, got %s", api.ValueTypeName(start.Definition().ResultTypes()[0]))
		}
	}

	// set up the background worker
	tm.backgroundWorker = &struct {
		_ctrlpipe   func(context.Context, int32) (int32, error)
		_start      func() (int32, error)
		exited      chan bool
		exitedWith  atomic.Value
		controlPipe *CtrlPipe
	}{
		_ctrlpipe: func(ctx context.Context, fd int32) (int32, error) {
			ret, err := ctrlPipe.Call(ctx, api.EncodeI32(fd))
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_ctrlpipe_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		},
		_start: func() (int32, error) {
			go func() {
				<-tm.Core().Context().Done()
				tm.Close() // this is to unblock the potential syscall (poll_oneoff) which will not be interrupted by the context cancellation.
			}()

			ret, err := start.Call(tm.Core().Context())
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_start_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		},
		exited: make(chan bool),
		// exitedWith:  nil,
		// controlPipe: nil,
	}

	// _dial_fixed
	dial_fixed := tm.Core().ExportedFunction("watm_dial_fixed_v1")
	if dial_fixed == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export watm_dial_fixed_v1, water.Dialer will not work.")
		tm._dial_fixed = func(context.Context, int32) (int32, error) {
			return 0, water.ErrUnimplementedFixedDialer
		}
	} else {
		// check signature:
		//  watm_dial_fixed_v1(callerFd i32) -> (remoteFd i32)
		if len(dial_fixed.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: watm_dial_fixed_v1 function expects 1 argument, got %d", len(dial_fixed.Definition().ParamTypes()))
		} else if dial_fixed.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_dial_fixed_v1 function expects argument type i32, got %s", api.ValueTypeName(dial_fixed.Definition().ParamTypes()[0]))
		}

		if len(dial_fixed.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_dial_fixed_v1 function expects 1 result, got %d", len(dial_fixed.Definition().ResultTypes()))
		} else if dial_fixed.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_dial_fixed_v1 function expects result type i32, got %s", api.ValueTypeName(dial_fixed.Definition().ResultTypes()[0]))
		}

		tm._dial_fixed = func(ctx context.Context, callerFd int32) (int32, error) {
			if err := tm.ControlPipe(ctx); err != nil {
				return 0, fmt.Errorf("water: setting up control pipe failed: %w", err)
			}

			ret, err := dial_fixed.Call(ctx, api.EncodeI32(callerFd))
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_dial_fixed_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _dial
	dial := tm.Core().ExportedFunction("watm_dial_v1")
	if dial == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export watm_dial_v1, water.Dialer will not work.")
		tm._dial = func(context.Context, int32) (int32, error) {
			return 0, water.ErrUnimplementedDialer
		}
	} else {
		// check signature:
		//  watm_dial_v1(callerFd i32) -> (remoteFd i32)
		if len(dial.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: watm_dial_v1 function expects 1 argument, got %d", len(dial.Definition().ParamTypes()))
		} else if dial.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_dial_v1 function expects argument type i32, got %s", api.ValueTypeName(dial.Definition().ParamTypes()[0]))
		}

		if len(dial.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_dial_v1 function expects 1 result, got %d", len(dial.Definition().ResultTypes()))
		} else if dial.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_dial_v1 function expects result type i32, got %s", api.ValueTypeName(dial.Definition().ResultTypes()[0]))
		}

		tm._dial = func(ctx context.Context, callerFd int32) (int32, error) {
			if err := tm.ControlPipe(ctx); err != nil {
				return 0, fmt.Errorf("water: setting up control pipe failed: %w", err)
			}

			ret, err := dial.Call(ctx, api.EncodeI32(callerFd))
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_dial_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _accept
	accept := tm.Core().ExportedFunction("watm_accept_v1")
	if accept == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export watm_accept_v1, water.Listener will not work.")
		tm._accept = func(context.Context, int32) (int32, error) {
			return 0, water.ErrUnimplementedListener
		}
	} else {
		// check signature:
		//  watm_accept_v1(callerFd i32) -> (sourceFd i32)
		if len(accept.Definition().ParamTypes()) != 1 {
			return fmt.Errorf("water: watm_accept_v1 function expects 1 argument, got %d", len(accept.Definition().ParamTypes()))
		} else if accept.Definition().ParamTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_accept_v1 function expects argument type i32, got %s", api.ValueTypeName(accept.Definition().ParamTypes()[0]))
		}

		if len(accept.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_accept_v1 function expects 1 result, got %d", len(accept.Definition().ResultTypes()))
		} else if accept.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_accept_v1 function expects result type i32, got %s", api.ValueTypeName(accept.Definition().ParamTypes()[0]))
		}

		tm._accept = func(ctx context.Context, callerFd int32) (int32, error) {
			if err := tm.ControlPipe(ctx); err != nil {
				return 0, fmt.Errorf("water: setting up control pipe failed: %w", err)
			}

			ret, err := accept.Call(ctx, api.EncodeI32(callerFd))
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_accept_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// _associate
	associate := tm.Core().ExportedFunction("watm_associate_v1")
	if associate == nil {
		log.LWarnf(tm.Core().Logger(), "water: WASM module does not export watm_associate_v1, water.Relay will not work.")
		tm._associate = func(context.Context) (int32, error) {
			return 0, water.ErrUnimplementedRelay
		}
	} else {
		// check signature:
		//  watm_associate_v1() -> (err i32)
		if len(associate.Definition().ParamTypes()) != 0 {
			return fmt.Errorf("water: watm_associate_v1 function expects 0 argument, got %d", len(associate.Definition().ParamTypes()))
		}

		if len(associate.Definition().ResultTypes()) != 1 {
			return fmt.Errorf("water: watm_associate_v1 function expects 1 result, got %d", len(associate.Definition().ResultTypes()))
		} else if associate.Definition().ResultTypes()[0] != api.ValueTypeI32 {
			return fmt.Errorf("water: watm_associate_v1 function expects result type i32, got %s", api.ValueTypeName(associate.Definition().ResultTypes()[0]))
		}

		tm._associate = func(ctx context.Context) (int32, error) {
			if err := tm.ControlPipe(ctx); err != nil {
				return 0, fmt.Errorf("water: setting up control pipe failed: %w", err)
			}

			ret, err := associate.Call(ctx)
			if err != nil {
				return 0, fmt.Errorf("water: calling watm_associate_v1 function returned error: %w", err)
			}

			return wasip1.DecodeWATERError(api.DecodeI32(ret[0]))
		}
	}

	// call _init
	if errno, err := tm._init(); err != nil {
		return fmt.Errorf("water: calling watm_init_v1 function returned error: %w", err)
	} else {
		_, err := wasip1.DecodeWATERError(errno)
		return err
	}
}

func (tm *TransportModule) ControlPipe(ctx context.Context) error {
	// create control pipe connection pair
	ctrlConnR, ctrlConnW, err := socket.TCPConnPair()
	if err != nil {
		return fmt.Errorf("water: creating cancel pipe failed: %w", err)
	}

	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized")
	}

	tm.backgroundWorker.controlPipe = &CtrlPipe{
		Conn: ctrlConnW,
	} // host will Write to this pipe to cancel the worker

	// push cancel pipe
	ctrlPipeFd, err := tm.PushConn(ctrlConnR)
	if err != nil {
		return fmt.Errorf("water: pushing cancel pipe failed: %w", err)
	}

	// pass the fd to the WASM module
	_, err = tm.backgroundWorker._ctrlpipe(ctx, ctrlPipeFd)
	if err != nil {
		return fmt.Errorf("water: calling watm_ctrlpipe_v1: %w", err)
	}

	return nil
}

func (tm *TransportModule) LinkNetworkInterface(dialer *networkDialer, listener net.Listener) error {
	var waterDial func(
		networkIovs, networkIovsLen int32,
		addressIovs, addressIovsLen int32,
	) (fd int32)
	if dialer != nil {
		waterDial = func(
			networkIovs, networkIovsLen int32,
			addressIovs, addressIovsLen int32,
		) (fd int32) {
			var network, address string
			var n int
			var err error

			networkStrBuf := make([]byte, 256)
			n, err = tm.Core().ReadIovs(networkIovs, networkIovsLen, networkStrBuf)
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: ReadIovs: %v", err)
				return wasip1.EncodeWATERError(syscall.EINVAL) // invalid argument
			}
			network = string(networkStrBuf[:n])

			addressStrBuf := make([]byte, 256)
			n, err = tm.Core().ReadIovs(addressIovs, addressIovsLen, addressStrBuf)
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: ReadIovs: %v", err)
				return wasip1.EncodeWATERError(syscall.EINVAL) // invalid argument
			}
			address = string(addressStrBuf[:n])

			conn, err := dialer.Dial(network, address)
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
		waterDial = func(
			networkPtr, networkLen int32,
			addrPtr, addrLen int32,
		) (fd int32) {
			return wasip1.EncodeWATERError(syscall.ENODEV) // no such device
		}
	}

	if err := tm.Core().ImportFunction("env", "water_dial", waterDial); err != nil {
		if err != water.ErrFuncNotImported {
			return fmt.Errorf("water: linking dialer function, (*water.Core).ImportFunction: %w", err)
		} else {
			log.LWarnf(tm.Core().Logger(), "water: water_dial function not imported by WATM, water.FixedDialer will not work")
		}

	}

	var waterDialFixed func() (fd int32)
	if dialer != nil {
		waterDialFixed = func() (fd int32) {
			conn, err := dialer.DialFixed()
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: dialer.DialFixed: %v", err)
				return wasip1.EncodeWATERError(syscall.ENOTCONN) // not connected
			}
			fd, err = tm.PushConn(conn)
			if err != nil {
				log.LErrorf(tm.Core().Logger(), "water: PushConn: %v", err)
			}
			return fd
		}
	} else {
		waterDialFixed = func() (fd int32) {
			return wasip1.EncodeWATERError(syscall.ENODEV) // no such device
		}
	}

	if err := tm.Core().ImportFunction("env", "water_dial_fixed", waterDialFixed); err != nil {
		if err != water.ErrFuncNotImported {
			return fmt.Errorf("water: linking dialer function, (*water.Core).ImportFunction: %w", err)
		} else {
			log.LWarnf(tm.Core().Logger(), "water: water_dial_fixed function not imported by WATM, water.Dialer will not work")
		}
	}

	var waterAccept func() (fd int32)
	if listener != nil {
		waterAccept = func() (fd int32) {
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
		waterAccept = func() (fd int32) {
			return wasip1.EncodeWATERError(syscall.ENODEV) // no such device
		}
	}

	if err := tm.Core().ImportFunction("env", "water_accept", waterAccept); err != nil {
		if listener != nil || err != water.ErrFuncNotImported { // skip error if listener is nil and the error is ErrFuncNotImported, which indicates intentional skip
			return fmt.Errorf("water: linking listener function, (*water.Core).ImportFunction: %w", err)
		}
	}

	return nil
}

// PushConn pushes a net.Conn into the Transport Module.
func (tm *TransportModule) PushConn(conn net.Conn) (fd int32, err error) {
	fd, err = tm.Core().InsertConn(conn)
	if err != nil {
		return wasip1.EncodeWATERError(syscall.EBADF), err // Cannot push a connection
	}

	tm.managedConnsMutex.Lock()
	tm.managedConns[fd] = conn
	tm.managedConnsMutex.Unlock()

	return fd, nil
}

// Worker spins up a worker thread for the WATM to run a blocking function, which is
// expected to be the mainloop.
//
// Optionally you may pass in a list of io.Closer to be closed when the worker thread
// exits. This is useful for cleaning up resources that are not managed by the Transport
// Module.
//
// This function is non-blocking, so it will not return any error once the WebAssembly
// worker thread is started. To get the error, use [TransportModule.WaitWorker] or
// [TransportModule.ExitedWith].
func (tm *TransportModule) StartWorker(closers ...io.Closer) error {
	// check if _worker is exported
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized properly for background worker")
	}

	log.LDebugf(tm.Core().Logger(), "water: starting worker thread")

	// in a goroutine, call _worker
	go func() {
		defer close(tm.backgroundWorker.exited)
		defer func(toClose []io.Closer) {
			for _, closer := range toClose {
				if closer != nil {
					closer.Close()
				}
			}
		}(closers)

		_, err := tm.backgroundWorker._start()
		if err != nil && !errors.Is(err, syscall.ECANCELED) {
			log.LErrorf(tm.Core().Logger(), "water: WATM worker thread exited with error: %v", err)
			tm.backgroundWorker.exitedWith.Store(err)
		} else {
			// tm.backgroundWorker.exitedWith.Store(nil) // can't store nil value
			log.LDebugf(tm.Core().Logger(), "water: WATM worker thread exited without error")
		}

	}()

	log.LDebugf(tm.Core().Logger(), "water: worker thread started")

	// last sanity check if the worker thread crashed immediately even before we return
	if err := tm.ExitedWith(); err != nil {
		return fmt.Errorf("water: worker thread returned error: %w", err)
	}

	return nil
}

// WaitWorker waits for the worker thread to exit and returns the error
// if any.
func (tm *TransportModule) WaitWorker() error {
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not initialized")
	}

	if tm.backgroundWorker.exited == nil {
		return fmt.Errorf("water: worker thread is not running")
	}

	<-tm.backgroundWorker.exited

	maybeErr := tm.backgroundWorker.exitedWith.Load()

	if maybeErr == nil {
		return nil
	}

	return maybeErr.(error)
}

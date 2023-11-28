//go:build !unixconn

package v0

import (
	"fmt"
	"net"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/internal/log"
	"github.com/gaukas/water/internal/socket"
	"github.com/gaukas/water/internal/system"
	"github.com/gaukas/water/internal/wasm"
)

// Worker spins up a worker thread for the WASM module to run a blocking function.
//
// This function is non-blocking UNLESS the error occurred before entering the worker
// thread. In that case, the error will be returned immediately.
func (tm *TransportModule) Worker() error {
	// check if _worker is exported
	if tm.backgroundWorker == nil {
		return fmt.Errorf("water: Transport Module is not cancellable")
	}

	// create cancel pipe
	cancelConnR, cancelConnW, err := socket.TCPConnPair()
	if err != nil {
		return fmt.Errorf("water: creating cancel pipe failed: %w", err)
	}
	tm.backgroundWorker.cancelSocket = cancelConnW // host will Write to this pipe to cancel the worker

	// push cancel pipe to store
	cancelPipeFd, err := tm.PushConn(cancelConnR)
	if err != nil {
		return fmt.Errorf("water: pushing cancel pipe to store failed: %w", err)
	}

	// pass the fd to the WASM module
	ret, err := tm.backgroundWorker._cancel_with.Call(tm.core.Store(), cancelPipeFd)
	if err != nil {
		return fmt.Errorf("water: calling _water_cancel_with function returned error: %w", err)
	}
	if ret.(int32) != 0 {
		return fmt.Errorf("water: _water_cancel_with returned error: %w", wasm.WASMErr(ret.(int32)))
	}

	log.Debugf("water: starting worker thread")

	// in a goroutine, call _worker
	go func() {
		defer close(tm.backgroundWorker.chanWorkerErr)
		ret, err := tm.backgroundWorker._worker.Call(tm.core.Store())
		if err != nil {
			// multiple copies in case of multiple receivers on the channel
			tm.backgroundWorker.chanWorkerErr <- err
			tm.backgroundWorker.chanWorkerErr <- err
			tm.backgroundWorker.chanWorkerErr <- err
			tm.backgroundWorker.chanWorkerErr <- err
			return
		}
		if ret.(int32) != 0 {
			// multiple copies in case of multiple receivers on the channel
			tm.backgroundWorker.chanWorkerErr <- wasm.WASMErr(ret.(int32))
			tm.backgroundWorker.chanWorkerErr <- wasm.WASMErr(ret.(int32))
			tm.backgroundWorker.chanWorkerErr <- wasm.WASMErr(ret.(int32))
			tm.backgroundWorker.chanWorkerErr <- wasm.WASMErr(ret.(int32))
			log.Warnf("water: worker thread exited with code %d", ret.(int32))
		} else {
			log.Debugf("water: worker thread exited with code 0")
		}
	}()

	log.Debugf("water: worker thread started")

	// last sanity check if the worker thread crashed immediately even before we return
	select {
	case err := <-tm.backgroundWorker.chanWorkerErr: // if already returned, basically it failed to start
		return fmt.Errorf("water: worker thread returned error: %w", err)
	default:
		log.Debugf("water: Worker (func, not the worker thread) returning")
		return nil
	}
}

// PushConn pushes a net.Conn to the WASM store.
//
// system.GC_BUG: if this flag is set, for each transport module, before the first
// time we push a net.Conn to the WASM store, we will push a dummy file to the
// WASM store. This is to fix a bug in Wasmtime where the GC will close the
// first file pushed to the WASM store.
//
// system.CONN_HALT_BUG: if this flag is set, we will use a workaround for the
// halting bug in WASI where when a TCPConn is closed by the peer, the WASI
// blocks on the next read call indefinitely. This workaround is to wrap the
// TCPConn in a TCPConnFileWrap, which we will explicitly close when the
// other end closes the connection.
func (tm *TransportModule) PushConn(conn net.Conn, wasiCtxOverride ...wasiCtx) (fd int32, err error) {
	var wasiCtx wasiCtx = nil
	if len(wasiCtxOverride) > 0 {
		wasiCtx = wasiCtxOverride[0]
	} else {
		wasiCtx = tm.core.Store()
	}

	tm.gcfixOnce.Do(func() {
		if system.GC_BUG {
			// create temp file
			var f *os.File
			f, err = os.CreateTemp("", "water-gcfix")
			if err != nil {
				return
			}

			// push dummy file
			fd, err := wasiCtx.PushFile(f, wasmtime.READ_ONLY)
			if err != nil {
				return
			}

			// save dummy file to map
			tm.pushedConnMutex.Lock()
			tm.pushedConn[int32(fd)] = &struct {
				groundTruthConn net.Conn
				pushedFile      *os.File
			}{
				groundTruthConn: nil,
				pushedFile:      f,
			}
			tm.pushedConnMutex.Unlock()

			log.Debugf("water: GC fix: pushed dummy file to WASM store with fd %d", fd)
		}
	})

	if err != nil {
		return wasm.INVALID_FD, fmt.Errorf("water: creating temp file for GC fix: %w", err)
	}

	var connFile *os.File
	// generate file from conn according to its type
	switch conn := conn.(type) {
	case *net.TCPConn:
		// set TCP_NODELAY
		if err := conn.SetNoDelay(true); err != nil {
			return wasm.INVALID_FD, fmt.Errorf("water: setting TCP_NODELAY failed: %w", err)
		}
		// the halting bug might need a workaround here
		if system.CONN_HALT_BUG {
			connFile, err = socket.TCPConnFileWrap(conn)
		} else {
			connFile, err = socket.AsFile(conn)
		}
	default:
		connFile, err = socket.AsFile(conn)
	}
	if err != nil {
		return wasm.INVALID_FD, fmt.Errorf("water: converting conn to file failed: %w", err)
	}

	fdu32, err := wasiCtx.PushFile(connFile, wasmtime.READ_WRITE)
	if err != nil {
		return wasm.INVALID_FD, fmt.Errorf("water: pushing conn file to store failed: %w", err)
	}
	fd = int32(fdu32)

	tm.pushedConnMutex.Lock()
	tm.pushedConn[fd] = &struct {
		groundTruthConn net.Conn
		pushedFile      *os.File
	}{
		groundTruthConn: conn,
		pushedFile:      connFile,
	}
	tm.pushedConnMutex.Unlock()

	return fd, nil
}

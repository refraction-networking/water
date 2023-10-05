package v0

import (
	"fmt"
	"net"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/internal/socket"
	"github.com/gaukas/water/internal/wasm"
)

type WASIListener struct {
	listener        net.Listener
	mapFdConn       map[int32]net.Conn // saves all the connections accepted by this WASIListener by their file descriptors!
	mapFdClonedFile map[int32]*os.File // saves all files so GC won't close them
}

func MakeWASIListener(listener net.Listener) *WASIListener {
	if listener == nil {
		panic("water: NewWASIListener: listener is nil")
	}

	return &WASIListener{
		listener:        listener,
		mapFdConn:       make(map[int32]net.Conn),
		mapFdClonedFile: make(map[int32]*os.File),
	}
}

func (wl *WASIListener) WrappedAccept() wasm.WASMTIMEStoreIndependentFunction {
	return wrapConnectFunc(wl.accept)
}

func (wl *WASIListener) accept(caller *wasmtime.Caller) (fd int32, err error) {
	conn, err := wl.listener.Accept()
	if err != nil {
		return -1, fmt.Errorf("listener.Accept: %w", err)
	}

	connFile, err := socket.AsFile(conn)
	if err != nil {
		return -1, fmt.Errorf("socket.AsFile: %w", err)
	}

	uintfd, err := caller.PushFile(connFile, wasmtime.READ_WRITE)
	if err != nil {
		return -1, fmt.Errorf("(*wasmtime.Caller).PushFile: %w", err)
	}

	wl.mapFdConn[int32(uintfd)] = conn // save the connection by its file descriptor

	// fix: Go GC will close the file descriptor clone created by (*net.XxxConn).File()
	wl.mapFdClonedFile[int32(uintfd)] = connFile

	return int32(uintfd), nil
}

// Close should not be called if the embedded listener is shared across
// multiple WASM instances or WASIListeners.
func (wl *WASIListener) Close() error {
	return wl.listener.Close()
}

func (wl *WASIListener) GetConnByFd(fd int32) net.Conn {
	if wl.mapFdConn == nil {
		return nil
	}
	return wl.mapFdConn[fd]
}

func (wl *WASIListener) GetFileByFd(fd int32) *os.File {
	if wl.mapFdClonedFile == nil {
		return nil
	}
	return wl.mapFdClonedFile[fd]
}

func (wl *WASIListener) CloseAllConn() {
	if wl == nil {
		return
	}

	if wl.mapFdConn != nil {
		for _, conn := range wl.mapFdConn {
			conn.Close()
		}
	}

	if wl.mapFdClonedFile != nil {
		for _, file := range wl.mapFdClonedFile {
			file.Close()
		}
	}
}

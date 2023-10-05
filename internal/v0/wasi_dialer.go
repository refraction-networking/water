package v0

import (
	"fmt"
	"net"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/internal/socket"
	"github.com/gaukas/water/internal/wasm"
)

// WASIDialer is a convenient wrapper around net.Dialer which
// restricts the dialer to only dialing to a single address on
// a single network.
//
// WASM module will (through WASI) call to the dialer to dial
// for network connections.
type WASIDialer struct {
	network         string
	address         string
	dialerFunc      func(network, address string) (net.Conn, error)
	mapFdConn       map[int32]net.Conn // saves all the connections created by this WasiDialer by their file descriptors! (So we could close them when needed)
	mapFdClonedFile map[int32]*os.File // saves all files so GC won't close them
}

func MakeWASIDialer(
	network, address string,
	dialerFunc func(network, address string) (net.Conn, error),
) *WASIDialer {
	return &WASIDialer{
		network:         network,
		address:         address,
		dialerFunc:      dialerFunc,
		mapFdConn:       make(map[int32]net.Conn),
		mapFdClonedFile: make(map[int32]*os.File),
	}
}

func (wd *WASIDialer) WrappedDial() wasm.WASMTIMEStoreIndependentFunction {
	return WrapConnectFunc(wd.dial)
}

// dial(apw i32) -> fd i32
func (wd *WASIDialer) dial(caller *wasmtime.Caller) (fd int32, err error) {
	conn, err := wd.dialerFunc(wd.network, wd.address)
	if err != nil {
		return wasm.GENERAL_ERROR, fmt.Errorf("dialerFunc: %w", err)
	}

	connFile, err := socket.AsFile(conn)
	if err != nil {
		return wasm.GENERAL_ERROR, fmt.Errorf("socket.AsFile: %w", err)
	}

	uintfd, err := caller.PushFile(connFile, wasmtime.READ_WRITE)
	if err != nil {
		return wasm.WASICTX_ERR, fmt.Errorf("(*wasmtime.Caller).PushFile: %w", err)
	}

	wd.mapFdConn[int32(uintfd)] = conn // save the connection by its file descriptor

	// fix: Go GC will close the file descriptor (clone) created by (*net.XxxConn).File()
	wd.mapFdClonedFile[int32(uintfd)] = connFile

	return int32(uintfd), nil
}

func (wd *WASIDialer) GetConnByFd(fd int32) net.Conn {
	if wd.mapFdConn == nil {
		return nil
	}
	return wd.mapFdConn[fd]
}

func (wd *WASIDialer) GetFileByFd(fd int32) *os.File {
	if wd.mapFdClonedFile == nil {
		return nil
	}
	return wd.mapFdClonedFile[fd]
}

func (wd *WASIDialer) CloseAllConn() {
	if wd == nil {
		return
	}

	if wd.mapFdConn != nil {
		for k, conn := range wd.mapFdConn {
			conn.Close()
			delete(wd.mapFdConn, k)
		}
	}

	if wd.mapFdClonedFile != nil {
		for k, file := range wd.mapFdClonedFile {
			file.Close()
			delete(wd.mapFdClonedFile, k)
		}
	}
}

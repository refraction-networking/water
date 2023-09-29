// WebAssembly System Interface (WASI) related definitions and functions.

package water

import (
	"fmt"
	"net"

	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/socket"
)

type wasmtimeStoreIndependentFunction = func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap)

type WASIConnectFunc = func(caller *wasmtime.Caller, applicationProtocol int32) (fd int32, err error)

var WASIConnectFuncType *wasmtime.FuncType = wasmtime.NewFuncType(
	[]*wasmtime.ValType{
		wasmtime.NewValType(wasmtime.KindI32), // WASIApplicationProtocol
	},
	[]*wasmtime.ValType{
		wasmtime.NewValType(wasmtime.KindI32), // fd
	},
)

func WrapWASIConnectFunc(f WASIConnectFunc) wasmtimeStoreIndependentFunction {
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		if len(vals) != 1 {
			return []wasmtime.Val{wasmtime.ValI32(-1)}, wasmtime.NewTrap(fmt.Sprintf("(*WASIDialer).dial expects 1 argument, got %d", len(vals)))
		}

		// Application Protocol
		if vals[0].Kind() != wasmtime.KindI32 {
			return []wasmtime.Val{wasmtime.ValI32(-1)}, wasmtime.NewTrap(fmt.Sprintf("(*WASIDialer).dial expects the argument to be i32, got %s", vals[0].Kind()))
		}
		applicationProtocolSelection := vals[0].I32()

		fd, err := f(caller, applicationProtocolSelection)
		if err != nil {
			return []wasmtime.Val{wasmtime.ValI32(-1)}, wasmtime.NewTrap(fmt.Sprintf("(*WASIDialer).dial: %v", err))
		}

		return []wasmtime.Val{wasmtime.ValI32(fd)}, nil
	}
}

// nopWASIConnectFunc is a WASIConnectFunc that does nothing.
func nopWASIConnectFunc(caller *wasmtime.Caller, applicationProtocol int32) (fd int32, err error) {
	return -1, fmt.Errorf("NOP WASIConnectFunc is called")
}

type WASIListener struct {
	listener  net.Listener
	apw       WASIApplicationProtocolWrapper
	mapFdConn map[int32]net.Conn // saves all the connections accepted by this WASIListener by their file descriptors!
}

func MakeWASIListener(listener net.Listener, apw WASIApplicationProtocolWrapper) *WASIListener {
	if listener == nil {
		panic("water: NewWASIListener: listener is nil")
	}

	if apw == nil {
		apw = noWASIApplicationProtocolWrapper{}
	}

	return &WASIListener{
		listener:  listener,
		apw:       apw,
		mapFdConn: make(map[int32]net.Conn),
	}
}

func (wl *WASIListener) accept(caller *wasmtime.Caller, applicationProtocol int32) (fd int32, err error) {
	conn, err := wl.listener.Accept()
	if err != nil {
		return -1, fmt.Errorf("listener.Accept: %w", err)
	}

	conn, err = wl.apw.Wrap(applicationProtocol, conn)
	if err != nil {
		return -1, fmt.Errorf("apw.Wrap: %w", err)
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

func (wl *WASIListener) CloseAllConn() {
	if wl.mapFdConn != nil {
		for _, conn := range wl.mapFdConn {
			conn.Close()
		}
	}
}

// WASIDialer is a convenient wrapper around net.Dialer which
// restricts the dialer to only dialing to a single address on
// a single network.
//
// WASM module will (through WASI) call to the dialer to dial
type WASIDialer struct {
	network    string
	address    string
	dialerFunc func(network, address string) (net.Conn, error)
	apw        WASIApplicationProtocolWrapper
	mapFdConn  map[int32]net.Conn // saves all the connections created by this WasiDialer by their file descriptors!
}

func MakeWASIDialer(
	network, address string,
	dialerFunc func(network, address string) (net.Conn, error),
	apw WASIApplicationProtocolWrapper,
) *WASIDialer {
	if apw == nil {
		apw = noWASIApplicationProtocolWrapper{}
	}

	return &WASIDialer{
		network:    network,
		address:    address,
		dialerFunc: dialerFunc,
		apw:        apw,
		mapFdConn:  make(map[int32]net.Conn),
	}
}

// dial(apw i32) -> fd i32
func (wd *WASIDialer) dial(caller *wasmtime.Caller, applicationProtocol int32) (fd int32, err error) {
	conn, err := wd.dialerFunc(wd.network, wd.address)
	if err != nil {
		return -1, fmt.Errorf("dialerFunc: %w", err)
	}

	conn, err = wd.apw.Wrap(applicationProtocol, conn)
	if err != nil {
		return -1, fmt.Errorf("apw.Wrap: %w", err)
	}

	connFile, err := socket.AsFile(conn)
	if err != nil {
		return -1, fmt.Errorf("socket.AsFile: %w", err)
	}

	uintfd, err := caller.PushFile(connFile, wasmtime.READ_WRITE)
	if err != nil {
		return -1, fmt.Errorf("(*wasmtime.Caller).PushFile: %w", err)
	}

	wd.mapFdConn[int32(uintfd)] = conn // save the connection by its file descriptor

	return int32(uintfd), nil
}

func (wd *WASIDialer) GetConnByFd(fd int32) net.Conn {
	if wd.mapFdConn == nil {
		return nil
	}
	return wd.mapFdConn[fd]
}

func (wd *WASIDialer) CloseAllConn() {
	if wd.mapFdConn != nil {
		for _, conn := range wd.mapFdConn {
			conn.Close()
		}
	}
}

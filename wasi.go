// Some wasi stuff

package water

import (
	"fmt"
	"net"

	"github.com/bytecodealliance/wasmtime-go/v12"
	"github.com/gaukas/water/socket"
)

type WasiNetwork int32

// WasiNetwork specifies the network types requested by the WASM module.
// The WASM module can use this to specify a preferred network type.
const (
	WASI_NETWORK_TCP WasiNetwork = iota       // default/unspecified/0 -> TCP
	WASI_NETWORK_UDP                          // 1 -> UDP
	WASI_NETWORK_TLS WasiNetwork = 0x03010000 // Enumerated after TLS 1.0, in memory of the good old days
)

var mapWasiNetworkNames = map[WasiNetwork]string{
	WASI_NETWORK_TCP: "tcp",
	WASI_NETWORK_UDP: "udp",
	WASI_NETWORK_TLS: "tls",
}

func (n WasiNetwork) String() string {
	if name, ok := mapWasiNetworkNames[n]; ok {
		return name
	}
	panic(fmt.Sprintf("water: unknown WasiNetwork: %d", n))
}

// WasiDialerFunc is used by the WASM module to request host to dial a network.
// The host should return a file descriptor (int32) for where this connection
// can be found in the WASM's world.
type WasiDialerFunc func(network WasiNetwork) (int32, wasmtime.Val)

type WasiDialer struct {
	address string
	dialer  Dialer
	store   *wasmtime.Store

	mapFdConn map[int32]net.Conn // saves all the connections created by this WasiDialer by their file descriptors!
}

func NewWasiDialer(address string, dialer Dialer, store *wasmtime.Store) *WasiDialer {
	return &WasiDialer{
		address:   address,
		dialer:    dialer,
		store:     store,
		mapFdConn: make(map[int32]net.Conn),
	}
}

func (wd *WasiDialer) WasiDialerFunc(network int32) (netFd int32) {
	wasiNetwork := WasiNetwork(network)
	conn, err := wd.dialer.Dial(wasiNetwork.String(), wd.address)
	if err != nil {
		return -1
	}

	connFile, err := socket.AsFile(conn)
	if err != nil {
		return -1
	}

	uintfd, err := wd.store.PushFile(connFile, wasmtime.READ_WRITE)
	if err != nil {
		return -1
	}

	if wd.mapFdConn == nil {
		wd.mapFdConn = make(map[int32]net.Conn)
	}
	wd.mapFdConn[int32(uintfd)] = conn // save the connection by its file descriptor

	return int32(uintfd)
}

func (wd *WasiDialer) GetConnByFd(fd int32) net.Conn {
	if wd.mapFdConn == nil {
		return nil
	}
	return wd.mapFdConn[fd]
}

func (wd *WasiDialer) CloseAll() error {
	if wd.mapFdConn != nil {
		for _, conn := range wd.mapFdConn {
			conn.Close()
		}
	}
	return nil
}

// WasiConfigEngine creates wasmtime.WasiConfig.
// Since WasiConfig cannot be cloned, we will instead save
// all the repeated setup functions in a slice and call them
// on newly created wasmtime.WasiConfig when needed.
type WasiConfigEngine struct {
	setupFuncs []func(*wasmtime.WasiConfig) error // if any of these functions returns an error, the whole setup will fail.
}

func NewWasiConfigEngine() *WasiConfigEngine {
	return &WasiConfigEngine{
		setupFuncs: make([]func(*wasmtime.WasiConfig) error, 0),
	}
}

// GetConfig sets up and returns the finished wasmtime.WasiConfig.
//
// If the setup fails, it will return nil and an error.
func (wce *WasiConfigEngine) GetConfig() (*wasmtime.WasiConfig, error) {
	wasiConfig := wasmtime.NewWasiConfig()
	if wce != nil && wce.setupFuncs != nil {
		for _, f := range wce.setupFuncs {
			if err := f(wasiConfig); err != nil {
				return nil, err
			}
		}
	}
	return wasiConfig, nil
}

func (wce *WasiConfigEngine) SetArgv(argv []string) {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.SetArgv(argv)
		return nil
	})
}

func (wce *WasiConfigEngine) InheritArgv() {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritArgv()
		return nil
	})
}

func (wce *WasiConfigEngine) SetEnv(keys, values []string) {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.SetEnv(keys, values)
		return nil
	})
}

func (wce *WasiConfigEngine) InheritEnv() {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritEnv()
		return nil
	})
}

func (wce *WasiConfigEngine) SetStdinFile(path string) {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStdinFile(path)
	})
}

func (wce *WasiConfigEngine) InheritStdin() {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStdin()
		return nil
	})
}

func (wce *WasiConfigEngine) SetStdoutFile(path string) {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStdoutFile(path)
	})
}

func (wce *WasiConfigEngine) InheritStdout() {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStdout()
		return nil
	})
}

func (wce *WasiConfigEngine) SetStderrFile(path string) {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStderrFile(path)
	})
}

func (wce *WasiConfigEngine) InheritStderr() {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStderr()
		return nil
	})
}

func (wce *WasiConfigEngine) SetPreopenDir(path string, guestPath string) {
	wce.setupFuncs = append(wce.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.PreopenDir(path, guestPath)
		return nil
	})
}

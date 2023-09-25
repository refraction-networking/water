// WebAssembly System Interface (WASI) related definitions and functions.

package water

import (
	"fmt"
	"net"

	"github.com/bytecodealliance/wasmtime-go/v12"
	"github.com/gaukas/water/socket"
)

// WASINetwork specifies the network types requested by the WASM module.
// The WASM module can use this to specify a preferred network type.
type WASINetwork int32

const (
	WASI_NETWORK_TCP WASINetwork = iota       // default/unspecified/0 -> TCP
	WASI_NETWORK_UDP                          // 1 -> UDP
	WASI_NETWORK_TLS WASINetwork = 0x03010000 // Enumerated after TLS 1.0, in memory of the good old days
)

var mapWASINetworkNames = map[WASINetwork]string{
	WASI_NETWORK_TCP: "tcp",
	WASI_NETWORK_UDP: "udp",
	WASI_NETWORK_TLS: "tls",
}

func (n WASINetwork) String() string {
	if name, ok := mapWASINetworkNames[n]; ok {
		return name
	}
	panic(fmt.Sprintf("water: unknown WASINetwork: %d", n))
}

// WASIDialerFunc is used by the WASM module to request host to dial a network.
// The host should return a file descriptor (int32) for where this connection
// can be found in the WASM's world.
type WASIDialerFunc func( /*network WASINetwork, apw ApplicationProtocol*/ ) (int32, wasmtime.Val)

type WASIDialer struct {
	network string
	address string
	dialer  NetworkDialer
	// apw    ApplicationProtocolWrapper
	store *wasmtime.Store

	mapFdConn map[int32]net.Conn // saves all the connections created by this WasiDialer by their file descriptors!
}

func NewWASIDialer(network, address string, dialer NetworkDialer /*, apw ApplicationProtocolWrapper */, store *wasmtime.Store) *WASIDialer {
	return &WASIDialer{
		network: network,
		address: address,
		dialer:  dialer,
		// apw:       apw,
		store:     store,
		mapFdConn: make(map[int32]net.Conn),
	}
}

func (wd *WASIDialer) WASIDialerFunc( /* network int32, apw int32*/ ) (netFd int32) {
	// wasiNetwork := WASINetwork(network)
	conn, err := wd.dialer.Dial(wd.network, wd.address)
	if err != nil {
		return -1
	}

	// TODO: implement apw
	// if apw != 0 && wd.apw != nil... // wrap application protocol around the connection

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

func (wd *WASIDialer) GetConnByFd(fd int32) net.Conn {
	if wd.mapFdConn == nil {
		return nil
	}
	return wd.mapFdConn[fd]
}

func (wd *WASIDialer) CloseAll() error {
	if wd.mapFdConn != nil {
		for _, conn := range wd.mapFdConn {
			conn.Close()
		}
	}
	return nil
}

// WASIConfigFactory creates wasmtime.WasiConfig.
// Since WasiConfig cannot be cloned, we will instead save
// all the repeated setup functions in a slice and call them
// on newly created wasmtime.WasiConfig when needed.
type WASIConfigFactory struct {
	setupFuncs []func(*wasmtime.WasiConfig) error // if any of these functions returns an error, the whole setup will fail.
}

func NewWasiConfigEngine() *WASIConfigFactory {
	return &WASIConfigFactory{
		setupFuncs: make([]func(*wasmtime.WasiConfig) error, 0),
	}
}

// GetConfig sets up and returns the finished wasmtime.WasiConfig.
//
// If the setup fails, it will return nil and an error.
func (wcf *WASIConfigFactory) GetConfig() (*wasmtime.WasiConfig, error) {
	wasiConfig := wasmtime.NewWasiConfig()
	if wcf != nil && wcf.setupFuncs != nil {
		for _, f := range wcf.setupFuncs {
			if err := f(wasiConfig); err != nil {
				return nil, err
			}
		}
	}
	return wasiConfig, nil
}

func (wcf *WASIConfigFactory) SetArgv(argv []string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.SetArgv(argv)
		return nil
	})
}

func (wcf *WASIConfigFactory) InheritArgv() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritArgv()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetEnv(keys, values []string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.SetEnv(keys, values)
		return nil
	})
}

func (wcf *WASIConfigFactory) InheritEnv() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritEnv()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetStdinFile(path string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStdinFile(path)
	})
}

func (wcf *WASIConfigFactory) InheritStdin() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStdin()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetStdoutFile(path string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStdoutFile(path)
	})
}

func (wcf *WASIConfigFactory) InheritStdout() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStdout()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetStderrFile(path string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		return wasiConfig.SetStderrFile(path)
	})
}

func (wcf *WASIConfigFactory) InheritStderr() {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.InheritStderr()
		return nil
	})
}

func (wcf *WASIConfigFactory) SetPreopenDir(path string, guestPath string) {
	wcf.setupFuncs = append(wcf.setupFuncs, func(wasiConfig *wasmtime.WasiConfig) error {
		wasiConfig.PreopenDir(path, guestPath)
		return nil
	})
}

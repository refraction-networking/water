package water

import (
	"net"
	"os"
)

type Config struct {
	// NetworkDialer is a dialer that dials a network address.
	// It will be used to provide network access to the WASM
	// instance.
	// If not specified, a default dialer will be used.
	//
	// Used by Dialer when calling DialConfig() or (*Dialer).Dial().
	// Used by Server when calling ServeConfig() or (*Server).Serve().
	NetworkDialer NetworkDialer

	// ApplicationProtocolWrapper is a wrapper function that wraps around
	// a given net.Conn to provide additional Application protocol
	// support, such as TLS.
	//
	// TODO: implement this feature
	// ApplicationProtocolWrapper ApplicationProtocolWrapper

	// NetworkListener points to a listener that listens on a
	// network address. It will be used to provide incoming
	// network connections to the WASM instance. Required by
	// ListenConfig().
	NetworkListener net.Listener

	// Feature specifies a series of experimental features to enable
	// for the WASM runtime.
	//
	// Feature flag is bit-masked and version-dependent. That is, a
	// feature flag may be supported in one version of the runtime but
	// not in another. If a feature flag is not supported in the runtime,
	// it will be ignored.
	Feature Feature

	// WABin is the WebAssembly module binary. In a typical use case, this
	// field is populated by reading in a .wasm file.
	//
	// This field is required.
	WABin []byte

	// WAConfig defines the configuration file used by the WASM module.
	WAConfig WAConfig

	// WasiConfigFactory is used to replicate the WASI config
	// for each WASM instance created.
	WASIConfigFactory *WASIConfigFactory
}

func (c *Config) defaultNetworkDialerIfNotSet() {
	if c.NetworkDialer == nil {
		c.NetworkDialer = DefaultNetworkDialer()
	}
}

func (c *Config) requireNetworkListener() {
	if c.NetworkListener == nil {
		panic("water: NetworkListener is not provided")
	}
}

func (c *Config) requireWABin() {
	if len(c.WABin) == 0 {
		panic("water: WASI binary is not provided")
	}
}

// WAConfig defines the configuration file used by the WASM module.
type WAConfig struct {
	FilePath string // Path to the config file.
}

func (c *WAConfig) File() *os.File {
	if c.FilePath == "" {
		return nil
	}

	f, err := os.Open(c.FilePath)
	if err != nil {
		panic("water: failed to open WASM config file: " + err.Error())
	}

	return f
}

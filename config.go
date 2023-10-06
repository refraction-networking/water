package water

import (
	"net"
	"os"

	"github.com/gaukas/water/internal/log"
	"github.com/gaukas/water/internal/wasm"
)

type Config struct {
	// WATMBin contains the binary format of the WebAssembly Transport Module.
	// In a typical use case, this mandatory field is populated by loading
	// from a .wasm file, downloaded from a remote target, or generated from
	// a .wat (WebAssembly Text Format) file.
	WATMBin []byte

	// DialerFunc specifies a func that dials the specified address on the
	// named network. This optional field can be set to override the Go
	// default dialer func:
	// 	net.Dial(network, address)
	DialerFunc func(network, address string) (net.Conn, error)

	// NetworkListener specifies a net.listener implementation that listens
	// on the specified address on the named network. This optional field
	// will be used to provide (incoming) network connections from a
	// presumably remote source to the WASM instance. Required by
	// ListenConfig().
	NetworkListener net.Listener

	// Feature specifies a series of experimental features for the WASM
	// runtime.
	//
	// Each feature flag is bit-masked and version-dependent, and flags
	// are independent of each other. This means that a particular
	// feature flag may be supported in one version of the runtime but
	// not in another. If a feature flag is not supported or not recognized
	// by the runtime, it will be silently ignored.
	Feature Feature

	// WATMConfig optionally provides a configuration file to be pushed into
	// the WASM Transport Module.
	WATMConfig WATMConfig

	// WasiConfigFactory is used to replicate the WASI config for each WASM
	// instance created. This field is for advanced use cases and/or debugging
	// purposes only.
	WASIConfigFactory *wasm.WASIConfigFactory
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	wasmClone := make([]byte, len(c.WATMBin))
	copy(wasmClone, c.WATMBin)

	return &Config{
		WATMBin:           c.WATMBin,
		DialerFunc:        c.DialerFunc,
		NetworkListener:   c.NetworkListener,
		Feature:           c.Feature,
		WATMConfig:        c.WATMConfig,
		WASIConfigFactory: c.WASIConfigFactory.Clone(),
	}
}

func (c *Config) DialerFuncOrDefault() func(network, address string) (net.Conn, error) {
	if c.DialerFunc == nil {
		return net.Dial
	}

	return c.DialerFunc
}

func (c *Config) NetworkListenerOrPanic() net.Listener {
	if c.NetworkListener == nil {
		panic("water: network listener is not provided in config")
	}

	return c.NetworkListener
}

func (c *Config) WATMBinOrPanic() []byte {
	if len(c.WATMBin) == 0 {
		panic("water: WebAssembly Transport Module binary is not provided in config")
	}

	return c.WATMBin
}

// WATMConfig defines the configuration file used by the WebAssembly Transport Module.
type WATMConfig struct {
	FilePath string // Path to the config file.
}

// File opens the config file and returns the file descriptor.
func (c *WATMConfig) File() *os.File {
	if c.FilePath == "" {
		log.Errorf("water: WASM config file path is not provided in config")
		return nil
	}

	f, err := os.Open(c.FilePath)
	if err != nil {
		log.Errorf("water: failed to open WATM config file: %v", err)
		return nil
	}

	return f
}

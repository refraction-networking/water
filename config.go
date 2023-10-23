package water

import (
	"net"

	"github.com/gaukas/water/internal/wasm"
)

// Config defines the configuration for the WATER Dialer/Config interface.
type Config struct {
	// TMBin contains the binary format of the WebAssembly Transport Module.
	// In a typical use case, this mandatory field is populated by loading
	// from a .wasm file, downloaded from a remote target, or generated from
	// a .wat (WebAssembly Text Format) file.
	TMBin []byte

	// NetworkDialerFunc specifies a func that dials the specified address on the
	// named network. This optional field can be set to override the Go
	// default dialer func:
	// 	net.Dial(network, address)
	NetworkDialerFunc func(network, address string) (net.Conn, error)

	// NetworkListener specifies a net.listener implementation that listens
	// on the specified address on the named network. This optional field
	// will be used to provide (incoming) network connections from a
	// presumably remote source to the WASM instance. Required by
	// water.WrapListener().
	NetworkListener net.Listener

	// TMConfig optionally provides a configuration file to be pushed into
	// the WASM Transport Module.
	TMConfig TMConfig

	// wasiConfigFactory is used to replicate the WASI config for each WASM
	// instance created. This field is for advanced use cases and/or debugging
	// purposes only.
	//
	// Caller is supposed to call c.WASIConfig() to get the pointer to the
	// WASIConfigFactory. If the pointer is nil, a new WASIConfigFactory will
	// be created and returned.
	wasiConfigFactory *wasm.WASIConfigFactory
}

// Clone creates a deep copy of the Config.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	wasmClone := make([]byte, len(c.TMBin))
	copy(wasmClone, c.TMBin)

	return &Config{
		TMBin:             c.TMBin,
		NetworkDialerFunc: c.NetworkDialerFunc,
		NetworkListener:   c.NetworkListener,
		TMConfig:          c.TMConfig,
		wasiConfigFactory: c.wasiConfigFactory.Clone(),
	}
}

// NetworkDialerFuncOrDefault returns the DialerFunc if it is not nil, otherwise
// returns the default net.Dial function.
func (c *Config) NetworkDialerFuncOrDefault() func(network, address string) (net.Conn, error) {
	if c.NetworkDialerFunc == nil {
		return net.Dial
	}

	return c.NetworkDialerFunc
}

// NetworkListenerOrDefault returns the NetworkListener if it is not nil,
// otherwise it panics.
func (c *Config) NetworkListenerOrPanic() net.Listener {
	if c.NetworkListener == nil {
		panic("water: network listener is not provided in config")
	}

	return c.NetworkListener
}

// WATMBinOrDefault returns the WATMBin if it is not nil, otherwise it panics.
func (c *Config) WATMBinOrPanic() []byte {
	if len(c.TMBin) == 0 {
		panic("water: WebAssembly Transport Module binary is not provided in config")
	}

	return c.TMBin
}

// WASIConfig returns the WASIConfigFactory. If the pointer is
// nil, a new WASIConfigFactory will be created and returned.
func (c *Config) WASIConfig() *wasm.WASIConfigFactory {
	if c.wasiConfigFactory == nil {
		c.wasiConfigFactory = wasm.NewWasiConfigFactory()
	}

	return c.wasiConfigFactory
}

func (c *Config) Listen(network, address string) (net.Listener, error) {
	lis, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	config := c.Clone()
	config.NetworkListener = lis

	return NewListener(config)
}

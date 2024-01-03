package water

import (
	"net"

	"github.com/gaukas/water/internal/log"
	"github.com/gaukas/water/internal/wasm"
)

// Config defines the configuration for the WATER Dialer/Config interface.
type Config struct {
	// TransportModuleBin contains the binary format of the WebAssembly
	// Transport Module.
	// In practice, this mandatory field could be populated by loading
	// a .wasm file, downloading from a remote host, or generating from
	// a .wat (WebAssembly Text Format) file.
	TransportModuleBin []byte

	// TransportModuleConfig optionally provides a configuration file to be pushed into
	// the WASM Transport Module.
	TransportModuleConfig TransportModuleConfig

	// NetworkDialerFunc specifies a func that dials the specified address on the
	// named network. This optional field can be set to override the Go
	// default dialer func:
	// 	net.Dial(network, address)
	NetworkDialerFunc func(network, address string) (net.Conn, error)

	// NetworkListener specifies a net.listener implementation that listens
	// on the specified address on the named network. This optional field
	// will be used to provide (incoming) network connections from a
	// presumably remote source to the WASM instance.
	//
	// Calling (*Config).Listen will override this field.
	NetworkListener net.Listener

	// ModuleConfigFactory is used to configure the system resource of
	// each WASM instance created. This field is for advanced use cases
	// and/or debugging purposes only.
	//
	// Caller is supposed to call c.ModuleConfig() to get the pointer to the
	// ModuleConfigFactory. If the pointer is nil, a new ModuleConfigFactory will
	// be created and returned.
	ModuleConfigFactory *wasm.ModuleConfigFactory

	OverrideLogger *log.Logger // essentially a *slog.Logger, currently using an alias to flatten the version discrepancy
}

// Clone creates a deep copy of the Config.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	wasmClone := make([]byte, len(c.TransportModuleBin))
	copy(wasmClone, c.TransportModuleBin)

	return &Config{
		TransportModuleBin:    c.TransportModuleBin,
		NetworkDialerFunc:     c.NetworkDialerFunc,
		NetworkListener:       c.NetworkListener,
		TransportModuleConfig: c.TransportModuleConfig,
		ModuleConfigFactory:   c.ModuleConfigFactory.Clone(),
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
	if len(c.TransportModuleBin) == 0 {
		panic("water: WebAssembly Transport Module binary is not provided in config")
	}

	return c.TransportModuleBin
}

// ModuleConfig returns the ModuleConfigFactory. If the pointer is
// nil, a new ModuleConfigFactory will be created and returned.
func (c *Config) ModuleConfig() *wasm.ModuleConfigFactory {
	if c.ModuleConfigFactory == nil {
		c.ModuleConfigFactory = wasm.NewModuleConfigFactory()

		// by default, stdout and stderr are inherited
		c.ModuleConfigFactory.InheritStdout()
		c.ModuleConfigFactory.InheritStderr()
	}

	return c.ModuleConfigFactory
}

func (c *Config) Listen(network, address string) (Listener, error) {
	lis, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	config := c.Clone()
	config.NetworkListener = lis

	return NewListener(config)
}

func (c *Config) Logger() *log.Logger {
	if c.OverrideLogger != nil {
		return c.OverrideLogger
	}

	return log.GetDefaultLogger()
}

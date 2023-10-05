package water

import (
	"net"
	"os"

	"github.com/gaukas/water/internal/wasm"
)

type Config struct {
	// EmbedDialer provides a dialer func that dials a remote
	// network address. It enables the configured Dialer/Relay
	// to dial a network address for the WASM module.
	//
	// If not specified, a default dialer func will be used.
	EmbedDialer func(network, address string) (net.Conn, error)

	// NetworkListener points to a listener that listens on a
	// network address. It will be used to provide incoming
	// network connections to the WASM instance. Required by
	// ListenConfig().
	EmbedListener net.Listener

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

	// WAConfig defines the configuration file to be pushed into the WASM module.
	WAConfig WAConfig

	// WasiConfigFactory is used to replicate the WASI config
	// for each WASM instance created.
	WASIConfigFactory *wasm.WASIConfigFactory
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	WABinClone := make([]byte, len(c.WABin))
	copy(WABinClone, c.WABin)

	return &Config{
		EmbedDialer:       c.EmbedDialer,
		EmbedListener:     c.EmbedListener,
		Feature:           c.Feature,
		WABin:             WABinClone,
		WAConfig:          c.WAConfig,
		WASIConfigFactory: c.WASIConfigFactory.Clone(),
	}
}

func (c *Config) embedDialerOrDefault() {
	if c.EmbedDialer == nil {
		c.EmbedDialer = net.Dial
	}
}

func (c *Config) mustEmbedListener() {
	if c.EmbedListener == nil {
		panic("water: no listener is provided")
	}
}

func (c *Config) mustSetWABin() {
	if len(c.WABin) == 0 {
		panic("water: WASI binary is not provided")
	}
}

// WAConfig defines the configuration file used by the WASM module.
type WAConfig struct {
	FilePath string // Path to the config file.
}

// File opens the config file and returns the file descriptor.
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

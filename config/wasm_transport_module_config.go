package config

import (
	"os"

	"github.com/gaukas/water/internal/log"
)

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

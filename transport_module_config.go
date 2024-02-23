package water

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

// TransportModuleConfig defines the configuration file used by
// the WebAssembly Transport Module.
//
// It is optional to WATER, but may be mandatory according to the
// WebAssembly Transport Module implementation.
type TransportModuleConfig interface {
	// AsFile returns the TransportModuleConfig as a file, which
	// then can be loaded into the WebAssembly Transport Module.
	//
	// If the returned error is nil, the *os.File MUST be valid
	// and in a readable state.
	AsFile() (*os.File, error)
}

// transportModuleConfigFile could be used to provide a config file
// on the local file system to the WebAssembly Transport Module by
// specifying the path to the config file.
//
// Implements TransportModuleConfig.
type transportModuleConfigFile string

// TransportModuleConfigFromFile creates a TransportModuleConfig from
// the given file path.
func TransportModuleConfigFromFile(filePath string) (TransportModuleConfig, error) {
	// Try opening the file to ensure it exists and is readable.
	_, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("water: failed to open WATM config file: %w", err)
	}

	return transportModuleConfigFile(filePath), nil
}

// AsFile implements TransportModuleConfig.
func (c transportModuleConfigFile) AsFile() (*os.File, error) {
	if string(c) == "" {
		return nil, errors.New("transport module config file path is empty")
	}

	f, err := os.Open(string(c))
	if err != nil {
		return nil, fmt.Errorf("failed to open transport module config file: %w", err)
	}

	return f, nil
}

type transportModuleConfigBytes []byte

// TransportModuleConfigFromBytes creates a TransportModuleConfig from
// the given byte slice.
func TransportModuleConfigFromBytes(configBytes []byte) TransportModuleConfig {
	return transportModuleConfigBytes(configBytes)
}

// AsFile implements TransportModuleConfig.
func (c transportModuleConfigBytes) AsFile() (*os.File, error) {
	if len(c) == 0 {
		return nil, errors.New("transport module config bytes is empty")
	}

	f, err := os.CreateTemp("", "water-wasm-config-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for transport module config: %w", err)
	}

	if _, err := f.Write(c); err != nil {
		return nil, fmt.Errorf("failed to write transport module config to temp file: %w", err)
	}
	// reset the file pointer to the beginning of the file
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to the beginning of the temp file: %w", err)
	}

	runtime.SetFinalizer(f, func(tmpFile *os.File) {
		tmpFile.Close()
		// Remove the temp file from local file system when collected.
		//
		// This does NOT guarantee the temp file will always be removed
		// from the local file system before the program exits. However,
		// it is still better than nothing when concurrency is high.
		os.Remove(tmpFile.Name())
	})

	return f, nil
}

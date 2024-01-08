package wasm

import (
	"io"
	"os"

	"github.com/tetratelabs/wazero"
)

// ModuleConfigFactory is used to spawn wazero.ModuleConfig.
type ModuleConfigFactory struct {
	moduleConfig wazero.ModuleConfig
	fsconfig     wazero.FSConfig
}

// NewModuleConfigFactory creates a new ModuleConfigFactory.
func NewModuleConfigFactory() *ModuleConfigFactory {
	return &ModuleConfigFactory{
		moduleConfig: wazero.NewModuleConfig(),
		fsconfig:     wazero.NewFSConfig(),
	}
}

func (mcf *ModuleConfigFactory) Clone() *ModuleConfigFactory {
	if mcf == nil {
		return nil
	}

	return &ModuleConfigFactory{
		moduleConfig: mcf.moduleConfig,
		fsconfig:     mcf.fsconfig,
	}
}

// GetConfig returns the latest wazero.ModuleConfig.
func (mcf *ModuleConfigFactory) GetConfig() (wazero.ModuleConfig, error) {
	if mcf == nil {
		return wazero.NewModuleConfig(), nil
	}

	return mcf.moduleConfig.WithFSConfig(mcf.fsconfig), nil
}

// SetArgv sets the arguments for the WebAssembly module.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file.
func (mcf *ModuleConfigFactory) SetArgv(argv []string) {
	mcf.moduleConfig = mcf.moduleConfig.WithArgs(argv...)
}

// InheritArgv sets the arguments for the WebAssembly module to os.Args.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file.
func (mcf *ModuleConfigFactory) InheritArgv() {
	// TODO: enumerate os.Args or deprecate this
}

// SetEnv sets the environment variables for the WebAssembly module.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file.
func (mcf *ModuleConfigFactory) SetEnv(keys, values []string) {
	if len(keys) != len(values) {
		panic("water: SetEnv: keys and values must have the same length")
	}

	for i := range keys {
		mcf.moduleConfig = mcf.moduleConfig.WithEnv(keys[i], values[i])
	}
}

// InheritEnv sets the environment variables for the WebAssembly module to
// os.Environ.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file.
func (wcf *ModuleConfigFactory) InheritEnv() {
	// TODO: enumerate os.Environ or deprecate this
}

func (wcf *ModuleConfigFactory) SetStdin(r io.Reader) {
	wcf.moduleConfig = wcf.moduleConfig.WithStdin(r)
}

func (wcf *ModuleConfigFactory) InheritStdin() {
	wcf.moduleConfig = wcf.moduleConfig.WithStdin(os.Stdin)
}

func (wcf *ModuleConfigFactory) SetStdout(w io.Writer) {
	wcf.moduleConfig = wcf.moduleConfig.WithStdout(w)
}

func (wcf *ModuleConfigFactory) InheritStdout() {
	wcf.moduleConfig = wcf.moduleConfig.WithStdout(os.Stdout)
}

func (wcf *ModuleConfigFactory) SetStderr(w io.Writer) {
	wcf.moduleConfig = wcf.moduleConfig.WithStderr(w)
}

func (wcf *ModuleConfigFactory) InheritStderr() {
	wcf.moduleConfig = wcf.moduleConfig.WithStderr(os.Stderr)
}

func (wcf *ModuleConfigFactory) SetPreopenDir(path string, guestPath string) {
	wcf.fsconfig = wcf.fsconfig.WithDirMount(path, guestPath)
}

// TODO: consider adding SetPreopenReadonlyDir
// TODO: consider adding SetPreopenFS

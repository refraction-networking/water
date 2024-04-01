package water

import (
	"fmt"
	"io"
	"os"
	"sync"

	rand "crypto/rand"

	"github.com/tetratelabs/wazero"
)

// WazeroModuleConfigFactory is used to spawn wazero.ModuleConfig.
type WazeroModuleConfigFactory struct {
	moduleConfig wazero.ModuleConfig
	fsconfig     wazero.FSConfig
}

// NewWazeroModuleConfigFactory creates a new WazeroModuleConfigFactory.
func NewWazeroModuleConfigFactory() *WazeroModuleConfigFactory {
	return &WazeroModuleConfigFactory{
		moduleConfig: wazero.NewModuleConfig().WithSysWalltime().WithSysNanotime().WithSysNanosleep().WithRandSource(rand.Reader),
		fsconfig:     wazero.NewFSConfig(),
	}
}

func (wmcf *WazeroModuleConfigFactory) Clone() *WazeroModuleConfigFactory {
	if wmcf == nil {
		return nil
	}

	return &WazeroModuleConfigFactory{
		moduleConfig: wmcf.moduleConfig,
		fsconfig:     wmcf.fsconfig,
	}
}

// GetConfig returns the latest wazero.ModuleConfig.
func (wmcf *WazeroModuleConfigFactory) GetConfig() wazero.ModuleConfig {
	if wmcf == nil {
		panic("water: GetConfig: wmcf is nil")
	}

	return wmcf.moduleConfig.WithFSConfig(wmcf.fsconfig)
}

// GetFSConfig returns the latest wazero.FSConfig.
func (wmcf *WazeroModuleConfigFactory) GetFSConfig() wazero.FSConfig {
	if wmcf == nil {
		panic("water: GetFSConfig: wmcf is nil")
	}

	return wmcf.fsconfig
}

// SetFSConfig sets the wazero.FSConfig for the WebAssembly module.
func (wmcf *WazeroModuleConfigFactory) SetFSConfig(fsconfig wazero.FSConfig) {
	wmcf.fsconfig = fsconfig
}

// SetArgv sets the arguments for the WebAssembly module.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file.
func (wmcf *WazeroModuleConfigFactory) SetArgv(argv []string) {
	wmcf.moduleConfig = wmcf.moduleConfig.WithArgs(argv...)
}

// InheritArgv sets the arguments for the WebAssembly module to os.Args.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file. Currently, this function is not implemented and will
// panic.
func (wmcf *WazeroModuleConfigFactory) InheritArgv() {
	// TODO: enumerate os.Args or deprecate this
	panic("water: InheritArgv: not implemented yet")
}

// SetEnv sets the environment variables for the WebAssembly module.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file.
func (wmcf *WazeroModuleConfigFactory) SetEnv(keys, values []string) {
	if len(keys) != len(values) {
		panic("water: SetEnv: keys and values must have the same length")
	}

	for i := range keys {
		wmcf.moduleConfig = wmcf.moduleConfig.WithEnv(keys[i], values[i])
	}
}

// InheritEnv sets the environment variables for the WebAssembly module to
// os.Environ.
//
// Warning: this isn't a recommended way to pass configuration to the
// WebAssembly module. Instead, use TransportModuleConfig for a serializable
// configuration file. Currently, this function is not implemented and will
// panic.
func (wmcf *WazeroModuleConfigFactory) InheritEnv() {
	// TODO: enumerate os.Environ or deprecate this
	panic("water: InheritEnv: not implemented yet")
}

// SetStdin sets the standard input for the WebAssembly module.
func (wmcf *WazeroModuleConfigFactory) SetStdin(r io.Reader) {
	wmcf.moduleConfig = wmcf.moduleConfig.WithStdin(r)
}

// InheritStdin sets the standard input for the WebAssembly module to os.Stdin.
func (wmcf *WazeroModuleConfigFactory) InheritStdin() {
	wmcf.moduleConfig = wmcf.moduleConfig.WithStdin(os.Stdin)
}

// SetStdout sets the standard output for the WebAssembly module.
func (wmcf *WazeroModuleConfigFactory) SetStdout(w io.Writer) {
	wmcf.moduleConfig = wmcf.moduleConfig.WithStdout(w)
}

// InheritStdout sets the standard output for the WebAssembly module to os.Stdout.
func (wmcf *WazeroModuleConfigFactory) InheritStdout() {
	wmcf.moduleConfig = wmcf.moduleConfig.WithStdout(os.Stdout)
}

// SetStderr sets the standard error for the WebAssembly module.
func (wmcf *WazeroModuleConfigFactory) SetStderr(w io.Writer) {
	wmcf.moduleConfig = wmcf.moduleConfig.WithStderr(w)
}

// InheritStderr sets the standard error for the WebAssembly module to os.Stderr.
func (wmcf *WazeroModuleConfigFactory) InheritStderr() {
	wmcf.moduleConfig = wmcf.moduleConfig.WithStderr(os.Stderr)
}

// SetPreopenDir sets the preopened directory for the WebAssembly module.
func (wmcf *WazeroModuleConfigFactory) SetPreopenDir(path string, guestPath string) {
	wmcf.fsconfig = wmcf.fsconfig.WithDirMount(path, guestPath)
}

// TODO: consider adding SetPreopenReadonlyDir
// TODO: consider adding SetPreopenFS

// WazeroRuntimeConfigFactory is used to spawn wazero.RuntimeConfig.
type WazeroRuntimeConfigFactory struct {
	runtimeConfig    wazero.RuntimeConfig
	compilationCache wazero.CompilationCache
}

// NewWazeroRuntimeConfigFactory creates a new WazeroRuntimeConfigFactory.
func NewWazeroRuntimeConfigFactory() *WazeroRuntimeConfigFactory {
	return &WazeroRuntimeConfigFactory{
		runtimeConfig:    wazero.NewRuntimeConfig().WithCloseOnContextDone(true),
		compilationCache: nil,
	}
}

// Clone returns a copy of the WazeroRuntimeConfigFactory.
func (wrcf *WazeroRuntimeConfigFactory) Clone() *WazeroRuntimeConfigFactory {
	if wrcf == nil {
		return nil
	}

	return &WazeroRuntimeConfigFactory{
		runtimeConfig:    wrcf.runtimeConfig,
		compilationCache: wrcf.compilationCache,
	}
}

// GetConfig returns the latest wazero.RuntimeConfig.
func (wrcf *WazeroRuntimeConfigFactory) GetConfig() wazero.RuntimeConfig {
	if wrcf == nil {
		panic("water: GetConfig: wrcf is nil")
	}

	if wrcf.compilationCache != nil {
		return wrcf.runtimeConfig.WithCompilationCache(wrcf.compilationCache)
	} else {
		return wrcf.runtimeConfig.WithCompilationCache(getGlobalCompilationCache())
	}
}

// Interpreter sets the WebAssembly module to run in the interpreter mode.
// In this mode, the WebAssembly module will run slower but it is available
// on all architectures/platforms.
//
// If no mode is set, the WebAssembly module will run in the compiler mode if
// supported, otherwise it will run in the interpreter mode.
func (wrcf *WazeroRuntimeConfigFactory) Interpreter() {
	wrcf.runtimeConfig = wazero.NewRuntimeConfigInterpreter()
}

// Compiler sets the WebAssembly module to run in the compiler mode.
// It may bring performance improvements, but meanwhile it will cause
// the program to panic if the architecture/platform is not supported.
//
// If no mode is set, the WebAssembly module will run in the compiler mode if
// supported, otherwise it will run in the interpreter mode.
func (wrcf *WazeroRuntimeConfigFactory) Compiler() {
	wrcf.runtimeConfig = wazero.NewRuntimeConfigCompiler()
}

// SetCloseOnContextDone sets the closeOnContextDone for the WebAssembly module.
// It closes the module when the context is done and prevents any further calls
// to the module, including all exported functions.
//
// By default it is set to true.
func (wrcf *WazeroRuntimeConfigFactory) SetCloseOnContextDone(close bool) {
	wrcf.runtimeConfig = wrcf.runtimeConfig.WithCloseOnContextDone(close)
}

// SetCompilationCache sets the CompilationCache for the WebAssembly module.
//
// Calling this function will not update the global CompilationCache and therefore
// disable the automatic sharing of the cache between multiple WebAssembly modules.
func (wrcf *WazeroRuntimeConfigFactory) SetCompilationCache(cache wazero.CompilationCache) {
	wrcf.compilationCache = cache
}

var globalCompilationCache wazero.CompilationCache
var globalCompilationCacheMutex = new(sync.Mutex)

func getGlobalCompilationCache() wazero.CompilationCache {
	globalCompilationCacheMutex.Lock()
	defer globalCompilationCacheMutex.Unlock()

	if globalCompilationCache == nil {
		var err error
		globalCompilationCache, err = wazero.NewCompilationCacheWithDir(fmt.Sprintf("%s%c%s", os.TempDir(), os.PathSeparator, "waterwazerocache"))
		if err != nil {
			panic(err)
		}
	}
	return globalCompilationCache
}

// SetGlobalCompilationCache sets the global CompilationCache for the WebAssembly
// runtime. This is useful for sharing the cache between multiple WebAssembly
// modules and should be called before any WebAssembly module is instantiated.
func SetGlobalCompilationCache(cache wazero.CompilationCache) {
	globalCompilationCacheMutex.Lock()
	globalCompilationCache = cache
	globalCompilationCacheMutex.Unlock()
}

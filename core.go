package water

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/gaukas/water/internal/log"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

var (
	ErrModuleNotImported = fmt.Errorf("water: importing a module not imported by the WebAssembly module")
	ErrFuncNotImported   = fmt.Errorf("water: importing a function not imported by the WebAssembly module")
)

// Core provides the low-level access to the WebAssembly runtime
// environment.
type Core interface {
	// Config returns the Config used to create the Core.
	//
	// Practically, this function is not supposed to be used
	// to retrieve the Config to be used for creating another
	// Core. Instead, it is used to retrieve the Config for
	// inspection/debugging purposes.
	Config() *Config

	// Context returns the base context used by the Core.
	Context() context.Context

	// Close closes the Core and releases all the resources
	// associated with it.
	Close() error

	// Exports dumps all the exported functions and globals which
	// are provided by the WebAssembly module.
	//
	// This function returns a map of export name to the type of
	// the export.
	Exports() map[string]api.ExternType

	// ExportedFunction returns the exported function with the
	// given name. If no function with the given name is exported,
	// this function returns nil.
	//
	// It is caller's responsibility to ensure that the function
	// returned has the correct signature, which can be checked
	// by inspecting the returned api.Function.Definition().
	ExportedFunction(name string) api.Function

	// Imports dumps all the imported functions and globals which
	// are required to be provided by the host environment to
	// instantiate the WebAssembly module.
	//
	// This function returns a map of module name to a map of
	// import name to the type of the import.
	ImportedFunctions() map[string]map[string]api.FunctionDefinition

	// ImportFunction imports a function into the WebAssembly module.
	//
	// The f argument must be a function with the signature matching
	// the signature of the function to be imported. Otherwise, the
	// behavior of the WebAssembly Transport Module after initialization
	// is undefined.
	//
	// This function can be called ONLY BEFORE calling Instantiate().
	ImportFunction(module, name string, f any) error

	// InsertConn inserts a net.Conn into the WebAssembly Transport
	// Module and returns the key of the inserted connection as a
	// file descriptor accessible from the WebAssembly instance.
	//
	// This function SHOULD be called only if the WebAssembly instance
	// execution is blocked/halted/stopped. Otherwise, race conditions
	// or undefined behaviors may occur.
	InsertConn(conn net.Conn) (fd int32, err error)

	// InsertListener inserts a net.Listener into the WebAssembly
	// Transport Module and returns the key of the inserted listener
	// as a file descriptor accessible from the WebAssembly instance.
	//
	// This function SHOULD be called only if the WebAssembly instance
	// execution is blocked/halted/stopped. Otherwise, race conditions
	// or undefined behaviors may occur.
	InsertListener(tcpListener net.Listener) (fd int32, err error)

	// InsertFile inserts a file into the WebAssembly Transport Module
	// and returns the key of the inserted file as a file descriptor
	// accessible from the WebAssembly instance.
	//
	// This function SHOULD be called only if the WebAssembly instance
	// execution is blocked/halted/stopped. Otherwise, race conditions
	// or undefined behaviors may occur.
	InsertFile(osFile *os.File) (fd int32, err error)

	// Instantiate instantiates the module into a new instance of
	// WebAssembly Transport Module.
	Instantiate() error

	// Invoke invokes a function in the WebAssembly instance.
	//
	// If the target function is not exported, this function returns an error.
	Invoke(funcName string, params ...uint64) (results []uint64, err error)

	// WASIPreview1 enables the WASI preview1 API.
	//
	// It is recommended that this function only to be invoked if
	// the WATM expects the WASI preview1 API to be available.
	//
	// Note that at the time of writing, all WATM implementations
	// expect the WASI preview1 API to be available.
	WASIPreview1() error

	// Logger returns the logger used by the Core. If not set, it
	// should return the default global logger instead of nil.
	Logger() *log.Logger
}

// type guard
var _ Core = (*core)(nil)

// core provides the WASM runtime base and is an internal struct
// that every RuntimeXxx implementation will embed.
//
// core is designed to be unmanaged and version-independent,
// which means it does not provide any functionalities other
// than simply collecting all the WASM runtime-related objects
// without overwriting access on them. And core is not subject
// to breaking changes unless a severe bug needs to be fixed
// in such a breaking manner inevitably.
//
// This struct implements Core.
type core struct {
	// config
	config *Config

	ctx      context.Context
	runtime  wazero.Runtime
	module   wazero.CompiledModule
	instance api.Module

	// saved after Exports() is called
	exportsLoadOnce sync.Once
	exports         map[string]api.ExternType

	// saved after ImportedFunctions() is called
	importedFuncsLoadOnce sync.Once
	importedFuncs         map[string]map[string]api.FunctionDefinition

	importModules map[string]wazero.HostModuleBuilder

	closeOnce sync.Once
}

// NewCore creates a new Core with the given config.
//
// It uses the default implementation of interface.Core as
// defined in this file.
func NewCore(config *Config) (Core, error) {
	return NewCoreWithContext(context.Background(), config)
}

func NewCoreWithContext(ctx context.Context, config *Config) (Core, error) {
	var err error

	c := &core{
		config:        config,
		importModules: make(map[string]wazero.HostModuleBuilder),
	}

	c.ctx = ctx
	c.runtime = wazero.NewRuntimeWithConfig(ctx, config.RuntimeConfig().GetConfig())

	if c.module, err = c.runtime.CompileModule(ctx, c.config.WATMBinOrPanic()); err != nil {
		return nil, fmt.Errorf("water: (*Runtime).CompileModule returned error: %w", err)
	}

	runtime.SetFinalizer(c, func(core *core) {
		c.Close()
	})

	return c, nil
}

// Config implements Core.
func (c *core) Config() *Config {
	return c.config
}

// Context implements Core.
func (c *core) Context() context.Context {
	return c.ctx
}

func (c *core) cleanup() {
	for i := range c.importModules {
		delete(c.importModules, i)
	}

	for i := range c.exports {
		delete(c.exports, i)
	}

	for i := range c.importedFuncs {
		delete(c.importedFuncs, i)
	}
}

// Close implements Core.
func (c *core) Close() error {
	var closeErr error

	c.closeOnce.Do(func() {
		if c.instance != nil {
			if err := c.instance.Close(c.ctx); err != nil {
				closeErr = fmt.Errorf("water: (*wazero/api.Module).Close returned error: %w", err)
				return
			}
			c.instance = nil // TODO: force dropped
			log.LDebugf(c.config.Logger(), "INSTANCE DROPPED")
		}

		if c.runtime != nil {
			if err := c.runtime.Close(c.ctx); err != nil {
				closeErr = fmt.Errorf("water: (*wazero.Runtime).Close returned error: %w", err)
				return
			}
			c.runtime = nil // TODO: force dropped
			log.LDebugf(c.config.Logger(), "RUNTIME DROPPED")
		}

		if c.module != nil {
			if err := c.module.Close(c.ctx); err != nil {
				closeErr = fmt.Errorf("water: (*wazero.CompiledModule).Close returned error: %w", err)
				return
			}
			c.module = nil // TODO: force dropped
			log.LDebugf(c.config.Logger(), "MODULE DROPPED")
		}

		c.cleanup()
	})

	return closeErr
}

// Exports implements Core.
func (c *core) Exports() map[string]api.ExternType {
	c.exportsLoadOnce.Do(func() {
		c.exports = c.module.AllExports()
	})

	return c.exports
}

// ExportedFunction implements Core.
func (c *core) ExportedFunction(name string) api.Function {
	if c.instance == nil {
		return nil
	}

	return c.instance.ExportedFunction(name)
}

// ImportedFunctions implements Core.
func (c *core) ImportedFunctions() map[string]map[string]api.FunctionDefinition {
	c.importedFuncsLoadOnce.Do(func() {
		importedFuncs := c.module.ImportedFunctions()

		c.importedFuncs = make(map[string]map[string]api.FunctionDefinition)
		for _, importedFunc := range importedFuncs {
			mod, name, ok := importedFunc.Import()
			if !ok {
				continue
			}

			if _, ok := c.importedFuncs[mod]; !ok {
				c.importedFuncs[mod] = make(map[string]api.FunctionDefinition)
			}
			c.importedFuncs[mod][name] = importedFunc
		}
	})

	return c.importedFuncs
}

// ImportFunction implements Core.
func (c *core) ImportFunction(module, name string, f any) error {
	if c.instance != nil {
		return fmt.Errorf("water: cannot import function after instantiation")
	}

	// Unsafe: check if the WebAssembly module really imports this function under
	// the given module and name. If not, we warn and skip the import.
	if mod, ok := c.ImportedFunctions()[module]; !ok {
		log.LDebugf(c.config.Logger(), "water: module %s is not imported.", module)
		return ErrModuleNotImported
	} else if _, ok := mod[name]; !ok {
		log.LWarnf(c.config.Logger(), "water: function %s.%s is not imported.", module, name)
		return ErrFuncNotImported
	}

	if _, ok := c.importModules[module]; !ok {
		c.importModules[module] = c.runtime.NewHostModuleBuilder(module)
	}

	// We don't do this:
	//
	//  _, err := c.importModules[module].NewFunctionBuilder().WithFunc(f).Export(name).Instantiate(c.ctx)
	//  if err != nil {
	// 		log.LErrorf(c.config.Logger(), "water: (*wazero.HostModuleBuilder).NewFunctionBuilder returned error: %v", err)
	//  }
	//
	// Instead we do not instantiate the function here, but wait until Instantiate() is called.
	c.importModules[module] = c.importModules[module].NewFunctionBuilder().WithFunc(f).Export(name)

	// TODO: return an error if the function already exists or the
	// module/function name is invalid.
	return nil
}

// Instantiate implements Core.
func (c *core) Instantiate() (err error) {
	if c.instance != nil {
		return fmt.Errorf("water: double instantiation is not allowed")
	}

	// Instantiate the imported functions
	for _, moduleBuilder := range c.importModules {
		if _, err := moduleBuilder.Instantiate(c.ctx); err != nil {
			return fmt.Errorf("water: (*wazero.HostModuleBuilder).Instantiate returned error: %w", err)
		}
	}

	if c.instance, err = c.runtime.InstantiateModule(
		c.ctx,
		c.module,
		c.config.ModuleConfig().GetConfig()); err != nil {
		return fmt.Errorf("water: (*Runtime).InstantiateWithConfig returned error: %w", err)
	}

	return nil
}

// Invoke implements Core.
func (c *core) Invoke(funcName string, params ...uint64) (results []uint64, err error) {
	if c.instance == nil {
		return nil, fmt.Errorf("water: cannot invoke function before instantiation")
	}

	expFunc := c.instance.ExportedFunction(funcName)
	if expFunc == nil {
		return nil, fmt.Errorf("water: function %q is not exported", funcName)
	}

	results, err = expFunc.Call(c.ctx, params...)
	if err != nil {
		return nil, fmt.Errorf("water: (*wazero.ExportedFunction).Call returned error: %w", err)
	}

	return
}

// WASIPreview1 implements Core.
func (c *core) WASIPreview1() error {
	if _, err := wasi_snapshot_preview1.Instantiate(c.ctx, c.runtime); err != nil {
		return fmt.Errorf("water: wazero/imports/wasi_snapshot_preview1.Instantiate returned error: %w", err)
	}
	return nil
}

// Logger implements Core.
func (c *core) Logger() *log.Logger {
	return c.config.Logger()
}

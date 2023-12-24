package water

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
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
	WASIPreview1() error
}

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
// Implements Core.
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
	c.runtime = wazero.NewRuntime(ctx)

	if c.module, err = c.runtime.CompileModule(ctx, c.config.WATMBinOrPanic()); err != nil {
		return nil, fmt.Errorf("water: (*Runtime).CompileModule returned error: %w", err)
	}

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
			if _, ok := c.importedFuncs[importedFunc.ModuleName()]; !ok {
				c.importedFuncs[importedFunc.ModuleName()] = make(map[string]api.FunctionDefinition)
			}

			c.importedFuncs[importedFunc.ModuleName()][importedFunc.Name()] = importedFunc
		}
	})

	return c.importedFuncs
}

// ImportFunction implements Core.
func (c *core) ImportFunction(module, name string, f any) error {
	if c.instance != nil {
		return fmt.Errorf("water: cannot import function after instantiation")
	}

	if _, ok := c.importModules[module]; !ok {
		c.importModules[module] = c.runtime.NewHostModuleBuilder(module)
	}

	c.importModules[module].NewFunctionBuilder().WithFunc(f).Export(name).Instantiate(c.ctx)

	// TODO: return an error if the function already exists or the
	// module/function name is invalid.
	return nil
}

// InsertConn implements Core.
func (c *core) InsertConn(conn net.Conn) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert TCPConn before instantiation")
	}

	switch conn := conn.(type) {
	case *net.TCPConn:
		key, ok := c.instance.InsertTCPConn(conn)
		if !ok {
			return 0, fmt.Errorf("water: (*wazero.Module).InsertTCPConn returned false")
		}
		if key <= 0 {
			return key, fmt.Errorf("water: (*wazero.Module).InsertTCPConn returned invalid key")
		}
		return key, nil
	default:
		// TODO: support other types of connections as much as possible
		return 0, fmt.Errorf("water: unsupported connection type: %T", conn)
	}
}

// InsertListener implements Core.
func (c *core) InsertListener(listener net.Listener) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert TCPListener before instantiation")
	}

	switch listener := listener.(type) {
	case *net.TCPListener:
		key, ok := c.instance.InsertTCPListener(listener)
		if !ok {
			return 0, fmt.Errorf("water: (*wazero.Module).InsertTCPListener returned false")
		}
		if key <= 0 {
			return key, fmt.Errorf("water: (*wazero.Module).InsertTCPListener returned invalid key")
		}
		return key, nil
	default:
		// TODO: support other types of listeners as much as possible
		return 0, fmt.Errorf("water: unsupported listener type: %T", listener)
	}
}

// InsertFile implements Core.
func (c *core) InsertFile(osFile *os.File) (fd int32, err error) {
	if c.instance == nil {
		return 0, fmt.Errorf("water: cannot insert File before instantiation")
	}

	key, ok := c.instance.InsertOSFile(osFile)
	if !ok {
		return 0, fmt.Errorf("water: (*wazero.Module).InsertFile returned false")
	}
	if key <= 0 {
		return key, fmt.Errorf("water: (*wazero.Module).InsertFile returned invalid key")
	}

	return key, nil
}

// Instantiate implements Core.
func (c *core) Instantiate() error {
	if c.instance != nil {
		return fmt.Errorf("water: double instantiation is not allowed")
	}

	moduleConfig, err := c.config.ModuleConfigFactory.GetConfig()
	if err != nil {
		return fmt.Errorf("water: (*RuntimeConfigFactory).GetConfig returned error: %w", err)
	}

	if c.instance, err = c.runtime.InstantiateModule(c.ctx, c.module, moduleConfig); err != nil {
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

// type guard
var _ Core = (*core)(nil)

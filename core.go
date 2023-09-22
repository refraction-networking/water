package water

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v12"
)

// runtimeCore provides the WASM runtime base and is an internal struct
// that every RuntimeXxx implementation will embed.
type runtimeCore struct {
	// wasmtime
	engine   *wasmtime.Engine
	module   *wasmtime.Module
	store    *wasmtime.Store
	linker   *wasmtime.Linker
	instance *wasmtime.Instance

	// wasi imports
	deferFuncs []func() // defer functions to be called by WASM module on exit

	// wasi exports
	_init    *wasmtime.Func
	_version *wasmtime.Func

	// other wasi related stuff
	wd *WasiDialer
}

func NewCore() *runtimeCore {
	return &runtimeCore{}
}

func (c *runtimeCore) Defer(f func()) {
	c.deferFuncs = append(c.deferFuncs, f)
}

func (c *runtimeCore) linkDefer() error {
	if c.linker == nil {
		return fmt.Errorf("linker not set")
	}

	if err := c.linker.DefineFunc(c.store, "env", "defer", c._defer); err != nil {
		return fmt.Errorf("(*wasmtime.Linker).DefineFunc: %w", err)
	}

	return nil
}

func (c *runtimeCore) linkDialer(dialer Dialer, address string) error {
	if c.linker == nil {
		return fmt.Errorf("linker not set")
	}

	if dialer == nil {
		return fmt.Errorf("dialer not set")
	}

	if c.wd == nil {
		c.wd = NewWasiDialer(address, dialer, c.store)
	} else {
		return fmt.Errorf("wasi dialer already set, double-linking?")
	}

	if err := c.linker.DefineFunc(c.store, "env", "dial", c.wd.WasiDialerFunc); err != nil {
		return fmt.Errorf("(*wasmtime.Linker).DefineFunc: %w", err)
	}

	return nil
}

func (c *runtimeCore) initializeConn() (RuntimeConn, error) {
	// get _init and _version functions
	c._init = c.instance.GetFunc(c.store, "_init")
	if c._init == nil {
		return nil, fmt.Errorf("instantiated WASM module does not export _init function")
	}
	c._version = c.instance.GetFunc(c.store, "_version")
	if c._version == nil {
		return nil, fmt.Errorf("instantiated WASM module does not export _version function")
	}

	// initialize WASM instance.
	// In a _init() call, the WASM module will setup all its internal states
	_, err := c._init.Call(c.store)
	if err != nil {
		return nil, fmt.Errorf("errored upon calling _init function: %w", err)
	}

	// get version
	// In a _version() call, the WASM module will return its version
	ret, err := c._version.Call(c.store)
	if err != nil {
		return nil, fmt.Errorf("errored upon calling _version function: %w", err)
	}
	if ver, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("_version function returned non-int32 value")
	} else {
		return RuntimeConnWithVersion(c, ver)
	}
}

func (c *runtimeCore) _defer() {
	for _, f := range c.deferFuncs {
		f()
	}
}

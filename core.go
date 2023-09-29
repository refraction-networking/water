package water

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v13"
)

// runtimeCore provides the WASM runtime base and is an internal struct
// that every RuntimeXxx implementation will embed.
type runtimeCore struct {
	// config
	config *Config

	// wasmtime
	engine   *wasmtime.Engine
	module   *wasmtime.Module
	store    *wasmtime.Store
	linker   *wasmtime.Linker
	instance *wasmtime.Instance

	/// WASI imports
	deferFuncs []func() // defer functions to be called by WASM module on exit

	/// WASI Exports
	_init    *wasmtime.Func // _init()
	_version *wasmtime.Func // _version()

	// WASI Dialer and Listener
	wd *WASIDialer
	wl *WASIListener
}

func Core(config *Config) (c *runtimeCore, err error) {
	c = &runtimeCore{
		config:     config,
		deferFuncs: make([]func(), 0),
	}

	var wasiConfig *wasmtime.WasiConfig
	wasiConfig, err = c.config.WASIConfigFactory.GetConfig()
	if err != nil {
		err = fmt.Errorf("water: (*WasiConfigFactory).GetConfig returned error: %w", err)
	}

	c.engine = wasmtime.NewEngine()
	c.module, err = wasmtime.NewModule(c.engine, c.config.WABin)
	if err != nil {
		err = fmt.Errorf("water: wasmtime.NewModule returned error: %w", err)
		return
	}
	c.store = wasmtime.NewStore(c.engine)
	c.store.SetWasiConfig(wasiConfig)
	c.linker = wasmtime.NewLinker(c.engine)
	err = c.linker.DefineWasi()
	if err != nil {
		err = fmt.Errorf("water: (*wasmtime.Linker).DefineWasi returned error: %w", err)
		return
	}

	return
}

func (c *runtimeCore) Config() *Config {
	return c.config
}

func (c *runtimeCore) WASIDialer() *WASIDialer {
	return c.wd
}

func (c *runtimeCore) WASIListener() *WASIListener {
	return c.wl
}

func (c *runtimeCore) DeferFunc(f func()) {
	c.deferFuncs = append(c.deferFuncs, f)
}

func (c *runtimeCore) linkExecDeferredFunc() error {
	if c.linker == nil {
		return fmt.Errorf("water: linker not set, is runtimeCore initialized?")
	}

	if err := c.linker.FuncNew("env", "deferh",
		wasmtime.NewFuncType(
			[]*wasmtime.ValType{},
			[]*wasmtime.ValType{},
		),
		func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			c.execDeferredFunc()
			return []wasmtime.Val{}, nil
		},
	); err != nil {
		return fmt.Errorf("water: (*wasmtime.Linker).FuncNew: %w", err)
	}

	return nil
}

func (c *runtimeCore) LinkNetworkInterface(dialer *WASIDialer, listener *WASIListener) error {
	if c.linker == nil {
		return fmt.Errorf("water: linker not set, is runtimeCore initialized?")
	}

	if dialer != nil {
		if err := c.linkWASIDialFunc(dialer.dial); err != nil {
			return fmt.Errorf("water: (*runtimeCore).linkWASIDialerFunc: %w", err)
		}
	} else {
		if err := c.linkNOPWASIDialFunc(); err != nil {
			return fmt.Errorf("water: (*runtimeCore).linkNOPWASIDialerFunc: %w", err)
		}
	}

	if listener != nil {
		if err := c.linkWASIAcceptFunc(listener.accept); err != nil {
			return fmt.Errorf("water: (*runtimeCore).linkWASIAcceptFunc: %w", err)
		}
	} else {
		if err := c.linkNOPWASIAcceptFunc(); err != nil {
			return fmt.Errorf("water: (*runtimeCore).linkNOPWASIAcceptFunc: %w", err)
		}
	}

	c.wd = dialer
	c.wl = listener

	return nil
}

func (c *runtimeCore) Initialize() (err error) {
	err = c.linkExecDeferredFunc()
	if err != nil {
		return fmt.Errorf("water: (*runtimeCore).linkExecDeferredFunc: %w", err)
	}

	// instantiate the WASM module
	c.instance, err = c.linker.Instantiate(c.store, c.module)
	if err != nil {
		err = fmt.Errorf("water: (*wasmtime.Linker).Instantiate returned error: %w", err)
		return
	}

	// get _init and _version functions
	c._init = c.instance.GetFunc(c.store, "_init")
	if c._init == nil {
		return fmt.Errorf("water: instantiated WASM module does not export _init function")
	}
	c._version = c.instance.GetFunc(c.store, "_version")
	if c._version == nil {
		return fmt.Errorf("water: instantiated WASM module does not export _version function")
	}

	// initialize WASM instance.
	// In a _init() call, the WASM module will setup all its internal states
	_, err = c._init.Call(c.store)
	if err != nil {
		return fmt.Errorf("water: errored upon calling _init function: %w", err)
	}

	return nil
}

func (c *runtimeCore) OutboundRuntimeConn() (RuntimeConn, error) {
	// get version
	// In a _version() call, the WASM module will return its version
	ret, err := c._version.Call(c.store)
	if err != nil {
		return nil, fmt.Errorf("water: calling _version function returned error: %w", err)
	}
	if ver, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _version function definition")
	} else {
		return OutboundRuntimeConnWithVersion(c, ver)
	}
}

func (c *runtimeCore) InboundRuntimeConn() (RuntimeConn, error) {
	// get version
	// In a _version() call, the WASM module will return its version
	ret, err := c._version.Call(c.store)
	if err != nil {
		return nil, fmt.Errorf("water: calling _version function returned error: %w", err)
	}
	if ver, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _version function definition")
	} else {
		return InboundRuntimeConnWithVersion(c, ver)
	}
}

func (c *runtimeCore) execDeferredFunc() {
	for _, f := range c.deferFuncs {
		f()
	}
}

func (c *runtimeCore) linkWASIDialFunc(f WASIConnectFunc) error {
	err := c.linker.FuncNew("env", "dialh", WASIConnectFuncType, WrapWASIConnectFunc(f))
	if err != nil {
		return fmt.Errorf("(*wasmtime.Linker).FuncNew: %w", err)
	}
	return nil
}

func (c *runtimeCore) linkNOPWASIDialFunc() error {
	return c.linkWASIDialFunc(nopWASIConnectFunc)
}

func (c *runtimeCore) linkWASIAcceptFunc(f WASIConnectFunc) error {
	err := c.linker.FuncNew("env", "accepth", WASIConnectFuncType, WrapWASIConnectFunc(f))
	if err != nil {
		return fmt.Errorf("(*wasmtime.Linker).FuncNew: %w", err)
	}
	return nil
}

func (c *runtimeCore) linkNOPWASIAcceptFunc() error {
	return c.linkWASIAcceptFunc(nopWASIConnectFunc)
}

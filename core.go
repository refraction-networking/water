package water

import (
	"fmt"
	"net"

	"github.com/bytecodealliance/wasmtime-go/v12"
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

	// wasi imports
	deferFuncs []func() // defer functions to be called by WASM module on exit

	// wasi exports
	_init    *wasmtime.Func
	_version *wasmtime.Func

	// wasi dialer
	wd *WASIDialer
}

func Core(config *Config) (c *runtimeCore, err error) {
	c = &runtimeCore{
		config: config,
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

func (c *runtimeCore) DeferFunc(f func()) {
	c.deferFuncs = append(c.deferFuncs, f)
}

func (c *runtimeCore) LinkDefer() error {
	if c.linker == nil {
		return fmt.Errorf("water: linker not set, is runtimeCore initialized?")
	}

	if err := c.linker.DefineFunc(c.store, "env", "defer", c._defer); err != nil {
		return fmt.Errorf("water: (*wasmtime.Linker).DefineFunc: %w", err)
	}

	return nil
}

func (c *runtimeCore) LinkNetworkDialer(netDialer NetworkDialer, network, address string) error {
	if c.linker == nil {
		return fmt.Errorf("water: linker not set, is runtimeCore initialized?")
	}

	if netDialer == nil {
		return fmt.Errorf("water: cannot link nil NetworkDialer")
	}

	if c.wd == nil {
		c.wd = NewWASIDialer(network, address, netDialer, c.store)
	} else {
		return fmt.Errorf("water: WASI dialer already set, are you double-linking?")
	}

	if err := c.linker.DefineFunc(c.store, "env", "dial", c.wd.WASIDialerFunc); err != nil {
		return fmt.Errorf("water: (*wasmtime.Linker).DefineFunc: %w", err)
	}

	return nil
}

func (c *runtimeCore) LinkNetworkListener(netListener net.Listener, network, address string) error {
	if c.linker == nil {
		return fmt.Errorf("water: linker not set, is runtimeCore initialized?")
	}

	if netListener == nil {
		return fmt.Errorf("water: cannot link nil net.Listener")
	}

	// TODO

	// empty dial func
	if err := c.linker.DefineFunc(c.store, "env", "dial", func() int32 { return 0 }); err != nil {
		return fmt.Errorf("water: (*wasmtime.Linker).DefineFunc: %w", err)
	}

	return nil
}

func (c *runtimeCore) Initialize() (err error) {
	// instantiate the WASM module
	c.instance, err = c.linker.Instantiate(c.store, c.module)
	if err != nil {
		err = fmt.Errorf("water: (*wasmtime.Linker).Instantiate returned error: %w", err)
		return
	}

	// get _init and _version functions
	c._init = c.instance.GetFunc(c.store, "_init")
	if c._init == nil {
		return fmt.Errorf("instantiated WASM module does not export _init function")
	}
	c._version = c.instance.GetFunc(c.store, "_version")
	if c._version == nil {
		return fmt.Errorf("instantiated WASM module does not export _version function")
	}

	// initialize WASM instance.
	// In a _init() call, the WASM module will setup all its internal states
	_, err = c._init.Call(c.store)
	if err != nil {
		return fmt.Errorf("errored upon calling _init function: %w", err)
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

func (c *runtimeCore) InboundRuntimeConn(ibc net.Conn) (RuntimeConn, error) {
	// get version
	// In a _version() call, the WASM module will return its version
	ret, err := c._version.Call(c.store)
	if err != nil {
		return nil, fmt.Errorf("water: calling _version function returned error: %w", err)
	}
	if ver, ok := ret.(int32); !ok {
		return nil, fmt.Errorf("water: invalid _version function definition")
	} else {
		return InboundRuntimeConnWithVersion(c, ver, ibc)
	}
}

func (c *runtimeCore) _defer() {
	for _, f := range c.deferFuncs {
		f()
	}
}

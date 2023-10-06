package water

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v13"
)

// Core provides the WASM runtime base and is an internal struct
// that every RuntimeXxx implementation will embed.
//
// Core is not versioned and is not subject to breaking changes
// unless a severe bug needs to be fixed in a breaking way.
type core struct {
	// config
	config *Config

	// wasmtime
	engine   *wasmtime.Engine
	module   *wasmtime.Module
	store    *wasmtime.Store // avoid directly accessing store once the instance is created
	linker   *wasmtime.Linker
	instance *wasmtime.Instance
}

// Core creates a new Core, which is the base of all
// WASM runtime functionalities.
func Core(config *Config) (c *core, err error) {
	c = &core{
		config: config,
	}

	var wasiConfig *wasmtime.WasiConfig
	wasiConfig, err = c.config.WASIConfig().GetConfig()
	if err != nil {
		err = fmt.Errorf("water: (*WasiConfigFactory).GetConfig returned error: %w", err)
		return
	}

	c.engine = wasmtime.NewEngine()
	c.module, err = wasmtime.NewModule(c.engine, c.config.WATMBinOrPanic())
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

func (c *core) DialVersion(network, address string) (Conn, error) {
	for _, export := range c.module.Exports() {
		if f, ok := mapCoreDialContext[export.Name()]; ok {
			return f(c, network, address)
		}
	}
	return nil, fmt.Errorf("water: core loaded a WASM module that does not implement any known version")
}

func (c *core) AcceptVersion() (Conn, error) {
	for _, export := range c.module.Exports() {
		if f, ok := mapCoreAccept[export.Name()]; ok {
			return f(c)
		}
	}
	return nil, fmt.Errorf("water: core loaded a WASM module that does not implement any known version")
}

// Config returns the Config used to create the Core.
func (c *core) Config() *Config {
	return c.config
}

func (c *core) Engine() *wasmtime.Engine {
	return c.engine
}

func (c *core) Instance() *wasmtime.Instance {
	return c.instance
}

func (c *core) Linker() *wasmtime.Linker {
	return c.linker
}

func (c *core) Module() *wasmtime.Module {
	return c.module
}

func (c *core) Store() *wasmtime.Store {
	return c.store
}

func (c *core) Instantiate() error {
	instance, err := c.linker.Instantiate(c.store, c.module)
	if err != nil {
		return fmt.Errorf("water: (*wasmtime.Linker).Instantiate returned error: %w", err)
	}

	c.instance = instance
	return nil
}

package water

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v13"
)

// Core provides the low-level access to the WebAssembly runtime
// environment.
//
// Currently it depends on the wasmtime-go API, but it is subject to
// change in the future.
type Core interface {
	Config() *Config
	Engine() *wasmtime.Engine
	Instance() *wasmtime.Instance
	Linker() *wasmtime.Linker
	Module() *wasmtime.Module
	Store() *wasmtime.Store
	Instantiate() error
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

	// wasmtime
	engine   *wasmtime.Engine
	module   *wasmtime.Module
	store    *wasmtime.Store // avoid directly accessing store once the instance is created
	linker   *wasmtime.Linker
	instance *wasmtime.Instance
}

// NewCore creates a new Core with the given config.
//
// It uses the default implementation of interface.Core as
// defined in this file.
func NewCore(config *Config) (Core, error) {
	c := &core{
		config: config,
	}

	wasiConfig, err := c.config.WASIConfig().GetConfig()
	if err != nil {
		err = fmt.Errorf("water: (*WasiConfigFactory).GetConfig returned error: %w", err)
		return nil, err
	}

	c.engine = wasmtime.NewEngine()
	c.module, err = wasmtime.NewModule(c.engine, c.config.WATMBinOrPanic())
	if err != nil {
		err = fmt.Errorf("water: wasmtime.NewModule returned error: %w", err)
		return nil, err
	}
	c.store = wasmtime.NewStore(c.engine)
	c.store.SetWasiConfig(wasiConfig)
	c.linker = wasmtime.NewLinker(c.engine)
	err = c.linker.DefineWasi()
	if err != nil {
		err = fmt.Errorf("water: (*wasmtime.Linker).DefineWasi returned error: %w", err)
		return nil, err
	}

	return c, nil
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

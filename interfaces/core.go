package interfaces

import (
	"github.com/bytecodealliance/wasmtime-go/v13"
	"github.com/gaukas/water/config"
)

type Core interface {
	Config() *config.Config
	Engine() *wasmtime.Engine
	Instance() *wasmtime.Instance
	Linker() *wasmtime.Linker
	Module() *wasmtime.Module
	Store() *wasmtime.Store
	Instantiate() error
}

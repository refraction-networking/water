package water

import (
	"context"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v12"
)

func Dial(address string, config *Config) (RuntimeConn, error) {
	rd := &RuntimeConnDialer{
		Config: config,
	}
	return rd.DialContext(context.Background(), address)
}

func DialContext(ctx context.Context, address string, config *Config) (RuntimeConn, error) {
	rd := &RuntimeConnDialer{
		Config: config,
	}

	return rd.DialContext(ctx, address)
}

type RuntimeConnDialer struct {
	Config           *Config
	WasiConfigEngine *WasiConfigEngine // WASI config, if any. If not specified, a default (empty) config will be used.
}

func (d *RuntimeConnDialer) Dial(address string) (RuntimeConn, error) {
	return d.DialContext(context.Background(), address)
}

func (d *RuntimeConnDialer) DialContext(ctx context.Context, address string) (RuntimeConn, error) {
	if d.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is not allowed")
	}
	d.Config.init()

	wasiConfig, err := d.WasiConfigEngine.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("water: (*WasiConfigEngine).GetConfig returned error: %w", err)
	}

	rc := new(runtimeCore)
	// Setup WASMTIME lib
	rc.engine = wasmtime.NewEngine()
	rc.module, err = wasmtime.NewModule(rc.engine, d.Config.WABin)
	if err != nil {
		return nil, fmt.Errorf("water: wasmtime.NewModule returned error: %w", err)
	}
	rc.store = wasmtime.NewStore(rc.engine)
	rc.store.SetWasiConfig(wasiConfig)
	rc.linker = wasmtime.NewLinker(rc.engine)
	err = rc.linker.DefineWasi()
	if err != nil {
		return nil, fmt.Errorf("water: (*wasmtime.Linker).DefineWasi returned error: %w", err)
	}

	// link dialer funcs
	if err := rc.linkDialer(d.Config.Dialer, address); err != nil {
		return nil, fmt.Errorf("water: (*runtimeConn).linkDialerFunc returned error: %w", err)
	}

	// link defer funcs
	if err := rc.linkDefer(); err != nil {
		return nil, fmt.Errorf("water: (*runtimeConn).linkDefer returned error: %w", err)
	}

	// instantiate the WASM module
	rc.instance, err = rc.linker.Instantiate(rc.store, rc.module)
	if err != nil {
		return nil, fmt.Errorf("water: (*wasmtime.Linker).Instantiate returned error: %w", err)
	}

	vrc, err := rc.initializeConn() // will return versioned RuntimeConn
	if err != nil {
		return nil, fmt.Errorf("water: (*runtimeConn).initialize returned error: %w", err)
	}

	return vrc, nil
}

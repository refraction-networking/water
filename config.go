package water

import (
	"context"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

type Config struct {
	// WASI contains the compiled WASI binary in bytes.
	WASI []byte

	// Dialer is used to dial a network connection.
	Dialer Dialer
}

// init() checks if the Config is valid and initializes
// the Config with default values if optional fields are not provided.
func (c *Config) init() {
	if len(c.WASI) == 0 {
		panic("water: WASI binary is not provided")
	}

	if c.Dialer == nil {
		c.Dialer = DefaultDialer()
	}
}

type RuntimeDialer struct {
	Config *Config
}

func (d *RuntimeDialer) DialContext(ctx context.Context, address string) (rc *RuntimeConn, err error) {
	if d.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is prohibited")
	}

	var wasiConfig *wasmtime.WasiConfig
	var ok bool
	if wasiConfig, ok = ctx.Value("wasi_config").(*wasmtime.WasiConfig); !ok {
		wasiConfig = wasmtime.NewWasiConfig()
	}

	rc = new(RuntimeConn)
	// preopen the socket directory
	err = rc.preopenSocketDir(wasiConfig)
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConn).preopenSoocketDir retirmed error: %w", err)
	}

	// load the WASI module
	if rc.engine, ok = ctx.Value("wasm_engine").(*wasmtime.Engine); !ok {
		rc.engine = wasmtime.NewEngine()
	}
	rc.module, err = wasmtime.NewModule(rc.engine, d.Config.WASI)
	if err != nil {
		return nil, fmt.Errorf("water: wasmtime.NewModule returned error: %w", err)
	}

	// create the store
	if rc.store, ok = ctx.Value("wasm_store").(*wasmtime.Store); !ok {
		rc.store = wasmtime.NewStore(rc.engine)
	}
	rc.store.SetWasi(wasiConfig)

	// create the linker
	if rc.linker, ok = ctx.Value("wasm_linker").(*wasmtime.Linker); !ok {
		rc.linker = wasmtime.NewLinker(rc.engine)
	}
	err = rc.linker.DefineWasi()
	if err != nil {
		return nil, fmt.Errorf("water: linker.DefineWasi returned error: %w", err)
	}

	// link dialer funcs
	err = rc.linkDialerFunc(d.Config.Dialer, address)
	if err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConn).linkDialerFunc returned error: %w", err)
	}

	// instantiate the WASI module
	rc.instance, err = rc.linker.Instantiate(rc.store, rc.module)
	if err != nil {
		return nil, fmt.Errorf("water: linker.Instantiate returned error: %w", err)
	}

	// check the WASI version
	versionFunc := rc.instance.GetFunc(rc.store, "_version")
	if versionFunc == nil {
		return nil, fmt.Errorf("water: WASI module does not have an _version function")
	}
	version, err := versionFunc.Call(rc.store)
	if err != nil {
		return nil, fmt.Errorf("water: _version function returned error: %w", err)
	}
	if version, ok := version.(int32); !ok {
		return nil, fmt.Errorf("water: _version function returned non-int32 value")
	} else if version != RUNTIME_VERSION_MAJOR {
		return nil, fmt.Errorf("water: WASI module is in v%d and not compatible with runtime version %s!", version, RUNTIME_VERSION)
	}

	// run the WASI init function
	initFunc := rc.instance.GetFunc(rc.store, "_init")
	if initFunc == nil {
		return nil, fmt.Errorf("water: WASI module does not have an _init function")
	}
	_, err = initFunc.Call(rc.store)
	if err != nil {
		return nil, fmt.Errorf("water: _init function returned error: %w", err)
	}

	// check if the WASI module is single-threaded
	backgroundWorker := rc.instance.GetFunc(rc.store, "_background_worker")
	if backgroundWorker == nil {
		// single-threaded WASI module, set user_write_ready and user_will_read
		// bind instance functions
		wasiUserWriteReady := rc.instance.GetFunc(rc.store, "user_write_ready")
		if wasiUserWriteReady == nil {
			return nil, fmt.Errorf("water: WASI module does not have a user_write_ready function")
		}
		rc.userWriteReady = func(n int) error {
			_, err := wasiUserWriteReady.Call(rc.store, int32(n))
			return err
		}

		wasiUserWillRead := rc.instance.GetFunc(rc.store, "user_will_read")
		if wasiUserWillRead == nil {
			return nil, fmt.Errorf("water: WASI module does not have a user_will_read function")
		}
		rc.userWillRead = func() (int, error) {
			ret, err := wasiUserWillRead.Call(rc.store)
			if err != nil {
				return 0, err
			}
			return int(ret.(int32)), nil
		}
	} else {
		// spawn thread for background_worker
		go func() {
			_, err := backgroundWorker.Call(rc.store)
			if err != nil {
				panic(fmt.Errorf("water: _background_worker function returned error: %w", err))
			}
		}()
		rc.nonBlockingIO = true
	}

	return nil, nil
}

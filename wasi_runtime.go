package water

import (
	"fmt"
	"net"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/gaukas/water/internal/filesocket"
)

// WASIRuntime Status Flags
const (
	WASI_RUNTIME_NEW    uint8 = 0
	WASI_RUNTIME_LOADED uint8 = 1 << (iota - 1)
	WASI_RUNTIME_CONFIGURED
	WASI_RUNTIME_DIALER_SET
	WASI_RUNTIME_DIALER_SELECT_FUNC_DEFINED
	WASI_RUNTIME_CONN_BOUND
)

type WASIRuntime struct {
	bundle     filesocket.Bundle
	dialer     AddressedDialer
	wasiConfig *WASIConfig

	// status flags
	status uint8

	// wasmtime
	engine   *wasmtime.Engine
	module   *wasmtime.Module
	store    *wasmtime.Store
	linker   *wasmtime.Linker
	instance *wasmtime.Instance
}

func LoadWASI(wasi []byte) (runtime *WASIRuntime, err error) {
	runtime = &WASIRuntime{
		status: WASI_RUNTIME_NEW,
	}
	runtime.engine = wasmtime.NewEngine()
	runtime.module, err = wasmtime.NewModule(runtime.engine, wasi)
	if err != nil {
		return nil, fmt.Errorf("wasmtime.NewModule: %w", err)
	}

	runtime.store = wasmtime.NewStore(runtime.engine)

	runtime.linker = wasmtime.NewLinker(runtime.engine)
	err = runtime.linker.DefineWasi()
	if err != nil {
		return nil, fmt.Errorf("linker.DefineWasi: %w", err)
	}

	runtime.status |= WASI_RUNTIME_LOADED

	return
}

func (w *WASIRuntime) UseWASIConfig(config *WASIConfig) {
	if w.status < WASI_RUNTIME_LOADED {
		panic("wasm module not loaded")
	}

	w.wasiConfig = config
	w.store.SetWasi(w.wasiConfig.WasiConfig)

	// SetWasi():
	// The `wasi` argument cannot be reused for another `Store`, it's consumed by
	// this function.
	w.wasiConfig.WasiConfig = nil
	w.wasiConfig = nil

	w.status |= WASI_RUNTIME_CONFIGURED
}

// UseLink optionally sets the linker to use when instantiating the module.
func (w *WASIRuntime) UseLinker(linker *wasmtime.Linker) {
	w.linker = linker
}

// UseAddressedDialer sets the dialer to use when instantiating the module.
//
// Must be called
func (w *WASIRuntime) UseAddressedDialer(dialer AddressedDialer) {
	if w.status >= WASI_RUNTIME_DIALER_SELECT_FUNC_DEFINED {
		panic("dialer select function already defined")
	}

	w.dialer = dialer
	w.status |= WASI_RUNTIME_DIALER_SET
}

// DefineDialerSelectFunc defines the dialer select function in the WASI runtime.
// Must be called by the caller of the WASIRuntime after calling UseAddressedDialer().
//
// Must be called after UseLinker() if UseLinker() is ever called.
func (w *WASIRuntime) DefineDialerSelectFunc() error {
	if w.status >= WASI_RUNTIME_DIALER_SELECT_FUNC_DEFINED {
		return fmt.Errorf("dialer select function already defined")
	}

	if w.linker == nil {
		return fmt.Errorf("linker not set")
	}

	if w.dialer == nil || w.status < WASI_RUNTIME_DIALER_SET {
		return fmt.Errorf("dialer not set")
	}

	// TCP
	if err := w.linker.DefineFunc(w.store, "dialer", "dial_tcp", func() {
		conn, err := w.dialer.Dial("tcp")
		if err != nil {
			// TODO: error handling and notify wasi
		}
		// w.conn = conn
		err = w.BindConn(conn)
		if err != nil {
			// TODO: error handling and notify wasi
		}
	}); err != nil {
		return fmt.Errorf("linker.DefineFunc: %w", err)
	}

	// UDP
	if err := w.linker.DefineFunc(w.store, "dialer", "dial_udp", func() {
		conn, err := w.dialer.Dial("udp")
		if err != nil {
			// TODO: error handling and notify wasi
		}
		// w.conn = conn
		err = w.BindConn(conn)
		if err != nil {
			// TODO: error handling and notify wasi
		}
	}); err != nil {
		return fmt.Errorf("linker.DefineFunc: %w", err)
	}

	// TLS
	if err := w.linker.DefineFunc(w.store, "dialer", "dial_tls", func() {
		conn, err := w.dialer.Dial("tls")
		if err != nil {
			// TODO: error handling and notify wasi
		}
		// w.conn = conn
		err = w.BindConn(conn)
		if err != nil {
			// TODO: error handling and notify wasi
		}
	}); err != nil {
		return fmt.Errorf("linker.DefineFunc: %w", err)
	}

	w.status |= WASI_RUNTIME_DIALER_SELECT_FUNC_DEFINED

	return nil
}

// BindConn binds a net.Conn to the WASI runtime by forwarding between
// the net.Conn and the WASI runtime (currently using a FileSocket).
func (w *WASIRuntime) BindConn(c net.Conn) error {
	if w.status >= WASI_RUNTIME_DIALER_SELECT_FUNC_DEFINED {
		return fmt.Errorf("dialer select function already defined")
	}

	if w.bundle != nil {
		return fmt.Errorf("conn already bound")
	}

	if w.wasiConfig == nil || w.status&WASI_RUNTIME_CONFIGURED == 0 {
		return fmt.Errorf("wasi config not set")
	}

	if w.wasiConfig.fs == nil {
		return fmt.Errorf("wasi config file socket not set")
	}

	w.bundle = filesocket.BundleFileSocket(c, w.wasiConfig.fs)
	// optionally call w.bundle.OnClose() to be notified when the connection is closed
	// w.bundle.OnClose(func() {})
	w.bundle.Start()

	w.status |= WASI_RUNTIME_CONN_BOUND

	return nil
}

// Finalize finalizes the WASI runtime by instantiating the module.
func (w *WASIRuntime) Finalize() error {
	if w.status < WASI_RUNTIME_DIALER_SELECT_FUNC_DEFINED {
		return fmt.Errorf("dialer select function not defined")
	}

	if w.status >= WASI_RUNTIME_CONN_BOUND {
		return fmt.Errorf("conn already bound -- finalization already done")
	}

	// TODO: assign memory?

	// Instantiate the module with the linker.
	instance, err := w.linker.Instantiate(w.store, w.module)
	if err != nil {
		return fmt.Errorf("linker.Instantiate: %w", err)
	}

	init := instance.GetFunc(w.store, "_init")
	_, err = init.Call(w.store)
	if err != nil {
		return fmt.Errorf("init.Call: %w", err)
	}
	if w.status < WASI_RUNTIME_CONN_BOUND { // init() MUST set the conn
		return fmt.Errorf("conn not bound")
	}

	// TODO: export more functions to support Read() and Write()?

	w.instance = instance

	return nil
}

func (w *WASIRuntime) Read([]byte) (int, error) {
	if w.status < WASI_RUNTIME_CONN_BOUND {
		panic("conn not bound")
	}

	panic("not implemented")
}

func (w *WASIRuntime) Write([]byte) (int, error) {
	if w.status < WASI_RUNTIME_CONN_BOUND {
		panic("conn not bound")
	}

	panic("not implemented")
}

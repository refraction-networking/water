package water

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/gaukas/water/internal/filesocket"
)

const (
	RUNTIME_VERSION_MAJOR int32  = 0x001aaaaa
	RUNTIME_VERSION       string = "v0.1-alpha"
)

type RuntimeConnDialer struct {
	Config *Config

	debug bool
}

func (d *RuntimeConnDialer) DebugMode() {
	d.debug = true
}

func (d *RuntimeConnDialer) Dial(address string) (rc *RuntimeConn, err error) {
	return d.DialContext(context.Background(), address)
}

func (d *RuntimeConnDialer) DialContext(ctx context.Context, address string) (rc *RuntimeConn, err error) {
	if d.Config == nil {
		return nil, fmt.Errorf("water: dialing with nil config is prohibited")
	}
	d.Config.init()

	var wasiConfig *wasmtime.WasiConfig
	var ok bool
	if wasiConfig, ok = ctx.Value("wasi_config").(*wasmtime.WasiConfig); !ok {
		wasiConfig = wasmtime.NewWasiConfig()
	}

	if d.debug { // bind stdin/stdout/stderr to host
		wasiConfig.InheritStdin()
		wasiConfig.InheritStdout()
		wasiConfig.InheritStderr()
	}

	rc = new(RuntimeConn)
	if d.debug {
		rc.debug = true
	}
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
	if err = rc._version(); err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConn)._version returned error: %w", err)
	}

	// run the WASI init function
	if err = rc._init(); err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConn)._init returned error: %w", err)
	}

	// check if the WASI module is single-threaded
	if err = rc.finalize(); err != nil {
		return nil, fmt.Errorf("water: (*RuntimeConn).finalize returned error: %w", err)
	}

	return rc, nil
}

// RuntimeConn is a net.Conn-like type which runs a WASI module to handle
// one connection.
type RuntimeConn struct {
	userWriteDone func(byteLen int) (byteSuccess int, err error) // notify the WASI instance that it should read from user, process the data, and write to net
	userWillRead  func() (byteLen int, err error)                // notify the WASI instance that it should read from net, process the data, and write to user

	debug           bool
	nonBlockingIO   bool // true only if WASI has a blocking-forever loop used for handling I/O
	uFs, netFs      filesocket.FileSocket
	netBundle       filesocket.Bundle
	deferFunc       func() // to be called on Close()
	onCloseCallback func() // to be called as Close() returns

	// wasmtime
	engine   *wasmtime.Engine
	module   *wasmtime.Module
	store    *wasmtime.Store
	linker   *wasmtime.Linker
	instance *wasmtime.Instance
}

func Dial(address string, config *Config) (*RuntimeConn, error) {
	rd := &RuntimeConnDialer{
		Config: config,
	}
	return rd.DialContext(context.Background(), address)
}

func DialContext(ctx context.Context, address string, config *Config) (*RuntimeConn, error) {
	rd := &RuntimeConnDialer{
		Config: config,
	}

	return rd.DialContext(ctx, address)
}

func (rc *RuntimeConn) Write(p []byte) (n int, err error) {
	n, err = rc.uFs.Write(p)
	if err != nil {
		return 0, fmt.Errorf("water: failed to write to WASI module: %w", err)
	}

	// single-thread WASI module requires explicit call to write() funcs
	if !rc.nonBlockingIO {
		nSuccess, err := rc.userWriteDone(n)
		if err != nil {
			return 0, fmt.Errorf("water: failed to notify WASI module: %w", err)
		}
		if nSuccess == ErrIO {
			return 0, fmt.Errorf("water: WASI module encountered I/O error")
		}

		if nSuccess != n {
			return 0, fmt.Errorf("water: length written to WASI does not match expected")
		}
	}

	return
}

func (rc *RuntimeConn) Read(p []byte) (n int, err error) {
	// single-thread WASI module requires explicit call to read() funcs
	var nExpect int
	if !rc.nonBlockingIO {
		nExpect, err = rc.userWillRead()
		if err != nil {
			return 0, fmt.Errorf("water: failed to notify WASI module: %w", err)
		}

		if nExpect == ErrIO {
			return 0, fmt.Errorf("water: WASI module encountered I/O error")
		}

		if rc.debug {
			log.Printf("WASI module expects %d bytes", nExpect)
		}
	}

	n, err = rc.uFs.Read(p)
	if err != nil {
		return n, fmt.Errorf("water: failed to read from WASI buffer: %w", err)
	}

	// check if short-buffer. No data-loss in this case but still need to notify the caller.
	if nExpect > len(p) {
		err = io.ErrShortBuffer
	}

	return
}

func (rc *RuntimeConn) Close() error {
	if rc.deferFunc != nil {
		rc.deferFunc()
	}

	if rc.onCloseCallback != nil {
		defer rc.onCloseCallback()
	}

	if rc.uFs != nil {
		rc.uFs.Close()
	}

	if rc.netFs != nil {
		rc.netFs.Close()
	}

	if rc.netBundle != nil {
		rc.netBundle.Close()
	}

	return nil
}

// preopenSocketDir preopens a temporary directory on host for the WASI module to
// interact with sockets and creates 4 files in the directory:
// - uin (input from the user, read-only)
// - uout (output to the user, write-only)
// - netrx (net socket RX, read-only)
// - nettx (net socket TX, write-only)
func (rc *RuntimeConn) preopenSocketDir(wasiConfig *wasmtime.WasiConfig) error {
	tmpDir, err := os.MkdirTemp("", "water_*") // create a dir with randomized name under os.TempDir()
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	rc.deferFunc = func() { os.RemoveAll(tmpDir) } // remove the temporary directory when wasi expires

	// create the 4 files
	uin, err := os.Create(tmpDir + "/uin")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	uout, err := os.Create(tmpDir + "/uout")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	rc.uFs = filesocket.NewFileSocket(uout, uin) // user reads from uout, writes to uin to interact with the WASI module

	netrx, err := os.Create(tmpDir + "/netrx")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	nettx, err := os.Create(tmpDir + "/nettx")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	rc.netFs = filesocket.NewFileSocket(nettx, netrx) // Runtime reads from nettx, writes to netrx as it relays data to the net.Conn

	// preopen the temporary directory
	if rc.debug {
		log.Printf("preopening %s as /tmp", tmpDir)
		// show what's in the temporary directory
		dir, err := os.ReadDir(tmpDir)
		if err != nil {
			return fmt.Errorf("failed to read temporary directory: %w", err)
		}
		for _, entry := range dir {
			log.Printf("- %s", entry.Name())
		}
	}
	err = wasiConfig.PreopenDir(tmpDir, "/tmp")
	return err
}

func (rc *RuntimeConn) linkDialerFunc(dialer Dialer, address string) error {
	if rc.linker == nil {
		return fmt.Errorf("linker not set")
	}

	if dialer == nil {
		return fmt.Errorf("dialer not set")
	}

	var arrNetworks []string = []string{
		"tcp",
		"udp",
		"tls", // experimental
	}

	for _, network := range arrNetworks {
		err := func(network string) error {
			if err := rc.linker.DefineFunc(rc.store, "env", "connect_"+network, func() int32 {
				log.Printf("dialer.Dial(%s, %s)", network, address)
				conn, err := dialer.Dial(network, address)
				if err != nil {
					log.Printf("failed to dial %s: %v", address, err)
					return -1 // TODO: remove magic number
				}
				rc.makeNetBundle(conn)
				return 0 // TODO: remove magic number
			}); err != nil {
				return fmt.Errorf("(*wasmtime.Linker).DefineFunc: %w", err)
			}

			return nil
		}(network)

		if err != nil {
			return err
		}
	}

	return nil
}

func (rc *RuntimeConn) makeNetBundle(conn net.Conn) {
	rc.netBundle = filesocket.BundleFileSocket(conn, rc.netFs)
	// rc.netBundle.OnClose()
	rc.netBundle.Start()
}

func (rc *RuntimeConn) _version() error {
	// check the WASI version
	versionFunc := rc.instance.GetFunc(rc.store, "_version")
	if versionFunc == nil {
		return fmt.Errorf("loaded WASI module does not export  _version function")
	}
	version, err := versionFunc.Call(rc.store)
	if err != nil {
		return err
	}
	if version, ok := version.(int32); !ok {
		return fmt.Errorf("_version function returned non-int32 value")
	} else if version != RUNTIME_VERSION_MAJOR {
		return fmt.Errorf("WASI module version `v%d` is not compatible with runtime version `%s`!", version, RUNTIME_VERSION)
	}

	return nil
}

func (rc *RuntimeConn) _init() error {
	initFunc := rc.instance.GetFunc(rc.store, "_init")
	if initFunc == nil {
		return fmt.Errorf("loaded WASI module does not export _init function")
	}
	_, err := initFunc.Call(rc.store)
	if err != nil {
		return err
	}
	return nil
}

func (rc *RuntimeConn) finalize() error {
	backgroundWorker := rc.instance.GetFunc(rc.store, "_background_worker")
	runBackgroundWorker := rc.instance.GetFunc(rc.store, "_run_background_worker")
	if backgroundWorker == nil || runBackgroundWorker == nil {
		if rc.debug {
			log.Printf("registering callback functions")
		}
		// single-threaded WASI module, set user_write_ready and user_will_read
		// bind instance functions
		wasiUserWriteReady := rc.instance.GetFunc(rc.store, "_user_write_done")
		if wasiUserWriteReady == nil {
			return fmt.Errorf("loaded WASI module does not export either _user_write_ready or _background_worker function")
		}
		rc.userWriteDone = func(n int) (int, error) {
			ret, err := wasiUserWriteReady.Call(rc.store, int32(n))
			if err != nil {
				return 0, err
			}
			return int(ret.(int32)), nil
		}

		wasiUserWillRead := rc.instance.GetFunc(rc.store, "_user_will_read")
		if wasiUserWillRead == nil {
			return fmt.Errorf("loaded WASI module does not export either _user_will_read or _background_worker function")
		}
		rc.userWillRead = func() (int, error) {
			ret, err := wasiUserWillRead.Call(rc.store)
			if err != nil {
				return 0, err
			}
			return int(ret.(int32)), nil
		}
	} else {
		if rc.debug {
			log.Printf("spawning background workers")
		}
		// call _background_worker to get the number of background workers needed
		bgWorkerNum, err := backgroundWorker.Call(rc.store)
		if err != nil {
			return fmt.Errorf("errored upon calling _background_worker function: %w", err)
		}
		if bgWorkerNum, ok := bgWorkerNum.(int32); !ok {
			return fmt.Errorf("_background_worker function returned non-int32 value")
		} else {
			var i int32
			for i = 0; i < bgWorkerNum; i++ {
				// spawn thread for background_worker
				go func(tid int32) {
					_, err := runBackgroundWorker.Call(rc.store, tid)
					if err != nil {
						panic(fmt.Errorf("errored upon calling _run_background_worker function: %w", err))
					}
				}(i)
			}
		}
		rc.nonBlockingIO = true
	}

	return nil
}

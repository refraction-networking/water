package water

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/gaukas/water/internal/filesocket"
)

const (
	RUNTIME_VERSION_MAJOR int32  = 0x001aaaaa
	RUNTIME_VERSION       string = "v0.0.1alpha"
)

// RuntimeConn is a net.Conn-like type which runs a WASI module to handle
// one connection.
type RuntimeConn struct {
	userWriteReady func(byteLen int) error         // notify the WASI instance that it should read from user, process the data, and write to net
	userWillRead   func() (byteLen int, err error) // notify the WASI instance that it should read from net, process the data, and write to user

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
	rd := &RuntimeDialer{
		Config: config,
	}
	return rd.DialContext(context.Background(), address)
}

func DialContext(ctx context.Context, address string, config *Config) (*RuntimeConn, error) {
	rd := &RuntimeDialer{
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
		rc.userWriteReady(n)
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
	}

	n, err = rc.uFs.Read(p)
	if err != nil {
		return 0, fmt.Errorf("water: failed to read from WASI buffer: %w", err)
	}
	if nExpect > 0 && nExpect != n {
		return 0, fmt.Errorf("water: length read from WASI does not match expected: %w", err)
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
	uin, err := os.CreateTemp(tmpDir, "uin")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	uout, err := os.CreateTemp(tmpDir, "uout")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	rc.uFs = filesocket.NewFileSocket(uout, uin) // user reads from uout, writes to uin to interact with the WASI module

	netrx, err := os.CreateTemp(tmpDir, "netrx")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	nettx, err := os.CreateTemp(tmpDir, "nettx")
	if err != nil {
		rc.deferFunc()
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	rc.netFs = filesocket.NewFileSocket(nettx, netrx) // Runtime reads from nettx, writes to netrx as it relays data to the net.Conn

	// preopen the temporary directory
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

	var mapNetworkDialerFunc map[string]string = map[string]string{
		"tcp": "dial_tcp",
		"udp": "dial_udp",
		"tls": "dial_tls", // experimental
	}

	for network, wasiFunc := range mapNetworkDialerFunc {
		if err := rc.linker.DefineFunc(rc.store, "dialer", wasiFunc, func() int32 {
			conn, err := dialer.Dial(network, address)
			if err != nil {
				return -1 // TODO: remove magic number
			}
			err = rc.makeNetBundle(conn)
			if err != nil {
				return -2 // TODO: remove magic number
			}
			return 0 // TODO: remove magic number
		}); err != nil {
			return fmt.Errorf("(*wasmtime.Linker).DefineFunc: %w", err)
		}
	}

	return nil
}

func (rc *RuntimeConn) makeNetBundle(conn net.Conn) error {
	rc.netBundle = filesocket.BundleFileSocket(conn, rc.netFs)
	// rc.netBundle.OnClose()
	rc.netBundle.Start()

	return nil
}

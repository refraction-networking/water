package water_test

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/refraction-networking/water"
	_ "github.com/refraction-networking/water/transport/v1"
)

// ExampleRelay demonstrates how to use water.Relay.
//
// This example is expected to demonstrate how to use the LATEST version of
// W.A.T.E.R. API, while other older examples could be found under transport/vX,
// where X is the version number (e.g. v0, v1, etc.).
//
// It is worth noting that unless the W.A.T.E.R. API changes, the version upgrade
// does not bring any essential changes to this example other than the import
// path and wasm file path.
func ExampleRelay() {
	// Relay destination: a local TCP server
	tcpListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close() // skipcq: GO-S2307

	config := &water.Config{
		TransportModuleBin:  wasmReverse,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}

	waterRelay, err := water.NewRelayWithContext(context.Background(), config)
	if err != nil {
		panic(err)
	}
	defer waterRelay.Close() // skipcq: GO-S2307

	// in a goroutine, start relay
	go func() {
		err := waterRelay.ListenAndRelayTo("tcp", "localhost:0", "tcp", tcpListener.Addr().String())
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(100 * time.Millisecond) // 100ms to spin up relay

	// test source: a local TCP client
	clientConn, err := net.Dial("tcp", waterRelay.Addr().String())
	if err != nil {
		panic(err)
	}
	defer clientConn.Close() // skipcq: GO-S2307

	serverConn, err := tcpListener.Accept()
	if err != nil {
		panic(err)
	}
	defer serverConn.Close() // skipcq: GO-S2307

	var msg = []byte("hello")
	n, err := clientConn.Write(msg)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic("short write")
	}

	buf := make([]byte, 1024)
	n, err = serverConn.Read(buf)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic("short read")
	}

	if err := waterRelay.Close(); err != nil {
		panic(err)
	}

	fmt.Println(string(buf[:n]))
	// Output: olleh
}

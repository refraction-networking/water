package water_test

import (
	"context"
	"fmt"
	"net"

	"github.com/refraction-networking/water"
	_ "github.com/refraction-networking/water/transport/v1"
)

// ExampleDialer demonstrates how to use water.Dialer.
//
// This example is expected to demonstrate how to use the LATEST version of
// W.A.T.E.R. API, while other older examples could be found under transport/vX,
// where X is the version number (e.g. v0, v1, etc.).
//
// It is worth noting that unless the W.A.T.E.R. API changes, the version upgrade
// does not bring any essential changes to this example other than the import
// path and wasm file path.
// ExampleFixedDialer demonstrates how to use v1.FixedDialer as a water.Dialer.
func ExampleFixedDialer() {
	config := &water.Config{
		TransportModuleBin:  wasmReverse,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
		DialedAddressValidator: func(network, address string) error {
			if network != "tcp" || address != "localhost:7700" {
				return fmt.Errorf("invalid address: %s", address)
			}
			return nil
		},
	}

	waterDialer, err := water.NewFixedDialerWithContext(context.Background(), config)
	if err != nil {
		panic(err)
	}

	// create a local TCP listener
	tcpListener, err := net.Listen("tcp", "localhost:7700")
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close() // skipcq: GO-S2307

	waterConn, err := waterDialer.DialFixedContext(context.Background())
	if err != nil {
		panic(err)
	}
	defer waterConn.Close() // skipcq: GO-S2307

	tcpConn, err := tcpListener.Accept()
	if err != nil {
		panic(err)
	}
	defer tcpConn.Close() // skipcq: GO-S2307

	var msg = []byte("hello")
	n, err := waterConn.Write(msg)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic("short write")
	}

	buf := make([]byte, 1024)
	n, err = tcpConn.Read(buf)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic("short read")
	}

	if err := waterConn.Close(); err != nil {
		panic(err)
	}

	fmt.Println(string(buf[:n]))
	// Output: olleh
}

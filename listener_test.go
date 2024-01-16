package water_test

import (
	"fmt"
	"net"

	"github.com/gaukas/water"
	_ "github.com/gaukas/water/transport/v0"
)

// ExampleListener demonstrates how to use water.Listener.
//
// This example is expected to demonstrate how to use the LATEST version of
// W.A.T.E.R. API, while other older examples could be found under transport/vX,
// where X is the version number (e.g. v0, v1, etc.).
//
// It is worth noting that unless the W.A.T.E.R. API changes, the version upgrade
// does not bring any essential changes to this example other than the import
// path and wasm file path.
func ExampleListener() {
	config := &water.Config{
		TransportModuleBin: wasmReverse,
	}

	waterListener, err := config.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	defer waterListener.Close() // skipcq: GO-S2307

	tcpConn, err := net.Dial("tcp", waterListener.Addr().String())
	if err != nil {
		panic(err)
	}
	defer tcpConn.Close() // skipcq: GO-S2307

	waterConn, err := waterListener.Accept()
	if err != nil {
		panic(err)
	}
	defer waterConn.Close() // skipcq: GO-S2307

	var msg = []byte("hello")
	n, err := tcpConn.Write(msg)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic(err)
	}

	buf := make([]byte, 1024)
	n, err = waterConn.Read(buf)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic(err)
	}

	fmt.Println(string(buf[:n]))
	// Output: olleh
}

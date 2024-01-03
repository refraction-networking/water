package water_test

import (
	"fmt"
	"net"
	"os"

	"github.com/gaukas/water"
	_ "github.com/gaukas/water/transport/v0"
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
func ExampleDialer() {
	// reverse.wasm reverses the message on read/write, bidirectionally.
	wasm, err := os.ReadFile("./testdata/v0/reverse.wasm")
	if err != nil {
		panic(fmt.Sprintf("failed to read wasm file: %v", err))
	}

	config := &water.Config{
		TransportModuleBin: wasm,
	}

	waterDialer, err := water.NewDialer(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create dialer: %v", err))
	}

	// create a local TCP listener
	tcpListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}
	defer tcpListener.Close()

	// start a goroutine to accept connections from the local TCP listener
	go func() {
		tcpConn, err := tcpListener.Accept()
		if err != nil {
			panic(fmt.Sprintf("failed to accept: %v", err))
		}

		// start a goroutine to handle the connection
		go func(tcpConn net.Conn) {
			// echo everything back
			defer tcpConn.Close()
			buf := make([]byte, 1024)
			for {
				n, err := tcpConn.Read(buf)
				if err != nil {
					return
				}

				if string(buf[:n]) != "olleh" {
					panic(fmt.Sprintf("unexpected message: %s", string(buf[:n])))
				}

				_, err = tcpConn.Write([]byte("hello"))
				if err != nil {
					return
				}
			}
		}(tcpConn)
	}()

	waterConn, err := waterDialer.Dial("tcp", tcpListener.Addr().String())
	if err != nil {
		panic(fmt.Sprintf("failed to dial: %v", err))
	}
	defer waterConn.Close()

	var msg = []byte("hello")
	n, err := waterConn.Write(msg)
	if err != nil {
		panic(fmt.Sprintf("failed to write: %v", err))
	}
	if n != len(msg) {
		panic(fmt.Sprintf("failed to write: %v", err))
	}

	buf := make([]byte, 1024)
	n, err = waterConn.Read(buf)
	if err != nil {
		panic(fmt.Sprintf("failed to read: %v", err))
	}
	if n != len(msg) {
		panic(fmt.Sprintf("failed to read: %v", err))
	}

	fmt.Println(string(buf[:n]))
	// Output: olleh
}

//go:build benchtcpref

package v0_test

import (
	"net"
	"sync"
	"testing"
)

// BenchmarkTCPReference can be used as a reference to compare to the
// benchmark result of Dialer/Listener/Relay.
//
// Separate benchmark for the latency measurement will be needed.
func BenchmarkTCPReference(b *testing.B) {
	testLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer testLis.Close()

	// goroutine to accept incoming connections
	var lConn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		lConn, goroutineErr = testLis.Accept()
	}()

	// Dial with net.Dial
	dConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer dConn.Close()

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}
	defer lConn.Close()

	benchmarkUnidirectionalStream(b, dConn, lConn)
}

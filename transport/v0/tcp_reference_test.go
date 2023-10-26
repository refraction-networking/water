//go:build benchtcpref

package v0_test

import (
	"crypto/rand"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"
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

	var sendMsg []byte = make([]byte, 1024)
	_, err = rand.Read(sendMsg)
	if err != nil {
		b.Fatalf("rand.Read error: %s", err)
	}

	// setup a goroutine to read from the conn
	var wg2 *sync.WaitGroup = new(sync.WaitGroup)
	var listenerRecvErr error
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		recvBytes := 0
		var n int
		recvbuf := make([]byte, 1024+1) //
		for recvBytes < b.N*1024 {
			n, listenerRecvErr = lConn.Read(recvbuf)
			recvBytes += n
			if listenerRecvErr != nil {
				return
			}
		}
	}()

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	b.SetBytes(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = dConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}
	}
	wg2.Wait()
	b.StopTimer()

	if listenerRecvErr != nil {
		b.Fatal(listenerRecvErr)
	}
}

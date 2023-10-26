package v0_test

import (
	"bytes"
	"crypto/rand"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gaukas/water"
	v0 "github.com/gaukas/water/transport/v0"
)

func TestListener(t *testing.T) {
	loadPlain()
	t.Run("plain must work", testListenerPlain)
	t.Run("bad addr must fail", testListenerBadAddr)
	t.Run("partial WATM must fail", testListenerPartialWATM)
}

func testListenerBadAddr(t *testing.T) { // skipcq: GO-R1005
	// prepare
	config := &water.Config{
		TMBin: plain,
	}

	_, err := config.Listen("tcp", "256.267.278.289:2023")
	if err == nil {
		t.Fatal("config.Listen should fail on bad address")
	}

	// trigger garbage collection
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
}

func testListenerPlain(t *testing.T) {
	// prepare
	config := &water.Config{
		TMBin: plain,
	}

	testLis, err := config.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer testLis.Close()

	// goroutine to accept incoming connections
	var conn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, goroutineErr = testLis.Accept()
	}()

	// Dial with net.Dial
	peerConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close()

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		t.Fatal(goroutineErr)
	}
	defer conn.Close()

	// type assertion: conn must be *v0.Conn
	if _, ok := conn.(*v0.Conn); !ok {
		t.Fatalf("conn is not *v0.Conn")
	}

	// trigger garbage collection for several times to simulate any
	// possible GC in the real world use case
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	var peerSendBuf []byte = make([]byte, 1024)
	var waterSendBuf []byte = make([]byte, 1024)
	var peerRecvBuf []byte = make([]byte, 1024)
	var waterRecvBuf []byte = make([]byte, 1024)
	// send 10 messages in each direction
	for i := 0; i < 10; i++ {
		_, err = rand.Read(peerSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		_, err = rand.Read(waterSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		// water -> peer
		_, err = conn.Write(waterSendBuf)
		if err != nil {
			t.Fatalf("conn.Write error: %s", err)
		}

		n, err := peerConn.Read(peerRecvBuf)
		if err != nil {
			t.Fatalf("lisConn.Read error: %s", err)
		}

		if n != len(waterSendBuf) {
			t.Fatalf("lisConn.Read error: read %d bytes, want %d bytes", n, len(waterSendBuf))
		}

		if !bytes.Equal(peerRecvBuf[:n], waterSendBuf) {
			t.Fatalf("peerRecvBuf != waterSendBuf")
		}

		// peer -> water
		_, err = peerConn.Write(peerSendBuf)
		if err != nil {
			t.Fatalf("lisConn.Write error: %s", err)
		}

		n, err = conn.Read(waterRecvBuf)
		if err != nil {
			t.Fatalf("conn.Read error: %s", err)
		}

		if n != len(peerSendBuf) {
			t.Fatalf("conn.Read error: read %d bytes, want %d bytes", n, len(peerSendBuf))
		}

		if !bytes.Equal(waterRecvBuf[:n], peerSendBuf) {
			t.Fatalf("waterRecvBuf != peerSendBuf")
		}

		// trigger garbage collection
		runtime.GC()
		time.Sleep(100 * time.Microsecond)
	}

	// reading with a deadline
	conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn.Read(waterRecvBuf)
	if err == nil {
		t.Fatalf("conn.Read must timeout")
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	// after closing the conn, read/write MUST fail
	_, err = conn.Write(waterSendBuf)
	if err == nil {
		t.Fatalf("conn.Write must fail after closing the conn")
	}

	_, err = conn.Read(waterRecvBuf)
	if err == nil {
		t.Fatalf("conn.Read must fail after closing the conn")
	}

	// trigger garbage collection
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
}

func testListenerPartialWATM(t *testing.T) {
	t.Skip() // TODO: implement this with a few WebAssembly Transport Modules which partially implement the v0 listener spec
}

// BenchmarkListenerInbound currently measures only the inbound throughput
// of the listener. Outbound throughput is not measured at the moment.
//
// Separate benchmark for the latency measurement will be needed.
func BenchmarkListenerInbound(b *testing.B) {
	loadPlain()
	// prepare
	config := &water.Config{
		TMBin: plain,
	}

	testLis, err := config.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer testLis.Close()

	// goroutine to accept incoming connections
	var conn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, goroutineErr = testLis.Accept()
	}()

	// Dial with net.Dial
	peerConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer peerConn.Close()

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}
	defer conn.Close()

	var sendMsg []byte = make([]byte, 1024)
	_, err = rand.Read(sendMsg)
	if err != nil {
		b.Fatalf("rand.Read error: %s", err)
	}

	// setup a goroutine to read from the conn
	var wg2 *sync.WaitGroup = new(sync.WaitGroup)
	var waterRecvErr error
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		recvBytes := 0
		var n int
		recvbuf := make([]byte, 1024+1) //
		for recvBytes < b.N*1024 {
			n, waterRecvErr = conn.Read(recvbuf)
			recvBytes += n
			if waterRecvErr != nil {
				return
			}
		}
	}()

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	b.SetBytes(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = peerConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}
	}
	wg2.Wait()
	b.StopTimer()

	if waterRecvErr != nil {
		b.Fatal(waterRecvErr)
	}
}

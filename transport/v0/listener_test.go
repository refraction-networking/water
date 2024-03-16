package v0_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/refraction-networking/water"
	v0 "github.com/refraction-networking/water/transport/v0"
)

// ExampleListener demonstrates how to use v0.Listener as a water.Listener.
func ExampleListener() {
	wrappedTcpListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}

	// start using W.A.T.E.R. API below this line, have fun!
	config := &water.Config{
		TransportModuleBin:  wasmReverse,
		NetworkListener:     wrappedTcpListener,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}

	waterListener, err := v0.NewListenerWithContext(context.Background(), config)
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
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
		panic("short write")
	}

	buf := make([]byte, 1024)
	n, err = waterConn.Read(buf)
	if err != nil {
		panic(err)
	}
	if n != len(msg) {
		panic("short read")
	}

	fmt.Println(string(buf[:n]))
	// Output: olleh
}

// TestListener covers the following cases:
//  1. Listener must work with a plain WebAssembly Transport Module that
//     doesn't transform the message.
//  2. Listener must work with a WebAssembly Transport Module that
//     transforms the message by reversing it.
//  3. Listener must fail when an invalid address is supplied.
//  4. Listener must fail when a WebAssembly Transport Module does not
//     fully implement the v0 listener spec.
func TestListener(t *testing.T) {
	t.Run("plain must work", testListenerPlain)
	t.Run("reverse must work", testListenerReverse)
	t.Run("bad addr must fail", testListenerBadAddr)
	t.Run("partial WATM must fail", testListenerPartialWATM)
}

func testListenerBadAddr(t *testing.T) {
	// prepare
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}

	_, err := config.ListenContext(context.Background(), "tcp", "256.267.278.289:2023")
	if err == nil {
		t.Fatal("config.ListenContext should fail on bad address")
	}
}

func testListenerPlain(t *testing.T) { // skipcq: GO-R1005
	// prepare
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}

	testLis, err := config.ListenContext(context.Background(), "tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer testLis.Close() // skipcq: GO-S2307

	// Dial with net.Dial
	peerConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	conn, err := testLis.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v0.Conn
	if _, ok := conn.(*v0.Conn); !ok {
		t.Fatalf("conn is not *v0.Conn")
	}

	tripleGC(100 * time.Microsecond)

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

		tripleGC(100 * time.Microsecond)
	}

	// reading with a deadline
	err = conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

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

	if err = testLis.Close(); err != nil {
		t.Fatal(err)
	}

	tripleGC(100 * time.Microsecond)
}

func testListenerReverse(t *testing.T) { // skipcq: GO-R1005
	// prepare
	config := &water.Config{
		TransportModuleBin: wasmReverse,
	}

	testLis, err := config.ListenContext(context.Background(), "tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer testLis.Close() // skipcq: GO-S2307

	// Dial with net.Dial
	peerConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	conn, err := testLis.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v0.Conn
	if _, ok := conn.(*v0.Conn); !ok {
		t.Fatalf("conn is not *v0.Conn")
	}

	tripleGC(100 * time.Microsecond)

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

		// reverse the waterSendBuf
		for i := 0; i < len(waterSendBuf)/2; i++ {
			waterSendBuf[i], waterSendBuf[len(waterSendBuf)-1-i] = waterSendBuf[len(waterSendBuf)-1-i], waterSendBuf[i]
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

		// reverse the peerSendBuf
		for i := 0; i < len(peerSendBuf)/2; i++ {
			peerSendBuf[i], peerSendBuf[len(peerSendBuf)-1-i] = peerSendBuf[len(peerSendBuf)-1-i], peerSendBuf[i]
		}

		if !bytes.Equal(waterRecvBuf[:n], peerSendBuf) {
			t.Fatalf("waterRecvBuf != peerSendBuf")
		}

		tripleGC(100 * time.Microsecond)
	}

	// reading with a deadline
	err = conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

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

	if err = testLis.Close(); err != nil {
		t.Fatal(err)
	}

	tripleGC(100 * time.Microsecond)
}

func testListenerPartialWATM(t *testing.T) {
	t.Skip("skipping [testListenerPartialWATM]...") // TODO: implement this with a few WebAssembly Transport Modules which partially implement the v0 listener spec
}

// BenchmarkInboundListener currently measures only the inbound throughput
// of the listener. Outbound throughput is measured for the dialer instead.
//
// Separate benchmark for the latency measurement will be needed.
func BenchmarkInboundListener(b *testing.B) {
	// prepare
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}

	testLis, err := config.ListenContext(context.Background(), "tcp", "localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer testLis.Close() // skipcq: GO-S2307

	// Dial with net.Dial
	peerConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	waterConn, err := testLis.Accept()
	if err != nil {
		b.Fatal(err)
	}
	defer waterConn.Close() // skipcq: GO-S2307

	err = sanityCheckConn(peerConn, waterConn, []byte("hello"), []byte("hello"))
	if err != nil {
		b.Fatal(err)
	}
	benchmarkUnidirectionalStream(b, peerConn, waterConn)

	if err = waterConn.Close(); err != nil {
		b.Fatal(err)
	}

	if err = testLis.Close(); err != nil {
		b.Fatal(err)
	}
}

// BenchmarkInboundListenerReverse currently measures only the inbound throughput
// of the listener. Outbound throughput is measured for the dialer instead.
//
// Different from BenchmarkInboundListener, this benchmark uses the reverse
// WebAssembly Transport Module, which reverse the bytes of each message before
// sending it to the peer.
//
// Separate benchmark for the latency measurement will be needed.
func BenchmarkInboundListenerReverse(b *testing.B) {
	// prepare
	config := &water.Config{
		TransportModuleBin: wasmReverse,
	}

	testLis, err := config.ListenContext(context.Background(), "tcp", "localhost:0")
	if err != nil {
		b.Fatal(err)
	}

	// Dial with net.Dial
	peerConn, err := net.Dial("tcp", testLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	waterConn, err := testLis.Accept()
	if err != nil {
		b.Fatal(err)
	}

	err = sanityCheckConn(peerConn, waterConn, []byte("hello"), []byte("olleh"))
	if err != nil {
		b.Fatal(err)
	}
	benchmarkUnidirectionalStream(b, peerConn, waterConn)

	if err = waterConn.Close(); err != nil {
		b.Fatal(err)
	}

	if err = testLis.Close(); err != nil {
		b.Fatal(err)
	}
}

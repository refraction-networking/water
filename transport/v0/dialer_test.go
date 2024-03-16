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

// ExampleDialer demonstrates how to use v0.Dialer as a water.Dialer.
func ExampleDialer() {
	config := &water.Config{
		TransportModuleBin:  wasmReverse,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}

	waterDialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		panic(err)
	}

	// create a local TCP listener
	tcpListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close() // skipcq: GO-S2307

	waterConn, err := waterDialer.DialContext(context.Background(), "tcp", tcpListener.Addr().String())
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

	fmt.Println(string(buf[:n]))
	// Output: olleh
}

// TestDialer covers the following cases:
//  1. Dialer must work with a plain WebAssembly Transport Module that
//     doesn't transform the message.
//  2. Dialer must work with a WebAssembly Transport Module that
//     transforms the message by reversing it.
//  3. Dialer must fail when an invalid address is supplied.
//  4. Dialer must fail when a WebAssembly Transport Module does not
//     fully implement the v0 dialer spec.
func TestDialer(t *testing.T) {
	t.Run("plain must work", testDialerPlain)
	t.Run("reverse must work", testDialerReverse)
	t.Run("bad addr must fail", testDialerBadAddr)
	t.Run("partial WATM must fail", testDialerPartialWATM)
}

func testDialerBadAddr(t *testing.T) {
	// Dial
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}

	dialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dialer.DialContext(context.Background(), "tcp", "256.267.278.289:2023")
	if err == nil {
		t.Fatal("dialer.Dial should fail")
	}
}

func testDialerPlain(t *testing.T) { // skipcq: GO-R1005
	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}
	dialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v0.Conn
	if _, ok := conn.(*v0.Conn); !ok {
		t.Fatalf("returned conn is not *v0.Conn")
	}

	peerConn, err := tcpLis.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	tripleGC(100 * time.Microsecond)

	var waterSendBuf []byte = make([]byte, 1024)
	var peerSendBuf []byte = make([]byte, 1024)
	var waterRecvBuf []byte = make([]byte, 1024)
	var peerRecvBuf []byte = make([]byte, 1024)
	// send 10 messages in each direction
	for i := 0; i < 10; i++ {
		_, err = rand.Read(waterSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		_, err = rand.Read(peerSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		// dialer -> listener
		_, err = conn.Write(waterSendBuf)
		if err != nil {
			t.Fatalf("conn.Write error: %s", err)
		}

		n, err := peerConn.Read(peerRecvBuf)
		if err != nil {
			t.Fatalf("peerConn.Read error: %s", err)
		}

		if n != len(waterSendBuf) {
			t.Fatalf("peerConn.Read error: read %d bytes, want %d bytes", n, len(waterSendBuf))
		}

		if !bytes.Equal(peerRecvBuf[:n], waterSendBuf) {
			t.Fatalf("peerRecvBuf != waterSendBuf")
		}

		// listener -> dialer
		_, err = peerConn.Write(peerSendBuf)
		if err != nil {
			t.Fatalf("peerConn.Write error: %s", err)
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

	if err := tcpLis.Close(); err != nil {
		t.Fatal(err)
	}

	tripleGC(100 * time.Microsecond)
}

func testDialerReverse(t *testing.T) { // skipcq: GO-R1005
	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin: wasmReverse,
	}
	dialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v0.Conn
	if _, ok := conn.(*v0.Conn); !ok {
		t.Fatalf("returned conn is not *v0.Conn")
	}

	peerConn, err := tcpLis.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	tripleGC(100 * time.Microsecond)

	var waterSendBuf []byte = make([]byte, 1024)
	var peerSendBuf []byte = make([]byte, 1024)
	var waterRecvBuf []byte = make([]byte, 1024)
	var peerRecvBuf []byte = make([]byte, 1024)
	// send 10 messages in each direction
	for i := 0; i < 10; i++ {
		_, err = rand.Read(waterSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		_, err = rand.Read(peerSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		// dialer -> listener
		_, err = conn.Write(waterSendBuf)
		if err != nil {
			t.Fatalf("conn.Write error: %s", err)
		}

		n, err := peerConn.Read(peerRecvBuf)
		if err != nil {
			t.Fatalf("peerConn.Read error: %s", err)
		}

		if n != len(waterSendBuf) {
			t.Fatalf("peerConn.Read error: read %d bytes, want %d bytes", n, len(waterSendBuf))
		}

		// reverse the waterSendBuf
		for i := 0; i < len(waterSendBuf)/2; i++ {
			waterSendBuf[i], waterSendBuf[len(waterSendBuf)-1-i] = waterSendBuf[len(waterSendBuf)-1-i], waterSendBuf[i]
		}

		if !bytes.Equal(peerRecvBuf[:n], waterSendBuf) {
			t.Fatalf("peerRecvBuf != waterSendBuf")
		}

		// listener -> dialer
		_, err = peerConn.Write(peerSendBuf)
		if err != nil {
			t.Fatalf("peerConn.Write error: %s", err)
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

	if err := tcpLis.Close(); err != nil {
		t.Fatal(err)
	}

	tripleGC(100 * time.Microsecond)
}

func testDialerPartialWATM(t *testing.T) {
	t.Skip("skipping [testDialerPartialWATM]...") // TODO: implement this with a few WebAssembly Transport Modules which partially implement the v0 dialer spec
}

// BenchmarkDialerOutbound currently measures only the outbound throughput
// of the dialer. Inbound throughput is measured for the listener instead.
//
// Separate benchmark for the latency measurement will be needed.
func BenchmarkDialerOutbound(b *testing.B) {
	// create random TCP listener listening on localhost
	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}
	dialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		b.Fatal(err)
	}

	waterConn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer waterConn.Close() // skipcq: GO-S2307

	peerConn, err := tcpLis.Accept()
	if err != nil {
		b.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	err = sanityCheckConn(peerConn, waterConn, []byte("hello"), []byte("hello"))
	if err != nil {
		b.Fatal(err)
	}

	benchmarkUnidirectionalStream(b, waterConn, peerConn)

	if err = waterConn.Close(); err != nil {
		b.Fatal(err)
	}

	if err = tcpLis.Close(); err != nil {
		b.Fatal(err)
	}
}

// BenchmarkDialerOutboundReverse currently measures only the outbound throughput
// of the dialer. Inbound throughput is measured for the listener instead.
//
// Different from BenchmarkDialerOutbound, this benchmark uses the reverse
// WebAssembly Transport Module, which reverse the bytes of each message before
// sending it to the peer.
//
// Separate benchmark for the latency measurement will be needed.
func BenchmarkDialerOutboundReverse(b *testing.B) {
	// create random TCP listener listening on localhost
	tcpLis, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP: net.ParseIP("127.0.0.1"),
		// Port: 0,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial
	config := &water.Config{
		TransportModuleBin: wasmReverse,
	}
	dialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		b.Fatal(err)
	}

	waterConn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer waterConn.Close() // skipcq: GO-S2307

	peerConn, err := tcpLis.Accept()
	if err != nil {
		b.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	err = sanityCheckConn(peerConn, waterConn, []byte("hello"), []byte("olleh"))
	if err != nil {
		b.Fatal(err)
	}

	benchmarkUnidirectionalStream(b, waterConn, peerConn)

	if err = waterConn.Close(); err != nil {
		b.Fatal(err)
	}

	if err = tcpLis.Close(); err != nil {
		b.Fatal(err)
	}
}

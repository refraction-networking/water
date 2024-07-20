package v1_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/refraction-networking/water"
	v1 "github.com/refraction-networking/water/transport/v1"
)

// ExampleDialer demonstrates how to use v1.Dialer as a water.Dialer.
func ExampleDialer() {
	config := &water.Config{
		TransportModuleBin:  wasmReverse,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}

	waterDialer, err := v1.NewDialerWithContext(context.Background(), config)
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

	if err := waterConn.Close(); err != nil {
		panic(err)
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
//     fully implement the v1 dialer spec.
func TestDialer(t *testing.T) {
	t.Run("PlainTransport", testDialer_Plain)
	t.Run("ReverseTransport", testDialer_Reverse)
	t.Run("DialBadAddress", testDialer_BadAddr)
	t.Run("ContextExpireAfterConnCreation", testDialer_ContextExpireAfterConnCreation)
}

func testDialer_Plain(t *testing.T) { // skipcq: GO-R1005
	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin:  wasmPlain,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}
	dialer, err := v1.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v1.Conn
	if _, ok := conn.(*v1.Conn); !ok {
		t.Fatalf("returned conn is not *v1.Conn")
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

func testDialer_Reverse(t *testing.T) { // skipcq: GO-R1005
	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin:  wasmReverse,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}
	dialer, err := v1.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v1.Conn
	if _, ok := conn.(*v1.Conn); !ok {
		t.Fatalf("returned conn is not *v1.Conn")
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

func testDialer_BadAddr(t *testing.T) {
	// Dial
	config := &water.Config{
		TransportModuleBin:  wasmPlain,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}

	dialer, err := v1.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dialer.DialContext(context.Background(), "tcp", "256.267.278.289:2023")
	if err == nil {
		t.Fatal("dialer.Dial should fail")
	}
}

func testDialer_ContextExpireAfterConnCreation(t *testing.T) {
	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin:   wasmPlain,
		ModuleConfigFactory:  water.NewWazeroModuleConfigFactory(),
		RuntimeConfigFactory: water.NewWazeroRuntimeConfigFactory(),
	}
	config.RuntimeConfigFactory.SetCloseOnContextDone(true)

	coreCtx, coreCtxCancel := context.WithCancel(context.Background())

	dialer, err := v1.NewDialerWithContext(coreCtx, config)
	if err != nil {
		t.Fatal(err)
	}

	dialCtx, dialCtxCancel := context.WithCancel(context.Background())

	conn, err := dialer.DialContext(dialCtx, "tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v1.Conn
	if _, ok := conn.(*v1.Conn); !ok {
		t.Fatalf("returned conn is not *v1.Conn")
	}

	peerConn, err := tcpLis.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close() // skipcq: GO-S2307

	tripleGC(100 * time.Microsecond)

	var wrBuf []byte = make([]byte, 128)
	var rdBuf []byte = make([]byte, 128)

	// send a message, it must be received by the peer
	n, err := rand.Read(wrBuf)
	if err != nil {
		t.Fatalf("rand.Read error: %s", err)
	}

	nWr, err := conn.Write(wrBuf)
	if err != nil {
		t.Fatalf("conn.Write error: %s", err)
	}

	if nWr != n {
		t.Fatalf("conn.Write error: wrote %d bytes, want %d bytes", nWr, n)
	}

	nRd, err := peerConn.Read(rdBuf)
	if err != nil {
		t.Fatalf("peerConn.Read error: %s", err)
	}

	if nRd != n {
		t.Fatalf("peerConn.Read error: read %d bytes, want %d bytes", nRd, n)
	}

	if !bytes.Equal(wrBuf[:n], rdBuf[:n]) {
		t.Fatalf("wrBuf != rdBuf")
	}

	// cancel the dial context
	dialCtxCancel()

	// now the conn must be still alive
	nWr, err = peerConn.Write(wrBuf)
	if err != nil {
		t.Fatalf("peerConn.Write error: %v", err)
	}

	nRd, err = conn.Read(rdBuf)
	if err != nil {
		t.Fatalf("conn.Read error: %v", err)
	}

	if nWr != nRd {
		t.Fatalf("conn.Read error: read %d bytes, want %d bytes", nRd, nWr)
	}

	if !bytes.Equal(wrBuf[:nWr], rdBuf[:nRd]) {
		t.Fatalf("wrBuf != rdBuf")
	}

	// cancel the core context
	coreCtxCancel()
	<-coreCtx.Done()
	time.Sleep(100 * time.Millisecond)

	// now the conn must be closed, read and/or write must fail
	_, err = conn.Write(wrBuf)
	if err == nil {
		t.Fatalf("conn.Write must fail after closing the conn")
	} else {
		t.Logf("conn.Write error: %v", err)
	}

	_, err = conn.Read(rdBuf)
	if err == nil {
		t.Fatalf("conn.Read must fail after closing the conn")
	} else {
		t.Logf("conn.Read error: %v", err)
	}

	if err := tcpLis.Close(); err != nil {
		t.Fatal(err)
	}
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
		TransportModuleBin:  wasmPlain,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}
	dialer, err := v1.NewDialerWithContext(context.Background(), config)
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
		TransportModuleBin:  wasmReverse,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
	}
	dialer, err := v1.NewDialerWithContext(context.Background(), config)
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

	waterDialer, err := v1.NewFixedDialerWithContext(context.Background(), config)
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

// TestFixedDialer covers the following cases:
//  1. FixedDialer must work with a plain WebAssembly Transport Module that
//     doesn't transform the message.
//  2. FixedDialer must work with a WebAssembly Transport Module that
//     transforms the message by reversing it.
//  3. FixedDialer must fail when an invalid address is supplied.
//  4. FixedDialer must fail when a WebAssembly Transport Module does not
//     fully implement the v1 dialer spec.
func TestFixedDialer(t *testing.T) {
	t.Run("plain must work", testFixedDialerPlain)
	t.Run("reverse must work", testFixedDialerReverse)
	t.Run("bad addr must fail", testFixedDialerBadAddr)
}

func testFixedDialerBadAddr(t *testing.T) {
	// Dial
	config := &water.Config{
		TransportModuleBin:  wasmPlain,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
		DialedAddressValidator: func(network, address string) error {
			if network != "tcp" || address != "localhost:7700" {
				return fmt.Errorf("invalid address: %s", address)
			}
			return nil
		},
	}

	dialer, err := v1.NewFixedDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dialer.DialFixedContext(context.Background())
	if err == nil {
		t.Fatal("dialer.DialFixed should fail")
	}
}

func testFixedDialerPlain(t *testing.T) { // skipcq: GO-R1005
	tcpLis, err := net.Listen("tcp", "localhost:7700")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin:  wasmPlain,
		ModuleConfigFactory: water.NewWazeroModuleConfigFactory(),
		DialedAddressValidator: func(network, address string) error {
			if network != "tcp" || address != "localhost:7700" {
				return fmt.Errorf("invalid address: %s", address)
			}
			return nil
		},
	}
	dialer, err := v1.NewFixedDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialFixedContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v1.Conn
	if _, ok := conn.(*v1.Conn); !ok {
		t.Fatalf("returned conn is not *v1.Conn")
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

func testFixedDialerReverse(t *testing.T) { // skipcq: GO-R1005
	tcpLis, err := net.Listen("tcp", "localhost:7700")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
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
	dialer, err := v1.NewFixedDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialFixedContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // skipcq: GO-S2307

	// type assertion: conn must be *v1.Conn
	if _, ok := conn.(*v1.Conn); !ok {
		t.Fatalf("returned conn is not *v1.Conn")
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

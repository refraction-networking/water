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
)

func TestRelay(t *testing.T) {
	loadPlain()
	loadReverse()
	t.Run("plain must work", testRelayPlain)
	t.Run("reverse must work", testRelayReverse)
}

func testRelayPlain(t *testing.T) { // skipcq: GO-R1005
	// test destination: a local TCP server
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	// goroutine to accept incoming connections
	var serverConn net.Conn
	var serverAcceptErr error
	var serverAcceptWg *sync.WaitGroup = new(sync.WaitGroup)
	serverAcceptWg.Add(1)
	go func() {
		serverConn, serverAcceptErr = tcpLis.Accept()
		serverAcceptWg.Done()
	}()

	// setup relay
	config := &water.Config{
		TMBin: plain,
	}
	relay, err := water.NewRelay(config)
	if err != nil {
		t.Fatal(err)
	}

	// in a goroutine, start relay
	var relayErr error
	var relayWg *sync.WaitGroup = new(sync.WaitGroup)
	relayWg.Add(1)
	go func() {
		relayErr = relay.ListenAndRelayTo("tcp", "localhost:0", "tcp", tcpLis.Addr().String())
		relayWg.Done()
	}()
	time.Sleep(100 * time.Millisecond) // 100ms to spin up relay

	// test source: a local TCP client
	clientConn, err := net.Dial("tcp", relay.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close() // skipcq: GO-S2307

	// wait for server to accept connection
	serverAcceptWg.Wait()
	if serverAcceptErr != nil {
		t.Fatal(serverAcceptErr)
	}
	defer serverConn.Close() // skipcq: GO-S2307

	// trigger garbage collection for several times to simulate any
	// possible GC in the real world use case
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	var clientSendBuf []byte = make([]byte, 1024)
	var serverSendBuf []byte = make([]byte, 1024)
	var clientRecvBuf []byte = make([]byte, 1024)
	var serverRecvBuf []byte = make([]byte, 1024)
	// send 10 messages in each direction
	for i := 0; i < 10; i++ {
		_, err = rand.Read(clientSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		_, err = rand.Read(serverSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		// client -> server
		_, err = clientConn.Write(clientSendBuf)
		if err != nil {
			t.Fatalf("conn.Write error: %s", err)
		}

		n, err := serverConn.Read(serverRecvBuf)
		if err != nil {
			t.Fatalf("serverConn.Read error: %s", err)
		}

		if n != len(clientSendBuf) {
			t.Fatalf("serverConn.Read error: read %d bytes, want %d bytes", n, len(clientSendBuf))
		}

		if !bytes.Equal(serverRecvBuf[:n], clientSendBuf) {
			t.Fatalf("serverRecvBuf != clientSendBuf")
		}

		// server -> client
		_, err = serverConn.Write(serverSendBuf)
		if err != nil {
			t.Fatalf("serverConn.Write error: %s", err)
		}

		n, err = clientConn.Read(clientRecvBuf)
		if err != nil {
			t.Fatalf("clientConn.Read error: %s", err)
		}

		if n != len(serverSendBuf) {
			t.Fatalf("clientConn.Read error: read %d bytes, want %d bytes", n, len(serverSendBuf))
		}

		if !bytes.Equal(clientRecvBuf[:n], serverSendBuf) {
			t.Fatalf("clientRecvBuf != serverSendBuf")
		}

		// trigger garbage collection
		runtime.GC()
		time.Sleep(100 * time.Microsecond)
	}

	// stop relay
	err = relay.Close()
	if err != nil {
		t.Fatal(err)
	}

	// wait for relay to stop
	relayWg.Wait()
	if relayErr != nil {
		t.Fatal(relayErr)
	}

	// at this time, connection must still be alive
	_, err = clientConn.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	n, err := serverConn.Read(serverRecvBuf)
	if err != nil {
		t.Fatal(err)
	}

	if string(serverRecvBuf[:n]) != "hello" {
		t.Fatalf("serverRecvBuf != \"hello\"")
	}
}

func testRelayReverse(t *testing.T) { // skipcq: GO-R1005
	// test destination: a local TCP server
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	// goroutine to accept incoming connections
	var serverConn net.Conn
	var serverAcceptErr error
	var serverAcceptWg *sync.WaitGroup = new(sync.WaitGroup)
	serverAcceptWg.Add(1)
	go func() {
		serverConn, serverAcceptErr = tcpLis.Accept()
		serverAcceptWg.Done()
	}()

	// setup relay
	config := &water.Config{
		TMBin: reverse,
	}
	relay, err := water.NewRelay(config)
	if err != nil {
		t.Fatal(err)
	}

	// in a goroutine, start relay
	var relayErr error
	var relayWg *sync.WaitGroup = new(sync.WaitGroup)
	relayWg.Add(1)
	go func() {
		relayErr = relay.ListenAndRelayTo("tcp", "localhost:0", "tcp", tcpLis.Addr().String())
		relayWg.Done()
	}()
	time.Sleep(100 * time.Millisecond) // 100ms to spin up relay

	// test source: a local TCP client
	clientConn, err := net.Dial("tcp", relay.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close() // skipcq: GO-S2307

	// wait for server to accept connection
	serverAcceptWg.Wait()
	if serverAcceptErr != nil {
		t.Fatal(serverAcceptErr)
	}
	defer serverConn.Close() // skipcq: GO-S2307

	// trigger garbage collection for several times to simulate any
	// possible GC in the real world use case
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	var clientSendBuf []byte = make([]byte, 1024)
	var serverSendBuf []byte = make([]byte, 1024)
	var clientRecvBuf []byte = make([]byte, 1024)
	var serverRecvBuf []byte = make([]byte, 1024)
	// send 10 messages in each direction
	for i := 0; i < 10; i++ {
		_, err = rand.Read(clientSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		_, err = rand.Read(serverSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		// client -> server
		_, err = clientConn.Write(clientSendBuf)
		if err != nil {
			t.Fatalf("conn.Write error: %s", err)
		}

		n, err := serverConn.Read(serverRecvBuf)
		if err != nil {
			t.Fatalf("serverConn.Read error: %s", err)
		}

		if n != len(clientSendBuf) {
			t.Fatalf("serverConn.Read error: read %d bytes, want %d bytes", n, len(clientSendBuf))
		}

		// reverse clientSendBuf
		for i := 0; i < len(clientSendBuf)/2; i++ {
			clientSendBuf[i], clientSendBuf[len(clientSendBuf)-1-i] = clientSendBuf[len(clientSendBuf)-1-i], clientSendBuf[i]
		}

		if !bytes.Equal(serverRecvBuf[:n], clientSendBuf) {
			t.Fatalf("serverRecvBuf != clientSendBuf")
		}

		// server -> client
		_, err = serverConn.Write(serverSendBuf)
		if err != nil {
			t.Fatalf("serverConn.Write error: %s", err)
		}

		n, err = clientConn.Read(clientRecvBuf)
		if err != nil {
			t.Fatalf("clientConn.Read error: %s", err)
		}

		if n != len(serverSendBuf) {
			t.Fatalf("clientConn.Read error: read %d bytes, want %d bytes", n, len(serverSendBuf))
		}

		// reverse serverSendBuf
		for i := 0; i < len(serverSendBuf)/2; i++ {
			serverSendBuf[i], serverSendBuf[len(serverSendBuf)-1-i] = serverSendBuf[len(serverSendBuf)-1-i], serverSendBuf[i]
		}

		if !bytes.Equal(clientRecvBuf[:n], serverSendBuf) {
			t.Fatalf("clientRecvBuf != serverSendBuf")
		}

		// trigger garbage collection
		runtime.GC()
		time.Sleep(100 * time.Microsecond)
	}

	// stop relay
	err = relay.Close()
	if err != nil {
		t.Fatal(err)
	}

	// wait for relay to stop
	relayWg.Wait()
	if relayErr != nil {
		t.Fatal(relayErr)
	}

	// at this time, connection must still be alive
	_, err = clientConn.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	n, err := serverConn.Read(serverRecvBuf)
	if err != nil {
		t.Fatal(err)
	}

	if string(serverRecvBuf[:n]) != "olleh" {
		t.Fatalf("serverRecvBuf != \"olleh\"")
	}
}

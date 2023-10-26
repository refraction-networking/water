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

func TestV0Dialer(t *testing.T) {
	loadPlain()
	t.Run("v0-Dialer-plain-must-work", testV0DialerPlain)
	t.Run("v0-Dialer-bad-WATM-must-fail", testV0DialerBadWATM)
	t.Run("v0-Dialer-bad-addr-must-fail", testV0DialerBadAddr)

}

func testV0DialerBadWATM(t *testing.T) {
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close()

	// goroutine to accept incoming connections
	go func(t *testing.T) {
		lisConn, goroutineErr := tcpLis.Accept()
		if goroutineErr == nil {
			lisConn.Close()
			// should not happen
			t.Errorf("tcpLis.Accept should fail")
		}
	}(t)

	// Dial
	config := &water.Config{
		TMBin: make([]byte, 1024),
	}
	_, err = rand.Read(config.TMBin)
	if err != nil {
		t.Fatal(err)
	}

	_, err = water.NewDialer(config)
	if err == nil {
		t.Fatal("water.NewDialer should fail")
	}

	// trigger garbage collection
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
}

func testV0DialerBadAddr(t *testing.T) {
	// Dial
	config := &water.Config{
		TMBin: plain,
	}

	dialer, err := water.NewDialer(config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dialer.Dial("tcp", "256.267.278.289:2023")
	if err == nil {
		t.Fatal("dialer.Dial should fail")
	}

	// trigger garbage collection
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
}

func testV0DialerPlain(t *testing.T) {
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close()

	// goroutine to accept incoming connections
	var lisConn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		lisConn, goroutineErr = tcpLis.Accept()
	}()

	// Dial
	config := &water.Config{
		TMBin: plain,
	}
	dialer, err := water.NewDialer(config)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	wg.Wait()
	if goroutineErr != nil {
		t.Fatal(goroutineErr)
	}
	defer lisConn.Close()

	// trigger garbage collection for several times to simulate any
	// possible GC in the real world use case
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	var dialerSendBuf []byte = make([]byte, 1024)
	var listenerSendBuf []byte = make([]byte, 1024)
	var dialerRecvBuf []byte = make([]byte, 1024)
	var listenerRecvBuf []byte = make([]byte, 1024)
	// send 10 messages in each direction
	for i := 0; i < 10; i++ {
		_, err = rand.Read(dialerSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		_, err = rand.Read(listenerSendBuf)
		if err != nil {
			t.Fatalf("rand.Read error: %s", err)
		}

		// dialer -> listener
		_, err = conn.Write(dialerSendBuf)
		if err != nil {
			t.Fatalf("conn.Write error: %s", err)
		}

		n, err := lisConn.Read(listenerRecvBuf)
		if err != nil {
			t.Fatalf("lisConn.Read error: %s", err)
		}

		if n != len(dialerSendBuf) {
			t.Fatalf("lisConn.Read error: read %d bytes, want %d bytes", n, len(dialerSendBuf))
		}

		if !bytes.Equal(listenerRecvBuf[:n], dialerSendBuf) {
			t.Fatalf("listenerRecvBuf != dialerSendBuf")
		}

		// listener -> dialer
		_, err = lisConn.Write(listenerSendBuf)
		if err != nil {
			t.Fatalf("lisConn.Write error: %s", err)
		}

		n, err = conn.Read(dialerRecvBuf)
		if err != nil {
			t.Fatalf("conn.Read error: %s", err)
		}

		if n != len(listenerSendBuf) {
			t.Fatalf("conn.Read error: read %d bytes, want %d bytes", n, len(listenerSendBuf))
		}

		if !bytes.Equal(dialerRecvBuf[:n], listenerSendBuf) {
			t.Fatalf("dialerRecvBuf != listenerSendBuf")
		}

		// trigger garbage collection
		runtime.GC()
		time.Sleep(100 * time.Microsecond)
	}

	// reading with a deadline
	conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn.Read(dialerRecvBuf)
	if err == nil {
		t.Fatalf("conn.Read should timeout")
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	// after closing the conn, read/write MUST fail
	_, err = conn.Write(dialerSendBuf)
	if err == nil {
		t.Fatalf("conn.Write should must after closing the conn")
	}

	_, err = conn.Read(dialerRecvBuf)
	if err == nil {
		t.Fatalf("conn.Read should must after closing the conn")
	}

	// trigger garbage collection
	runtime.GC()
	time.Sleep(100 * time.Microsecond)
}
